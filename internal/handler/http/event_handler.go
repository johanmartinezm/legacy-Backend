package http

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"applegacy/backend/internal/core/services"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type EventHandler struct {
	service ports.EventService
	// notifier avisa a los usuarios de la app cuando se publica un evento
	// nuevo. Admite nil: sin él, el evento se crea igual y solo se omite el
	// aviso, que es como funcionó hasta ahora.
	notifier ports.NotificationService
}

func NewEventHandler(service ports.EventService, notifier ports.NotificationService) *EventHandler {
	return &EventHandler{service: service, notifier: notifier}
}

func (h *EventHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.service.ListCategories(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categories)
}

// ListEvents y GetEventDetails son rutas **públicas**: responden sin sesión. Por
// eso el enlace de la sesión virtual se retira salvo que quien pregunte sea
// administrador —el panel lo necesita para no borrarlo al editar un evento—.
// Quien está inscrito recibe el suyo por GET /api/me/registrations, que aplica
// su propia regla.
func (h *EventHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.service.ListEvents(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !IsAdmin(r.Context()) {
		for i := range events {
			events[i].OcultarEnlaceDeAcceso()
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

func (h *EventHandler) GetEventDetails(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	event, err := h.service.GetEventDetails(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if !IsAdmin(r.Context()) {
		event.OcultarEnlaceDeAcceso()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(event)
}

func (h *EventHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var event domain.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.service.CreateEvent(r.Context(), &event); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Aviso a los usuarios de la app. Va después de crear el evento y no puede
	// hacer fallar la respuesta: si el envío falla, queda en el log y el evento
	// existe igual.
	//
	// El admin sale del token —esta ruta está bajo AdminOnly— para que el
	// historial de notificaciones diga quién publicó la novedad, igual que en un
	// envío manual desde el panel.
	adminID, _ := r.Context().Value(UserIDKey).(string)
	notificarNovedad(r.Context(), h.notifier, adminID,
		"Nuevo evento: "+event.Title,
		textoDe(event.Description),
		map[string]string{"type": "event", "id": event.ID})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(event)
}

func (h *EventHandler) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var event domain.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	event.ID = id

	if err := h.service.UpdateEvent(r.Context(), &event); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(event)
}

func (h *EventHandler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteEvent(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *EventHandler) Register(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")

	// Get UserID from Context (set by AuthMiddleware) - This is the requester
	requesterID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Default registration values
	registration := &domain.Registration{
		UserID:  requesterID,
		EventID: eventID,
	}

	// Try to decode body if present (optional)
	var req struct {
		UserID        string   `json:"userID"`
		PaymentStatus string   `json:"paymentStatus"`
		Workshops     []string `json:"workshops"`
		// Contacto para este evento. Lo manda cualquiera para SU propia
		// inscripcion: no dice de quien es la entrada —eso lo fija el token—,
		// solo a quien llamar si no aparece.
		ParticipantName  string `json:"participant_name"`
		ParticipantEmail string `json:"participant_email"`
		ParticipantPhone string `json:"participant_phone"`
	}
	if r.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			// userID y paymentStatus estan para que un administrador inscriba a
			// mano, por ejemplo a quien pago por transferencia. Hasta ahora los
			// honraba cualquiera con sesion, y esta ruta esta bajo
			// AuthMiddleware, no AdminOnly. Es decir:
			//
			//   {"paymentStatus":"paid"}  -> entrada gratis a un evento de pago,
			//                                con QR valido y sin una sola
			//                                transaccion registrada.
			//   {"userID":"<otro>"}       -> inscribir a un tercero, dejandole
			//                                una deuda a su nombre.
			//
			// Se rechaza en vez de ignorarlos en silencio: quien los manda cree
			// que surtieron efecto, y un 201 mudo le daria la razon. La app no
			// envia cuerpo en esta llamada, asi que esto solo salta ante un
			// abuso.
			if !IsAdmin(r.Context()) && (req.UserID != "" || req.PaymentStatus != "") {
				http.Error(w,
					"Solo un administrador puede inscribir a otro usuario o fijar el estado de pago",
					http.StatusForbidden)
				return
			}
			if req.UserID != "" {
				registration.UserID = req.UserID
			}
			if req.PaymentStatus != "" {
				registration.PaymentStatus = req.PaymentStatus
			}
			for _, wID := range req.Workshops {
				registration.Workshops = append(registration.Workshops, domain.Workshop{ID: wID})
			}

			registration.ParticipantName = req.ParticipantName
			registration.ParticipantEmail = req.ParticipantEmail
			registration.ParticipantPhone = req.ParticipantPhone
		}
	}

	err := h.service.RegisterUser(r.Context(), registration)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// El QR de una inscripción sin pagar no sale del servidor, igual que en
	// GetMyRegistrations. La reserva se crea antes de ir a la pasarela, así que
	// sin esto la respuesta de "reservar cupo" entregaba una credencial de un
	// evento todavía impago. Se vacía sobre una copia para no tocar la fila que
	// el servicio acaba de escribir.
	//
	// La app lee el QR de GET /api/me/registrations (campo `qrData`), no de
	// aquí (`qr_data`), así que no se queda sin nada que mostrar.
	respuesta := *registration
	if respuesta.IsPendingPayment() {
		respuesta.QRData = ""
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(respuesta)
}

// GetEventRegistrants sirve la lista de inscritos de un evento. Va registrada
// en el bloque AdminOnly de main.go: son nombres, correos y teléfonos de
// terceros, además de quién debe dinero.
func (h *EventHandler) GetEventRegistrants(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")

	registrants, err := h.service.GetEventRegistrants(r.Context(), eventID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(registrants)
}

func (h *EventHandler) SubmitWorkshopRating(w http.ResponseWriter, r *http.Request) {
	workshopID := chi.URLParam(r, "id")
	userID, _ := r.Context().Value(UserIDKey).(string)

	var rating domain.WorkshopRating
	if err := json.NewDecoder(r.Body).Decode(&rating); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rating.WorkshopID = workshopID
	rating.UserID = userID

	if err := h.service.SubmitWorkshopRating(r.Context(), &rating); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rating)
}

// submitEventSurveyRequest es explícito a propósito: decodificar directo sobre
// domain.EventSurvey dejaría que el cliente mandara su propio id, userId o
// createdAt. El usuario sale del token y el evento de la URL.
type submitEventSurveyRequest struct {
	OverallRating      int     `json:"overallRating"`
	OrganizationRating *int    `json:"organizationRating"`
	ContentRating      *int    `json:"contentRating"`
	SpeakersRating     *int    `json:"speakersRating"`
	WouldRecommend     *bool   `json:"wouldRecommend"`
	Comment            *string `json:"comment"`
}

func (h *EventHandler) SubmitEventSurvey(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req submitEventSurveyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	survey := &domain.EventSurvey{
		EventID:            eventID,
		UserID:             userID,
		OverallRating:      req.OverallRating,
		OrganizationRating: req.OrganizationRating,
		ContentRating:      req.ContentRating,
		SpeakersRating:     req.SpeakersRating,
		WouldRecommend:     req.WouldRecommend,
		Comment:            req.Comment,
	}

	if err := h.service.SubmitEventSurvey(r.Context(), survey); err != nil {
		switch {
		case errors.Is(err, services.ErrSurveyInvalidRating):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, services.ErrSurveyEventNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, services.ErrSurveyNotRegistered):
			http.Error(w, err.Error(), http.StatusForbidden)
		case errors.Is(err, services.ErrSurveyAlreadySent):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(survey)
}

// GetMyEventSurvey responde 204 si el usuario aún no ha contestado. La app lo
// usa para decidir si ofrece el formulario, así que "no hay" es una respuesta
// esperada y no un 404 de error.
func (h *EventHandler) GetMyEventSurvey(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	survey, err := h.service.GetMyEventSurvey(r.Context(), eventID, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if survey == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(survey)
}

func (h *EventHandler) GetEventSurveySummary(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	summary, err := h.service.GetEventSurveySummary(r.Context(), eventID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func (h *EventHandler) GetEventFeedback(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	feedback, err := h.service.GetEventFeedback(r.Context(), eventID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(feedback)
}

// GetMyRegistrations alimenta la pantalla "Mi credencial": todos los eventos en
// los que el usuario está inscrito, cada uno con su QR de acceso.
func (h *EventHandler) GetMyRegistrations(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	registrations, err := h.service.GetMyRegistrations(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(registrations)
}

func (h *EventHandler) GetAgenda(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agenda, err := h.service.GetAgenda(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agenda)
}

func (h *EventHandler) AddToAgenda(w http.ResponseWriter, r *http.Request) {
	workshopID := chi.URLParam(r, "id")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err := h.service.AddToAgenda(r.Context(), userID, workshopID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *EventHandler) RemoveFromAgenda(w http.ResponseWriter, r *http.Request) {
	workshopID := chi.URLParam(r, "id")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err := h.service.RemoveFromAgenda(r.Context(), userID, workshopID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *EventHandler) CheckIn(w http.ResponseWriter, r *http.Request) {
	staffID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		QRData string `json:"qrData"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.service.CheckIn(r.Context(), req.QRData, staffID)
	if err != nil {
		// Quien está en la puerta necesita distinguir los dos casos: un código
		// que no existe es un QR ajeno o inventado, y uno pendiente de pago es
		// un asistente real al que hay que cobrarle antes de dejarlo entrar.
		// Antes ambos salían como 404 con el mismo texto.
		switch {
		case errors.Is(err, services.ErrCheckInPendingPayment):
			http.Error(w, "La inscripción está pendiente de pago", http.StatusPaymentRequired)
		case errors.Is(err, services.ErrCheckInNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

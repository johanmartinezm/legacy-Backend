package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type EventService struct {
	repo ports.EventRepository
}

func NewEventService(repo ports.EventRepository) *EventService {
	return &EventService{repo: repo}
}

func (s *EventService) ListCategories(ctx context.Context) ([]domain.EventCategory, error) {
	return s.repo.ListCategories(ctx)
}

func (s *EventService) ListEvents(ctx context.Context) ([]domain.Event, error) {
	events, err := s.repo.GetEvents(ctx)
	if err != nil {
		return nil, err
	}
	for i := range events {
		s.populatePriceLabel(&events[i])
		workshops, err := s.repo.GetWorkshopsByEventID(ctx, events[i].ID)
		if err == nil {
			events[i].Workshops = workshops
		}
	}
	return events, nil
}

func (s *EventService) GetEventDetails(ctx context.Context, id string) (*domain.Event, error) {
	event, err := s.repo.GetEventByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.populatePriceLabel(event)

	workshops, err := s.repo.GetWorkshopsByEventID(ctx, id)
	if err != nil {
		return nil, err
	}
	event.Workshops = workshops

	return event, nil
}

func (s *EventService) CreateEvent(ctx context.Context, e *domain.Event) error {
	err := s.repo.CreateEvent(ctx, e)
	if err != nil {
		return err
	}

	for i := range e.Workshops {
		e.Workshops[i].EventID = e.ID
		err = s.repo.CreateWorkshop(ctx, &e.Workshops[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *EventService) UpdateEvent(ctx context.Context, e *domain.Event) error {
	err := s.repo.UpdateEvent(ctx, e)
	if err != nil {
		return err
	}

	// Simple workshop sync: delete and recreate
	err = s.repo.DeleteWorkshopsByEventID(ctx, e.ID)
	if err != nil {
		return err
	}

	for i := range e.Workshops {
		e.Workshops[i].EventID = e.ID
		err = s.repo.CreateWorkshop(ctx, &e.Workshops[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *EventService) DeleteEvent(ctx context.Context, id string) error {
	// Workshops will be deleted via cascade if set up, or explicitly here
	_ = s.repo.DeleteWorkshopsByEventID(ctx, id)
	return s.repo.DeleteEvent(ctx, id)
}

func (s *EventService) RegisterUser(ctx context.Context, reg *domain.Registration) error {
	// 1. Check if already registered
	existing, _ := s.repo.GetRegistrationByUserAndEvent(ctx, reg.UserID, reg.EventID)
	if existing != nil {
		reg.ID = existing.ID
		reg.PaymentStatus = existing.PaymentStatus
		reg.RegistrationStatus = existing.RegistrationStatus
		reg.RegistrationDate = existing.RegistrationDate
		reg.QRData = existing.QRData
		reg.TotalPaid = existing.TotalPaid
		reg.AttendanceConfirmed = existing.AttendanceConfirmed
		return nil
	}

	// 2. Fetch event to check price and free status
	event, err := s.repo.GetEventByID(ctx, reg.EventID)
	if err != nil {
		return err
	}

	// 3. Set payment status and total if not already set (e.g., from an admin override)
	if reg.PaymentStatus == "" {
		reg.PaymentStatus = "pending"
		if event.IsFree {
			reg.PaymentStatus = "free"
		}
	}

	// 4. Estado de la inscripción. Un evento gratuito queda confirmado en el
	// acto; uno de pago nace pendiente y solo pasa a confirmado cuando la
	// pasarela aprueba el cobro (ver paymentService.VerifyPayment).
	//
	// Se mira el estado de pago y no event.IsFree para que una inscripción que
	// un administrador crea ya pagada quede confirmada de una vez, sin obligarle
	// a pasar por la pasarela.
	if reg.RegistrationStatus == "" {
		if reg.PaymentStatus == "free" || reg.PaymentStatus == "paid" {
			reg.RegistrationStatus = domain.RegistrationConfirmed
		} else {
			reg.RegistrationStatus = domain.RegistrationPendingPayment
		}
	}

	if reg.TotalPaid == 0 && !event.IsFree {
		reg.TotalPaid = event.Price
	}

	if reg.QRData == "" {
		// Aleatorio, no derivado del usuario y el evento. Antes era
		// "REG-{user_id}-{event_id}": dos uuid que el propio interesado conoce,
		// asi que cualquiera podia fabricar el codigo de otro y CheckIn lo daba
		// por bueno. uuid.NewString() usa crypto/rand.
		reg.QRData = "REG-" + uuid.NewString()
	}

	// 4. Create Registration in repository
	err = s.repo.CreateRegistration(ctx, reg)
	if err != nil {
		return err
	}

	// 5. Add workshops if any
	if len(reg.Workshops) > 0 {
		var workshopIDs []string
		for _, w := range reg.Workshops {
			workshopIDs = append(workshopIDs, w.ID)
		}
		return s.repo.AddRegistrationWorkshops(ctx, reg.ID, workshopIDs)
	}

	return nil
}

// GetMyRegistrations alimenta la pantalla "Mi credencial".
//
// Se devuelven también las inscripciones pendientes de pago: al usuario le sirve
// ver que su cupo está reservado y que le falta pagar. Lo que no se le manda es
// su QR —una credencial que no da derecho a entrar no debería salir del
// servidor—, así que el cliente no tiene que acordarse de ocultarlo.
func (s *EventService) GetMyRegistrations(ctx context.Context, userID string) ([]domain.UserRegistration, error) {
	registrations, err := s.repo.GetRegistrationsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range registrations {
		if registrations[i].RegistrationStatus == domain.RegistrationPendingPayment {
			registrations[i].QRData = ""
		}
	}
	return registrations, nil
}

func (s *EventService) SubmitWorkshopRating(ctx context.Context, rating *domain.WorkshopRating) error {
	return s.repo.CreateWorkshopRating(ctx, rating)
}

func (s *EventService) GetEventFeedback(ctx context.Context, eventID string) ([]domain.WorkshopRating, error) {
	return s.repo.GetRatingsByEventID(ctx, eventID)
}

// Errores de la encuesta general. Son centinelas para que el handler distinga
// 400, 403 y 409 con errors.Is; el resto del repositorio compara errores por
// texto, que se rompe en cuanto alguien reescribe un mensaje.
var (
	ErrSurveyInvalidRating = errors.New("rating out of range")
	ErrSurveyNotRegistered = errors.New("user is not registered for this event")
	ErrSurveyAlreadySent   = errors.New("survey already submitted for this event")
	ErrSurveyEventNotFound = errors.New("event not found")
)

// SubmitEventSurvey guarda la encuesta general de un evento.
//
// Solo puede responder quien esté registrado en el evento. Se exige registro y
// no asistencia confirmada (registrations.attendance_confirmed) a propósito:
// dejaría fuera a quien sí fue pero el personal no alcanzó a escanear.
func (s *EventService) SubmitEventSurvey(ctx context.Context, survey *domain.EventSurvey) error {
	if err := validateSurveyRatings(survey); err != nil {
		return err
	}

	// Distinguir "no existe" de "la consulta falló": traducir todo a un mismo
	// error escondía averías del repositorio detrás de un 500 con el texto
	// "event not found", que manda a buscar en el sitio equivocado.
	if _, err := s.repo.GetEventByID(ctx, survey.EventID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrSurveyEventNotFound
		}
		return err
	}

	registration, err := s.repo.GetRegistrationByUserAndEvent(ctx, survey.UserID, survey.EventID)
	if err != nil {
		return err
	}
	if registration == nil {
		return ErrSurveyNotRegistered
	}

	existing, err := s.repo.GetEventSurveyByUser(ctx, survey.EventID, survey.UserID)
	if err != nil {
		return err
	}
	if existing != nil {
		return ErrSurveyAlreadySent
	}

	if survey.Comment != nil {
		trimmed := strings.TrimSpace(*survey.Comment)
		if trimmed == "" {
			survey.Comment = nil
		} else {
			survey.Comment = &trimmed
		}
	}

	// La comprobación de arriba cubre el caso normal, pero dos peticiones
	// simultáneas la pasan las dos y solo el UNIQUE de la tabla las separa. Sin
	// esta traducción, un doble toque en el botón devolvería un 500.
	if err := s.repo.CreateEventSurvey(ctx, survey); err != nil {
		if errors.Is(err, domain.ErrUniqueViolation) {
			return ErrSurveyAlreadySent
		}
		return err
	}
	return nil
}

// GetMyEventSurvey devuelve (nil, nil) si el usuario todavía no ha respondido.
// La app lo usa para decidir si ofrece el formulario o muestra lo ya enviado.
func (s *EventService) GetMyEventSurvey(ctx context.Context, eventID, userID string) (*domain.EventSurvey, error) {
	return s.repo.GetEventSurveyByUser(ctx, eventID, userID)
}

func (s *EventService) GetEventSurveySummary(ctx context.Context, eventID string) (*domain.EventSurveySummary, error) {
	return s.repo.GetEventSurveySummary(ctx, eventID)
}

// validateSurveyRatings no delega en los CHECK de la tabla: un rating fuera de
// rango es un error del cliente (400), y dejarlo llegar a la base lo convertiría
// en un 500 con el mensaje de Postgres dentro.
func validateSurveyRatings(survey *domain.EventSurvey) error {
	if survey.OverallRating < 1 || survey.OverallRating > 5 {
		return fmt.Errorf("%w: overallRating must be between 1 and 5", ErrSurveyInvalidRating)
	}
	optional := map[string]*int{
		"organizationRating": survey.OrganizationRating,
		"contentRating":      survey.ContentRating,
		"speakersRating":     survey.SpeakersRating,
	}
	for name, value := range optional {
		if value != nil && (*value < 1 || *value > 5) {
			return fmt.Errorf("%w: %s must be between 1 and 5", ErrSurveyInvalidRating, name)
		}
	}
	return nil
}

func (s *EventService) populatePriceLabel(e *domain.Event) {
	if e.IsFree {
		e.PriceLabel = "GRATIS"
	} else {
		e.PriceLabel = fmt.Sprintf("$%.0f.000.000", e.Price)
	}
}

func (s *EventService) GetAgenda(ctx context.Context, userID string) ([]domain.Workshop, error) {
	return s.repo.GetAgenda(ctx, userID)
}

func (s *EventService) AddToAgenda(ctx context.Context, userID, workshopID string) error {
	return s.repo.AddToAgenda(ctx, userID, workshopID)
}

func (s *EventService) RemoveFromAgenda(ctx context.Context, userID, workshopID string) error {
	return s.repo.RemoveFromAgenda(ctx, userID, workshopID)
}

func (s *EventService) CheckIn(ctx context.Context, qrData, staffID string) (*domain.CheckInResponse, error) {
	// 1. Find registration by QR
	reg, resp, err := s.repo.GetRegistrationByQR(ctx, qrData)
	if err != nil {
		return nil, fmt.Errorf("invalid QR data or registration not found")
	}

	// 2. Record attendance (updates confirmation and logs)
	err = s.repo.RecordAttendance(ctx, reg.ID, staffID)
	if err != nil {
		return nil, fmt.Errorf("failed to record attendance: %v", err)
	}

	// 3. Get workshops for this registration
	workshops, err := s.repo.GetWorkshopsByRegistrationID(ctx, reg.ID)
	if err == nil {
		resp.Workshops = workshops
	}

	return resp, nil
}

package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"applegacy/backend/internal/core/services"
	"github.com/google/uuid"
)

type PaymentHandler struct {
	paymentService ports.PaymentService
}

func NewPaymentHandler(paymentService ports.PaymentService) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
	}
}

type CreateIntentRequest struct {
	ReferenceType domain.ReferenceType `json:"reference_type"`
	ReferenceID   uuid.UUID            `json:"reference_id"`
	Amount        float64              `json:"amount"`
	ReturnURL     string               `json:"return_url"`
	// PaymentMethod es informativo: queda registrado lo que el usuario eligio,
	// pero quien decide los medios de pago es la pasarela.
	PaymentMethod string `json:"payment_method"`
}

type CreateIntentResponse struct {
	FormURL string `json:"form_url"`
}

func (h *PaymentHandler) CreatePaymentIntent(w http.ResponseWriter, r *http.Request) {
	// El usuario sale del token, no de la cabecera X-User-ID que llegaba antes:
	// esta ruta esta bajo AuthMiddleware, asi que el `sub` ya esta en el
	// contexto. Con la cabecera, cualquiera con una sesion valida podia iniciar
	// una transaccion a nombre de otra persona con solo cambiar ese valor, y la
	// transaccion quedaba registrada contra la victima.
	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userIDStr == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusUnauthorized)
		return
	}

	var req CreateIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	formUrl, err := h.paymentService.InitiatePayment(r.Context(), userID, req.ReferenceType, req.ReferenceID, req.Amount, req.ReturnURL, req.PaymentMethod)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPaymentEventNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, services.ErrPaymentEventIsFree):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, services.ErrPaymentAmountMismatch):
			// 409: el precio cambió o el cliente traía otro. Que recargue el
			// evento y lo intente con el importe correcto.
			http.Error(w, err.Error(), http.StatusConflict)
		case errors.Is(err, services.ErrPaymentGatewayUnavailable):
			// 502: la avería es de la pasarela, no nuestra. El detalle queda en
			// el log del servidor; al cliente solo se le dice de quién es el
			// problema, para no filtrar códigos del banco.
			http.Error(w, "La pasarela de pagos no está disponible", http.StatusBadGateway)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CreateIntentResponse{FormURL: formUrl})
}

// nombresDeReferencia son los parámetros donde puede venir el identificador de
// la operación. `orderNumber` es el que nosotros enviamos al registrar el pago
// (es el id de nuestra transacción) y `mdOrder` el que asigna CredibanCo; los
// demás son alias vistos en distintas integraciones de esta misma plataforma.
// Se aceptan todos porque no está confirmado cuál usará: probar por orden es
// más barato que un despliegue para añadir un nombre.
var nombresDeReferencia = []string{"orderNumber", "tx_id", "mdOrder", "orderId", "order_id"}

// CredibancoCallback atiende la notificación de la pasarela.
//
// **Va en el bloque de rutas públicas, sin AuthMiddleware**, porque quien llama
// es CredibanCo y no tiene un token nuestro. Eso no la convierte en un agujero:
// el servicio no se cree nada de lo que llega: usa la referencia solo para saber
// qué transacción mirar y pregunta el estado a la pasarela con nuestras
// credenciales. Una notificación falsa no puede declarar un pago aprobado.
//
// Acepta GET y POST, y busca la referencia tanto en la query como en el cuerpo:
// esta familia de pasarelas notifica por GET, pero conviene no depender de ello.
func (h *PaymentHandler) CredibancoCallback(w http.ResponseWriter, r *http.Request) {
	// ParseForm llena r.Form con la query y, en un POST con
	// application/x-www-form-urlencoded, también con el cuerpo.
	_ = r.ParseForm()

	referencia := ""
	for _, nombre := range nombresDeReferencia {
		if v := strings.TrimSpace(r.Form.Get(nombre)); v != "" {
			referencia = v
			break
		}
	}

	// Se registra siempre, incluso lo que no se entiende: cuando CredibanCo
	// active las notificaciones, este log es lo único que dirá con qué nombres
	// y con qué formato las manda de verdad.
	log.Printf("[PAGO][webhook] notificacion recibida: metodo=%s parametros=%v referencia=%q",
		r.Method, r.Form, referencia)

	tx, err := h.paymentService.ProcessGatewayNotification(r.Context(), referencia)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPaymentNotificationEmpty):
			// 400: sin identificador no hay nada que mirar, y conviene que la
			// pasarela lo vea como petición mal formada.
			http.Error(w, "missing transaction reference", http.StatusBadRequest)
		case errors.Is(err, services.ErrPaymentNotificationUnknown):
			// 200 a propósito. Puede ser una prueba de la pasarela, un reenvío
			// viejo o alguien tanteando la URL; un error haría que CredibanCo
			// reintentara en bucle algo que nunca va a existir. Queda en el log.
			log.Printf("[PAGO][webhook] referencia desconocida: %v", err)
			escribirOK(w)
		default:
			// 500 solo aquí: es un fallo nuestro y sí interesa que reintenten.
			log.Printf("[PAGO][webhook] error procesando la notificacion: %v", err)
			http.Error(w, "error processing notification", http.StatusInternalServerError)
		}
		return
	}

	log.Printf("[PAGO][webhook] transaccion %s quedo en estado %s", tx.ID, tx.Status)
	escribirOK(w)
}

// escribirOK responde el acuse mínimo. El cuerpo se mantiene corto y sin datos
// de la transacción: quien llama no está autenticado, así que la respuesta no
// debe contar nada que no supiera ya.
func escribirOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (h *PaymentHandler) VerifyPayment(w http.ResponseWriter, r *http.Request) {
	txIDStr := r.URL.Query().Get("tx_id")
	txID, err := uuid.Parse(txIDStr)
	if err != nil {
		http.Error(w, "invalid transaction id", http.StatusBadRequest)
		return
	}

	tx, err := h.paymentService.VerifyPayment(r.Context(), txID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tx)
}

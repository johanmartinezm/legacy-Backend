package http

import (
	"encoding/json"
	"errors"
	"net/http"

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

	formUrl, err := h.paymentService.InitiatePayment(r.Context(), userID, req.ReferenceType, req.ReferenceID, req.Amount, req.ReturnURL)
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

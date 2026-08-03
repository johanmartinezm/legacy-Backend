package http

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
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
	// In a real app, UserID is extracted from JWT
	userIDStr := r.Header.Get("X-User-ID")
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

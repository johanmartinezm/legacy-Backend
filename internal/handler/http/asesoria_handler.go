package http

import (
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"net/http"
)

type AsesoriaHandler struct {
	asesoriaService ports.AsesoriaService
	authService     ports.AuthService
}

func NewAsesoriaHandler(asesoriaService ports.AsesoriaService, authService ports.AuthService) *AsesoriaHandler {
	return &AsesoriaHandler{
		asesoriaService: asesoriaService,
		authService:     authService,
	}
}

func (h *AsesoriaHandler) Request(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Category string `json:"category"` // crecer, formar, ordenar
		Message  string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Category == "" {
		http.Error(w, "category is required", http.StatusBadRequest)
		return
	}

	// Fetch sender profile
	sender, err := h.authService.GetProfile(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to fetch sender profile", http.StatusInternalServerError)
		return
	}

	senderName := sender.FirstName + " " + sender.LastName
	senderEmail := sender.Email

	// Send notification
	err = h.asesoriaService.RequestAsesoria(r.Context(), req.Category, senderName, senderEmail, req.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Solicitud de asesoría enviada con éxito"})
}

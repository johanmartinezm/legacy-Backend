package http

import (
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"net/http"
)

type BoardHandler struct {
	boardService ports.BoardService
	authService  ports.AuthService
}

func NewBoardHandler(boardService ports.BoardService, authService ports.AuthService) *BoardHandler {
	return &BoardHandler{
		boardService: boardService,
		authService:  authService,
	}
}

func (h *BoardHandler) Contact(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ContactID string `json:"contact_id"`
		Message   string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ContactID == "" || req.Message == "" {
		http.Error(w, "contact_id and message are required", http.StatusBadRequest)
		return
	}

	// Fetch sender profile to get their name
	sender, err := h.authService.GetProfile(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to fetch sender profile", http.StatusInternalServerError)
		return
	}

	senderName := sender.FirstName + " " + sender.LastName
	senderEmail := sender.Email

	// Send notification
	err = h.boardService.NotifyContact(r.Context(), req.ContactID, senderName, senderEmail, req.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Solicitud enviada con éxito"})
}

package http

import (
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type LikeHandler struct {
	service ports.LikeService
}

func NewLikeHandler(service ports.LikeService) *LikeHandler {
	return &LikeHandler{service: service}
}

func (h *LikeHandler) ToggleLike(w http.ResponseWriter, r *http.Request) {
	postID := chi.URLParam(r, "id")
	if postID == "" {
		http.Error(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	status, err := h.service.ToggleLike(r.Context(), userID, postID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (h *LikeHandler) GetLikeStatus(w http.ResponseWriter, r *http.Request) {
	postID := chi.URLParam(r, "id")
	if postID == "" {
		http.Error(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	// UserID is optional here
	userID, _ := r.Context().Value(UserIDKey).(string)

	status, err := h.service.GetLikeStatus(r.Context(), userID, postID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (h *LikeHandler) RecordView(w http.ResponseWriter, r *http.Request) {
	postID := chi.URLParam(r, "id")
	if postID == "" {
		http.Error(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// If decoding fails, we still record the view but without the title
		body.Title = ""
	}

	// UserID is optional for views
	userID, _ := r.Context().Value(UserIDKey).(string)

	err := h.service.RecordView(r.Context(), userID, postID, body.Title)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

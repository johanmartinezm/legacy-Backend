package http

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type SynergyHandler struct {
	service ports.SynergyService
}

func NewSynergyHandler(service ports.SynergyService) *SynergyHandler {
	return &SynergyHandler{service: service}
}

func (h *SynergyHandler) ListSynergies(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	synergies, err := h.service.ListSynergies(r.Context(), category, status, search, page, pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(synergies)
}

func (h *SynergyHandler) GetSynergy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID, _ := r.Context().Value(UserIDKey).(string)

	synergy, err := h.service.GetSynergy(r.Context(), id, userID)
	if err != nil {
		http.Error(w, "synergy not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(synergy)
}

func (h *SynergyHandler) CreateSynergy(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var synergy domain.Synergy
	if err := json.NewDecoder(r.Body).Decode(&synergy); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	synergy.AuthorID = userID

	if err := h.service.ProposeSynergy(r.Context(), &synergy); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(synergy)
}

func (h *SynergyHandler) CommentSynergy(w http.ResponseWriter, r *http.Request) {
	synergyID := chi.URLParam(r, "id")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var comment domain.SynergyComment
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	comment.SynergyID = synergyID
	comment.UserID = userID

	if err := h.service.CommentSynergy(r.Context(), &comment); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(comment)
}

func (h *SynergyHandler) ToggleLike(w http.ResponseWriter, r *http.Request) {
	synergyID := chi.URLParam(r, "id")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	isLiked, err := h.service.ToggleLike(r.Context(), synergyID, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"liked": isLiked,
	})
}

package http

import (
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type GroupHandler struct {
	service ports.GroupService
}

func NewGroupHandler(service ports.GroupService) *GroupHandler {
	return &GroupHandler{service: service}
}

// CreateGroup handles creating a new custom group
func (h *GroupHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "cuerpo de petición inválido", http.StatusBadRequest)
		return
	}

	if payload.Name == "" {
		http.Error(w, "el nombre del grupo es requerido", http.StatusBadRequest)
		return
	}

	group, err := h.service.CreateGroup(r.Context(), payload.Name, payload.Description)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(group)
}

// ListGroups returns all custom groups
func (h *GroupHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.service.ListGroups(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
}

// DeleteGroup deletes a custom group
func (h *GroupHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "el ID del grupo es requerido", http.StatusBadRequest)
		return
	}

	err := h.service.DeleteGroup(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "grupo eliminado con éxito"})
}

// GetMembers returns all user IDs belonging to a group
func (h *GroupHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "el ID del grupo es requerido", http.StatusBadRequest)
		return
	}

	members, err := h.service.GetMembers(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

// ReplaceMembers sets the list of users belonging to a group
func (h *GroupHandler) ReplaceMembers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "el ID del grupo es requerido", http.StatusBadRequest)
		return
	}

	var payload struct {
		UserIDs []string `json:"user_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "cuerpo de petición inválido", http.StatusBadRequest)
		return
	}

	err := h.service.ReplaceMembers(r.Context(), id, payload.UserIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "miembros del grupo actualizados con éxito"})
}

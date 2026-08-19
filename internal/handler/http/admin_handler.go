package http

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type AdminHandler struct {
	auth   ports.AuthService
	logger *log.Logger
}

func NewAdminHandler(auth ports.AuthService) *AdminHandler {
	return &AdminHandler{auth: auth}
}

// RegisterAdmin creates a new admin account.
func (h *AdminHandler) RegisterAdmin(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Role      string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	admin := &domain.AdminUser{Email: payload.Email, FirstName: payload.FirstName, LastName: payload.LastName, Role: payload.Role}
	if err := h.auth.RegisterAdmin(context.Background(), admin, payload.Password); err != nil {
		// Una contraseña corta la manda quien pide; el resto sí son fallos del
		// servidor.
		if errors.Is(err, domain.ErrContrasenaCorta) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "admin created"})
}

// AdminLogin authenticates an admin and returns a JWT.
func (h *AdminHandler) AdminLogin(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	token, err := h.auth.AdminLogin(context.Background(), payload.Email, payload.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// ListAdmins returns all admin users (admin only).
func (h *AdminHandler) ListAdmins(w http.ResponseWriter, r *http.Request) {
	// token already validated by middleware, we trust role admin
	admins, err := h.auth.ListAdmins(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// strip password hash before sending
	for _, a := range admins {
		a.PasswordHash = ""
	}
	json.NewEncoder(w).Encode(admins)
}

// UpdateAdmin updates an admin user.
func (h *AdminHandler) UpdateAdmin(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var admin domain.AdminUser
	if err := json.NewDecoder(r.Body).Decode(&admin); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	admin.ID = id
	if err := h.auth.UpdateAdmin(context.Background(), &admin); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(admin)
}

// DeleteAdmin removes an admin user.
func (h *AdminHandler) DeleteAdmin(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.auth.DeleteAdmin(context.Background(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

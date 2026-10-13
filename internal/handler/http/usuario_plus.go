package http

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// MensajeSoloPlus es el cuerpo del 403: la app lo muestra tal cual.
const MensajeSoloPlus = "Esta sección es para usuarios Legacy+."

// RequierePlus deja pasar solo a usuarios Legacy+. Va detrás de
// AuthMiddleware, que pone el id del usuario en el contexto: sin él, 401.
//
// Lee es_plus de la base en cada petición, no del token, para que quitar el
// plus desde el panel corte el acceso al momento y no cuando caduque la sesión.
func RequierePlus(service ports.UsuarioPlusService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value(UserIDKey).(string)
			if !ok || userID == "" {
				http.Error(w, "Authorization required", http.StatusUnauthorized)
				return
			}
			esPlus, err := service.EsPlus(r.Context(), userID)
			if err != nil && !errors.Is(err, domain.ErrNotFound) {
				log.Printf("legacy+: error comprobando el plus de %s: %v", userID, err)
				http.Error(w, "no se pudo comprobar el acceso", http.StatusInternalServerError)
				return
			}
			if !esPlus {
				http.Error(w, MensajeSoloPlus, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type UsuarioPlusHandler struct {
	service ports.UsuarioPlusService
}

func NewUsuarioPlusHandler(service ports.UsuarioPlusService) *UsuarioPlusHandler {
	return &UsuarioPlusHandler{service: service}
}

// AdminCambiar atiende PUT /api/admin/users/{id}/plus con {"es_plus": bool}.
// Es la única forma de cambiar el plus: el UPDATE general del usuario no
// escribe la columna.
func (h *UsuarioPlusHandler) AdminCambiar(w http.ResponseWriter, r *http.Request) {
	var cuerpo struct {
		EsPlus *bool `json:"es_plus"`
	}
	if err := json.NewDecoder(r.Body).Decode(&cuerpo); err != nil || cuerpo.EsPlus == nil {
		http.Error(w, `se espera {"es_plus": true|false}`, http.StatusBadRequest)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.service.CambiarPlus(r.Context(), id, *cuerpo.EsPlus); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "el usuario no existe", http.StatusNotFound)
			return
		}
		log.Printf("legacy+: error cambiando el plus de %s: %v", id, err)
		http.Error(w, "no se pudo guardar", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": id, "es_plus": *cuerpo.EsPlus})
}

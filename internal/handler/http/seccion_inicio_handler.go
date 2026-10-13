package http

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"applegacy/backend/internal/core/services"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type SeccionInicioHandler struct {
	service ports.SeccionInicioService
}

func NewSeccionInicioHandler(service ports.SeccionInicioService) *SeccionInicioHandler {
	return &SeccionInicioHandler{service: service}
}

// List atiende GET /api/inicio/secciones (la app) y GET /api/admin/inicio/secciones
// (el panel). Es la misma lista: no hay nada que ocultar a la app, y la que no
// tiene imagen le dice que pinte el icono.
func (h *SeccionInicioHandler) List(w http.ResponseWriter, r *http.Request) {
	secciones, err := h.service.ListSecciones(r.Context())
	if err != nil {
		log.Printf("secciones de inicio: error listando: %v", err)
		http.Error(w, "no se pudieron leer las secciones", http.StatusInternalServerError)
		return
	}

	// Un array vacío y no null: la app y el panel recorren la respuesta.
	if secciones == nil {
		secciones = []*domain.SeccionInicio{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(secciones)
}

// AdminUpdate atiende PUT /api/admin/inicio/secciones/{slug} con
// {"imagen_url": "...", "titulo": "...", "texto": "..."}. El campo que no venga
// se deja como está; el slug sale de la URL para que una errata del formulario
// no pise otra sección.
func (h *SeccionInicioHandler) AdminUpdate(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var cambios domain.CambiosSeccionInicio
	if err := json.NewDecoder(r.Body).Decode(&cambios); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	seccion, err := h.service.Actualizar(r.Context(), slug, cambios)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			http.Error(w, "la sección no existe", http.StatusNotFound)
		case errors.Is(err, services.ErrImagenNoEsURL),
			errors.Is(err, services.ErrTituloSeccionLargo),
			errors.Is(err, services.ErrTextoSeccionLargo):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			log.Printf("secciones de inicio: error actualizando %q: %v", slug, err)
			http.Error(w, "no se pudo guardar la sección", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(seccion)
}

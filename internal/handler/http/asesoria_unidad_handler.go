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

type AsesoriaUnidadHandler struct {
	service ports.AsesoriaUnidadService
}

func NewAsesoriaUnidadHandler(service ports.AsesoriaUnidadService) *AsesoriaUnidadHandler {
	return &AsesoriaUnidadHandler{service: service}
}

func responderUnidades(w http.ResponseWriter, unidades []*domain.AsesoriaUnidad) {
	// Un array vacío y no null: la app y el panel recorren la respuesta.
	if unidades == nil {
		unidades = []*domain.AsesoriaUnidad{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(unidades)
}

// ListActivas atiende GET /api/asesoria/unidades: lo que pinta la app.
func (h *AsesoriaUnidadHandler) ListActivas(w http.ResponseWriter, r *http.Request) {
	unidades, err := h.service.ListActivas(r.Context())
	if err != nil {
		log.Printf("unidades de asesoría: error listando activas: %v", err)
		http.Error(w, "no se pudieron leer las unidades", http.StatusInternalServerError)
		return
	}
	responderUnidades(w, unidades)
}

// AdminList atiende GET /api/admin/asesoria/unidades: también las ocultas.
func (h *AsesoriaUnidadHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	unidades, err := h.service.ListTodas(r.Context())
	if err != nil {
		log.Printf("unidades de asesoría: error listando: %v", err)
		http.Error(w, "no se pudieron leer las unidades", http.StatusInternalServerError)
		return
	}
	responderUnidades(w, unidades)
}

func (h *AsesoriaUnidadHandler) AdminCreate(w http.ResponseWriter, r *http.Request) {
	var u domain.AsesoriaUnidad
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.service.Crear(r.Context(), &u); err != nil {
		h.responderError(w, "creando", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(u)
}

func (h *AsesoriaUnidadHandler) AdminUpdate(w http.ResponseWriter, r *http.Request) {
	var u domain.AsesoriaUnidad
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	// El id sale de la URL, no del cuerpo: una errata no pisa otra unidad.
	u.ID = chi.URLParam(r, "id")
	if err := h.service.Actualizar(r.Context(), &u); err != nil {
		h.responderError(w, "actualizando", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(u)
}

func (h *AsesoriaUnidadHandler) AdminDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Borrar(r.Context(), chi.URLParam(r, "id")); err != nil {
		h.responderError(w, "borrando", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// responderError traduce el error sin filtrar detalles de la base: los de
// validación van tal cual (están escritos para el panel), el resto genérico.
func (h *AsesoriaUnidadHandler) responderError(w http.ResponseWriter, accion string, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, "la unidad no existe", http.StatusNotFound)
	case services.EsErrorDeValidacionUnidad(err):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		log.Printf("unidades de asesoría: error %s: %v", accion, err)
		http.Error(w, "no se pudo guardar la unidad", http.StatusInternalServerError)
	}
}

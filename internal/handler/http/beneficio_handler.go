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

type BeneficioHandler struct {
	service ports.BeneficioService
}

func NewBeneficioHandler(service ports.BeneficioService) *BeneficioHandler {
	return &BeneficioHandler{service: service}
}

func responderBeneficios(w http.ResponseWriter, beneficios []*domain.Beneficio) {
	// Un array vacío y no null: la app y el panel recorren la respuesta.
	if beneficios == nil {
		beneficios = []*domain.Beneficio{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(beneficios)
}

// ListActivos atiende GET /api/beneficios, detrás de RequierePlus: los dos
// sliders de la sección Beneficios de la app (beneficios y alianzas), por tipo
// y orden.
func (h *BeneficioHandler) ListActivos(w http.ResponseWriter, r *http.Request) {
	beneficios, err := h.service.ListActivos(r.Context())
	if err != nil {
		log.Printf("beneficios: error listando activos: %v", err)
		http.Error(w, "no se pudieron leer los beneficios", http.StatusInternalServerError)
		return
	}
	responderBeneficios(w, beneficios)
}

// AdminList atiende GET /api/admin/beneficios: también los ocultos.
func (h *BeneficioHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	beneficios, err := h.service.ListTodos(r.Context())
	if err != nil {
		log.Printf("beneficios: error listando: %v", err)
		http.Error(w, "no se pudieron leer los beneficios", http.StatusInternalServerError)
		return
	}
	responderBeneficios(w, beneficios)
}

func (h *BeneficioHandler) AdminCreate(w http.ResponseWriter, r *http.Request) {
	var b domain.Beneficio
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.service.Crear(r.Context(), &b); err != nil {
		h.responderError(w, "creando", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(b)
}

func (h *BeneficioHandler) AdminUpdate(w http.ResponseWriter, r *http.Request) {
	var b domain.Beneficio
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	// El id sale de la URL, no del cuerpo.
	b.ID = chi.URLParam(r, "id")
	if err := h.service.Actualizar(r.Context(), &b); err != nil {
		h.responderError(w, "actualizando", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(b)
}

func (h *BeneficioHandler) AdminDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Borrar(r.Context(), chi.URLParam(r, "id")); err != nil {
		h.responderError(w, "borrando", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *BeneficioHandler) responderError(w http.ResponseWriter, accion string, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, "el beneficio no existe", http.StatusNotFound)
	case services.EsErrorDeValidacionBeneficio(err):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		log.Printf("beneficios: error %s: %v", accion, err)
		http.Error(w, "no se pudo guardar el beneficio", http.StatusInternalServerError)
	}
}

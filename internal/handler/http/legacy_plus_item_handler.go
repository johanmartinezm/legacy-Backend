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

type LegacyPlusItemHandler struct {
	service ports.LegacyPlusItemService
}

func NewLegacyPlusItemHandler(service ports.LegacyPlusItemService) *LegacyPlusItemHandler {
	return &LegacyPlusItemHandler{service: service}
}

func responderLegacyPlusItems(w http.ResponseWriter, items []*domain.LegacyPlusItem) {
	// Un array vacío y no null: la app y el panel recorren la respuesta.
	if items == nil {
		items = []*domain.LegacyPlusItem{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// ListActivos atiende GET /api/legacy-plus/items: lo que pinta la app.
func (h *LegacyPlusItemHandler) ListActivos(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListActivos(r.Context())
	if err != nil {
		log.Printf("legacy+: error listando puntos activos: %v", err)
		http.Error(w, "no se pudieron leer los puntos", http.StatusInternalServerError)
		return
	}
	responderLegacyPlusItems(w, items)
}

// AdminList atiende GET /api/admin/legacy-plus/items: también los ocultos.
func (h *LegacyPlusItemHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListTodos(r.Context())
	if err != nil {
		log.Printf("legacy+: error listando puntos: %v", err)
		http.Error(w, "no se pudieron leer los puntos", http.StatusInternalServerError)
		return
	}
	responderLegacyPlusItems(w, items)
}

func (h *LegacyPlusItemHandler) AdminCreate(w http.ResponseWriter, r *http.Request) {
	var i domain.LegacyPlusItem
	if err := json.NewDecoder(r.Body).Decode(&i); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.service.Crear(r.Context(), &i); err != nil {
		h.responderError(w, "creando", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(i)
}

func (h *LegacyPlusItemHandler) AdminUpdate(w http.ResponseWriter, r *http.Request) {
	var i domain.LegacyPlusItem
	if err := json.NewDecoder(r.Body).Decode(&i); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	// El id sale de la URL, no del cuerpo.
	i.ID = chi.URLParam(r, "id")
	if err := h.service.Actualizar(r.Context(), &i); err != nil {
		h.responderError(w, "actualizando", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(i)
}

func (h *LegacyPlusItemHandler) AdminDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Borrar(r.Context(), chi.URLParam(r, "id")); err != nil {
		h.responderError(w, "borrando", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *LegacyPlusItemHandler) responderError(w http.ResponseWriter, accion string, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, "el punto no existe", http.StatusNotFound)
	case services.EsErrorDeValidacionLegacyPlus(err):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		log.Printf("legacy+: error %s un punto: %v", accion, err)
		http.Error(w, "no se pudo guardar el punto", http.StatusInternalServerError)
	}
}

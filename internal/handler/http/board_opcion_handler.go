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

// BoardOpcionHandler sirve las listas de los formularios de Legacy Board: la
// app las lee para pintar los formularios A y B, y el panel las administra.
type BoardOpcionHandler struct {
	service ports.BoardOpcionService
}

func NewBoardOpcionHandler(service ports.BoardOpcionService) *BoardOpcionHandler {
	return &BoardOpcionHandler{service: service}
}

// opcionBoardEntrada es el cuerpo de POST y PUT. Activo es puntero para
// distinguir «no vino» de «false»: al crear, no venir es activa; al editar es
// obligatorio, para que un cuerpo incompleto no desactive una opción sin querer.
type opcionBoardEntrada struct {
	Catalogo string `json:"catalogo"`
	Etiqueta string `json:"etiqueta"`
	Orden    int    `json:"orden"`
	Activo   *bool  `json:"activo"`
}

func responderJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// Activas atiende GET /api/board/opciones (con sesión): las opciones activas
// de los 18 catálogos, agrupadas por catálogo y ya ordenadas. Una sola llamada
// basta para pintar cualquiera de los dos formularios.
func (h *BoardOpcionHandler) Activas(w http.ResponseWriter, r *http.Request) {
	agrupadas, err := h.service.Activas(r.Context())
	if err != nil {
		log.Printf("board opciones: error leyendo activas: %v", err)
		http.Error(w, "no se pudieron leer las opciones", http.StatusInternalServerError)
		return
	}
	responderJSON(w, http.StatusOK, agrupadas)
}

// AdminCatalogos atiende GET /api/admin/board/catalogos: los 18 catálogos con
// cuántas opciones tienen y cuáles alimentan el matching.
func (h *BoardOpcionHandler) AdminCatalogos(w http.ResponseWriter, r *http.Request) {
	catalogos, err := h.service.Catalogos(r.Context())
	if err != nil {
		log.Printf("board opciones: error leyendo catálogos: %v", err)
		http.Error(w, "no se pudieron leer los catálogos", http.StatusInternalServerError)
		return
	}
	responderJSON(w, http.StatusOK, catalogos)
}

// AdminList atiende GET /api/admin/board/opciones?catalogo=<clave>: todas las
// opciones del catálogo, también las inactivas.
func (h *BoardOpcionHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	opciones, err := h.service.DeCatalogo(r.Context(), r.URL.Query().Get("catalogo"))
	if err != nil {
		h.responderError(w, "listando", err)
		return
	}
	if opciones == nil {
		opciones = []*domain.OpcionBoard{}
	}
	responderJSON(w, http.StatusOK, opciones)
}

func (h *BoardOpcionHandler) AdminCreate(w http.ResponseWriter, r *http.Request) {
	var in opcionBoardEntrada
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	o := &domain.OpcionBoard{Catalogo: in.Catalogo, Etiqueta: in.Etiqueta, Orden: in.Orden, Activo: true}
	if in.Activo != nil {
		o.Activo = *in.Activo
	}
	if err := h.service.Crear(r.Context(), o); err != nil {
		h.responderError(w, "creando", err)
		return
	}
	responderJSON(w, http.StatusCreated, o)
}

// AdminUpdate atiende PUT /api/admin/board/opciones/{id}. Cambia nombre, orden
// y si está activa; el catálogo y el código no cambian aunque vengan.
func (h *BoardOpcionHandler) AdminUpdate(w http.ResponseWriter, r *http.Request) {
	var in opcionBoardEntrada
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if in.Activo == nil {
		http.Error(w, "falta indicar si la opción está activa", http.StatusBadRequest)
		return
	}
	o := &domain.OpcionBoard{ID: chi.URLParam(r, "id"), Etiqueta: in.Etiqueta, Orden: in.Orden, Activo: *in.Activo}
	if err := h.service.Actualizar(r.Context(), o); err != nil {
		h.responderError(w, "actualizando", err)
		return
	}
	responderJSON(w, http.StatusOK, o)
}

func (h *BoardOpcionHandler) responderError(w http.ResponseWriter, accion string, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, "la opción no existe", http.StatusNotFound)
	case errors.Is(err, services.ErrOpcionBoardDuplicada):
		http.Error(w, err.Error(), http.StatusConflict)
	case services.EsErrorDeValidacionOpcionBoard(err):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		log.Printf("board opciones: error %s: %v", accion, err)
		http.Error(w, "no se pudo guardar la opción", http.StatusInternalServerError)
	}
}

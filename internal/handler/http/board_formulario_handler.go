package http

import (
	"applegacy/backend/internal/core/ports"
	"applegacy/backend/internal/core/services"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// BoardFormularioHandler sirve los formularios de Legacy Board a la app y
// recibe los envíos. Las dos rutas van detrás de RequierePlus.
type BoardFormularioHandler struct {
	service ports.BoardFormularioService
}

func NewBoardFormularioHandler(service ports.BoardFormularioService) *BoardFormularioHandler {
	return &BoardFormularioHandler{service: service}
}

// errorFormularioBoard es el cuerpo de los errores: `message`, como el resto de
// la API que lee la app, y `campo` para que el formulario señale cuál falló.
type errorFormularioBoard struct {
	Message string `json:"message"`
	Campo   string `json:"campo,omitempty"`
}

// Formulario atiende GET /api/board/formularios/{clave}: la configuración con
// sus opciones y, si ya lo envió, el estado de su envío.
func (h *BoardFormularioHandler) Formulario(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(UserIDKey).(string)
	f, err := h.service.Formulario(r.Context(), userID, chi.URLParam(r, "clave"))
	if err != nil {
		h.responderError(w, "leyendo el formulario", err)
		return
	}
	responderJSON(w, http.StatusOK, f)
}

// Enviar atiende POST /api/board/formularios/{clave}. El remitente sale del
// token, nunca del cuerpo.
func (h *BoardFormularioHandler) Enviar(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(UserIDKey).(string)
	// 64 KB sobran: el texto más largo admite 2.000 caracteres y la foto va aparte.
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var in ports.EntradaEnvioBoard
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		responderJSON(w, http.StatusBadRequest, errorFormularioBoard{Message: "no se pudo leer el formulario"})
		return
	}
	envio, err := h.service.Enviar(r.Context(), userID, chi.URLParam(r, "clave"), in)
	if err != nil {
		h.responderError(w, "guardando el envío", err)
		return
	}
	responderJSON(w, http.StatusCreated, envio)
}

func (h *BoardFormularioHandler) responderError(w http.ResponseWriter, accion string, err error) {
	var ec *services.ErrCampoBoard
	switch {
	case errors.As(err, &ec):
		responderJSON(w, http.StatusBadRequest, errorFormularioBoard{Message: ec.Mensaje, Campo: ec.Campo})
	case errors.Is(err, services.ErrFormularioBoardTerminos):
		responderJSON(w, http.StatusBadRequest, errorFormularioBoard{Message: err.Error(), Campo: "acepta_terminos"})
	case errors.Is(err, services.ErrFormularioBoardNoExiste):
		responderJSON(w, http.StatusNotFound, errorFormularioBoard{Message: err.Error()})
	case errors.Is(err, services.ErrFormularioBoardEnviado):
		responderJSON(w, http.StatusConflict, errorFormularioBoard{Message: err.Error()})
	default:
		log.Printf("board formularios: error %s: %v", accion, err)
		responderJSON(w, http.StatusInternalServerError, errorFormularioBoard{Message: "no se pudo procesar el formulario; inténtalo de nuevo"})
	}
}

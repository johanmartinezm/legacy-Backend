package http

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"applegacy/backend/internal/core/services"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// BoardBandejaHandler: la bandeja de Legacy Board en el panel. Todas sus rutas
// van bajo AdminOnly: devuelven datos personales descifrados.
type BoardBandejaHandler struct {
	service ports.BoardBandejaService
}

func NewBoardBandejaHandler(service ports.BoardBandejaService) *BoardBandejaHandler {
	return &BoardBandejaHandler{service: service}
}

// porPaginaBandejaBoard: la bandeja descifra cada fila, así que la página por
// defecto es corta. El techo general (200) lo pone Paginacion.
const porPaginaBandejaBoard = 25

func fechaFiltro(valor string, finDeDia bool) (*time.Time, bool) {
	if valor == "" {
		return nil, true
	}
	f, err := time.ParseInLocation("2006-01-02", valor, time.Local)
	if err != nil {
		return nil, false
	}
	if finDeDia {
		f = f.AddDate(0, 0, 1) // «hasta» incluye todo ese día
	}
	return &f, true
}

// Listar atiende GET /api/admin/board/envios?formulario=A&estado=&desde=&hasta=
// &filtro=sector|expertise&opcion=<id>&limit=&offset=. El total va en
// X-Total-Count, como el resto de listados paginados.
func (h *BoardBandejaHandler) Listar(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	desde, ok1 := fechaFiltro(q.Get("desde"), false)
	hasta, ok2 := fechaFiltro(q.Get("hasta"), true)
	if !ok1 || !ok2 {
		http.Error(w, "las fechas van como AAAA-MM-DD", http.StatusBadRequest)
		return
	}
	filtro := domain.FiltroEnviosBoard{
		Formulario: strings.ToUpper(q.Get("formulario")),
		Estado:     q.Get("estado"),
		Desde:      desde,
		Hasta:      hasta,
		Campo:      q.Get("filtro"),
		Opcion:     q.Get("opcion"),
	}
	if filtro.Opcion == "" {
		filtro.Campo = ""
	}
	limit, offset := Paginacion(r, porPaginaBandejaBoard)
	lista, total, err := h.service.Listar(r.Context(), filtro, limit, offset)
	if err != nil {
		h.responderError(w, "listando", err)
		return
	}
	EscribirTotal(w, total)
	responderJSON(w, http.StatusOK, lista)
}

func (h *BoardBandejaHandler) Detalle(w http.ResponseWriter, r *http.Request) {
	d, err := h.service.Detalle(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.responderError(w, "leyendo el detalle", err)
		return
	}
	responderJSON(w, http.StatusOK, d)
}

// CambiarEstado atiende PUT /api/admin/board/envios/{id}/estado con
// {"estado": "aprobada"|"rechazada", "motivo": "..."}. Quién lo cambia sale
// del token del panel.
func (h *BoardBandejaHandler) CambiarEstado(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Estado string `json:"estado"`
		Motivo string `json:"motivo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	adminID, _ := r.Context().Value(UserIDKey).(string)
	d, err := h.service.CambiarEstado(r.Context(), chi.URLParam(r, "id"), in.Estado, in.Motivo, adminID)
	if err != nil {
		h.responderError(w, "cambiando el estado", err)
		return
	}
	responderJSON(w, http.StatusOK, d)
}

func (h *BoardBandejaHandler) Destinatarios(w http.ResponseWriter, r *http.Request) {
	correos, err := h.service.Destinatarios(r.Context())
	if err != nil {
		h.responderError(w, "leyendo destinatarios", err)
		return
	}
	responderJSON(w, http.StatusOK, map[string][]string{"correos": correos})
}

// GuardarDestinatarios atiende PUT /api/admin/board/destinatarios con
// {"correos": [...]}: deja exactamente esa lista.
func (h *BoardBandejaHandler) GuardarDestinatarios(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Correos []string `json:"correos"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	correos, err := h.service.GuardarDestinatarios(r.Context(), in.Correos)
	if err != nil {
		h.responderError(w, "guardando destinatarios", err)
		return
	}
	responderJSON(w, http.StatusOK, map[string][]string{"correos": correos})
}

func (h *BoardBandejaHandler) responderError(w http.ResponseWriter, accion string, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, "el envío no existe", http.StatusNotFound)
	case errors.Is(err, services.ErrFormularioBoardNoExiste):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, services.ErrEstadoBoardIgual), errors.Is(err, domain.ErrUniqueViolation):
		http.Error(w, "el envío ya está en ese estado o la persona tiene otra solicitud pendiente", http.StatusConflict)
	case services.EsErrorDeValidacionBandejaBoard(err):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		log.Printf("board bandeja: error %s: %v", accion, err)
		http.Error(w, "no se pudo completar la operación", http.StatusInternalServerError)
	}
}

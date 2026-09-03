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

type PaginaHandler struct {
	paginaService ports.PaginaService
}

func NewPaginaHandler(paginaService ports.PaginaService) *PaginaHandler {
	return &PaginaHandler{paginaService: paginaService}
}

// Get atiende GET /api/paginas/{slug}, que es por donde la app pide el
// contenido. Solo devuelve páginas publicadas.
func (h *PaginaHandler) Get(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	pagina, err := h.paginaService.GetPaginaPublicada(r.Context(), slug)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "la página no existe o no está publicada", http.StatusNotFound)
			return
		}
		log.Printf("paginas: error leyendo %q: %v", slug, err)
		http.Error(w, "no se pudo leer la página", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pagina)
}

// AdminList atiende GET /api/admin/paginas: devuelve todas, publicadas o no,
// que es lo que necesita el listado del panel.
func (h *PaginaHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	paginas, err := h.paginaService.ListPaginas(r.Context())
	if err != nil {
		log.Printf("paginas: error listando: %v", err)
		http.Error(w, "no se pudieron leer las páginas", http.StatusInternalServerError)
		return
	}

	// Un array vacío y no null: el panel recorre la respuesta directamente.
	if paginas == nil {
		paginas = []*domain.PaginaInformativa{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(paginas)
}

// AdminUpdate atiende PUT /api/admin/paginas/{slug}.
//
// El slug se toma de la URL y no del cuerpo: si viniera del cuerpo, editar una
// página podría sobrescribir otra por una errata del formulario.
func (h *PaginaHandler) AdminUpdate(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var pagina domain.PaginaInformativa
	if err := json.NewDecoder(r.Body).Decode(&pagina); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	pagina.Slug = slug

	if err := h.paginaService.ActualizarPagina(r.Context(), &pagina); err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			http.Error(w, "la página no existe", http.StatusNotFound)
		case errors.Is(err, services.ErrTituloVacio), errors.Is(err, services.ErrCuerpoDemasiadoLargo):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			log.Printf("paginas: error actualizando %q: %v", slug, err)
			http.Error(w, "no se pudo guardar la página", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pagina)
}

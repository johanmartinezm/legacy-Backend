package http

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type ContentHandler struct {
	contentService ports.ContentService
	// notifier avisa cuando un contenido pasa a estar publicado. Admite nil.
	notifier ports.NotificationService
}

func NewContentHandler(contentService ports.ContentService, notifier ports.NotificationService) *ContentHandler {
	return &ContentHandler{contentService: contentService, notifier: notifier}
}

// PUBLIC ENDPOINTS

func (h *ContentHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.contentService.ListCategories(r.Context())
	if err != nil {
		fmt.Printf("Error ListCategories: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(cats)
}

func (h *ContentHandler) ListContent(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	// For public users, we only show published content
	contents, err := h.contentService.ListContent(r.Context(), category, true)
	if err != nil {
		fmt.Printf("Error ListContent (public): %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(contents)
}

func (h *ContentHandler) GetContent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	content, err := h.contentService.GetContentByID(r.Context(), id)
	if err != nil {
		http.Error(w, "content not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(content)
}

// ADMIN ENDPOINTS (CRUD Categories)

func (h *ContentHandler) AdminCreateCategory(w http.ResponseWriter, r *http.Request) {
	var cat domain.ContentCategory
	if err := json.NewDecoder(r.Body).Decode(&cat); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.contentService.CreateCategory(r.Context(), &cat); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cat)
}

func (h *ContentHandler) AdminUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var cat domain.ContentCategory
	if err := json.NewDecoder(r.Body).Decode(&cat); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	cat.ID = id
	if err := h.contentService.UpdateCategory(r.Context(), &cat); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(cat)
}

func (h *ContentHandler) AdminDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.contentService.DeleteCategory(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ADMIN ENDPOINTS (CRUD Content)

func (h *ContentHandler) AdminListContent(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	// For admin, we show everything (published and unpublished)
	contents, err := h.contentService.ListContent(r.Context(), category, false)
	if err != nil {
		fmt.Printf("Error AdminListContent: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(contents)
}

func (h *ContentHandler) AdminCreateContent(w http.ResponseWriter, r *http.Request) {
	var c domain.CustomContent
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.contentService.CreateContent(r.Context(), &c); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Solo se avisa de lo que el usuario puede abrir. Un borrador no interesa a
	// nadie todavía, y avisar de él mandaría a la app a una pantalla vacía.
	if c.IsPublished {
		adminID, _ := r.Context().Value(UserIDKey).(string)
		notificarNovedad(r.Context(), h.notifier, adminID, c.Title, c.Excerpt,
			map[string]string{"type": "content", "id": c.ID})
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(c)
}

func (h *ContentHandler) AdminUpdateContent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var c domain.CustomContent
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	c.ID = id

	// Se mira cómo estaba ANTES de guardar, porque lo que dispara el aviso es el
	// paso de borrador a publicado, no cada edición. Sin esto, corregir una
	// errata en un artículo ya publicado volvería a notificar a todo el mundo.
	//
	// Si no se puede leer el estado anterior no se avisa: es preferible perder
	// un aviso a repetirlo, y el error no debe impedir guardar los cambios.
	estabaPublicado := true
	if anterior, err := h.contentService.GetContentByID(r.Context(), id); err == nil && anterior != nil {
		estabaPublicado = anterior.IsPublished
	}

	if err := h.contentService.UpdateContent(r.Context(), &c); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if c.IsPublished && !estabaPublicado {
		adminID, _ := r.Context().Value(UserIDKey).(string)
		notificarNovedad(r.Context(), h.notifier, adminID, c.Title, c.Excerpt,
			map[string]string{"type": "content", "id": c.ID})
	}

	json.NewEncoder(w).Encode(c)
}

func (h *ContentHandler) AdminDeleteContent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.contentService.DeleteContent(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

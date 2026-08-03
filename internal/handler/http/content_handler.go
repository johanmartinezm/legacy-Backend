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
}

func NewContentHandler(contentService ports.ContentService) *ContentHandler {
	return &ContentHandler{contentService: contentService}
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
	if err := h.contentService.UpdateContent(r.Context(), &c); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

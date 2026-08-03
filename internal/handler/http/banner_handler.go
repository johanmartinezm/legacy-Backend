package http

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type BannerHandler struct {
	bannerService ports.BannerService
}

func NewBannerHandler(bannerService ports.BannerService) *BannerHandler {
	return &BannerHandler{bannerService: bannerService}
}

func (h *BannerHandler) ListActive(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	if category == "" {
		category = "home"
	}

	banners, err := h.bannerService.GetActiveBanners(r.Context(), category)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(banners)
}

func (h *BannerHandler) AdminListAll(w http.ResponseWriter, r *http.Request) {
	banners, err := h.bannerService.ListAllBanners(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(banners)
}

func (h *BannerHandler) AdminCreate(w http.ResponseWriter, r *http.Request) {
	var banner domain.Banner
	if err := json.NewDecoder(r.Body).Decode(&banner); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.bannerService.CreateBanner(r.Context(), &banner); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(banner)
}

func (h *BannerHandler) AdminUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var banner domain.Banner
	if err := json.NewDecoder(r.Body).Decode(&banner); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	banner.ID = id

	if err := h.bannerService.UpdateBanner(r.Context(), &banner); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(banner)
}

func (h *BannerHandler) AdminDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.bannerService.DeleteBanner(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

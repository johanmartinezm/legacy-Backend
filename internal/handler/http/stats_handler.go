package http

import (
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"net/http"
)

type StatsHandler struct {
	service ports.StatsService
}

func NewStatsHandler(service ports.StatsService) *StatsHandler {
	return &StatsHandler{service: service}
}

func (h *StatsHandler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetDashboardStats(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

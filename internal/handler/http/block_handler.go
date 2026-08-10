package http

import (
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// respondJSON escribe la respuesta con su cabecera y su código. Con 204 no se
// escribe cuerpo: un No Content con contenido confunde a los clientes.
func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if status == http.StatusNoContent || payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

type BlockHandler struct {
	service ports.BlockService
}

func NewBlockHandler(service ports.BlockService) *BlockHandler {
	return &BlockHandler{service: service}
}

// BlockUser bloquea a otra persona.
//
// Quien bloquea sale SIEMPRE del token, nunca del cuerpo ni de la URL: si
// viniera de fuera, cualquiera podría bloquear en nombre de otra persona y
// aislarla de la comunidad.
func (h *BlockHandler) BlockUser(w http.ResponseWriter, r *http.Request) {
	blockerID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || blockerID == "" {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"message": "No autorizado"})
		return
	}
	blockedID := chi.URLParam(r, "userID")

	if err := h.service.BlockUser(r.Context(), blockerID, blockedID); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *BlockHandler) UnblockUser(w http.ResponseWriter, r *http.Request) {
	blockerID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || blockerID == "" {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"message": "No autorizado"})
		return
	}
	blockedID := chi.URLParam(r, "userID")

	if err := h.service.UnblockUser(r.Context(), blockerID, blockedID); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

// ListBlocked devuelve a quién ha bloqueado quien pregunta, para poder
// desbloquear desde la app.
func (h *BlockHandler) ListBlocked(w http.ResponseWriter, r *http.Request) {
	blockerID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || blockerID == "" {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"message": "No autorizado"})
		return
	}

	bloqueados, err := h.service.ListBlocked(r.Context(), blockerID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"message": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, bloqueados)
}

type reportUserRequest struct {
	Reason    string  `json:"reason"`
	MessageID *string `json:"message_id,omitempty"`
}

// ReportUser denuncia a una persona. Puede señalar un mensaje concreto o no.
func (h *BlockHandler) ReportUser(w http.ResponseWriter, r *http.Request) {
	reporterID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || reporterID == "" {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"message": "No autorizado"})
		return
	}
	reportedID := chi.URLParam(r, "userID")

	var req reportUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"message": "Cuerpo de la petición no válido"})
		return
	}

	if err := h.service.ReportUser(r.Context(), reporterID, reportedID, req.Reason, req.MessageID); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"message": "Reporte enviado"})
}

// ListReports es la bandeja del panel administrativo. Acepta ?status=pending.
func (h *BlockHandler) ListReports(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	reportes, err := h.service.ListReports(r.Context(), status)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"message": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, reportes)
}

type resolveReportRequest struct {
	Status string `json:"status"`
}

func (h *BlockHandler) ResolveReport(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "reportID")

	var req resolveReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"message": "Cuerpo de la petición no válido"})
		return
	}

	if err := h.service.ResolveReport(r.Context(), reportID, req.Status); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "Reporte actualizado"})
}

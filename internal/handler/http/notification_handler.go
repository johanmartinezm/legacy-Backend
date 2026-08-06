package http

import (
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"net/http"
	"strconv"
)

type NotificationHandler struct {
	service ports.NotificationService
}

func NewNotificationHandler(service ports.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: service}
}

// SubscribeAll suscribe al tópico general todos los dispositivos ya
// registrados. Va bajo AdminOnly.
//
// Hace falta porque la app nunca llamó a subscribeToTopic: los tokens estaban
// guardados pero ninguno recibía los envíos a "todos". Los registros nuevos se
// suscriben solos; esto arregla los anteriores, y sirve después para reparar
// suscripciones sin tocar la base.
func (h *NotificationHandler) SubscribeAll(w http.ResponseWriter, r *http.Request) {
	suscritos, err := h.service.SubscribeAllToTopic(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"subscribed": suscritos})
}

// RegisterToken registers a new FCM token for the logged-in mobile user
func (h *NotificationHandler) RegisterToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "no autorizado", http.StatusUnauthorized)
		return
	}

	var req struct {
		FCMToken   string `json:"fcm_token"`
		DeviceType string `json:"device_type"` // 'android', 'ios', 'web'
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "cuerpo de petición inválido", http.StatusBadRequest)
		return
	}

	if req.FCMToken == "" || req.DeviceType == "" {
		http.Error(w, "fcm_token y device_type son requeridos", http.StatusBadRequest)
		return
	}

	err := h.service.RegisterToken(r.Context(), userID, req.FCMToken, req.DeviceType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "token registrado con éxito"})
}

// Send broadcasts or targets a push notification
func (h *NotificationHandler) Send(w http.ResponseWriter, r *http.Request) {
	adminID, _ := r.Context().Value(UserIDKey).(string) // Opcional, si admin_middleware inyecta el ID

	var req struct {
		Title       string            `json:"title"`
		Body        string            `json:"body"`
		TargetType  string            `json:"target_type"`  // 'all', 'group', 'user'
		TargetValue string            `json:"target_value"` // e.g. group name, user_id
		Data        map[string]string `json:"data"`         // Additional payloads
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "cuerpo de petición inválido", http.StatusBadRequest)
		return
	}

	if req.Title == "" || req.Body == "" || req.TargetType == "" {
		http.Error(w, "title, body y target_type son campos obligatorios", http.StatusBadRequest)
		return
	}

	err := h.service.SendNotification(r.Context(), adminID, req.Title, req.Body, req.TargetType, req.TargetValue, req.Data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Notificación push despachada con éxito"})
}

// GetHistory lists all sent notifications history
func (h *NotificationHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	history, err := h.service.GetHistory(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

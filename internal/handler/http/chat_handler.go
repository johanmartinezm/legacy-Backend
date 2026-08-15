package http

import (
	"applegacy/backend/internal/core/ports"
	"applegacy/backend/internal/infrastructure/websocket"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type ChatHandler struct {
	chatService ports.ChatService
	hub         *websocket.Hub
}

func NewChatHandler(chatService ports.ChatService, hub *websocket.Hub) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		hub:         hub,
	}
}

func (h *ChatHandler) HandleWS(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	websocket.ServeWs(h.hub, userID, w, r)
}

func (h *ChatHandler) SendInvite(w http.ResponseWriter, r *http.Request) {
	requesterID := r.Context().Value(UserIDKey).(string)
	receiverID := chi.URLParam(r, "receiverID")

	if err := h.chatService.SendInvite(r.Context(), requesterID, receiverID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Invite sent"})
}

func (h *ChatHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserIDKey).(string)
	connectionID := chi.URLParam(r, "connectionID")
	fmt.Printf("[DEBUG CHAT] User %s trying to accept connection %s\n", userID, connectionID)

	if err := h.chatService.AcceptInvite(r.Context(), connectionID, userID); err != nil {
		fmt.Printf("[DEBUG CHAT] Failed to accept connection %s: %v\n", connectionID, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Printf("[DEBUG CHAT] Successfully accepted connection %s\n", connectionID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Invite accepted"})
}

func (h *ChatHandler) ListConnections(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserIDKey).(string)
	fmt.Printf("[DEBUG CHAT] Listing connections for userID: %s\n", userID)
	conns, err := h.chatService.ListMyConnections(r.Context(), userID)
	fmt.Printf("[DEBUG CHAT] Found %d connections for userID: %s\n", len(conns), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(conns)
}

func (h *ChatHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserIDKey).(string)
	connectionID := chi.URLParam(r, "connectionID")

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	messages, err := h.chatService.GetChatHistory(r.Context(), connectionID, userID, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(messages)
}

func (h *ChatHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	// El directorio se filtra por bloqueos, así que depende de quién pregunta.
	userID := r.Context().Value(UserIDKey).(string)
	members, err := h.chatService.ListMembers(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(members)
}

func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserIDKey).(string)
	var req struct {
		ConnectionID string `json:"connection_id"`
		Content      string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	msg, err := h.chatService.SendMessage(r.Context(), userID, req.ConnectionID, req.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// La app envía por aquí, no por el WebSocket —el socket solo lo usa para
	// recibir—, y este camino nunca repartía el mensaje: el destinatario no veía
	// nada hasta recargar la pantalla, aunque tuviera el chat abierto delante.
	// El hub entrega a quien esté conectado; a quien no lo esté ya lo avisa la
	// push que dispara el servicio.
	if h.hub != nil {
		h.hub.DeliverMessage(msg)
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(msg)
}

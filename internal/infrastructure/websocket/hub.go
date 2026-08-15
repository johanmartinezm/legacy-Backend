package websocket

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Adjust for production
	},
}

type Client struct {
	Hub    *Hub
	Conn   *websocket.Conn
	Send   chan []byte
	UserID string
}

type Hub struct {
	clients     map[string]*Client // map[userID]*Client
	register    chan *Client
	unregister  chan *Client
	mu          sync.RWMutex
	chatService ports.ChatService
	chatRepo    ports.ChatRepository // Added to avoid circularity if possible or just use service
}

func NewHub(chatService ports.ChatService, chatRepo ports.ChatRepository) *Hub {
	return &Hub{
		clients:     make(map[string]*Client),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		mu:          sync.RWMutex{},
		chatService: chatService,
		chatRepo:    chatRepo,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.UserID] = client
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.UserID]; ok {
				delete(h.clients, client.UserID)
				close(client.Send)
			}
			h.mu.Unlock()
		}
	}
}

type WSMessage struct {
	Type         string `json:"type"` // "message", "typing", "error"
	ID           string `json:"id,omitempty"`
	ConnectionID string `json:"connection_id"`
	Content      string `json:"content"`
	SenderID     string `json:"sender_id"`
	CreatedAt    string `json:"created_at"`
}

func ServeWs(hub *Hub, userID string, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}
	client := &Client{Hub: hub, Conn: conn, Send: make(chan []byte, 256), UserID: userID}
	client.Hub.register <- client

	go client.WritePump()
	go client.ReadPump()
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			continue
		}

		if wsMsg.Type == "message" {
			msg, err := c.Hub.chatService.SendMessage(context.Background(), c.UserID, wsMsg.ConnectionID, wsMsg.Content)
			if err != nil {
				errResp, _ := json.Marshal(WSMessage{Type: "error", Content: err.Error()})
				c.Send <- errResp
				continue
			}

			// Delivery
			c.Hub.DeliverMessage(msg)
		}
	}
}

func (h *Hub) DeliverMessage(msg *domain.Message) {
	// Find connection to know recipient
	conn, err := h.chatRepo.GetConnection(context.Background(), msg.ConnectionID)
	if err != nil {
		return
	}

	recipientID := conn.ReceiverID
	if msg.SenderID == conn.ReceiverID {
		recipientID = conn.RequesterID
	}

	wsResp := WSMessage{
		Type:         "message",
		ID:           msg.ID,
		ConnectionID: msg.ConnectionID,
		Content:      msg.ContentEncrypted, // This is unencrypted in the msg returned by SendMessage
		SenderID:     msg.SenderID,
		CreatedAt:    msg.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	data, _ := json.Marshal(wsResp)

	h.mu.RLock()
	defer h.mu.RUnlock()

	// Delivery to recipient
	if recipientClient, ok := h.clients[recipientID]; ok {
		entregar(recipientClient, data)
	}

	// Also send back to sender for confirmation if they have multiple devices or just for consistency
	if senderClient, ok := h.clients[msg.SenderID]; ok {
		entregar(senderClient, data)
	}
}

// entregar deja el mensaje en la cola del cliente sin poder tumbar a quien
// llama.
//
// Escribir directamente en el canal tenía dos formas de hacer daño, y ahora que
// esto se invoca también desde el handler HTTP alcanzarían a la petición de
// quien escribe: si la cola está llena —un cliente que no lee, un móvil con la
// pantalla apagada— el envío bloquea para siempre, y si el cliente acaba de
// desconectarse el canal está cerrado y escribir en él es un panic que se lleva
// por delante el proceso entero.
//
// Perder la entrega en vivo de un mensaje no es grave: está guardado, la push
// avisa igual y la pantalla lo carga al abrirse.
func entregar(c *Client, data []byte) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[WS] no se pudo entregar a %s: cliente desconectado", c.UserID)
		}
	}()

	select {
	case c.Send <- data:
	default:
		log.Printf("[WS] cola llena para %s, se omite la entrega en vivo", c.UserID)
	}
}

func (c *Client) WritePump() {
	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}

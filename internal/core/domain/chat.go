package domain

import "time"

type ConnectionStatus string

const (
	StatusPending  ConnectionStatus = "PENDING"
	StatusAccepted ConnectionStatus = "ACCEPTED"
	StatusRejected ConnectionStatus = "REJECTED"
	StatusBlocked  ConnectionStatus = "BLOCKED"
)

type ChatConnection struct {
	ID          string           `json:"id" db:"id"`
	RequesterID string           `json:"requester_id" db:"requester_id"`
	ReceiverID  string           `json:"receiver_id" db:"receiver_id"`
	Status      ConnectionStatus `json:"status" db:"status"`
	CreatedAt   time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at" db:"updated_at"`

	// Optional: Information about the other user and unread count for UI
	OtherUser   *User `json:"other_user,omitempty" db:"-"`
	UnreadCount int   `json:"unread_count" db:"unread_count"`
}

type Message struct {
	ID               string    `json:"id" db:"id"`
	ConnectionID     string    `json:"connection_id" db:"connection_id"`
	SenderID         string    `json:"sender_id" db:"sender_id"`
	ContentEncrypted string    `json:"content" db:"content_encrypted"` // Renamed in JSON for simplicity
	IsRead           bool      `json:"is_read" db:"is_read"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

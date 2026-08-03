package domain

import "time"

type FCMToken struct {
	UserID     string    `json:"user_id"`
	FCMToken   string    `json:"fcm_token"`
	DeviceType string    `json:"device_type"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type NotificationHistory struct {
	ID          string    `json:"id"`
	AdminID     string    `json:"admin_id,omitempty"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	TargetType  string    `json:"target_type"`  // "all", "group", "user"
	TargetValue string    `json:"target_value"` // e.g. "role_name", "user_id", or ""
	SentAt      time.Time `json:"sent_at"`
	Status      string    `json:"status"`
}

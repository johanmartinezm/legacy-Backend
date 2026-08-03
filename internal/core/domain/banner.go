package domain

import "time"

type Banner struct {
	ID           string    `json:"id" db:"id"`
	Title        string    `json:"title" db:"title"`
	Subtitle     string    `json:"subtitle" db:"subtitle"`
	Category     string    `json:"category" db:"category"` // 'home', 'community'
	ImageURL     string    `json:"image_url" db:"image_url"`
	ActionType   string    `json:"action_type" db:"action_type"`
	ActionTarget string    `json:"action_target" db:"action_target"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	SortOrder    int       `json:"sort_order" db:"sort_order"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

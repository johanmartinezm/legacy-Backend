package domain

import "time"

type SynergyStatus string

const (
	SynergyStatusActive   SynergyStatus = "active"
	SynergyStatusClosed   SynergyStatus = "closed"
	SynergyStatusArchived SynergyStatus = "archived"
)

type Synergy struct {
	ID          string        `json:"id" db:"id"`
	AuthorID    string        `json:"author_id" db:"author_id"`
	Title       string        `json:"title" db:"title"`
	Description string        `json:"description" db:"description"`
	Category    string        `json:"category" db:"category"`
	ImageURL    string        `json:"image_url,omitempty" db:"image_url"`
	Status      SynergyStatus `json:"status" db:"status"`
	ViewsCount  int           `json:"views_count" db:"views_count"`
	LikesCount    int           `json:"likes_count" db:"likes_count"`
	CommentsCount int           `json:"comments_count" db:"comments_count"`
	CreatedAt   time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at" db:"updated_at"`

	// Extra information for UI
	Author   *User             `json:"author,omitempty" db:"-"`
	Comments []SynergyComment `json:"comments,omitempty" db:"-"`
}

type SynergyComment struct {
	ID              string    `json:"id" db:"id"`
	SynergyID       string    `json:"synergy_id" db:"synergy_id"`
	UserID          string    `json:"user_id" db:"user_id"`
	Content         string    `json:"content" db:"content"`
	ParentCommentID *string   `json:"parent_comment_id,omitempty" db:"parent_comment_id"`
	IsExpertFeedback bool      `json:"is_expert_feedback" db:"is_expert_feedback"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`

	// Extra information for UI
	User *User `json:"user,omitempty" db:"-"`
}

type SynergyLike struct {
	SynergyID string    `json:"synergy_id" db:"synergy_id"`
	UserID    string    `json:"user_id" db:"user_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

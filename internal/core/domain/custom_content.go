package domain

import "time"

type ContentType string

const (
	ContentTypeText  ContentType = "text"
	ContentTypeVideo ContentType = "video"
)

type CustomContent struct {
	ID           string      `json:"id" db:"id"`
	CategoryID   *string     `json:"category_id" db:"category_id"`
	Type         ContentType `json:"type" db:"type"`
	Title        string      `json:"title" db:"title"`
	Excerpt      string      `json:"excerpt" db:"excerpt"`
	BodyText     string      `json:"body_text" db:"body_text"`
	VideoURL     string      `json:"video_url" db:"video_url"`
	ThumbnailURL string      `json:"thumbnail_url" db:"thumbnail_url"`
	IsPublished  bool        `json:"is_published" db:"is_published"`
	PublishedAt  *time.Time  `json:"published_at" db:"published_at"`
	CreatedAt    time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at" db:"updated_at"`

	// Bonus: Category name for convenience in lists
	CategoryName string `json:"category_name,omitempty" db:"category_name"`
}

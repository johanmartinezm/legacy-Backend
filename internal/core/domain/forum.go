package domain

import "time"

type ForumStatus string
type PostStatus string

const (
	ForumStatusActive  ForumStatus = "active"
	ForumStatusLocked  ForumStatus = "locked"
	ForumStatusHidden  ForumStatus = "hidden"
	ForumStatusDeleted ForumStatus = "deleted"

	PostStatusActive  PostStatus = "active"
	PostStatusDeleted PostStatus = "deleted"
	PostStatusFlagged PostStatus = "flagged"
)

// Forum represents a forum topic/category.
type Forum struct {
	ID              string      `json:"id"               db:"id"`
	Title           string      `json:"title"            db:"title"`
	Description     string      `json:"description"      db:"description"`
	CoverURL        string      `json:"cover_url"        db:"cover_url"`
	Status          ForumStatus `json:"status"           db:"status"`
	CreatedByUserID *string     `json:"created_by_user_id" db:"created_by_user_id"`
	CreatedByAdmin  bool        `json:"created_by_admin" db:"created_by_admin"`
	PostCount       int         `json:"post_count"       db:"-"`                  // calculated
	AuthorAlias     string      `json:"author_alias,omitempty" db:"author_alias"` // joined
	CreatedAt       time.Time   `json:"created_at"       db:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"       db:"updated_at"`
}

// ForumPost represents a reply/post within a forum.
// NEVER expose the real name, email, or profile photo of the author.
type ForumPost struct {
	ID          string     `json:"id"           db:"id"`
	ForumID     string     `json:"forum_id"     db:"forum_id"`
	ParentID    *string    `json:"parent_id"    db:"parent_id"`
	AuthorAlias string     `json:"author_alias" db:"author_alias"` // the only public identifier
	Content     string     `json:"content"      db:"content"`
	ImageURL    string     `json:"image_url"    db:"image_url"`
	Status      PostStatus `json:"status"       db:"status"`
	ReplyCount  int        `json:"reply_count"  db:"-"` // calculated
	CreatedAt   time.Time  `json:"created_at"   db:"created_at"`
}

// ForumPostReport represents a report made by a user on a post.
type ForumPostReport struct {
	ID         string    `json:"id"          db:"id"`
	PostID     string    `json:"post_id"     db:"post_id"`
	ReporterID string    `json:"-"           db:"reporter_id"` // hidden
	Reason     string    `json:"reason"      db:"reason"`
	CreatedAt  time.Time `json:"created_at"  db:"created_at"`
}

package domain

import "time"

type PostLike struct {
	ID        int       `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	PostID    string    `json:"post_id" db:"post_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type LikeStatus struct {
	PostID     string `json:"post_id"`
	IsLiked    bool   `json:"is_liked"`
	TotalLikes int    `json:"total_likes"`
	TotalViews int    `json:"total_views"`
}

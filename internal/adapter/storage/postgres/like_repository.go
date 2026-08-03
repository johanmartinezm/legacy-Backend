package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

type LikeRepository struct {
	db *pgxpool.Pool
}

func NewLikeRepository(db *pgxpool.Pool) *LikeRepository {
	return &LikeRepository{db: db}
}

func (r *LikeRepository) ToggleLike(ctx context.Context, userID, postID string) (bool, error) {
	// Check if already liked
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM core.post_likes WHERE user_id = $1 AND post_id = $2)"
	err := r.db.QueryRow(ctx, query, userID, postID).Scan(&exists)
	if err != nil {
		return false, err
	}

	if exists {
		// Delete
		_, err = r.db.Exec(ctx, "DELETE FROM core.post_likes WHERE user_id = $1 AND post_id = $2", userID, postID)
		return false, err
	} else {
		// Insert
		_, err = r.db.Exec(ctx, "INSERT INTO core.post_likes (user_id, post_id) VALUES ($1, $2)", userID, postID)
		return true, err
	}
}

func (r *LikeRepository) GetLikeStatus(ctx context.Context, userID, postID string) (*domain.LikeStatus, error) {
	status := &domain.LikeStatus{PostID: postID}

	// Total likes
	queryLikes := "SELECT COUNT(*) FROM core.post_likes WHERE post_id = $1"
	err := r.db.QueryRow(ctx, queryLikes, postID).Scan(&status.TotalLikes)
	if err != nil {
		return nil, err
	}

	// Total views
	queryViews := "SELECT COUNT(*) FROM core.post_views WHERE post_id = $1"
	err = r.db.QueryRow(ctx, queryViews, postID).Scan(&status.TotalViews)
	if err != nil {
		// If table empty or error, default to 0 is okay, but let's handle error
		return nil, err
	}

	// Is liked by user
	if userID != "" {
		queryExists := "SELECT EXISTS(SELECT 1 FROM core.post_likes WHERE user_id = $1 AND post_id = $2)"
		err = r.db.QueryRow(ctx, queryExists, userID, postID).Scan(&status.IsLiked)
		if err != nil {
			return nil, err
		}
	}

	return status, nil
}

func (r *LikeRepository) RecordView(ctx context.Context, userID, postID, title string) error {
	var err error
	if userID != "" {
		_, err = r.db.Exec(ctx, "INSERT INTO core.post_views (user_id, post_id, title) VALUES ($1, $2, $3)", userID, postID, title)
	} else {
		_, err = r.db.Exec(ctx, "INSERT INTO core.post_views (post_id, title) VALUES ($1, $2)", postID, title)
	}
	return err
}

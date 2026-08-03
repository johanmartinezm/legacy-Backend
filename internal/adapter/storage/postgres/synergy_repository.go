package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"github.com/jackc/pgx/v4/pgxpool"
)

type SynergyRepository struct {
	db *pgxpool.Pool
}

func NewSynergyRepository(db *pgxpool.Pool) *SynergyRepository {
	return &SynergyRepository{db: db}
}

func (r *SynergyRepository) CreateSynergy(ctx context.Context, s *domain.Synergy) error {
	query := `
		INSERT INTO community.synergies (author_id, title, description, category, image_url, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		s.AuthorID, s.Title, s.Description, s.Category, s.ImageURL, s.Status,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

func (r *SynergyRepository) GetSynergyByID(ctx context.Context, id string) (*domain.Synergy, error) {
	query := `
		SELECT s.id, s.author_id, s.title, s.description, s.category, s.image_url, s.status, 
		       s.views_count, s.likes_count, s.comments_count, s.created_at, s.updated_at,
		       u.first_name, u.last_name, u.profile_image_url
		FROM community.synergies s
		JOIN core.users u ON s.author_id = u.id
		WHERE s.id = $1
	`
	var s domain.Synergy
	var author domain.User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.AuthorID, &s.Title, &s.Description, &s.Category, &s.ImageURL, &s.Status,
		&s.ViewsCount, &s.LikesCount, &s.CommentsCount, &s.CreatedAt, &s.UpdatedAt,
		&author.FirstName, &author.LastName, &author.ProfileImageUrl,
	)
	if err != nil {
		return nil, err
	}
	author.ID = s.AuthorID
	s.Author = &author
	return &s, nil
}

func (r *SynergyRepository) ListSynergies(ctx context.Context, category string, status string, search string, offset, limit int) ([]domain.Synergy, error) {
	query := `
		SELECT s.id, s.author_id, s.title, s.description, s.category, s.image_url, s.status, 
		       s.views_count, s.likes_count, s.comments_count, s.created_at, s.updated_at,
		       u.first_name, u.last_name, u.profile_image_url
		FROM community.synergies s
		JOIN core.users u ON s.author_id = u.id
		WHERE ($1 = '' OR s.category = $1)
		  AND ($2 = '' OR s.status = $2)
		  AND ($5 = '' OR s.title ILIKE '%' || $5 || '%' OR s.description ILIKE '%' || $5 || '%')
		ORDER BY s.created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.Query(ctx, query, category, status, limit, offset, search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var synergies []domain.Synergy
	for rows.Next() {
		var s domain.Synergy
		var author domain.User
		err := rows.Scan(
			&s.ID, &s.AuthorID, &s.Title, &s.Description, &s.Category, &s.ImageURL, &s.Status,
			&s.ViewsCount, &s.LikesCount, &s.CommentsCount, &s.CreatedAt, &s.UpdatedAt,
			&author.FirstName, &author.LastName, &author.ProfileImageUrl,
		)
		if err != nil {
			return nil, err
		}
		author.ID = s.AuthorID
		s.Author = &author
		synergies = append(synergies, s)
	}
	return synergies, nil
}

func (r *SynergyRepository) UpdateSynergy(ctx context.Context, s *domain.Synergy) error {
	query := `
		UPDATE community.synergies 
		SET title = $1, description = $2, category = $3, image_url = $4, status = $5, updated_at = NOW()
		WHERE id = $6
	`
	_, err := r.db.Exec(ctx, query, s.Title, s.Description, s.Category, s.ImageURL, s.Status, s.ID)
	return err
}

func (r *SynergyRepository) DeleteSynergy(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM community.synergies WHERE id = $1`, id)
	return err
}

func (r *SynergyRepository) CreateComment(ctx context.Context, c *domain.SynergyComment) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO community.synergy_comments (synergy_id, user_id, content, parent_comment_id, is_expert_feedback)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	err = tx.QueryRow(ctx, query,
		c.SynergyID, c.UserID, c.Content, c.ParentCommentID, c.IsExpertFeedback,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `UPDATE community.synergies SET comments_count = comments_count + 1 WHERE id = $1`, c.SynergyID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *SynergyRepository) GetCommentsBySynergyID(ctx context.Context, synergyID string) ([]domain.SynergyComment, error) {
	query := `
		SELECT c.id, c.synergy_id, c.user_id, c.content, c.parent_comment_id, c.is_expert_feedback, c.created_at, c.updated_at,
		       u.first_name, u.last_name, u.profile_image_url
		FROM community.synergy_comments c
		JOIN core.users u ON c.user_id = u.id
		WHERE c.synergy_id = $1
		ORDER BY c.created_at ASC
	`
	rows, err := r.db.Query(ctx, query, synergyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []domain.SynergyComment
	for rows.Next() {
		var c domain.SynergyComment
		var author domain.User
		err := rows.Scan(
			&c.ID, &c.SynergyID, &c.UserID, &c.Content, &c.ParentCommentID, &c.IsExpertFeedback, &c.CreatedAt, &c.UpdatedAt,
			&author.FirstName, &author.LastName, &author.ProfileImageUrl,
		)
		if err != nil {
			return nil, err
		}
		author.ID = c.UserID
		c.User = &author
		comments = append(comments, c)
	}
	return comments, nil
}

func (r *SynergyRepository) AddLike(ctx context.Context, synergyID, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `INSERT INTO community.synergy_likes (synergy_id, user_id) VALUES ($1, $2)`, synergyID, userID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `UPDATE community.synergies SET likes_count = likes_count + 1 WHERE id = $1`, synergyID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *SynergyRepository) RemoveLike(ctx context.Context, synergyID, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `DELETE FROM community.synergy_likes WHERE synergy_id = $1 AND user_id = $2`, synergyID, userID)
	if err != nil {
		return err
	}

	if tag.RowsAffected() > 0 {
		_, err = tx.Exec(ctx, `UPDATE community.synergies SET likes_count = likes_count - 1 WHERE id = $1 AND likes_count > 0`, synergyID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *SynergyRepository) IsLikedByUser(ctx context.Context, synergyID, userID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM community.synergy_likes WHERE synergy_id = $1 AND user_id = $2)`
	err := r.db.QueryRow(ctx, query, synergyID, userID).Scan(&exists)
	return exists, err
}

func (r *SynergyRepository) IncrementViews(ctx context.Context, synergyID string) error {
	_, err := r.db.Exec(ctx, `UPDATE community.synergies SET views_count = views_count + 1 WHERE id = $1`, synergyID)
	return err
}

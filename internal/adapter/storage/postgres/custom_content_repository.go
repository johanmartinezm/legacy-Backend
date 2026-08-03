package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
)

type CustomContentRepository struct {
	db *pgxpool.Pool
}

func NewCustomContentRepository(db *pgxpool.Pool) *CustomContentRepository {
	return &CustomContentRepository{db: db}
}

func (r *CustomContentRepository) Create(ctx context.Context, c *domain.CustomContent) error {
	sql := `
		INSERT INTO core.custom_contents (
			category_id, type, title, excerpt, body_text, video_url, thumbnail_url, is_published, published_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, sql,
		c.CategoryID, c.Type, c.Title, c.Excerpt, c.BodyText, c.VideoURL, c.ThumbnailURL, c.IsPublished, c.PublishedAt,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *CustomContentRepository) Update(ctx context.Context, c *domain.CustomContent) error {
	sql := `
		UPDATE core.custom_contents
		SET category_id = $1, type = $2, title = $3, excerpt = $4, body_text = $5, 
		    video_url = $6, thumbnail_url = $7, is_published = $8, published_at = $9, updated_at = CURRENT_TIMESTAMP
		WHERE id = $10
	`
	_, err := r.db.Exec(ctx, sql,
		c.CategoryID, c.Type, c.Title, c.Excerpt, c.BodyText, c.VideoURL, c.ThumbnailURL, c.IsPublished, c.PublishedAt, c.ID,
	)
	return err
}

func (r *CustomContentRepository) Delete(ctx context.Context, id string) error {
	sql := `DELETE FROM core.custom_contents WHERE id = $1`
	_, err := r.db.Exec(ctx, sql, id)
	return err
}

func (r *CustomContentRepository) List(ctx context.Context, categorySlug string, onlyPublished bool) ([]*domain.CustomContent, error) {
	sql := `
		SELECT c.id, c.category_id, c.type, c.title, 
		       COALESCE(c.excerpt, ''), COALESCE(c.body_text, ''), 
		       COALESCE(c.video_url, ''), COALESCE(c.thumbnail_url, ''), 
		       c.is_published, c.published_at, c.created_at, c.updated_at, 
		       COALESCE(cat.name, '') as category_name
		FROM core.custom_contents c
		LEFT JOIN core.content_categories cat ON c.category_id = cat.id
		WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

	if categorySlug != "" {
		sql += fmt.Sprintf(" AND cat.slug = $%d", argCount)
		args = append(args, categorySlug)
		argCount++
	}

	if onlyPublished {
		sql += " AND c.is_published = true"
	}

	sql += " ORDER BY c.created_at DESC"

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contents []*domain.CustomContent
	for rows.Next() {
		var c domain.CustomContent
		err := rows.Scan(
			&c.ID, &c.CategoryID, &c.Type, &c.Title, &c.Excerpt, &c.BodyText, &c.VideoURL, &c.ThumbnailURL,
			&c.IsPublished, &c.PublishedAt, &c.CreatedAt, &c.UpdatedAt, &c.CategoryName,
		)
		if err != nil {
			return nil, err
		}
		contents = append(contents, &c)
	}
	return contents, nil
}

func (r *CustomContentRepository) GetByID(ctx context.Context, id string) (*domain.CustomContent, error) {
	sql := `
		SELECT c.id, c.category_id, c.type, c.title, 
		       COALESCE(c.excerpt, ''), COALESCE(c.body_text, ''), 
		       COALESCE(c.video_url, ''), COALESCE(c.thumbnail_url, ''), 
		       c.is_published, c.published_at, c.created_at, c.updated_at, 
		       COALESCE(cat.name, '') as category_name
		FROM core.custom_contents c
		LEFT JOIN core.content_categories cat ON c.category_id = cat.id
		WHERE c.id = $1
	`
	var c domain.CustomContent
	err := r.db.QueryRow(ctx, sql, id).Scan(
		&c.ID, &c.CategoryID, &c.Type, &c.Title, &c.Excerpt, &c.BodyText, &c.VideoURL, &c.ThumbnailURL,
		&c.IsPublished, &c.PublishedAt, &c.CreatedAt, &c.UpdatedAt, &c.CategoryName,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

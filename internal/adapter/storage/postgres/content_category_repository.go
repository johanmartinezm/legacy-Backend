package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

type ContentCategoryRepository struct {
	db *pgxpool.Pool
}

func NewContentCategoryRepository(db *pgxpool.Pool) *ContentCategoryRepository {
	return &ContentCategoryRepository{db: db}
}

func (r *ContentCategoryRepository) Create(ctx context.Context, cat *domain.ContentCategory) error {
	sql := `
		INSERT INTO core.content_categories (name, slug, description, icon_url, is_active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, sql,
		cat.Name, cat.Slug, cat.Description, cat.IconURL, cat.IsActive,
	).Scan(&cat.ID, &cat.CreatedAt, &cat.UpdatedAt)
}

func (r *ContentCategoryRepository) ListAll(ctx context.Context) ([]*domain.ContentCategory, error) {
	sql := `SELECT id, name, slug, COALESCE(description, ''), COALESCE(icon_url, ''), is_active, created_at, updated_at 
	        FROM core.content_categories ORDER BY name ASC`
	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []*domain.ContentCategory
	for rows.Next() {
		var c domain.ContentCategory
		err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.IconURL, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, err
		}
		categories = append(categories, &c)
	}
	return categories, nil
}

func (r *ContentCategoryRepository) Update(ctx context.Context, cat *domain.ContentCategory) error {
	sql := `
		UPDATE core.content_categories
		SET name = $1, slug = $2, description = $3, icon_url = $4, is_active = $5, updated_at = CURRENT_TIMESTAMP
		WHERE id = $6
	`
	_, err := r.db.Exec(ctx, sql, cat.Name, cat.Slug, cat.Description, cat.IconURL, cat.IsActive, cat.ID)
	return err
}

func (r *ContentCategoryRepository) Delete(ctx context.Context, id string) error {
	sql := `DELETE FROM core.content_categories WHERE id = $1`
	_, err := r.db.Exec(ctx, sql, id)
	return err
}

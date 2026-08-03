package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

type BannerRepository struct {
	db *pgxpool.Pool
}

func NewBannerRepository(db *pgxpool.Pool) *BannerRepository {
	return &BannerRepository{db: db}
}

func (r *BannerRepository) Create(ctx context.Context, banner *domain.Banner) error {
	sql := `
		INSERT INTO core.banners (title, subtitle, category, image_url, action_type, action_target, is_active, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, sql,
		banner.Title, banner.Subtitle, banner.Category, banner.ImageURL,
		banner.ActionType, banner.ActionTarget, banner.IsActive, banner.SortOrder,
	).Scan(&banner.ID, &banner.CreatedAt, &banner.UpdatedAt)
}

func (r *BannerRepository) Update(ctx context.Context, banner *domain.Banner) error {
	sql := `
		UPDATE core.banners
		SET title = $1, subtitle = $2, category = $3, image_url = $4, action_type = $5, action_target = $6, is_active = $7, sort_order = $8, updated_at = CURRENT_TIMESTAMP
		WHERE id = $9
	`
	_, err := r.db.Exec(ctx, sql,
		banner.Title, banner.Subtitle, banner.Category, banner.ImageURL,
		banner.ActionType, banner.ActionTarget, banner.IsActive, banner.SortOrder, banner.ID,
	)
	return err
}

func (r *BannerRepository) Delete(ctx context.Context, id string) error {
	sql := `DELETE FROM core.banners WHERE id = $1`
	_, err := r.db.Exec(ctx, sql, id)
	return err
}

func (r *BannerRepository) GetByID(ctx context.Context, id string) (*domain.Banner, error) {
	sql := `SELECT id, title, subtitle, category, image_url, action_type, action_target, is_active, sort_order, created_at, updated_at FROM core.banners WHERE id = $1`
	var b domain.Banner
	err := r.db.QueryRow(ctx, sql, id).Scan(
		&b.ID, &b.Title, &b.Subtitle, &b.Category, &b.ImageURL, &b.ActionType, &b.ActionTarget, &b.IsActive, &b.SortOrder, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *BannerRepository) ListActive(ctx context.Context, category string) ([]*domain.Banner, error) {
	sql := `
		SELECT id, title, subtitle, category, image_url, action_type, action_target, is_active, sort_order, created_at, updated_at 
		FROM core.banners 
		WHERE is_active = true AND category = $1
		ORDER BY sort_order ASC, created_at DESC
	`
	rows, err := r.db.Query(ctx, sql, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var banners []*domain.Banner
	for rows.Next() {
		var b domain.Banner
		err := rows.Scan(
			&b.ID, &b.Title, &b.Subtitle, &b.Category, &b.ImageURL, &b.ActionType, &b.ActionTarget, &b.IsActive, &b.SortOrder, &b.CreatedAt, &b.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		banners = append(banners, &b)
	}
	return banners, nil
}

func (r *BannerRepository) ListAll(ctx context.Context) ([]*domain.Banner, error) {
	sql := `
		SELECT id, title, subtitle, category, image_url, action_type, action_target, is_active, sort_order, created_at, updated_at 
		FROM core.banners 
		ORDER BY sort_order ASC, created_at DESC
	`
	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var banners []*domain.Banner
	for rows.Next() {
		var b domain.Banner
		err := rows.Scan(
			&b.ID, &b.Title, &b.Subtitle, &b.Category, &b.ImageURL, &b.ActionType, &b.ActionTarget, &b.IsActive, &b.SortOrder, &b.CreatedAt, &b.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		banners = append(banners, &b)
	}
	return banners, nil
}

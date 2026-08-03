package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
)

type StatsRepository struct {
	db *pgxpool.Pool
}

func NewStatsRepository(db *pgxpool.Pool) *StatsRepository {
	return &StatsRepository{db: db}
}

func (r *StatsRepository) GetTopArticles(ctx context.Context, limit int) ([]domain.ArticleStat, error) {
	sql := `
		SELECT 
			v.post_id, 
			COALESCE(MAX(v.title), c.title, 'Contenido Desconocido') as title, 
			COUNT(v.id) as views_count
		FROM core.post_views v
		LEFT JOIN core.custom_contents c ON v.post_id = c.id::text
		GROUP BY v.post_id, c.title
		ORDER BY views_count DESC
		LIMIT $1
	`
	rows, err := r.db.Query(ctx, sql, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []domain.ArticleStat
	for rows.Next() {
		var s domain.ArticleStat
		if err := rows.Scan(&s.ArticleID, &s.Title, &s.Views); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}

func (r *StatsRepository) GetTopUsers(ctx context.Context, limit int) ([]domain.UserStat, error) {
	sql := `
		SELECT v.user_id, u.first_name, u.last_name, COUNT(v.id) as reads_count
		FROM core.post_views v
		JOIN core.users u ON v.user_id = u.id
		GROUP BY v.user_id, u.first_name, u.last_name
		ORDER BY reads_count DESC
		LIMIT $1
	`
	rows, err := r.db.Query(ctx, sql, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []domain.UserStat
	for rows.Next() {
		var s domain.UserStat
		if err := rows.Scan(&s.UserID, &s.FirstName, &s.LastName, &s.Reads); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}

func (r *StatsRepository) GetViewsByPeriod(ctx context.Context, period string) ([]domain.PeriodStats, error) {
	// period can be 'day', 'week', 'month', 'year'
	// We'll use date_trunc for grouping
	sql := fmt.Sprintf(`
		SELECT TO_CHAR(date_trunc('%s', created_at), 'YYYY-MM-DD') as period_label, COUNT(*)
		FROM core.post_views
		GROUP BY period_label
		ORDER BY period_label ASC
		LIMIT 12
	`, period)

	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []domain.PeriodStats
	for rows.Next() {
		var s domain.PeriodStats
		if err := rows.Scan(&s.Period, &s.Views); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}

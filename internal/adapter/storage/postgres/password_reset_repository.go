package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

type PasswordResetRepository struct {
	db *pgxpool.Pool
}

func NewPasswordResetRepository(db *pgxpool.Pool) *PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}

func (r *PasswordResetRepository) StoreToken(ctx context.Context, email, token string) error {
	sql := `
		INSERT INTO core.password_reset_tokens (email, token, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (email) DO UPDATE SET
			token = EXCLUDED.token,
			expires_at = EXCLUDED.expires_at,
			created_at = CURRENT_TIMESTAMP
	`
	expiresAt := time.Now().Add(1 * time.Hour)
	_, err := r.db.Exec(ctx, sql, email, token, expiresAt)
	return err
}

func (r *PasswordResetRepository) GetToken(ctx context.Context, email string) (string, error) {
	sql := `
		SELECT token FROM core.password_reset_tokens
		WHERE email = $1 AND expires_at > $2
	`
	var token string
	err := r.db.QueryRow(ctx, sql, email, time.Now()).Scan(&token)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (r *PasswordResetRepository) DeleteToken(ctx context.Context, email string) error {
	sql := "DELETE FROM core.password_reset_tokens WHERE email = $1"
	_, err := r.db.Exec(ctx, sql, email)
	return err
}

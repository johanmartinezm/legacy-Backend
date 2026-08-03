package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

type EmailVerificationRepository struct {
	db *pgxpool.Pool
}

func NewEmailVerificationRepository(db *pgxpool.Pool) *EmailVerificationRepository {
	return &EmailVerificationRepository{db: db}
}

// StoreToken saves a new email verification token.
func (r *EmailVerificationRepository) StoreToken(ctx context.Context, emailBlindIndex, token string, expiresAt time.Time) error {
	// First, delete any existing token for this email to avoid duplicates
	deleteSql := "DELETE FROM core.email_verification_tokens WHERE email_blind_index = $1"
	_, err := r.db.Exec(ctx, deleteSql, emailBlindIndex)
	if err != nil {
		return err
	}

	insertSql := `
		INSERT INTO core.email_verification_tokens (token, email_blind_index, expires_at)
		VALUES ($1, $2, $3)
	`
	_, err = r.db.Exec(ctx, insertSql, token, emailBlindIndex, expiresAt)
	return err
}

// ValidateToken checks if a token is valid and returns the associated email blind index.
func (r *EmailVerificationRepository) ValidateToken(ctx context.Context, token string) (string, error) {
	sql := `
		SELECT email_blind_index, expires_at 
		FROM core.email_verification_tokens 
		WHERE token = $1
	`
	var emailBlindIndex string
	var expiresAt time.Time

	err := r.db.QueryRow(ctx, sql, token).Scan(&emailBlindIndex, &expiresAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return "", errors.New("invalid or expired token")
		}
		return "", err
	}

	if time.Now().After(expiresAt) {
		// Delete expired token
		_ = r.DeleteToken(ctx, token)
		return "", errors.New("token has expired")
	}

	return emailBlindIndex, nil
}

// DeleteToken removes a used or expired token.
func (r *EmailVerificationRepository) DeleteToken(ctx context.Context, token string) error {
	sql := "DELETE FROM core.email_verification_tokens WHERE token = $1"
	_, err := r.db.Exec(ctx, sql, token)
	return err
}

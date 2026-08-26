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

// GetEmailByToken resuelve a quién pertenece un token de recuperación.
//
// Existe para que el enlace del correo no tenga que llevar la dirección en la
// URL: el token ya identifica la solicitud, y lo que viaja en la barra de
// direcciones acaba en el historial, en la cabecera Referer y en los registros
// de cualquier proxy por el medio.
//
// Se apoya en el índice único de `scripts/20260825_password_reset_token_index.sql`,
// que además garantiza que no haya dos filas con el mismo token.
func (r *PasswordResetRepository) GetEmailByToken(ctx context.Context, token string) (string, error) {
	sql := `
		SELECT email FROM core.password_reset_tokens
		WHERE token = $1 AND expires_at > $2
	`
	var email string
	if err := r.db.QueryRow(ctx, sql, token, time.Now()).Scan(&email); err != nil {
		return "", err
	}
	return email, nil
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

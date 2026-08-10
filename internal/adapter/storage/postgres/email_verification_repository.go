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

// StoreToken guarda un token de verificación para una persona.
//
// La tabla identifica por user_id, no por blind index del correo: es lo que
// declara su clave foránea a core.users, con borrado en cascada. Guardar además
// el correo aquí duplicaría la identidad de la persona en un sitio más, y
// dejaría un rastro que el borrado de cuenta tendría que perseguir.
func (r *EmailVerificationRepository) StoreToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	// Se retira el token anterior: solo debe haber uno vivo por persona, o un
	// enlace viejo de un correo anterior seguiría sirviendo para verificar.
	deleteSql := "DELETE FROM core.email_verification_tokens WHERE user_id = $1"
	_, err := r.db.Exec(ctx, deleteSql, userID)
	if err != nil {
		return err
	}

	insertSql := `
		INSERT INTO core.email_verification_tokens (user_id, token, expires_at)
		VALUES ($1, $2, $3)
	`
	_, err = r.db.Exec(ctx, insertSql, userID, token, expiresAt)
	return err
}

// ValidateToken comprueba el token y devuelve el id de su dueño.
func (r *EmailVerificationRepository) ValidateToken(ctx context.Context, token string) (string, error) {
	sql := `
		SELECT user_id, expires_at
		FROM core.email_verification_tokens
		WHERE token = $1
	`
	var userID string
	var expiresAt time.Time

	err := r.db.QueryRow(ctx, sql, token).Scan(&userID, &expiresAt)
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

	return userID, nil
}

// DeleteToken removes a used or expired token.
func (r *EmailVerificationRepository) DeleteToken(ctx context.Context, token string) error {
	sql := "DELETE FROM core.email_verification_tokens WHERE token = $1"
	_, err := r.db.Exec(ctx, sql, token)
	return err
}

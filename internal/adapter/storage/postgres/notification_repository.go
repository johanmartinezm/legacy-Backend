package postgres

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

type NotificationRepository struct {
	db *pgxpool.Pool
}

func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{db: pool}
}

func (r *NotificationRepository) SaveToken(ctx context.Context, token *domain.FCMToken) error {
	sql := `INSERT INTO core.user_fcm_tokens (user_id, fcm_token, device_type, created_at, updated_at)
            VALUES ($1, $2, $3, now(), now())
            ON CONFLICT (user_id, fcm_token) DO UPDATE 
            SET device_type = $3, updated_at = now()`
	_, err := r.db.Exec(ctx, sql, token.UserID, token.FCMToken, token.DeviceType)
	return err
}

// GetAllTokens devuelve todos los tokens registrados.
//
// Lo usa la suscripción en bloque al tópico "all": los dispositivos que ya
// estaban registrados antes de que existiera la suscripción automática no se
// suscribirían nunca por sí solos, porque eso solo ocurre al registrar el token
// y esos usuarios ya lo hicieron.
func (r *NotificationRepository) GetAllTokens(ctx context.Context) ([]*domain.FCMToken, error) {
	sql := `SELECT user_id, fcm_token, device_type, created_at, updated_at FROM core.user_fcm_tokens`
	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tokens := make([]*domain.FCMToken, 0)
	for rows.Next() {
		var t domain.FCMToken
		if err := rows.Scan(&t.UserID, &t.FCMToken, &t.DeviceType, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, &t)
	}
	return tokens, rows.Err()
}

func (r *NotificationRepository) GetTokensByUserID(ctx context.Context, userID string) ([]*domain.FCMToken, error) {
	sql := `SELECT user_id, fcm_token, device_type, created_at, updated_at FROM core.user_fcm_tokens WHERE user_id = $1`
	rows, err := r.db.Query(ctx, sql, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tokens := make([]*domain.FCMToken, 0)
	for rows.Next() {
		var t domain.FCMToken
		if err := rows.Scan(&t.UserID, &t.FCMToken, &t.DeviceType, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, &t)
	}
	return tokens, nil
}

func (r *NotificationRepository) DeleteToken(ctx context.Context, userID, token string) error {
	sql := `DELETE FROM core.user_fcm_tokens WHERE user_id = $1 AND fcm_token = $2`
	_, err := r.db.Exec(ctx, sql, userID, token)
	return err
}

func (r *NotificationRepository) SaveHistory(ctx context.Context, h *domain.NotificationHistory) error {
	sql := `INSERT INTO core.notifications_history (id, admin_id, title, body, target_type, target_value, sent_at, status)
            VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, now(), $6)
            RETURNING id, sent_at`
	var adminID *string
	if h.AdminID != "" {
		adminID = &h.AdminID
	}
	err := r.db.QueryRow(ctx, sql, adminID, h.Title, h.Body, h.TargetType, h.TargetValue, h.Status).Scan(&h.ID, &h.SentAt)
	return err
}

func (r *NotificationRepository) GetHistory(ctx context.Context, limit, offset int) ([]*domain.NotificationHistory, error) {
	sql := `SELECT id, COALESCE(admin_id::text, ''), title, body, target_type, COALESCE(target_value, ''), sent_at, status 
            FROM core.notifications_history 
            ORDER BY sent_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(ctx, sql, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	histories := make([]*domain.NotificationHistory, 0)
	for rows.Next() {
		var h domain.NotificationHistory
		if err := rows.Scan(&h.ID, &h.AdminID, &h.Title, &h.Body, &h.TargetType, &h.TargetValue, &h.SentAt, &h.Status); err != nil {
			return nil, err
		}
		histories = append(histories, &h)
	}
	return histories, nil
}

var _ ports.NotificationRepository = (*NotificationRepository)(nil)

package ports

import (
	"applegacy/backend/internal/core/domain"
	"context"
)

type NotificationRepository interface {
	SaveToken(ctx context.Context, token *domain.FCMToken) error
	GetTokensByUserID(ctx context.Context, userID string) ([]*domain.FCMToken, error)
	DeleteToken(ctx context.Context, userID, token string) error
	
	SaveHistory(ctx context.Context, history *domain.NotificationHistory) error
	GetHistory(ctx context.Context, limit, offset int) ([]*domain.NotificationHistory, error)
}

type NotificationService interface {
	RegisterToken(ctx context.Context, userID, token, deviceType string) error
	SendNotification(ctx context.Context, adminID, title, body, targetType, targetValue string, data map[string]string) error
	GetHistory(ctx context.Context, limit, offset int) ([]*domain.NotificationHistory, error)
}

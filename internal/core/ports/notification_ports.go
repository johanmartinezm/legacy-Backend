package ports

import (
	"applegacy/backend/internal/core/domain"
	"context"
)

type NotificationRepository interface {
	SaveToken(ctx context.Context, token *domain.FCMToken) error
	GetTokensByUserID(ctx context.Context, userID string) ([]*domain.FCMToken, error)
	// GetAllTokens alimenta la suscripción en bloque al tópico "all".
	GetAllTokens(ctx context.Context) ([]*domain.FCMToken, error)
	DeleteToken(ctx context.Context, userID, token string) error

	SaveHistory(ctx context.Context, history *domain.NotificationHistory) error
	GetHistory(ctx context.Context, limit, offset int) ([]*domain.NotificationHistory, error)
}

// PushSender es lo que el servicio necesita de la pasarela de notificaciones.
//
// El servicio dependía del tipo concreto *firebase.FCMClient, lo que impedía
// probar qué hace ante un fallo de envío —justo la parte delicada, porque de
// ahí depende que un token se borre o se conserve—.
type PushSender interface {
	SendToToken(ctx context.Context, token, title, body string, data map[string]string) (string, error)
	SendToTopic(ctx context.Context, topic, title, body string, data map[string]string) (string, error)
	SubscribeToTopic(ctx context.Context, tokens []string, topic string) (int, error)
}

type NotificationService interface {
	RegisterToken(ctx context.Context, userID, token, deviceType string) error
	SendNotification(ctx context.Context, adminID, title, body, targetType, targetValue string, data map[string]string) error
	// SendToUser avisa a los dispositivos de una persona por algo que ocurrió
	// dentro de la app, no por un envío del panel.
	//
	// Se separa de SendNotification porque aquella deja constancia en el
	// historial de notificaciones, que es la bitácora de lo que ha mandado un
	// administrador: un mensaje de chat no lo manda nadie del panel y anotarlo
	// ahí llenaría la pantalla de historial con miles de filas ajenas.
	SendToUser(ctx context.Context, userID, title, body string, data map[string]string) error
	GetHistory(ctx context.Context, limit, offset int) ([]*domain.NotificationHistory, error)
	// SubscribeAllToTopic suscribe al tópico general los tokens ya registrados.
	// Devuelve cuántos quedaron suscritos.
	SubscribeAllToTopic(ctx context.Context) (int, error)
}

package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"applegacy/backend/internal/infrastructure/firebase"
	"context"
	"errors"
	"fmt"
)

type NotificationService struct {
	repo      ports.NotificationRepository
	fcmClient *firebase.FCMClient
}

func NewNotificationService(repo ports.NotificationRepository, fcmClient *firebase.FCMClient) *NotificationService {
	return &NotificationService{
		repo:      repo,
		fcmClient: fcmClient,
	}
}

// TopicTodos es el tópico al que se envían los avisos generales: el destino
// "all" del panel y las novedades automáticas. Los dispositivos se suscriben
// desde el servidor al registrar su token, porque la app no lo hace.
const TopicTodos = "all"

func (s *NotificationService) RegisterToken(ctx context.Context, userID, token, deviceType string) error {
	if userID == "" || token == "" || deviceType == "" {
		return errors.New("userID, token y deviceType no pueden estar vacíos")
	}

	t := &domain.FCMToken{
		UserID:     userID,
		FCMToken:   token,
		DeviceType: deviceType,
	}

	if err := s.repo.SaveToken(ctx, t); err != nil {
		return err
	}

	// Suscripción al tópico general. La app registra el token y nada más: sin
	// esto, el dispositivo queda guardado pero los envíos a "todos" no le
	// llegan, que es exactamente lo que pasaba hasta ahora.
	//
	// Un fallo aquí no invalida el registro del token: el dispositivo queda
	// guardado y puede recibir envíos dirigidos a él, y la suscripción se puede
	// reintentar en bloque con SubscribeAllToTopic.
	if _, err := s.fcmClient.SubscribeToTopic(ctx, []string{token}, TopicTodos); err != nil {
		fmt.Printf("[FCM] no se pudo suscribir el token del usuario %s al tópico %s: %v\n", userID, TopicTodos, err)
	}

	return nil
}

// SubscribeAllToTopic suscribe al tópico general todos los tokens registrados.
//
// Hace falta una vez: los dispositivos que ya estaban registrados antes de que
// existiera la suscripción automática no se suscribirían solos, porque eso solo
// ocurre al registrar el token y esos usuarios ya lo hicieron. Después queda
// disponible para reparar suscripciones perdidas sin tocar la base.
//
// Es idempotente: suscribir un token ya suscrito no tiene efecto ni error.
func (s *NotificationService) SubscribeAllToTopic(ctx context.Context) (int, error) {
	tokens, err := s.repo.GetAllTokens(ctx)
	if err != nil {
		return 0, fmt.Errorf("no se pudieron leer los tokens: %w", err)
	}
	if len(tokens) == 0 {
		return 0, nil
	}

	valores := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t.FCMToken != "" {
			valores = append(valores, t.FCMToken)
		}
	}

	return s.fcmClient.SubscribeToTopic(ctx, valores, TopicTodos)
}

func (s *NotificationService) SendNotification(ctx context.Context, adminID, title, body, targetType, targetValue string, data map[string]string) error {
	if title == "" || body == "" {
		return errors.New("el título y cuerpo del mensaje son requeridos")
	}

	var err error
	var status = "sent"

	switch targetType {
	case "all":
		_, err = s.fcmClient.SendToTopic(ctx, "all", title, body, data)
	case "group":
		if targetValue == "" {
			return errors.New("el nombre de grupo es requerido para envíos por grupo")
		}
		topicName := fmt.Sprintf("group_%s", targetValue)
		_, err = s.fcmClient.SendToTopic(ctx, topicName, title, body, data)
	case "user":
		if targetValue == "" {
			return errors.New("el ID del usuario de destino es requerido")
		}
		tokens, fetchErr := s.repo.GetTokensByUserID(ctx, targetValue)
		if fetchErr != nil {
			err = fetchErr
			status = "failed"
			break
		}
		if len(tokens) == 0 {
			err = fmt.Errorf("no se encontraron dispositivos registrados para el usuario %s", targetValue)
			status = "failed"
			break
		}

		// Enviar a todos los dispositivos del usuario
		var sendErrors []error
		for _, t := range tokens {
			_, fcmErr := s.fcmClient.SendToToken(ctx, t.FCMToken, title, body, data)
			if fcmErr != nil {
				sendErrors = append(sendErrors, fcmErr)
				// Si un token ya no es válido, se puede eliminar de la BD aquí
				_ = s.repo.DeleteToken(ctx, targetValue, t.FCMToken)
			}
		}
		if len(sendErrors) == len(tokens) {
			err = fmt.Errorf("fallaron todos los envíos a los tokens del usuario: %v", sendErrors)
			status = "failed"
		}
	default:
		return fmt.Errorf("tipo de destino no soportado: %s", targetType)
	}

	// Guardar el registro en el historial de la BD
	history := &domain.NotificationHistory{
		AdminID:     adminID,
		Title:       title,
		Body:        body,
		TargetType:  targetType,
		TargetValue: targetValue,
		Status:      status,
	}

	saveErr := s.repo.SaveHistory(ctx, history)
	if saveErr != nil {
		// Loguear pero no interrumpir la respuesta si ya se intentó enviar por FCM
		fmt.Printf("Error al guardar historial de notificación: %v\n", saveErr)
	}

	return err
}

func (s *NotificationService) GetHistory(ctx context.Context, limit, offset int) ([]*domain.NotificationHistory, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.repo.GetHistory(ctx, limit, offset)
}

var _ ports.NotificationService = (*NotificationService)(nil)

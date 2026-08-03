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

func (s *NotificationService) RegisterToken(ctx context.Context, userID, token, deviceType string) error {
	if userID == "" || token == "" || deviceType == "" {
		return errors.New("userID, token y deviceType no pueden estar vacíos")
	}

	t := &domain.FCMToken{
		UserID:     userID,
		FCMToken:   token,
		DeviceType: deviceType,
	}

	return s.repo.SaveToken(ctx, t)
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

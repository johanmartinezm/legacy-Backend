package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"applegacy/backend/internal/security"
	"context"
	"errors"
)

type ChatService struct {
	repo      ports.ChatRepository
	userRepo  ports.UserRepository
	blockRepo ports.BlockRepository
	crypto    *security.CryptoService
	// notifier manda la push al destinatario de cada mensaje. Puede ser nil
	// —así lo dejan los tests—: entonces el chat funciona igual, solo que sin
	// avisar.
	notifier ports.NotificationService
}

func NewChatService(repo ports.ChatRepository, userRepo ports.UserRepository, blockRepo ports.BlockRepository, crypto *security.CryptoService, notifier ports.NotificationService) *ChatService {
	return &ChatService{
		repo:      repo,
		userRepo:  userRepo,
		blockRepo: blockRepo,
		crypto:    crypto,
		notifier:  notifier,
	}
}

// errBloqueado es el mismo mensaje para las dos direcciones del bloqueo, a
// propósito: si dijera "te han bloqueado" estaría revelando una decisión de la
// otra persona, y quien busca acosar sabría que debe cambiar de cuenta.
//
// Se mudó a domain el 2026-08-18 para que el handler pueda reconocerlo y
// responder 403 en vez de 500. Aquí queda el nombre corto, que es el que usan
// los tres sitios que lo devuelven.
var errBloqueado = domain.ErrBloqueado

func (s *ChatService) SendInvite(ctx context.Context, requesterID, receiverID string) error {
	if requesterID == receiverID {
		return errors.New("cannot invite yourself")
	}

	bloqueado, err := s.blockRepo.AreBlocked(ctx, requesterID, receiverID)
	if err != nil {
		return err
	}
	if bloqueado {
		return errBloqueado
	}

	// Check if a connection already exists
	existing, _ := s.repo.FindConnectionBetweenUsers(ctx, requesterID, receiverID)
	if existing != nil {
		return errors.New("connection already exists or is pending")
	}

	return s.repo.CreateConnection(ctx, requesterID, receiverID)
}

func (s *ChatService) AcceptInvite(ctx context.Context, connectionID, userID string) error {
	conn, err := s.repo.GetConnection(ctx, connectionID)
	if err != nil {
		return err
	}

	if conn.ReceiverID != userID {
		return errors.New("only the receiver can accept the invitation")
	}

	if conn.Status != domain.StatusPending {
		return errors.New("invitation is not pending")
	}

	return s.repo.UpdateConnectionStatus(ctx, connectionID, domain.StatusAccepted)
}

func (s *ChatService) RejectInvite(ctx context.Context, connectionID, userID string) error {
	conn, err := s.repo.GetConnection(ctx, connectionID)
	if err != nil {
		return err
	}

	if conn.ReceiverID != userID {
		return errors.New("only the receiver can reject the invitation")
	}

	return s.repo.UpdateConnectionStatus(ctx, connectionID, domain.StatusRejected)
}

func (s *ChatService) ListMyConnections(ctx context.Context, userID string) ([]*domain.ChatConnection, error) {
	conns, err := s.repo.ListConnections(ctx, userID)
	if err != nil {
		return nil, err
	}

	for _, conn := range conns {
		otherUserID := conn.RequesterID
		if conn.RequesterID == userID {
			otherUserID = conn.ReceiverID
		}

		otherUser, err := s.userRepo.FindByID(ctx, otherUserID)
		if err == nil && otherUser != nil {
			// Decrypt basic info for UI
			if dec, err := s.crypto.Decrypt(otherUser.FirstName); err == nil {
				otherUser.FirstName = dec
			}
			if dec, err := s.crypto.Decrypt(otherUser.LastName); err == nil {
				otherUser.LastName = dec
			}
			if dec, err := s.crypto.Decrypt(otherUser.CompanyName); err == nil {
				otherUser.CompanyName = dec
			}
			if dec, err := s.crypto.Decrypt(otherUser.JobTitle); err == nil {
				otherUser.JobTitle = dec
			}
			conn.OtherUser = otherUser
		}
	}

	return conns, nil
}

func (s *ChatService) GetChatHistory(ctx context.Context, connectionID, userID string, limit, offset int) ([]*domain.Message, error) {
	// Verify user belongs to connection
	conn, err := s.repo.GetConnection(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	if conn.RequesterID != userID && conn.ReceiverID != userID {
		return nil, errors.New("unauthorized to view these messages")
	}

	// La conversación desaparece de la lista al bloquear, pero el id sigue
	// siendo válido: sin este guarda se podría seguir leyendo pidiéndolo
	// directamente.
	bloqueado, err := s.blockRepo.AreBlocked(ctx, conn.RequesterID, conn.ReceiverID)
	if err != nil {
		return nil, err
	}
	if bloqueado {
		return nil, errBloqueado
	}

	messages, err := s.repo.GetMessages(ctx, connectionID, limit, offset)
	if err != nil {
		return nil, err
	}

	// Decrypt messages
	for _, m := range messages {
		decrypted, _ := s.crypto.Decrypt(m.ContentEncrypted)
		m.ContentEncrypted = decrypted
	}

	// Mark as read
	_ = s.repo.MarkAsRead(ctx, connectionID, userID)

	return messages, nil
}

func (s *ChatService) SendMessage(ctx context.Context, senderID, connectionID, content string) (*domain.Message, error) {
	// Verify connection is accepted and user belongs to it
	conn, err := s.repo.GetConnection(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	if conn.Status != domain.StatusAccepted {
		return nil, domain.ErrConexionNoAceptada
	}

	if conn.RequesterID != senderID && conn.ReceiverID != senderID {
		return nil, domain.ErrNoEsDeLaConversacion
	}

	// El guarda va aquí y no solo en la lista de conversaciones: quien tenga la
	// pantalla de chat abierta cuando le bloquean conserva el connectionID y
	// podría seguir enviando. Esconder la conversación no basta.
	bloqueado, err := s.blockRepo.AreBlocked(ctx, conn.RequesterID, conn.ReceiverID)
	if err != nil {
		return nil, err
	}
	if bloqueado {
		return nil, errBloqueado
	}

	encrypted, err := s.crypto.Encrypt(content)
	if err != nil {
		return nil, err
	}

	msg := &domain.Message{
		ConnectionID:     connectionID,
		SenderID:         senderID,
		ContentEncrypted: encrypted,
	}

	if err := s.repo.SaveMessage(ctx, msg); err != nil {
		return nil, err
	}

	// El aviso va después de guardar y nunca antes: si la push saliera primero,
	// un fallo al escribir dejaría a alguien abriendo la app para leer un
	// mensaje que no existe.
	s.avisarMensajeNuevo(conn, connectionID, senderID, content)

	// For the speaker's convenience, return the message with unencrypted content
	msg.ContentEncrypted = content

	return msg, nil
}

// ListMembers devuelve el directorio tal como lo ve viewerID: sin él mismo, sin
// cuentas eliminadas y sin las personas con las que hay un bloqueo.
func (s *ChatService) ListMembers(ctx context.Context, viewerID string) ([]*domain.User, error) {
	users, err := s.repo.ListMembers(ctx, viewerID)
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		if dec, err := s.crypto.Decrypt(u.FirstName); err == nil {
			u.FirstName = dec
		}
		if dec, err := s.crypto.Decrypt(u.LastName); err == nil {
			u.LastName = dec
		}
		if dec, err := s.crypto.Decrypt(u.CompanyName); err == nil {
			u.CompanyName = dec
		}
		if dec, err := s.crypto.Decrypt(u.JobTitle); err == nil {
			u.JobTitle = dec
		}
	}

	return users, nil
}

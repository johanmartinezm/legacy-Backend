package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"applegacy/backend/internal/security"
	"context"
	"errors"
)

type ChatService struct {
	repo     ports.ChatRepository
	userRepo ports.UserRepository
	crypto   *security.CryptoService
}

func NewChatService(repo ports.ChatRepository, userRepo ports.UserRepository, crypto *security.CryptoService) *ChatService {
	return &ChatService{
		repo:     repo,
		userRepo: userRepo,
		crypto:   crypto,
	}
}

func (s *ChatService) SendInvite(ctx context.Context, requesterID, receiverID string) error {
	if requesterID == receiverID {
		return errors.New("cannot invite yourself")
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
		return nil, errors.New("cannot send messages to an unaccepted connection")
	}

	if conn.RequesterID != senderID && conn.ReceiverID != senderID {
		return nil, errors.New("unauthorized to send messages to this connection")
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

	// For the speaker's convenience, return the message with unencrypted content
	msg.ContentEncrypted = content

	return msg, nil
}

func (s *ChatService) ListMembers(ctx context.Context) ([]*domain.User, error) {
	users, err := s.repo.ListMembers(ctx)
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

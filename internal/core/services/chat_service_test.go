package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/security"
	"context"
	"testing"
)

type MockChatRepository struct {
	CreateConnectionFunc           func(ctx context.Context, requesterID, receiverID string) error
	UpdateConnectionStatusFunc     func(ctx context.Context, connectionID string, status domain.ConnectionStatus) error
	GetConnectionFunc              func(ctx context.Context, connectionID string) (*domain.ChatConnection, error)
	FindConnectionBetweenUsersFunc func(ctx context.Context, user1, user2 string) (*domain.ChatConnection, error)
	ListConnectionsFunc            func(ctx context.Context, userID string) ([]*domain.ChatConnection, error)
	SaveMessageFunc                func(ctx context.Context, msg *domain.Message) error
	GetMessagesFunc                func(ctx context.Context, connectionID string, limit, offset int) ([]*domain.Message, error)
	MarkAsReadFunc                 func(ctx context.Context, connectionID, userID string) error
	ListMembersFunc                func(ctx context.Context) ([]*domain.User, error)
}

func (m *MockChatRepository) CreateConnection(ctx context.Context, reqID, resID string) error {
	return m.CreateConnectionFunc(ctx, reqID, resID)
}
func (m *MockChatRepository) UpdateConnectionStatus(ctx context.Context, id string, s domain.ConnectionStatus) error {
	return m.UpdateConnectionStatusFunc(ctx, id, s)
}
func (m *MockChatRepository) GetConnection(ctx context.Context, id string) (*domain.ChatConnection, error) {
	return m.GetConnectionFunc(ctx, id)
}
func (m *MockChatRepository) FindConnectionBetweenUsers(ctx context.Context, u1, u2 string) (*domain.ChatConnection, error) {
	return m.FindConnectionBetweenUsersFunc(ctx, u1, u2)
}
func (m *MockChatRepository) ListConnections(ctx context.Context, uID string) ([]*domain.ChatConnection, error) {
	return m.ListConnectionsFunc(ctx, uID)
}
func (m *MockChatRepository) SaveMessage(ctx context.Context, msg *domain.Message) error {
	return m.SaveMessageFunc(ctx, msg)
}
func (m *MockChatRepository) GetMessages(ctx context.Context, id string, l, o int) ([]*domain.Message, error) {
	return m.GetMessagesFunc(ctx, id, l, o)
}
func (m *MockChatRepository) MarkAsRead(ctx context.Context, id, uID string) error {
	return m.MarkAsReadFunc(ctx, id, uID)
}
func (m *MockChatRepository) ListMembers(ctx context.Context) ([]*domain.User, error) {
	return m.ListMembersFunc(ctx)
}

func TestChatService_SendInvite(t *testing.T) {
	crypto, _ := security.NewCryptoService("12345678901234567890123456789012")
	ctx := context.Background()

	t.Run("Should create a new connection if none exists", func(t *testing.T) {
		mockRepo := &MockChatRepository{
			FindConnectionBetweenUsersFunc: func(ctx context.Context, u1, u2 string) (*domain.ChatConnection, error) {
				return nil, nil // No connection
			},
			CreateConnectionFunc: func(ctx context.Context, u1, u2 string) error {
				return nil
			},
		}

		service := NewChatService(mockRepo, nil, crypto)
		err := service.SendInvite(ctx, "user1", "user2")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Should return error if connection already exists", func(t *testing.T) {
		mockRepo := &MockChatRepository{
			FindConnectionBetweenUsersFunc: func(ctx context.Context, u1, u2 string) (*domain.ChatConnection, error) {
				return &domain.ChatConnection{ID: "existing-id"}, nil
			},
		}

		service := NewChatService(mockRepo, nil, crypto)
		err := service.SendInvite(ctx, "user1", "user2")
		if err == nil {
			t.Error("Expected error for existing connection, got nil")
		}
	})
}

func TestChatService_SendMessage(t *testing.T) {
	crypto, _ := security.NewCryptoService("12345678901234567890123456789012")
	ctx := context.Background()

	t.Run("Should fail if connection is not accepted", func(t *testing.T) {
		mockRepo := &MockChatRepository{
			GetConnectionFunc: func(ctx context.Context, id string) (*domain.ChatConnection, error) {
				return &domain.ChatConnection{ID: id, Status: domain.StatusPending}, nil
			},
		}

		service := NewChatService(mockRepo, nil, crypto)
		_, err := service.SendMessage(ctx, "user1", "conn1", "hello")
		if err == nil {
			t.Error("Expected error because connection is pending, got nil")
		}
	})

	t.Run("Should successfully send and encrypt message if accepted", func(t *testing.T) {
		mockRepo := &MockChatRepository{
			GetConnectionFunc: func(ctx context.Context, id string) (*domain.ChatConnection, error) {
				return &domain.ChatConnection{ID: id, Status: domain.StatusAccepted, RequesterID: "user1", ReceiverID: "user2"}, nil
			},
			SaveMessageFunc: func(ctx context.Context, msg *domain.Message) error {
				if msg.ContentEncrypted == "hello" {
					t.Error("Message content should be encrypted")
				}
				return nil
			},
		}

		service := NewChatService(mockRepo, nil, crypto)
		msg, err := service.SendMessage(ctx, "user1", "conn1", "hello")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if msg.ContentEncrypted != "hello" {
			t.Errorf("Expected decrypted content in response for UI convenience, got %s", msg.ContentEncrypted)
		}
	})
}

func TestChatService_AcceptInvite(t *testing.T) {
	crypto, _ := security.NewCryptoService("12345678901234567890123456789012")
	ctx := context.Background()

	t.Run("Should fail if user is not the receiver", func(t *testing.T) {
		mockRepo := &MockChatRepository{
			GetConnectionFunc: func(ctx context.Context, id string) (*domain.ChatConnection, error) {
				return &domain.ChatConnection{ID: id, RequesterID: "user1", ReceiverID: "user2", Status: domain.StatusPending}, nil
			},
		}

		service := NewChatService(mockRepo, nil, crypto)
		err := service.AcceptInvite(ctx, "conn1", "user1") // user1 is requester, cannot accept
		if err == nil {
			t.Error("Expected error because requester tried to accept, got nil")
		}
	})

	t.Run("Should succeed if user is the receiver", func(t *testing.T) {
		mockRepo := &MockChatRepository{
			GetConnectionFunc: func(ctx context.Context, id string) (*domain.ChatConnection, error) {
				return &domain.ChatConnection{ID: id, RequesterID: "user1", ReceiverID: "user2", Status: domain.StatusPending}, nil
			},
			UpdateConnectionStatusFunc: func(ctx context.Context, id string, s domain.ConnectionStatus) error {
				if s != domain.StatusAccepted {
					t.Errorf("Expected status ACCEPTED, got %s", s)
				}
				return nil
			},
		}

		service := NewChatService(mockRepo, nil, crypto)
		err := service.AcceptInvite(ctx, "conn1", "user2")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

func TestChatService_ListMembers(t *testing.T) {
	crypto, _ := security.NewCryptoService("12345678901234567890123456789012")
	ctx := context.Background()

	t.Run("Should decrypt user fields", func(t *testing.T) {
		encryptedName, _ := crypto.Encrypt("John")
		mockRepo := &MockChatRepository{
			ListMembersFunc: func(ctx context.Context) ([]*domain.User, error) {
				return []*domain.User{
					{ID: "u1", FirstName: encryptedName},
				}, nil
			},
		}

		service := NewChatService(mockRepo, nil, crypto)
		users, err := service.ListMembers(ctx)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if users[0].FirstName != "John" {
			t.Errorf("Expected John, got %s", users[0].FirstName)
		}
	})
}

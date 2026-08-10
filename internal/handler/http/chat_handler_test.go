package http

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type MockChatService struct {
	SendInviteFunc        func(ctx context.Context, requesterID, receiverID string) error
	AcceptInviteFunc      func(ctx context.Context, connectionID, userID string) error
	RejectInviteFunc      func(ctx context.Context, connectionID, userID string) error
	ListMyConnectionsFunc func(ctx context.Context, userID string) ([]*domain.ChatConnection, error)
	GetChatHistoryFunc    func(ctx context.Context, connectionID, userID string, limit, offset int) ([]*domain.Message, error)
	SendMessageFunc       func(ctx context.Context, senderID, connectionID, content string) (*domain.Message, error)
	ListMembersFunc       func(ctx context.Context, viewerID string) ([]*domain.User, error)
}

func (m *MockChatService) SendInvite(ctx context.Context, reqID, resID string) error {
	return m.SendInviteFunc(ctx, reqID, resID)
}
func (m *MockChatService) AcceptInvite(ctx context.Context, id, uID string) error {
	return m.AcceptInviteFunc(ctx, id, uID)
}
func (m *MockChatService) RejectInvite(ctx context.Context, id, uID string) error {
	return m.RejectInviteFunc(ctx, id, uID)
}
func (m *MockChatService) ListMyConnections(ctx context.Context, uID string) ([]*domain.ChatConnection, error) {
	return m.ListMyConnectionsFunc(ctx, uID)
}
func (m *MockChatService) GetChatHistory(ctx context.Context, id, uID string, l, o int) ([]*domain.Message, error) {
	return m.GetChatHistoryFunc(ctx, id, uID, l, o)
}
func (m *MockChatService) SendMessage(ctx context.Context, sID, id, c string) (*domain.Message, error) {
	return m.SendMessageFunc(ctx, sID, id, c)
}
func (m *MockChatService) ListMembers(ctx context.Context, viewerID string) ([]*domain.User, error) {
	return m.ListMembersFunc(ctx, viewerID)
}

func TestChatHandler_ListConnections(t *testing.T) {
	mockService := &MockChatService{
		ListMyConnectionsFunc: func(ctx context.Context, userID string) ([]*domain.ChatConnection, error) {
			return []*domain.ChatConnection{
				{ID: "conn1", RequesterID: userID, ReceiverID: "other"},
			}, nil
		},
	}

	handler := NewChatHandler(mockService, nil)

	req, _ := http.NewRequest("GET", "/api/chat/connections", nil)
	// Inject user ID into context (as AuthMiddleware would)
	ctx := context.WithValue(req.Context(), UserIDKey, "user123")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ListConnections(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var conns []*domain.ChatConnection
	json.NewDecoder(rr.Body).Decode(&conns)
	if len(conns) != 1 || conns[0].ID != "conn1" {
		t.Errorf("handler returned unexpected body: %v", rr.Body.String())
	}
}

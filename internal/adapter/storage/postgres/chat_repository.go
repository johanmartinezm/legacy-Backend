package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

type ChatRepository struct {
	db *pgxpool.Pool
}

func NewChatRepository(db *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) CreateConnection(ctx context.Context, requesterID, receiverID string) error {
	sql := `
		INSERT INTO chat.connections (requester_id, receiver_id, status)
		VALUES ($1, $2, 'PENDING')
		ON CONFLICT (requester_id, receiver_id) DO NOTHING
	`
	_, err := r.db.Exec(ctx, sql, requesterID, receiverID)
	return err
}

func (r *ChatRepository) UpdateConnectionStatus(ctx context.Context, connectionID string, status domain.ConnectionStatus) error {
	sql := `
		UPDATE chat.connections 
		SET status = $1, updated_at = $2 
		WHERE id = $3
	`
	_, err := r.db.Exec(ctx, sql, status, time.Now(), connectionID)
	return err
}

func (r *ChatRepository) GetConnection(ctx context.Context, connectionID string) (*domain.ChatConnection, error) {
	sql := `
		SELECT id, requester_id, receiver_id, status, created_at, updated_at
		FROM chat.connections
		WHERE id = $1
	`
	var conn domain.ChatConnection
	err := r.db.QueryRow(ctx, sql, connectionID).Scan(
		&conn.ID,
		&conn.RequesterID,
		&conn.ReceiverID,
		&conn.Status,
		&conn.CreatedAt,
		&conn.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &conn, nil
}

func (r *ChatRepository) FindConnectionBetweenUsers(ctx context.Context, user1, user2 string) (*domain.ChatConnection, error) {
	sql := `
		SELECT id, requester_id, receiver_id, status, created_at, updated_at
		FROM chat.connections
		WHERE (requester_id = $1 AND receiver_id = $2) OR (requester_id = $2 AND receiver_id = $1)
	`
	var conn domain.ChatConnection
	err := r.db.QueryRow(ctx, sql, user1, user2).Scan(
		&conn.ID,
		&conn.RequesterID,
		&conn.ReceiverID,
		&conn.Status,
		&conn.CreatedAt,
		&conn.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &conn, nil
}

func (r *ChatRepository) ListConnections(ctx context.Context, userID string) ([]*domain.ChatConnection, error) {
	sql := `
		SELECT 
			c.id, c.requester_id, c.receiver_id, c.status, c.created_at, c.updated_at,
			(SELECT COUNT(*) FROM chat.messages m WHERE m.connection_id = c.id AND m.is_read = FALSE AND m.sender_id != $1) as unread_count
		FROM chat.connections c
		WHERE c.requester_id = $1 OR c.receiver_id = $1
		ORDER BY c.updated_at DESC
	`
	rows, err := r.db.Query(ctx, sql, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var connections []*domain.ChatConnection
	for rows.Next() {
		var conn domain.ChatConnection
		err := rows.Scan(
			&conn.ID,
			&conn.RequesterID,
			&conn.ReceiverID,
			&conn.Status,
			&conn.CreatedAt,
			&conn.UpdatedAt,
			&conn.UnreadCount,
		)
		if err != nil {
			return nil, err
		}
		connections = append(connections, &conn)
	}
	return connections, nil
}

func (r *ChatRepository) SaveMessage(ctx context.Context, msg *domain.Message) error {
	sql := `
		INSERT INTO chat.messages (connection_id, sender_id, content_encrypted)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, sql, msg.ConnectionID, msg.SenderID, msg.ContentEncrypted).Scan(&msg.ID, &msg.CreatedAt)
	if err != nil {
		return err
	}

	// Update connection updated_at
	_, _ = r.db.Exec(ctx, "UPDATE chat.connections SET updated_at = $1 WHERE id = $2", time.Now(), msg.ConnectionID)

	return nil
}

func (r *ChatRepository) GetMessages(ctx context.Context, connectionID string, limit, offset int) ([]*domain.Message, error) {
	sql := `
		SELECT id, connection_id, sender_id, content_encrypted, is_read, created_at
		FROM chat.messages
		WHERE connection_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, sql, connectionID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*domain.Message
	for rows.Next() {
		var msg domain.Message
		err := rows.Scan(
			&msg.ID,
			&msg.ConnectionID,
			&msg.SenderID,
			&msg.ContentEncrypted,
			&msg.IsRead,
			&msg.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, &msg)
	}
	return messages, nil
}

func (r *ChatRepository) MarkAsRead(ctx context.Context, connectionID, userID string) error {
	sql := `
		UPDATE chat.messages 
		SET is_read = TRUE 
		WHERE connection_id = $1 AND sender_id != $2 AND is_read = FALSE
	`
	_, err := r.db.Exec(ctx, sql, connectionID, userID)
	return err
}

func (r *ChatRepository) ListMembers(ctx context.Context) ([]*domain.User, error) {
	// For now, list all users. In a real scenario, we would filter by 'community' status/role
	sql := `
		SELECT id, first_name, last_name, email_encrypted, company_name, job_title
		FROM core.users
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var u domain.User
		err := rows.Scan(
			&u.ID,
			&u.FirstName,
			&u.LastName,
			&u.EmailEncrypted,
			&u.CompanyName,
			&u.JobTitle,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, nil
}

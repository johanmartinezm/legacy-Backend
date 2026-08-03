package postgres

import (
	"context"
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
)

type transactionRepository struct {
	db *pgxpool.Pool
}

func NewTransactionRepository(db *pgxpool.Pool) ports.TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) CreateTransaction(ctx context.Context, tx *domain.Transaction) error {
	query := `
		INSERT INTO core.transactions (id, user_id, reference_type, reference_id, amount, credibanco_order_id, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query,
		tx.ID,
		tx.UserID,
		tx.ReferenceType,
		tx.ReferenceID,
		tx.Amount,
		tx.CredibancoOrderID,
		tx.Status,
	)
	return err
}

func (r *transactionRepository) GetTransactionByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	query := `
		SELECT id, user_id, reference_type, reference_id, amount, credibanco_order_id, status, created_at, updated_at
		FROM core.transactions
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, id)
	var tx domain.Transaction
	err := row.Scan(
		&tx.ID,
		&tx.UserID,
		&tx.ReferenceType,
		&tx.ReferenceID,
		&tx.Amount,
		&tx.CredibancoOrderID,
		&tx.Status,
		&tx.CreatedAt,
		&tx.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func (r *transactionRepository) UpdateTransactionStatus(ctx context.Context, id uuid.UUID, status domain.TransactionStatus, credibancoOrderID string) error {
	query := `
		UPDATE core.transactions
		SET status = $1, credibanco_order_id = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
	`
	_, err := r.db.Exec(ctx, query, status, credibancoOrderID, id)
	return err
}

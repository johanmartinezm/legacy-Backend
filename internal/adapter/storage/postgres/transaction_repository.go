package postgres

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
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

// GetTransactionByOrderID busca por el identificador de CredibanCo.
//
// La notificación de la pasarela puede venir con su id (mdOrder) en vez del
// nuestro, así que hace falta poder entrar por ese lado. Se ordena por fecha y
// se toma la más reciente por prudencia: credibanco_order_id no tiene índice
// único, y si por un reintento hubiera dos filas con el mismo valor, la que
// interesa es la última.
func (r *transactionRepository) GetTransactionByOrderID(ctx context.Context, credibancoOrderID string) (*domain.Transaction, error) {
	if credibancoOrderID == "" {
		return nil, domain.ErrNotFound
	}

	query := `
		SELECT id, user_id, reference_type, reference_id, amount, credibanco_order_id, status, created_at, updated_at
		FROM core.transactions
		WHERE credibanco_order_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	row := r.db.QueryRow(ctx, query, credibancoOrderID)
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
		// pgx v4 devuelve pgx.ErrNoRows, no sql.ErrNoRows: comparar con este
		// último nunca casa y "no hay transacción" saldría como error de base.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
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

package ports

import (
	"context"

	"github.com/google/uuid"
	"applegacy/backend/internal/core/domain"
)

type TransactionRepository interface {
	CreateTransaction(ctx context.Context, tx *domain.Transaction) error
	GetTransactionByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error)
	UpdateTransactionStatus(ctx context.Context, id uuid.UUID, status domain.TransactionStatus, credibancoOrderID string) error
}

type PaymentGateway interface {
	CreatePaymentIntent(ctx context.Context, amount float64, orderNumber string, returnUrl string) (orderId string, formUrl string, err error)
	GetPaymentStatus(ctx context.Context, orderId string) (status domain.TransactionStatus, err error)
}

type PaymentService interface {
	InitiatePayment(ctx context.Context, userID uuid.UUID, refType domain.ReferenceType, refID uuid.UUID, amount float64, returnUrl string) (string, error)
	VerifyPayment(ctx context.Context, txID uuid.UUID) (*domain.Transaction, error)
}

package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
)

type paymentService struct {
	txRepo    ports.TransactionRepository
	gateway   ports.PaymentGateway
}

func NewPaymentService(txRepo ports.TransactionRepository, gateway ports.PaymentGateway) ports.PaymentService {
	return &paymentService{
		txRepo:  txRepo,
		gateway: gateway,
	}
}

func (s *paymentService) InitiatePayment(ctx context.Context, userID uuid.UUID, refType domain.ReferenceType, refID uuid.UUID, amount float64, returnUrl string) (string, error) {
	// Create transaction in pending state
	tx := &domain.Transaction{
		ID:            uuid.New(),
		UserID:        userID,
		ReferenceType: refType,
		ReferenceID:   refID,
		Amount:        amount,
		Status:        domain.TxStatusPending,
	}

	if err := s.txRepo.CreateTransaction(ctx, tx); err != nil {
		return "", fmt.Errorf("failed to create transaction: %w", err)
	}

	// Call CredibanCo gateway
	orderId, formUrl, err := s.gateway.CreatePaymentIntent(ctx, amount, tx.ID.String(), returnUrl)
	if err != nil {
		// Update status to FAILED since gateway failed
		_ = s.txRepo.UpdateTransactionStatus(ctx, tx.ID, domain.TxStatusFailed, "")
		return "", fmt.Errorf("failed to initiate gateway payment: %w", err)
	}

	// Update with credibanco order id
	if err := s.txRepo.UpdateTransactionStatus(ctx, tx.ID, domain.TxStatusPending, orderId); err != nil {
		return "", fmt.Errorf("failed to update transaction with order id: %w", err)
	}

	return formUrl, nil
}

func (s *paymentService) VerifyPayment(ctx context.Context, txID uuid.UUID) (*domain.Transaction, error) {
	tx, err := s.txRepo.GetTransactionByID(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}

	if tx.CredibancoOrderID == "" {
		return nil, fmt.Errorf("transaction does not have a credibanco order id")
	}

	// Always verify against the bank
	status, err := s.gateway.GetPaymentStatus(ctx, tx.CredibancoOrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway status: %w", err)
	}

	if status != tx.Status {
		if err := s.txRepo.UpdateTransactionStatus(ctx, tx.ID, status, tx.CredibancoOrderID); err != nil {
			return nil, fmt.Errorf("failed to update transaction status: %w", err)
		}
		tx.Status = status
	}

	return tx, nil
}

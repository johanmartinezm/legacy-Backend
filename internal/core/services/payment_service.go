package services

import (
	"context"
	"errors"
	"fmt"
	"log"

	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"github.com/google/uuid"
)

type paymentService struct {
	txRepo    ports.TransactionRepository
	gateway   ports.PaymentGateway
	eventRepo ports.EventRepository
}

// NewPaymentService recibe eventRepo para poder confirmar la inscripción cuando
// la pasarela aprueba un cobro de tipo EVENT. Admite nil: en ese caso el pago se
// verifica igual y solo se omite la confirmación de la inscripción.
func NewPaymentService(txRepo ports.TransactionRepository, gateway ports.PaymentGateway, eventRepo ports.EventRepository) ports.PaymentService {
	return &paymentService{
		txRepo:    txRepo,
		gateway:   gateway,
		eventRepo: eventRepo,
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

	// Un pago aprobado tiene que confirmar la inscripción: hasta hoy esto no
	// existía y quien pagaba de verdad se quedaba sin inscribir, con la
	// transacción en APPROVED y ninguna fila en events.registrations que lo
	// reflejara.
	//
	// Se ejecuta siempre que el estado sea APPROVED, no solo cuando acaba de
	// cambiar: verificar dos veces debe dejar el mismo resultado, y el UPDATE es
	// idempotente. Si la primera verificación confirmó el pago pero falló al
	// tocar la inscripción, la segunda lo arregla.
	if tx.Status == domain.TxStatusApproved && tx.ReferenceType == domain.RefTypeEvent && s.eventRepo != nil {
		err := s.eventRepo.ConfirmEventRegistration(ctx, tx.UserID.String(), tx.ReferenceID.String())
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("payment approved but registration could not be confirmed: %w", err)
		}
		// ErrNotFound significa que se pagó sin inscripción previa. No se
		// inventa una aquí: no hay forma de saber qué talleres eligió ni con qué
		// datos, y crear una a medias sería peor que dejar constancia. Queda el
		// pago aprobado y visible para reconciliar a mano.
		if errors.Is(err, domain.ErrNotFound) {
			log.Printf("[PAGO] transaccion %s aprobada sin inscripcion previa (usuario %s, evento %s): revisar a mano",
				tx.ID, tx.UserID, tx.ReferenceID)
		}
	}

	return tx, nil
}

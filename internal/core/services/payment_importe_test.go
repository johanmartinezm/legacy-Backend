package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// stubGateway registra con qué importe se llamó a la pasarela: es lo único que
// de verdad importa: lo que se le cobra al usuario.
type stubGateway struct {
	importeCobrado float64
	llamadas       int
}

func (g *stubGateway) CreatePaymentIntent(ctx context.Context, amount float64, orderNumber, returnUrl string) (string, string, error) {
	g.importeCobrado = amount
	g.llamadas++
	return "orden-123", "https://pasarela/formulario", nil
}

func (g *stubGateway) GetPaymentStatus(ctx context.Context, orderId string) (domain.TransactionStatus, error) {
	return domain.TxStatusPending, nil
}

type stubTxRepo struct{}

func (r *stubTxRepo) CreateTransaction(ctx context.Context, tx *domain.Transaction) error { return nil }
func (r *stubTxRepo) GetTransactionByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	return nil, nil
}
func (r *stubTxRepo) GetTransactionByOrderID(ctx context.Context, orderID string) (*domain.Transaction, error) {
	return nil, domain.ErrNotFound
}
func (r *stubTxRepo) UpdateTransactionStatus(ctx context.Context, id uuid.UUID, s domain.TransactionStatus, orderID string) error {
	return nil
}

func servicioCon(precio float64, gratis bool, encontrado bool) (*stubGateway, *paymentService) {
	gateway := &stubGateway{}
	repo := &MockEventRepository{
		GetEventByIDFunc: func(ctx context.Context, id string) (*domain.Event, error) {
			if !encontrado {
				return nil, domain.ErrNotFound
			}
			return &domain.Event{ID: id, Price: precio, IsFree: gratis}, nil
		},
	}
	svc := NewPaymentService(&stubTxRepo{}, gateway, repo).(*paymentService)
	return gateway, svc
}

func TestInitiatePayment_ElImporteLoDecideElServidor(t *testing.T) {
	ctx := context.Background()
	usuario := uuid.New()
	evento := uuid.New()

	t.Run("Rechaza un importe menor que el precio real", func(t *testing.T) {
		// El fallo original: {"amount": 1000} en un evento de 250000 se cobraba
		// por mil pesos.
		gateway, svc := servicioCon(250000, false, true)

		_, err := svc.InitiatePayment(ctx, usuario, domain.RefTypeEvent, evento, 1000, "app://volver")

		if !errors.Is(err, ErrPaymentAmountMismatch) {
			t.Fatalf("esperaba ErrPaymentAmountMismatch, llegó %v", err)
		}
		if gateway.llamadas != 0 {
			t.Error("no debería haberse llamado a la pasarela")
		}
	})

	t.Run("Rechaza también un importe mayor", func(t *testing.T) {
		// Cobrar de más tampoco vale, aunque no sea fraude: el usuario no vio
		// ese precio en pantalla.
		gateway, svc := servicioCon(250000, false, true)

		_, err := svc.InitiatePayment(ctx, usuario, domain.RefTypeEvent, evento, 500000, "app://volver")

		if !errors.Is(err, ErrPaymentAmountMismatch) {
			t.Fatalf("esperaba ErrPaymentAmountMismatch, llegó %v", err)
		}
		if gateway.llamadas != 0 {
			t.Error("no debería haberse llamado a la pasarela")
		}
	})

	t.Run("Acepta el importe correcto y cobra el precio del servidor", func(t *testing.T) {
		gateway, svc := servicioCon(250000, false, true)

		url, err := svc.InitiatePayment(ctx, usuario, domain.RefTypeEvent, evento, 250000, "app://volver")

		if err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}
		if url == "" {
			t.Error("esperaba la url del formulario")
		}
		if gateway.importeCobrado != 250000 {
			t.Errorf("esperaba cobrar 250000, se cobró %v", gateway.importeCobrado)
		}
	})

	t.Run("Tolera la imprecisión de los decimales", func(t *testing.T) {
		// Los importes viajan como float y se guardan como numeric(10,2);
		// comparar con == daría falsos rechazos.
		gateway, svc := servicioCon(99.90, false, true)

		_, err := svc.InitiatePayment(ctx, usuario, domain.RefTypeEvent, evento, 99.900000001, "app://volver")

		if err != nil {
			t.Fatalf("una diferencia de una millonésima no debe rechazarse: %v", err)
		}
		if gateway.importeCobrado != 99.90 {
			t.Errorf("debe cobrarse el precio del servidor, se cobró %v", gateway.importeCobrado)
		}
	})

	t.Run("Un evento gratuito no pasa por la pasarela", func(t *testing.T) {
		gateway, svc := servicioCon(0, true, true)

		_, err := svc.InitiatePayment(ctx, usuario, domain.RefTypeEvent, evento, 0, "app://volver")

		if !errors.Is(err, ErrPaymentEventIsFree) {
			t.Fatalf("esperaba ErrPaymentEventIsFree, llegó %v", err)
		}
		if gateway.llamadas != 0 {
			t.Error("no debería haberse llamado a la pasarela")
		}
	})

	t.Run("Un evento inexistente se distingue de una avería", func(t *testing.T) {
		_, svc := servicioCon(0, false, false)

		_, err := svc.InitiatePayment(ctx, usuario, domain.RefTypeEvent, evento, 100, "app://volver")

		if !errors.Is(err, ErrPaymentEventNotFound) {
			t.Fatalf("esperaba ErrPaymentEventNotFound, llegó %v", err)
		}
	})

	t.Run("El carrito no se valida contra eventos", func(t *testing.T) {
		// RefTypeCart no tiene precio que consultar; queda como estaba y se
		// anota como pendiente en el informe del flujo de pago.
		gateway, svc := servicioCon(250000, false, true)

		_, err := svc.InitiatePayment(ctx, usuario, domain.RefTypeCart, evento, 1234, "app://volver")

		if err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}
		if gateway.importeCobrado != 1234 {
			t.Errorf("el carrito conserva su importe, se cobró %v", gateway.importeCobrado)
		}
	})
}

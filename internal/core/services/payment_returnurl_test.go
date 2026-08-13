package services

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"applegacy/backend/internal/core/domain"
	"github.com/google/uuid"
)

// gatewayEspia guarda la URL de retorno con la que se llamó a la pasarela: es lo
// que determina a dónde vuelve el usuario y qué puede verificar la app.
type gatewayEspia struct {
	returnUrlRecibida string
}

func (g *gatewayEspia) CreatePaymentIntent(ctx context.Context, amount float64, orderNumber, returnUrl string) (string, string, error) {
	g.returnUrlRecibida = returnUrl
	return "orden-1", "https://pasarela/formulario", nil
}

func (g *gatewayEspia) GetPaymentStatus(ctx context.Context, orderId string) (domain.TransactionStatus, error) {
	return domain.TxStatusPending, nil
}

func TestInitiatePayment_ElReturnUrlLlevaElTxID(t *testing.T) {
	ctx := context.Background()

	nuevoServicio := func() (*gatewayEspia, *paymentService) {
		g := &gatewayEspia{}
		repo := &MockEventRepository{
			GetEventByIDFunc: func(ctx context.Context, id string) (*domain.Event, error) {
				return &domain.Event{ID: id, Price: 250000}, nil
			},
		}
		return g, NewPaymentService(&stubTxRepo{}, g, repo).(*paymentService)
	}

	t.Run("Añade tx_id a la URL de retorno", func(t *testing.T) {
		// Sin esto, al volver de la pasarela la app no sabe qué transacción
		// verificar: /api/payments/verify espera NUESTRO id, no el de
		// CredibanCo, y depender de cómo llame la pasarela a su parámetro sería
		// frágil.
		g, svc := nuevoServicio()

		_, err := svc.InitiatePayment(ctx, uuid.New(), domain.RefTypeEvent, uuid.New(),
			250000, "legacyapp://app/payment-callback", "")
		if err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}

		u, err := url.Parse(g.returnUrlRecibida)
		if err != nil {
			t.Fatalf("la url de retorno quedó ilegible: %q", g.returnUrlRecibida)
		}
		txID := u.Query().Get("tx_id")
		if txID == "" {
			t.Fatalf("falta tx_id en %q", g.returnUrlRecibida)
		}
		if _, err := uuid.Parse(txID); err != nil {
			t.Errorf("tx_id no es un uuid: %q", txID)
		}
		if u.Scheme != "legacyapp" || u.Host != "app" || u.Path != "/payment-callback" {
			t.Errorf("se alteró el destino de la vuelta: %q", g.returnUrlRecibida)
		}
	})

	t.Run("Conserva los parámetros que ya trajera la URL", func(t *testing.T) {
		g, svc := nuevoServicio()

		_, err := svc.InitiatePayment(ctx, uuid.New(), domain.RefTypeEvent, uuid.New(),
			250000, "legacyapp://app/payment-callback?origen=evento", "")
		if err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}

		u, _ := url.Parse(g.returnUrlRecibida)
		if u.Query().Get("origen") != "evento" {
			t.Errorf("se perdió un parámetro previo: %q", g.returnUrlRecibida)
		}
		if u.Query().Get("tx_id") == "" {
			t.Errorf("falta tx_id: %q", g.returnUrlRecibida)
		}
	})

	t.Run("El tx_id coincide con el orderNumber que recibe la pasarela", func(t *testing.T) {
		// Si no coincidieran, la verificación consultaría una transacción
		// distinta de la que se cobró.
		g := &gatewayEspia{}
		var orderNumber string
		gw := &gatewayCapturaOrden{espia: g, orden: &orderNumber}
		repo := &MockEventRepository{
			GetEventByIDFunc: func(ctx context.Context, id string) (*domain.Event, error) {
				return &domain.Event{ID: id, Price: 100}, nil
			},
		}
		svc := NewPaymentService(&stubTxRepo{}, gw, repo).(*paymentService)

		_, err := svc.InitiatePayment(ctx, uuid.New(), domain.RefTypeEvent, uuid.New(),
			100, "legacyapp://app/payment-callback", "")
		if err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}

		u, _ := url.Parse(g.returnUrlRecibida)
		if u.Query().Get("tx_id") != orderNumber {
			t.Errorf("tx_id %q no coincide con el orderNumber %q",
				u.Query().Get("tx_id"), orderNumber)
		}
	})

	t.Run("Una URL malformada no pierde el tx_id", func(t *testing.T) {
		g, svc := nuevoServicio()

		_, err := svc.InitiatePayment(ctx, uuid.New(), domain.RefTypeEvent, uuid.New(),
			250000, "://esto no es una url", "")
		if err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}

		if !strings.Contains(g.returnUrlRecibida, "tx_id=") {
			t.Errorf("se perdió el tx_id: %q", g.returnUrlRecibida)
		}
	})
}

// gatewayCapturaOrden envuelve al espía para quedarse además con el orderNumber.
type gatewayCapturaOrden struct {
	espia *gatewayEspia
	orden *string
}

func (g *gatewayCapturaOrden) CreatePaymentIntent(ctx context.Context, amount float64, orderNumber, returnUrl string) (string, string, error) {
	*g.orden = orderNumber
	return g.espia.CreatePaymentIntent(ctx, amount, orderNumber, returnUrl)
}

func (g *gatewayCapturaOrden) GetPaymentStatus(ctx context.Context, orderId string) (domain.TransactionStatus, error) {
	return domain.TxStatusPending, nil
}

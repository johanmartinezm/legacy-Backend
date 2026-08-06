package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// repoWebhook lleva la cuenta de lo que se le pide, para poder afirmar no solo
// el resultado sino cuántas veces se llamó a cada cosa: la idempotencia y el
// límite de llamadas salientes son la mitad del comportamiento que importa.
type repoWebhook struct {
	tx            *domain.Transaction
	porID         int
	porOrden      int
	actualizada   domain.TransactionStatus
	nActualizadas int
}

func (r *repoWebhook) CreateTransaction(ctx context.Context, tx *domain.Transaction) error { return nil }

func (r *repoWebhook) GetTransactionByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	r.porID++
	if r.tx == nil || r.tx.ID != id {
		return nil, domain.ErrNotFound
	}
	copia := *r.tx
	return &copia, nil
}

func (r *repoWebhook) GetTransactionByOrderID(ctx context.Context, orderID string) (*domain.Transaction, error) {
	r.porOrden++
	if r.tx == nil || r.tx.CredibancoOrderID != orderID {
		return nil, domain.ErrNotFound
	}
	copia := *r.tx
	return &copia, nil
}

func (r *repoWebhook) UpdateTransactionStatus(ctx context.Context, id uuid.UUID, s domain.TransactionStatus, orderID string) error {
	r.actualizada = s
	r.nActualizadas++
	if r.tx != nil {
		r.tx.Status = s
	}
	return nil
}

// gatewayWebhook responde el estado que diga el test y cuenta las consultas.
type gatewayWebhook struct {
	estado    domain.TransactionStatus
	consultas int
}

func (g *gatewayWebhook) CreatePaymentIntent(ctx context.Context, amount float64, orderNumber, returnUrl string) (string, string, error) {
	return "orden-123", "https://pasarela/formulario", nil
}

func (g *gatewayWebhook) GetPaymentStatus(ctx context.Context, orderId string) (domain.TransactionStatus, error) {
	g.consultas++
	return g.estado, nil
}

func escenario(estadoTx domain.TransactionStatus, estadoPasarela domain.TransactionStatus) (*paymentService, *repoWebhook, *gatewayWebhook, *bool) {
	confirmada := false
	tx := &domain.Transaction{
		ID:                uuid.New(),
		UserID:            uuid.New(),
		ReferenceType:     domain.RefTypeEvent,
		ReferenceID:       uuid.New(),
		Amount:            250000,
		CredibancoOrderID: "orden-credibanco-abc",
		Status:            estadoTx,
	}
	repo := &repoWebhook{tx: tx}
	gateway := &gatewayWebhook{estado: estadoPasarela}
	eventos := &MockEventRepository{
		ConfirmEventRegistrationFunc: func(ctx context.Context, uID, eID string) error {
			confirmada = true
			return nil
		},
	}
	svc := NewPaymentService(repo, gateway, eventos).(*paymentService)
	return svc, repo, gateway, &confirmada
}

func TestWebhook_ConfirmaLaInscripcionSinQueElUsuarioVuelva(t *testing.T) {
	// El motivo de existir del webhook: hasta ahora, si el usuario cerraba el
	// navegador tras pagar, el cobro quedaba sin confirmar para siempre.
	svc, repo, gateway, confirmada := escenario(domain.TxStatusPending, domain.TxStatusApproved)

	tx, err := svc.ProcessGatewayNotification(context.Background(), repo.tx.ID.String())

	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if tx.Status != domain.TxStatusApproved {
		t.Errorf("estado esperado APPROVED, llegó %s", tx.Status)
	}
	if gateway.consultas != 1 {
		t.Errorf("debe consultarse el estado a la pasarela una vez, hubo %d", gateway.consultas)
	}
	if !*confirmada {
		t.Error("la inscripción debe quedar confirmada")
	}
}

func TestWebhook_AceptaElIdDeLaPasarela(t *testing.T) {
	// No está confirmado si CredibanCo enviará nuestro orderNumber o su mdOrder,
	// así que se aceptan los dos caminos.
	svc, repo, _, confirmada := escenario(domain.TxStatusPending, domain.TxStatusApproved)

	tx, err := svc.ProcessGatewayNotification(context.Background(), "orden-credibanco-abc")

	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if tx.ID != repo.tx.ID {
		t.Errorf("se resolvió otra transacción: %s", tx.ID)
	}
	if !*confirmada {
		t.Error("la inscripción debe quedar confirmada")
	}
}

func TestWebhook_UnaNotificacionFalsaNoApruebaNada(t *testing.T) {
	// El punto de seguridad de todo el diseño. La ruta es pública: quien la
	// descubra puede inventarse una notificación. Como el estado se pregunta a
	// la pasarela y no se lee de lo que llega, un pago rechazado sigue
	// rechazado por muchas notificaciones que reciba.
	svc, repo, gateway, confirmada := escenario(domain.TxStatusPending, domain.TxStatusDeclined)

	tx, err := svc.ProcessGatewayNotification(context.Background(), repo.tx.ID.String())

	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if tx.Status != domain.TxStatusDeclined {
		t.Errorf("estado esperado DECLINED, llegó %s", tx.Status)
	}
	if *confirmada {
		t.Error("una transacción rechazada NUNCA debe confirmar la inscripción")
	}
	if gateway.consultas != 1 {
		t.Errorf("el estado debe salir de la pasarela, no de la notificación (%d consultas)", gateway.consultas)
	}
}

func TestWebhook_ReferenciaDesconocida(t *testing.T) {
	svc, _, gateway, confirmada := escenario(domain.TxStatusPending, domain.TxStatusApproved)

	_, err := svc.ProcessGatewayNotification(context.Background(), uuid.New().String())

	if !errors.Is(err, ErrPaymentNotificationUnknown) {
		t.Fatalf("se esperaba ErrPaymentNotificationUnknown, llegó %v", err)
	}
	if gateway.consultas != 0 {
		t.Error("no debe consultarse la pasarela por una transacción que no existe")
	}
	if *confirmada {
		t.Error("no debe confirmarse nada")
	}
}

func TestWebhook_ReferenciaVacia(t *testing.T) {
	svc, _, _, _ := escenario(domain.TxStatusPending, domain.TxStatusApproved)

	if _, err := svc.ProcessGatewayNotification(context.Background(), "   "); !errors.Is(err, ErrPaymentNotificationEmpty) {
		t.Fatalf("se esperaba ErrPaymentNotificationEmpty, llegó %v", err)
	}
}

func TestWebhook_NoRepiteLlamadasSobreUnPagoYaAprobado(t *testing.T) {
	// La pasarela puede reintentar la notificación, y la ruta es pública. Sin
	// este corte, cualquiera con una referencia válida podría hacernos repetir
	// llamadas salientes a CredibanCo indefinidamente.
	svc, repo, gateway, confirmada := escenario(domain.TxStatusApproved, domain.TxStatusApproved)

	for i := 0; i < 3; i++ {
		if _, err := svc.ProcessGatewayNotification(context.Background(), repo.tx.ID.String()); err != nil {
			t.Fatalf("notificación %d: error inesperado: %v", i+1, err)
		}
	}

	if gateway.consultas != 0 {
		t.Errorf("una transacción ya aprobada no se vuelve a consultar, hubo %d consultas", gateway.consultas)
	}
	if repo.nActualizadas != 0 {
		t.Errorf("no debe reescribirse el estado, hubo %d actualizaciones", repo.nActualizadas)
	}
	// Sí se reintenta la confirmación local: es idempotente y repara el caso en
	// que el pago se marcó aprobado pero la inscripción no llegó a confirmarse.
	if !*confirmada {
		t.Error("debe reintentarse la confirmación de la inscripción")
	}
}

func TestWebhook_UnPagoRechazadoNoSeReconsulta(t *testing.T) {
	svc, repo, gateway, _ := escenario(domain.TxStatusDeclined, domain.TxStatusApproved)

	tx, err := svc.ProcessGatewayNotification(context.Background(), repo.tx.ID.String())

	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if tx.Status != domain.TxStatusDeclined {
		t.Errorf("un rechazo es definitivo, llegó %s", tx.Status)
	}
	if gateway.consultas != 0 {
		t.Errorf("no debe reconsultarse, hubo %d consultas", gateway.consultas)
	}
}

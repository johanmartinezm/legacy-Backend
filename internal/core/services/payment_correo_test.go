package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// correoDePagoEspia registra lo que se le pidió enviar, sin mandar nada.
//
// Lleva mutex porque el envío ocurre en una goroutine: el test escribe desde el
// hilo principal y lee desde el suyo.
type correoDePagoEspia struct {
	mu      sync.Mutex
	envios  []domain.CorreoPago
	llamado chan struct{}
}

func nuevoEspia() *correoDePagoEspia {
	return &correoDePagoEspia{llamado: make(chan struct{}, 8)}
}

func (c *correoDePagoEspia) SendEventPaymentEmail(datos domain.CorreoPago) error {
	c.mu.Lock()
	c.envios = append(c.envios, datos)
	c.mu.Unlock()
	c.llamado <- struct{}{}
	return nil
}

func (c *correoDePagoEspia) cuantos() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.envios)
}

func (c *correoDePagoEspia) ultimo() domain.CorreoPago {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.envios[len(c.envios)-1]
}

// esperaEnvio da tiempo a la goroutine. Si no llega, el test falla por lo que
// esperaba y no por un pánico al leer una lista vacía.
func (c *correoDePagoEspia) esperaEnvio(t *testing.T) {
	t.Helper()
	select {
	case <-c.llamado:
	case <-time.After(2 * time.Second):
		t.Fatal("no se envio ningun correo")
	}
}

func (c *correoDePagoEspia) SendResetPasswordEmail(to, resetURL string) error { return nil }
func (c *correoDePagoEspia) SendBoardContactEmail(to, n, e, m string) error   { return nil }
func (c *correoDePagoEspia) SendAsesoriaEmail(to, n, e, cat, m string) error  { return nil }
func (c *correoDePagoEspia) SendContactoEmail(to, a, n, e, m string) error    { return nil }
func (c *correoDePagoEspia) SendWelcomeEmail(to, userName string) error       { return nil }
func (c *correoDePagoEspia) SendVerificationEmail(to, link string) error      { return nil }
func (c *correoDePagoEspia) SendEventRegistrationEmail(d domain.CorreoInscripcion) error {
	return nil
}

type usuariosDePrueba struct{}

func (u *usuariosDePrueba) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return &domain.User{ID: id, EmailEncrypted: "quien@pago.test", FirstName: "Pagador"}, nil
}
func (u *usuariosDePrueba) Create(ctx context.Context, user *domain.User) error { return nil }
func (u *usuariosDePrueba) FindByEmailBlindIndex(ctx context.Context, b string) (*domain.User, error) {
	return nil, nil
}
func (u *usuariosDePrueba) FindBySocialID(ctx context.Context, p, s string) (*domain.User, error) {
	return nil, nil
}
func (u *usuariosDePrueba) LinkSocialID(ctx context.Context, uID, p, s string) error { return nil }
func (u *usuariosDePrueba) FindAll(ctx context.Context) ([]*domain.User, error)      { return nil, nil }
func (u *usuariosDePrueba) Update(ctx context.Context, user *domain.User) error      { return nil }
func (u *usuariosDePrueba) Delete(ctx context.Context, id string) error              { return nil }
func (u *usuariosDePrueba) AnonymizeUser(ctx context.Context, id string) error       { return nil }
func (u *usuariosDePrueba) UpdatePassword(ctx context.Context, id, h string) error   { return nil }
func (u *usuariosDePrueba) UpdatePasswordByEmail(ctx context.Context, e, h string) error {
	return nil
}
func (u *usuariosDePrueba) MarkEmailAsVerified(ctx context.Context, userID string) error {
	return nil
}

// escenarioPago monta un cobro aprobado sobre un evento, con la inscripción en
// el estado que se le pida.
func escenarioPago(t *testing.T, virtual bool, estadoInscripcion string) (*paymentService, *domain.Transaction, *correoDePagoEspia) {
	t.Helper()

	tx := &domain.Transaction{
		ID:                uuid.New(),
		UserID:            uuid.New(),
		ReferenceType:     domain.RefTypeEvent,
		ReferenceID:       uuid.New(),
		Amount:            250000,
		CredibancoOrderID: "orden-credibanco-abc",
		Status:            domain.TxStatusPending,
	}

	enlace := "https://meet.example.com/sesion"
	lugar := "Bogota, Hotel"
	evento := &domain.Event{
		ID:        tx.ReferenceID.String(),
		Title:     "Legacy Summit",
		StartDate: time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC),
		IsVirtual: virtual,
	}
	if virtual {
		evento.AccessURL = &enlace
	} else {
		evento.Location = &lugar
	}

	eventos := &MockEventRepository{
		GetEventByIDFunc: func(ctx context.Context, id string) (*domain.Event, error) {
			return evento, nil
		},
		GetRegistrationByUserAndEventFunc: func(ctx context.Context, uID, eID string) (*domain.Registration, error) {
			return &domain.Registration{
				RegistrationStatus: estadoInscripcion,
				QRData:             "REG-codigo-de-acceso",
			}, nil
		},
		ConfirmEventRegistrationFunc: func(ctx context.Context, uID, eID string) error { return nil },
	}

	espia := nuevoEspia()
	svc := NewPaymentService(&stubTxRepo{}, &gatewayWebhook{estado: domain.TxStatusApproved}, eventos).(*paymentService)
	ConCorreoDePago(svc, &usuariosDePrueba{}, espia, nil)

	return svc, tx, espia
}

// El caso que da sentido a todo: quien paga recibe constancia del cobro y su
// codigo de acceso.
func TestPagoAprobado_MandaCorreoConElQR(t *testing.T) {
	svc, tx, espia := escenarioPago(t, false, domain.RegistrationPendingPayment)

	tx.Status = domain.TxStatusApproved
	if err := svc.confirmarInscripcionSiProcede(context.Background(), tx); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	espia.esperaEnvio(t)

	enviado := espia.ultimo()
	if enviado.QRData != "REG-codigo-de-acceso" {
		t.Errorf("se esperaba el codigo de acceso y llego %q", enviado.QRData)
	}
	if enviado.Importe != 250000 {
		t.Errorf("importe %v, se esperaba 250000", enviado.Importe)
	}
	if enviado.Referencia != "orden-credibanco-abc" {
		t.Errorf("referencia %q", enviado.Referencia)
	}
	if enviado.Evento != "Legacy Summit" {
		t.Errorf("evento %q", enviado.Evento)
	}
	if enviado.Para != "quien@pago.test" {
		t.Errorf("destinatario %q", enviado.Para)
	}
}

// Verificar dos veces no puede mandar dos correos. La app verifica al volver de
// la pasarela y el webhook tambien, asi que este caso ocurre siempre.
func TestPagoAprobado_NoRepiteElCorreoSiYaEstabaConfirmada(t *testing.T) {
	svc, tx, espia := escenarioPago(t, false, domain.RegistrationConfirmed)

	tx.Status = domain.TxStatusApproved
	if err := svc.confirmarInscripcionSiProcede(context.Background(), tx); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	// Sin canal que esperar: se comprueba que NO llega nada.
	time.Sleep(300 * time.Millisecond)
	if n := espia.cuantos(); n != 0 {
		t.Errorf("se enviaron %d correos y no debia enviarse ninguno", n)
	}
}

// Un QR en una masterclass virtual no abre ninguna puerta: lo que hace falta es
// el enlace de la sesion.
func TestPagoAprobado_ElVirtualLlevaEnlaceYNoQR(t *testing.T) {
	svc, tx, espia := escenarioPago(t, true, domain.RegistrationPendingPayment)

	tx.Status = domain.TxStatusApproved
	if err := svc.confirmarInscripcionSiProcede(context.Background(), tx); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	espia.esperaEnvio(t)

	enviado := espia.ultimo()
	if enviado.QRData != "" {
		t.Errorf("un evento virtual no debe llevar QR, llego %q", enviado.QRData)
	}
	if !enviado.EsVirtual {
		t.Error("no se marco como virtual")
	}
	if enviado.EnlaceLugar != "https://meet.example.com/sesion" {
		t.Errorf("enlace %q", enviado.EnlaceLugar)
	}
}

// Un pago que no aprueba no confirma nada ni avisa a nadie.
func TestPagoNoAprobado_NoMandaCorreo(t *testing.T) {
	svc, tx, espia := escenarioPago(t, false, domain.RegistrationPendingPayment)

	tx.Status = domain.TxStatusDeclined
	if err := svc.confirmarInscripcionSiProcede(context.Background(), tx); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	if n := espia.cuantos(); n != 0 {
		t.Errorf("se enviaron %d correos con el pago rechazado", n)
	}
}

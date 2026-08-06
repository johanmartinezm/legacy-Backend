package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"testing"
)

// regMock arma un repositorio con un evento del precio indicado y sin
// inscripción previa, que es lo único que RegisterUser consulta.
func regMock(gratis bool, precio float64) *MockEventRepository {
	return &MockEventRepository{
		GetEventByIDFunc: func(ctx context.Context, id string) (*domain.Event, error) {
			return &domain.Event{ID: id, IsFree: gratis, Price: precio}, nil
		},
		GetRegistrationByUserAndEventFunc: func(ctx context.Context, uID, eID string) (*domain.Registration, error) {
			return nil, nil
		},
		AddRegistrationWorkshopsFunc: func(ctx context.Context, id string, wIDs []string) error {
			return nil
		},
	}
}

func TestRegisterUser_EstadoDeInscripcion(t *testing.T) {
	ctx := context.Background()

	t.Run("Un evento gratuito queda confirmado en el acto", func(t *testing.T) {
		mock := regMock(true, 0)
		var guardada *domain.Registration
		mock.CreateRegistrationFunc = func(ctx context.Context, r *domain.Registration) error {
			guardada = r
			return nil
		}

		reg := &domain.Registration{EventID: "event-1", UserID: "user-1"}
		if err := NewEventService(mock).RegisterUser(ctx, reg); err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}

		if guardada.RegistrationStatus != domain.RegistrationConfirmed {
			t.Errorf("esperaba %q, llegó %q", domain.RegistrationConfirmed, guardada.RegistrationStatus)
		}
		if guardada.PaymentStatus != "free" {
			t.Errorf("esperaba pago 'free', llegó %q", guardada.PaymentStatus)
		}
		if reg.IsPendingPayment() {
			t.Error("una inscripción gratuita no debe quedar pendiente de pago")
		}
	})

	t.Run("Un evento de pago queda pendiente de pago", func(t *testing.T) {
		mock := regMock(false, 250000)
		var guardada *domain.Registration
		mock.CreateRegistrationFunc = func(ctx context.Context, r *domain.Registration) error {
			guardada = r
			return nil
		}

		reg := &domain.Registration{EventID: "event-1", UserID: "user-1"}
		if err := NewEventService(mock).RegisterUser(ctx, reg); err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}

		if guardada.RegistrationStatus != domain.RegistrationPendingPayment {
			t.Errorf("esperaba %q, llegó %q", domain.RegistrationPendingPayment, guardada.RegistrationStatus)
		}
		if guardada.PaymentStatus != "pending" {
			t.Errorf("esperaba pago 'pending', llegó %q", guardada.PaymentStatus)
		}
		if !reg.IsPendingPayment() {
			t.Error("una inscripción de pago sin pagar debe quedar pendiente")
		}
		// La inscripción existe pese a no estar pagada: de ahí se sabe quién
		// intentó comprar y no llegó a completar el pago.
		if guardada.TotalPaid != 250000 {
			t.Errorf("esperaba el importe del evento, llegó %v", guardada.TotalPaid)
		}
	})

	t.Run("Un evento de pago que el admin marca pagado queda confirmado", func(t *testing.T) {
		// Sin esto, inscribir a mano a alguien que pagó por transferencia lo
		// dejaría pendiente para siempre: no hay pasarela que lo confirme.
		mock := regMock(false, 250000)
		var guardada *domain.Registration
		mock.CreateRegistrationFunc = func(ctx context.Context, r *domain.Registration) error {
			guardada = r
			return nil
		}

		reg := &domain.Registration{
			EventID: "event-1", UserID: "user-1", PaymentStatus: "paid",
		}
		if err := NewEventService(mock).RegisterUser(ctx, reg); err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}

		if guardada.RegistrationStatus != domain.RegistrationConfirmed {
			t.Errorf("esperaba %q, llegó %q", domain.RegistrationConfirmed, guardada.RegistrationStatus)
		}
	})

	t.Run("Reinscribirse devuelve el estado que ya tenía, sin duplicar", func(t *testing.T) {
		previa := &domain.Registration{
			ID:                 "reg-1",
			PaymentStatus:      "pending",
			RegistrationStatus: domain.RegistrationPendingPayment,
			QRData:             "REG-user-1-event-1",
			TotalPaid:          250000,
		}
		mock := regMock(false, 250000)
		mock.GetRegistrationByUserAndEventFunc = func(ctx context.Context, uID, eID string) (*domain.Registration, error) {
			return previa, nil
		}
		mock.CreateRegistrationFunc = func(ctx context.Context, r *domain.Registration) error {
			t.Fatal("no debería crear una segunda inscripción")
			return nil
		}

		reg := &domain.Registration{EventID: "event-1", UserID: "user-1"}
		if err := NewEventService(mock).RegisterUser(ctx, reg); err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}

		if reg.ID != "reg-1" {
			t.Errorf("esperaba la inscripción existente, llegó %q", reg.ID)
		}
		if !reg.IsPendingPayment() {
			t.Errorf("el estado previo se perdió: %q", reg.RegistrationStatus)
		}
	})
}

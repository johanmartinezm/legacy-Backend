package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"strings"
	"testing"
	"time"
)

func TestGetMyRegistrations(t *testing.T) {
	ctx := context.Background()

	t.Run("Oculta el QR de las inscripciones pendientes de pago", func(t *testing.T) {
		// Una credencial que no da derecho a entrar no debe salir del servidor:
		// así el cliente no tiene que acordarse de ocultarla.
		mock := &MockEventRepository{
			GetRegistrationsByUserFunc: func(ctx context.Context, uID string) ([]domain.UserRegistration, error) {
				return []domain.UserRegistration{
					{
						EventTitle:         "Evento pagado",
						RegistrationStatus: domain.RegistrationConfirmed,
						QRData:             "REG-aaaa",
					},
					{
						EventTitle:         "Evento sin pagar",
						RegistrationStatus: domain.RegistrationPendingPayment,
						QRData:             "REG-bbbb",
					},
				}, nil
			},
		}

		regs, err := NewEventService(mock, nil).GetMyRegistrations(ctx, "user-1")
		if err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}
		if len(regs) != 2 {
			t.Fatalf("esperaba 2 inscripciones, llegaron %d", len(regs))
		}
		if regs[0].QRData != "REG-aaaa" {
			t.Errorf("la confirmada debe conservar su QR, llegó %q", regs[0].QRData)
		}
		if regs[1].QRData != "" {
			t.Errorf("la pendiente no debe llevar QR, llegó %q", regs[1].QRData)
		}
		// La pendiente sí se devuelve: al usuario le sirve ver que tiene el cupo
		// reservado y que le falta pagar.
		if regs[1].EventTitle != "Evento sin pagar" {
			t.Error("la inscripción pendiente debe seguir apareciendo en la lista")
		}
	})

	t.Run("Sin inscripciones devuelve lista vacía, no error", func(t *testing.T) {
		mock := &MockEventRepository{
			GetRegistrationsByUserFunc: func(ctx context.Context, uID string) ([]domain.UserRegistration, error) {
				return []domain.UserRegistration{}, nil
			},
		}

		regs, err := NewEventService(mock, nil).GetMyRegistrations(ctx, "user-1")
		if err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}
		if len(regs) != 0 {
			t.Errorf("esperaba lista vacía, llegaron %d", len(regs))
		}
	})
}

func TestQRDataEsImpredecible(t *testing.T) {
	ctx := context.Background()

	nuevoMock := func() *MockEventRepository {
		return &MockEventRepository{
			GetEventByIDFunc: func(ctx context.Context, id string) (*domain.Event, error) {
				return &domain.Event{ID: id, IsFree: true}, nil
			},
			GetRegistrationByUserAndEventFunc: func(ctx context.Context, uID, eID string) (*domain.Registration, error) {
				return nil, nil
			},
			CreateRegistrationFunc: func(ctx context.Context, r *domain.Registration) error { return nil },
			AddRegistrationWorkshopsFunc: func(ctx context.Context, id string, w []string) error {
				return nil
			},
		}
	}

	t.Run("No se deriva del usuario ni del evento", func(t *testing.T) {
		// Antes era "REG-{user_id}-{event_id}": cualquiera que conociera ambos
		// uuid podía fabricar el código de otra persona.
		reg := &domain.Registration{EventID: "event-1", UserID: "user-1"}
		if err := NewEventService(nuevoMock(), nil).RegisterUser(ctx, reg); err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}

		if strings.Contains(reg.QRData, "user-1") {
			t.Errorf("el QR no debe contener el id del usuario: %q", reg.QRData)
		}
		if strings.Contains(reg.QRData, "event-1") {
			t.Errorf("el QR no debe contener el id del evento: %q", reg.QRData)
		}
		if !strings.HasPrefix(reg.QRData, "REG-") {
			t.Errorf("se esperaba el prefijo REG-, llegó %q", reg.QRData)
		}
	})

	t.Run("Dos inscripciones al mismo evento dan códigos distintos", func(t *testing.T) {
		uno := &domain.Registration{EventID: "event-1", UserID: "user-1"}
		otro := &domain.Registration{EventID: "event-1", UserID: "user-2"}

		service := NewEventService(nuevoMock(), nil)
		if err := service.RegisterUser(ctx, uno); err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}
		if err := service.RegisterUser(ctx, otro); err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}

		if uno.QRData == otro.QRData {
			t.Errorf("dos inscripciones no pueden compartir código: %q", uno.QRData)
		}
	})

	t.Run("Una inscripción que ya tenía código lo conserva", func(t *testing.T) {
		// Si no, reinscribirse invalidaría el QR que el usuario ya tiene abierto.
		mock := nuevoMock()
		mock.GetRegistrationByUserAndEventFunc = func(ctx context.Context, uID, eID string) (*domain.Registration, error) {
			return &domain.Registration{
				ID:                 "reg-1",
				QRData:             "REG-codigo-existente",
				RegistrationStatus: domain.RegistrationConfirmed,
			}, nil
		}

		reg := &domain.Registration{EventID: "event-1", UserID: "user-1"}
		if err := NewEventService(mock, nil).RegisterUser(ctx, reg); err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}

		if reg.QRData != "REG-codigo-existente" {
			t.Errorf("esperaba conservar el código, llegó %q", reg.QRData)
		}
	})
}

// Decision del 2026-10-14: la credencial ensenaba el QR de eventos de prueba
// internos ya desactivados. Se ocultan los desactivados que ya pasaron; los
// desactivados futuros se conservan, porque desactivar solo quita el evento del
// listado y quien ya esta inscrito tiene que poder entrar.
func TestGetMyRegistrationsOcultaDesactivadosPasados(t *testing.T) {
	dia := func(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d }
	ayer := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	manana := time.Now().AddDate(0, 0, 2).Format("2006-01-02")
	finFuturo := dia(manana)

	mock := &MockEventRepository{
		GetRegistrationsByUserFunc: func(ctx context.Context, uID string) ([]domain.UserRegistration, error) {
			return []domain.UserRegistration{
				{EventTitle: "inactivo pasado", EventStatus: domain.EventoInactivo, EventStartDate: dia(ayer)},
				{EventTitle: "inactivo futuro", EventStatus: domain.EventoInactivo, EventStartDate: dia(manana)},
				{EventTitle: "activo pasado", EventStatus: domain.EventoActivo, EventStartDate: dia(ayer)},
				// Empezo antes pero sigue en curso: no ha terminado.
				{EventTitle: "inactivo en curso", EventStatus: domain.EventoInactivo, EventStartDate: dia(ayer), EventEndDate: &finFuturo},
			}, nil
		},
	}

	regs, err := NewEventService(mock, nil).GetMyRegistrations(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("no esperaba error, llego %v", err)
	}
	var titulos []string
	for _, r := range regs {
		titulos = append(titulos, r.EventTitle)
	}
	if strings.Join(titulos, "|") != "inactivo futuro|activo pasado|inactivo en curso" {
		t.Fatalf("inscripciones devueltas: %v", titulos)
	}
}

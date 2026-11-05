package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"applegacy/backend/internal/core/domain"
)

// Parte B del plan de workshops: link de pago obligatorio en eventos de pago,
// aprobación y rechazo manual, cupo del evento y volver a reservar.

func TestLinkDePago(t *testing.T) {
	casos := []struct {
		nombre string
		evento domain.Event
		motivo string // vacío = válido
	}{
		{"gratuito sin link", domain.Event{IsFree: true}, ""},
		{"de pago con link https", domain.Event{PaymentURL: ptr(" https://pagos.example.com/x ")}, ""},
		{"de pago sin link", domain.Event{}, "necesita el link de pago"},
		{"de pago con link en blanco", domain.Event{PaymentURL: ptr("   ")}, "necesita el link de pago"},
		{"de pago con http", domain.Event{PaymentURL: ptr("http://pagos.example.com")}, "https://"},
		{"de pago con texto", domain.Event{PaymentURL: ptr("pagar aquí")}, "https://"},
	}
	for _, c := range casos {
		e := c.evento
		err := validarLinkDePago(&e)
		if c.motivo == "" {
			if err != nil {
				t.Errorf("%s: err = %v", c.nombre, err)
			}
			continue
		}
		var inv *ErrorEvento
		if !errors.As(err, &inv) || !strings.Contains(inv.Motivo, c.motivo) {
			t.Errorf("%s: err = %v, se esperaba %q", c.nombre, err, c.motivo)
		}
	}

	e := domain.Event{PaymentURL: ptr(" https://pagos.example.com/x ")}
	validarLinkDePago(&e)
	if *e.PaymentURL != "https://pagos.example.com/x" {
		t.Errorf("el link no se recortó: %q", *e.PaymentURL)
	}
}

func limite(n int) *int { return &n }

// eventoDePago empieza dentro de 30 días: la inscripción desde la app sigue
// abierta.
func eventoDePago(limite *int) *domain.Event {
	inicio := time.Now().UTC().AddDate(0, 0, 30).Truncate(24 * time.Hour)
	return &domain.Event{ID: "ev-1", Title: "Masterclass", StartDate: inicio, Price: 100,
		PaymentURL: ptr("https://pagos.example.com/x"), AttendeesLimit: limite}
}

func repoDePago(ev *domain.Event) (*MockEventRepository, *int) {
	creadas := 0
	repo := &MockEventRepository{
		GetEventByIDFunc: func(ctx context.Context, id string) (*domain.Event, error) { return ev, nil },
		CreateRegistrationFunc: func(ctx context.Context, r *domain.Registration) error {
			creadas++
			r.ID = "reg-nueva"
			return nil
		},
	}
	return repo, &creadas
}

func TestCupoDelEvento(t *testing.T) {
	ctx := context.Background()

	t.Run("lleno: la app no inscribe", func(t *testing.T) {
		repo, creadas := repoDePago(eventoDePago(limite(30)))
		repo.CountActiveRegistrationsFunc = func(ctx context.Context, id string) (int, error) { return 30, nil }
		err := NewEventService(repo, nil).RegisterUser(ctx, &domain.Registration{UserID: "u", EventID: "ev-1", DesdeApp: true})
		if !errors.Is(err, ErrCupoLleno) || *creadas != 0 {
			t.Fatalf("err = %v, creadas = %d", err, *creadas)
		}
	})

	t.Run("queda uno: entra, y nace pendiente", func(t *testing.T) {
		repo, creadas := repoDePago(eventoDePago(limite(30)))
		repo.CountActiveRegistrationsFunc = func(ctx context.Context, id string) (int, error) { return 29, nil }
		reg := &domain.Registration{UserID: "u", EventID: "ev-1", DesdeApp: true}
		if err := NewEventService(repo, nil).RegisterUser(ctx, reg); err != nil || *creadas != 1 {
			t.Fatalf("err = %v, creadas = %d", err, *creadas)
		}
		if reg.RegistrationStatus != domain.RegistrationPendingPayment {
			t.Errorf("estado = %q", reg.RegistrationStatus)
		}
	})

	t.Run("lleno: el panel sí puede", func(t *testing.T) {
		repo, creadas := repoDePago(eventoDePago(limite(30)))
		repo.CountActiveRegistrationsFunc = func(ctx context.Context, id string) (int, error) { return 30, nil }
		reg := &domain.Registration{UserID: "u", EventID: "ev-1", PaymentStatus: "paid"}
		if err := NewEventService(repo, nil).RegisterUser(ctx, reg); err != nil || *creadas != 1 {
			t.Fatalf("err = %v, creadas = %d", err, *creadas)
		}
	})

	t.Run("sin límite no hay tope", func(t *testing.T) {
		repo, creadas := repoDePago(eventoDePago(nil))
		repo.CountActiveRegistrationsFunc = func(ctx context.Context, id string) (int, error) {
			t.Fatal("sin límite no debería contar")
			return 0, nil
		}
		if err := NewEventService(repo, nil).RegisterUser(ctx, &domain.Registration{UserID: "u", EventID: "ev-1", DesdeApp: true}); err != nil || *creadas != 1 {
			t.Fatalf("err = %v, creadas = %d", err, *creadas)
		}
	})
}

func TestVolverAReservarTrasUnRechazo(t *testing.T) {
	repo, creadas := repoDePago(eventoDePago(nil))
	repo.GetRegistrationByUserAndEventFunc = func(ctx context.Context, u, e string) (*domain.Registration, error) {
		return &domain.Registration{ID: "reg-1", QRData: "REG-x", RegistrationStatus: domain.RegistrationRejected}, nil
	}
	var reactivada *domain.Registration
	repo.ReactivateRegistrationFunc = func(ctx context.Context, r *domain.Registration, ids []string) error {
		reactivada = r
		return nil
	}

	reg := &domain.Registration{UserID: "u", EventID: "ev-1", DesdeApp: true}
	if err := NewEventService(repo, nil).RegisterUser(context.Background(), reg); err != nil {
		t.Fatal(err)
	}
	if *creadas != 0 {
		t.Error("creó otra inscripción en vez de reactivar la rechazada")
	}
	if reactivada == nil || reactivada.ID != "reg-1" {
		t.Fatalf("no reactivó reg-1: %+v", reactivada)
	}
	if reg.RegistrationStatus != domain.RegistrationPendingPayment || reg.YaEstabaInscrito {
		t.Errorf("estado = %q, ya inscrito = %v", reg.RegistrationStatus, reg.YaEstabaInscrito)
	}
}

func repoConInscripcion(estado string) (*MockEventRepository, *bool, *bool) {
	confirmada, rechazada := false, false
	repo := &MockEventRepository{
		GetRegistrationByIDFunc: func(ctx context.Context, id string) (*domain.Registration, error) {
			return &domain.Registration{ID: id, UserID: "u-1", EventID: "ev-1", RegistrationStatus: estado}, nil
		},
		ConfirmEventRegistrationFunc: func(ctx context.Context, u, e string) error {
			confirmada = true
			return nil
		},
		RejectRegistrationFunc: func(ctx context.Context, id string) error {
			rechazada = true
			return nil
		},
		GetEventByIDFunc: func(ctx context.Context, id string) (*domain.Event, error) {
			return &domain.Event{ID: id, Title: "Masterclass", IsVirtual: true}, nil
		},
	}
	return repo, &confirmada, &rechazada
}

func esperarAviso(t *testing.T, n *notificadorChat) avisoRecibido {
	t.Helper()
	select {
	case a := <-n.avisos:
		return a
	case <-time.After(2 * time.Second):
		t.Fatal("no llegó el aviso push")
		return avisoRecibido{}
	}
}

func TestAprobarInscripcion(t *testing.T) {
	ctx := context.Background()

	t.Run("pendiente: se confirma y se avisa", func(t *testing.T) {
		repo, confirmada, _ := repoConInscripcion(domain.RegistrationPendingPayment)
		avisos := nuevoNotificadorChat(nil)
		reg, err := NewEventService(repo, nil).ConAvisos(avisos).AprobarInscripcion(ctx, "reg-1")
		if err != nil || !*confirmada || reg.RegistrationStatus != domain.RegistrationConfirmed {
			t.Fatalf("reg = %+v, err = %v, confirmada = %v", reg, err, *confirmada)
		}
		a := esperarAviso(t, avisos)
		if a.userID != "u-1" || a.datos["type"] != "inscripcion_aprobada" || a.datos["id"] != "ev-1" {
			t.Errorf("aviso = %+v", a)
		}
	})

	t.Run("ya confirmada: no repite nada", func(t *testing.T) {
		repo, confirmada, _ := repoConInscripcion(domain.RegistrationConfirmed)
		if _, err := NewEventService(repo, nil).AprobarInscripcion(ctx, "reg-1"); err != nil || *confirmada {
			t.Fatalf("err = %v, confirmada = %v", err, *confirmada)
		}
	})

	t.Run("rechazada: no se aprueba", func(t *testing.T) {
		repo, confirmada, _ := repoConInscripcion(domain.RegistrationRejected)
		if _, err := NewEventService(repo, nil).AprobarInscripcion(ctx, "reg-1"); !errors.Is(err, ErrInscripcionNoPendiente) || *confirmada {
			t.Fatalf("err = %v, confirmada = %v", err, *confirmada)
		}
	})
}

func TestRechazarInscripcion(t *testing.T) {
	ctx := context.Background()

	t.Run("pendiente: se rechaza y se avisa", func(t *testing.T) {
		repo, _, rechazada := repoConInscripcion(domain.RegistrationPendingPayment)
		avisos := nuevoNotificadorChat(nil)
		reg, err := NewEventService(repo, nil).ConAvisos(avisos).RechazarInscripcion(ctx, "reg-1")
		if err != nil || !*rechazada || reg.RegistrationStatus != domain.RegistrationRejected {
			t.Fatalf("reg = %+v, err = %v", reg, err)
		}
		if a := esperarAviso(t, avisos); a.datos["type"] != "inscripcion_rechazada" {
			t.Errorf("aviso = %+v", a)
		}
	})

	t.Run("confirmada: no se rechaza", func(t *testing.T) {
		repo, _, rechazada := repoConInscripcion(domain.RegistrationConfirmed)
		if _, err := NewEventService(repo, nil).RechazarInscripcion(ctx, "reg-1"); !errors.Is(err, ErrInscripcionNoPendiente) || *rechazada {
			t.Fatalf("err = %v, rechazada = %v", err, *rechazada)
		}
	})
}

func TestUnaRechazadaNoEntraNiVeSuCredencial(t *testing.T) {
	ctx := context.Background()
	repo := &MockEventRepository{
		GetRegistrationByQRFunc: func(ctx context.Context, qr string) (*domain.Registration, *domain.CheckInResponse, error) {
			return &domain.Registration{ID: "reg-1", RegistrationStatus: domain.RegistrationRejected}, &domain.CheckInResponse{}, nil
		},
		GetRegistrationsByUserFunc: func(ctx context.Context, u string) ([]domain.UserRegistration, error) {
			return []domain.UserRegistration{{
				EventID: "ev-1", RegistrationStatus: domain.RegistrationRejected, QRData: "REG-x",
				EventStartDate: time.Now().AddDate(0, 0, 30),
			}}, nil
		},
	}
	svc := NewEventService(repo, nil)

	if _, err := svc.CheckIn(ctx, "REG-x", "staff"); !errors.Is(err, ErrCheckInRechazada) {
		t.Errorf("check-in: err = %v, se esperaba ErrCheckInRechazada", err)
	}
	regs, err := svc.GetMyRegistrations(ctx, "u")
	if err != nil || len(regs) != 1 || regs[0].QRData != "" {
		t.Errorf("mi credencial: %+v, err = %v", regs, err)
	}
}

package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"applegacy/backend/internal/core/domain"
)

// Reglas del §10 del plan de workshops (2026-11-05): uno por franja, elección y
// app cerradas desde el primer día, calificar solo lo elegido.

// hora fija un instante en hora de Colombia.
func hora(dia, hhmm string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", dia+" "+hhmm, colombia)
	if err != nil {
		panic(err)
	}
	return t
}

func ws(id, dia, desde, hasta string) domain.Workshop {
	return domain.Workshop{ID: id, Name: "Workshop " + id, StartDateTime: hora(dia, desde), EndDateTime: hora(dia, hasta)}
}

// eventoDel imita una columna `date`: medianoche UTC del primer día.
func eventoDel(dia string) *domain.Event {
	d, _ := time.Parse("2006-01-02", dia)
	return &domain.Event{ID: "ev-1", StartDate: d, IsFree: true}
}

func TestFranjasAgrupaLosQueSeCruzanAunqueSeaEnParte(t *testing.T) {
	franjas := calcularFranjas([]domain.Workshop{
		ws("b", "2026-12-10", "09:30", "11:00"),
		ws("a", "2026-12-10", "09:00", "10:00"),
		ws("c", "2026-12-10", "11:00", "12:00"), // empieza justo cuando acaba la anterior
		ws("d", "2026-12-11", "09:00", "10:00"), // otro día
	})
	if len(franjas) != 3 {
		t.Fatalf("franjas = %d, se esperaban 3", len(franjas))
	}
	if n := len(franjas[0].workshops); n != 2 {
		t.Errorf("la primera franja tiene %d workshops, se esperaban 2 (a y b)", n)
	}
	if franjas[0].fin != hora("2026-12-10", "11:00") {
		t.Errorf("la primera franja acaba a las %v, se esperaba 11:00", franjas[0].fin.In(colombia))
	}
}

func TestFranjasCortanPorElDiaDeColombiaNoElDeUTC(t *testing.T) {
	// 19:00 a 20:00 en Colombia es 00:00 a 01:00 UTC del día siguiente. Con el
	// día de UTC caería con el workshop de la mañana siguiente si se cruzaran;
	// lo que importa es que los dos de la noche queden juntos.
	franjas := calcularFranjas([]domain.Workshop{
		ws("a", "2026-12-10", "18:30", "20:00"),
		ws("b", "2026-12-10", "19:00", "20:30"),
	})
	if len(franjas) != 1 {
		t.Fatalf("franjas = %d, se esperaba 1", len(franjas))
	}
}

func TestValidarEleccion(t *testing.T) {
	delEvento := []domain.Workshop{
		ws("a", "2026-12-10", "09:00", "10:00"),
		ws("b", "2026-12-10", "09:00", "10:00"),
		ws("c", "2026-12-10", "11:00", "12:00"),
	}
	casos := []struct {
		nombre   string
		elegidos []string
		motivo   string // vacío = válida
	}{
		{"uno por franja", []string{"a", "c"}, ""},
		{"falta una franja", []string{"a"}, "falta elegir un workshop el 10/12 de 11:00 a 12:00"},
		{"dos en la misma franja", []string{"a", "b", "c"}, "se cruzan"},
		{"workshop de otro evento", []string{"a", "c", "x"}, "no es de este evento"},
		{"ninguno", nil, "falta elegir"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			err := validarEleccion(delEvento, c.elegidos)
			if c.motivo == "" {
				if err != nil {
					t.Fatalf("err = %v, se esperaba válida", err)
				}
				return
			}
			var e *ErrorEleccion
			if !errors.As(err, &e) || !strings.Contains(e.Motivo, c.motivo) {
				t.Fatalf("err = %v, se esperaba un motivo con %q", err, c.motivo)
			}
		})
	}
}

func TestEleccionAbiertaHastaElDiaAnterior(t *testing.T) {
	ev := eventoDel("2026-12-10")
	casos := []struct {
		ahora   time.Time
		abierta bool
	}{
		{hora("2026-12-09", "23:59"), true},
		{hora("2026-12-10", "00:00"), false},
		{hora("2026-12-10", "15:00"), false},
		{hora("2026-12-12", "09:00"), false},
	}
	for _, c := range casos {
		if got := eleccionAbierta(ev, c.ahora); got != c.abierta {
			t.Errorf("a las %v: abierta = %v, se esperaba %v", c.ahora, got, c.abierta)
		}
	}
}

// repoDeEleccion guarda lo que el servicio intenta escribir.
type repoDeEleccion struct {
	MockEventRepository
	evento     *domain.Event
	workshops  []domain.Workshop
	inscrito   *domain.Registration
	creadas    int
	guardados  []string
	reemplazo  []string
	elegidosDe []string
}

func nuevoRepoDeEleccion(ev *domain.Event, workshops []domain.Workshop) *repoDeEleccion {
	r := &repoDeEleccion{evento: ev, workshops: workshops}
	r.GetEventByIDFunc = func(ctx context.Context, id string) (*domain.Event, error) { return r.evento, nil }
	r.GetRegistrationByUserAndEventFunc = func(ctx context.Context, u, e string) (*domain.Registration, error) {
		return r.inscrito, nil
	}
	r.GetWorkshopsByEventIDFunc = func(ctx context.Context, id string) ([]domain.Workshop, error) {
		return r.workshops, nil
	}
	r.CreateRegistrationFunc = func(ctx context.Context, reg *domain.Registration) error {
		r.creadas++
		reg.ID = "reg-1"
		return nil
	}
	r.AddRegistrationWorkshopsFunc = func(ctx context.Context, id string, ids []string) error {
		r.guardados = ids
		return nil
	}
	r.ReplaceRegistrationWorkshopsFunc = func(ctx context.Context, id string, ids []string) error {
		r.reemplazo = ids
		return nil
	}
	r.GetChosenWorkshopIDsFunc = func(ctx context.Context, u, e string) ([]string, error) {
		return r.elegidosDe, nil
	}
	return r
}

func servicioA(repo *repoDeEleccion, ahora time.Time) *EventService {
	svc := NewEventService(repo, nil)
	svc.reloj = func() time.Time { return ahora }
	return svc
}

func dosFranjas() []domain.Workshop {
	return []domain.Workshop{
		ws("a", "2026-12-10", "09:00", "10:00"),
		ws("b", "2026-12-10", "09:00", "10:00"),
		ws("c", "2026-12-10", "11:00", "12:00"),
	}
}

func inscripcionApp(ids ...string) *domain.Registration {
	reg := &domain.Registration{UserID: "u-1", EventID: "ev-1", DesdeApp: true}
	for _, id := range ids {
		reg.Workshops = append(reg.Workshops, domain.Workshop{ID: id})
	}
	return reg
}

func TestInscribirseDesdeLaAppGuardaLaEleccion(t *testing.T) {
	repo := nuevoRepoDeEleccion(eventoDel("2026-12-10"), dosFranjas())
	svc := servicioA(repo, hora("2026-12-09", "10:00"))

	if err := svc.RegisterUser(context.Background(), inscripcionApp("b", "c", "b")); err != nil {
		t.Fatalf("err = %v", err)
	}
	if strings.Join(repo.guardados, ",") != "b,c" {
		t.Errorf("guardados = %v, se esperaban b y c sin repetir", repo.guardados)
	}
}

func TestEleccionInvalidaNoCreaLaInscripcion(t *testing.T) {
	repo := nuevoRepoDeEleccion(eventoDel("2026-12-10"), dosFranjas())
	svc := servicioA(repo, hora("2026-12-09", "10:00"))

	err := svc.RegisterUser(context.Background(), inscripcionApp("a"))
	var e *ErrorEleccion
	if !errors.As(err, &e) {
		t.Fatalf("err = %v, se esperaba un ErrorEleccion", err)
	}
	if repo.creadas != 0 {
		t.Errorf("se crearon %d inscripciones con una elección inválida", repo.creadas)
	}
}

func TestLaAppNoInscribeDesdeElPrimerDia(t *testing.T) {
	repo := nuevoRepoDeEleccion(eventoDel("2026-12-10"), nil)
	svc := servicioA(repo, hora("2026-12-10", "06:00"))

	err := svc.RegisterUser(context.Background(), inscripcionApp())
	if !errors.Is(err, ErrInscripcionesCerradas) {
		t.Fatalf("err = %v, se esperaba ErrInscripcionesCerradas", err)
	}
	if repo.creadas != 0 {
		t.Error("se creó la inscripción con el evento ya empezado")
	}
}

func TestEventoSinWorkshopsSeInscribeComoSiempre(t *testing.T) {
	repo := nuevoRepoDeEleccion(eventoDel("2026-12-10"), nil)
	svc := servicioA(repo, hora("2026-12-09", "10:00"))

	if err := svc.RegisterUser(context.Background(), inscripcionApp()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if repo.creadas != 1 {
		t.Errorf("creadas = %d, se esperaba 1", repo.creadas)
	}
}

func TestElPanelQuedaExento(t *testing.T) {
	repo := nuevoRepoDeEleccion(eventoDel("2026-12-10"), dosFranjas())
	// Ya empezó el evento y no elige nada: desde el panel vale igual.
	svc := servicioA(repo, hora("2026-12-11", "10:00"))
	reg := &domain.Registration{UserID: "u-2", EventID: "ev-1", PaymentStatus: "paid"}

	if err := svc.RegisterUser(context.Background(), reg); err != nil {
		t.Fatalf("err = %v", err)
	}
	if repo.creadas != 1 {
		t.Errorf("creadas = %d, se esperaba 1", repo.creadas)
	}
}

func TestElPanelNoGuardaWorkshopsDeOtroEvento(t *testing.T) {
	repo := nuevoRepoDeEleccion(eventoDel("2026-12-10"), dosFranjas())
	svc := servicioA(repo, hora("2026-12-09", "10:00"))
	reg := &domain.Registration{UserID: "u-2", EventID: "ev-1", Workshops: []domain.Workshop{{ID: "otro"}}}

	var e *ErrorEleccion
	if err := svc.RegisterUser(context.Background(), reg); !errors.As(err, &e) {
		t.Fatalf("err = %v, se esperaba un ErrorEleccion", err)
	}
}

func TestCambiarMisWorkshops(t *testing.T) {
	ctx := context.Background()

	t.Run("sin inscripción", func(t *testing.T) {
		repo := nuevoRepoDeEleccion(eventoDel("2026-12-10"), dosFranjas())
		svc := servicioA(repo, hora("2026-12-09", "10:00"))
		if err := svc.ElegirMisWorkshops(ctx, "u-1", "ev-1", []string{"a", "c"}); !errors.Is(err, ErrSinInscripcion) {
			t.Fatalf("err = %v, se esperaba ErrSinInscripcion", err)
		}
	})

	t.Run("el día del evento", func(t *testing.T) {
		repo := nuevoRepoDeEleccion(eventoDel("2026-12-10"), dosFranjas())
		repo.inscrito = &domain.Registration{ID: "reg-1"}
		svc := servicioA(repo, hora("2026-12-10", "00:00"))
		if err := svc.ElegirMisWorkshops(ctx, "u-1", "ev-1", []string{"a", "c"}); !errors.Is(err, ErrEleccionCerrada) {
			t.Fatalf("err = %v, se esperaba ErrEleccionCerrada", err)
		}
		if repo.reemplazo != nil {
			t.Error("se cambió la elección con la elección cerrada")
		}
	})

	t.Run("el día anterior", func(t *testing.T) {
		repo := nuevoRepoDeEleccion(eventoDel("2026-12-10"), dosFranjas())
		repo.inscrito = &domain.Registration{ID: "reg-1"}
		svc := servicioA(repo, hora("2026-12-09", "23:00"))
		if err := svc.ElegirMisWorkshops(ctx, "u-1", "ev-1", []string{"b", "c"}); err != nil {
			t.Fatalf("err = %v", err)
		}
		if strings.Join(repo.reemplazo, ",") != "b,c" {
			t.Errorf("reemplazo = %v, se esperaban b y c", repo.reemplazo)
		}
	})
}

func TestCalificarSoloLoElegido(t *testing.T) {
	ctx := context.Background()
	terminado := ws("a", "2026-12-10", "09:00", "10:00")
	terminado.EventID = "ev-1"
	despues := hora("2026-12-10", "12:00")

	casos := []struct {
		nombre    string
		elegidos  []string
		esperado  error
		guardadas int
	}{
		{"lo eligió", []string{"a", "c"}, nil, 1},
		{"eligió otro", []string{"b", "c"}, ErrWorkshopNoElegido, 0},
		{"inscripción sin elección", nil, nil, 1},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			repo := &repoDeCalificaciones{taller: &terminado}
			repo.GetChosenWorkshopIDsFunc = func(ctx context.Context, u, e string) ([]string, error) {
				return c.elegidos, nil
			}
			svc := NewEventService(repo, nil)
			svc.reloj = func() time.Time { return despues }

			err := svc.SubmitWorkshopRating(ctx, &domain.WorkshopRating{WorkshopID: "a", UserID: "u-1", Rating: 5})
			if !errors.Is(err, c.esperado) {
				t.Fatalf("err = %v, se esperaba %v", err, c.esperado)
			}
			if repo.guardadas != c.guardadas {
				t.Errorf("guardadas = %d, se esperaban %d", repo.guardadas, c.guardadas)
			}
		})
	}
}

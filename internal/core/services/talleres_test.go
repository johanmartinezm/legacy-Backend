package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"applegacy/backend/internal/core/domain"
)

// Hasta el 2026-10-28 UpdateEvent sincronizaba los talleres borrándolos y
// creándolos de nuevo. Por ON DELETE CASCADE, cada guardado del evento en el
// panel borraba la agenda de los usuarios, los talleres elegidos al inscribirse
// y las calificaciones. Ahora los talleres van por sus propias rutas.

type repoDeTalleres struct {
	MockEventRepository
	borrados  int
	creados   []domain.Workshop
	editados  []domain.Workshop
	sinEvento bool
}

func (r *repoDeTalleres) GetEventByID(ctx context.Context, id string) (*domain.Event, error) {
	if r.sinEvento {
		return nil, domain.ErrNotFound
	}
	// Evento del 4 al 6 de noviembre: tallerValido cae dentro.
	fin := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	return &domain.Event{ID: id, StartDate: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC), EndDate: &fin}, nil
}

func (r *repoDeTalleres) GetWorkshopByID(ctx context.Context, id string) (*domain.Workshop, error) {
	return &domain.Workshop{ID: id, EventID: "evt-1"}, nil
}

func (r *repoDeTalleres) DeleteWorkshopsByEventID(ctx context.Context, id string) error {
	r.borrados++
	return nil
}

func (r *repoDeTalleres) CreateWorkshop(ctx context.Context, w *domain.Workshop) error {
	r.creados = append(r.creados, *w)
	return nil
}

func (r *repoDeTalleres) UpdateWorkshop(ctx context.Context, w *domain.Workshop) error {
	r.editados = append(r.editados, *w)
	return nil
}

func tallerValido() domain.Workshop {
	inicio := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	return domain.Workshop{Name: "Sucesión familiar", StartDateTime: inicio, EndDateTime: inicio.Add(time.Hour)}
}

func TestEditarEvento_NoTocaSusTalleres(t *testing.T) {
	repo := &repoDeTalleres{}
	svc := NewEventService(repo, nil)

	// Un panel viejo todavía podría mandar talleres en el PUT: se ignoran.
	evento := &domain.Event{ID: "evt-1", Title: "Summit", IsFree: true, Workshops: []domain.Workshop{tallerValido()}}
	if err := svc.UpdateEvent(context.Background(), evento); err != nil {
		t.Fatal(err)
	}
	if repo.borrados != 0 {
		t.Error("editar el evento borró sus talleres, y con ellos agenda, inscripciones y calificaciones")
	}
	if len(repo.creados) != 0 {
		t.Errorf("editar el evento creó %d talleres", len(repo.creados))
	}
}

func TestCrearTaller_QuedaEnSuEvento(t *testing.T) {
	repo := &repoDeTalleres{}
	svc := NewEventService(repo, nil)

	taller := tallerValido()
	taller.EventID = "otro-evento" // lo manda la URL, no el cuerpo
	if err := svc.CreateWorkshop(context.Background(), "evt-1", &taller); err != nil {
		t.Fatal(err)
	}
	if len(repo.creados) != 1 || repo.creados[0].EventID != "evt-1" {
		t.Fatalf("se esperaba un taller del evento evt-1 y llegó %+v", repo.creados)
	}
}

func TestCrearTaller_EventoInexistente(t *testing.T) {
	repo := &repoDeTalleres{sinEvento: true}
	svc := NewEventService(repo, nil)

	taller := tallerValido()
	err := svc.CreateWorkshop(context.Background(), "no-existe", &taller)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("se esperaba ErrNotFound y llegó %v", err)
	}
	if len(repo.creados) != 0 {
		t.Error("se creó un taller huérfano")
	}
}

func TestTallerInvalido_NoLlegaALaBase(t *testing.T) {
	sinNombre := tallerValido()
	sinNombre.Name = "   "

	sinFin := tallerValido()
	sinFin.EndDateTime = time.Time{}

	alReves := tallerValido()
	alReves.EndDateTime = alReves.StartDateTime.Add(-time.Minute)

	casos := []struct {
		nombre string
		taller domain.Workshop
		err    error
	}{
		{"sin nombre", sinNombre, domain.ErrTallerSinNombre},
		{"sin fin", sinFin, domain.ErrTallerHorarioInvalido},
		{"fin antes del inicio", alReves, domain.ErrTallerHorarioInvalido},
	}
	for _, c := range casos {
		repo := &repoDeTalleres{}
		svc := NewEventService(repo, nil)

		crear := c.taller
		if err := svc.CreateWorkshop(context.Background(), "evt-1", &crear); !errors.Is(err, c.err) {
			t.Errorf("crear %s: se esperaba %v y llegó %v", c.nombre, c.err, err)
		}
		editar := c.taller
		editar.ID = "ws-1"
		if err := svc.UpdateWorkshop(context.Background(), &editar); !errors.Is(err, c.err) {
			t.Errorf("editar %s: se esperaba %v y llegó %v", c.nombre, c.err, err)
		}
		if len(repo.creados)+len(repo.editados) != 0 {
			t.Errorf("%s: llegó a la base", c.nombre)
		}
	}
}

func TestEditarTaller_ConservaElID(t *testing.T) {
	repo := &repoDeTalleres{}
	svc := NewEventService(repo, nil)

	taller := tallerValido()
	taller.ID = "ws-1"
	if err := svc.UpdateWorkshop(context.Background(), &taller); err != nil {
		t.Fatal(err)
	}
	if len(repo.editados) != 1 || repo.editados[0].ID != "ws-1" {
		t.Fatalf("se esperaba editar ws-1 y llegó %+v", repo.editados)
	}
}

// Un workshop fuera de las fechas del evento habilitaba la calificación y el
// material antes de que el evento empezara (datos de prueba, 2026-11-06).
func TestTallerFueraDelEvento_NoLlegaALaBase(t *testing.T) {
	// El evento va del 4 al 6 de noviembre (repoDeTalleres).
	hora := func(dia, h int) time.Time { return time.Date(2026, 11, dia, h, 0, 0, 0, colombia) }

	casos := []struct {
		nombre        string
		inicio, fin   time.Time
		fueraDeFechas bool
	}{
		{"antes del evento", hora(3, 9), hora(3, 10), true},
		{"empieza el día anterior", hora(3, 23), hora(4, 1), true},
		{"después del evento", hora(7, 9), hora(7, 10), true},
		{"termina el día siguiente", hora(6, 23), hora(7, 1), true},
		{"primer día a primera hora", hora(4, 0), hora(4, 1), false},
		{"último día por la noche", hora(6, 22), hora(6, 23), false},
	}
	for _, c := range casos {
		repo := &repoDeTalleres{}
		svc := NewEventService(repo, nil)

		crear := domain.Workshop{Name: "Sucesión", StartDateTime: c.inicio, EndDateTime: c.fin}
		errCrear := svc.CreateWorkshop(context.Background(), "evt-1", &crear)
		editar := domain.Workshop{ID: "ws-1", Name: "Sucesión", StartDateTime: c.inicio, EndDateTime: c.fin}
		errEditar := svc.UpdateWorkshop(context.Background(), &editar)

		for _, err := range []error{errCrear, errEditar} {
			if c.fueraDeFechas && !errors.Is(err, domain.ErrTallerFueraDelEvento) {
				t.Errorf("%s: se esperaba ErrTallerFueraDelEvento y llegó %v", c.nombre, err)
			}
			if !c.fueraDeFechas && err != nil {
				t.Errorf("%s: se rechazó un workshop dentro del evento: %v", c.nombre, err)
			}
		}
		if c.fueraDeFechas && len(repo.creados)+len(repo.editados) != 0 {
			t.Errorf("%s: llegó a la base", c.nombre)
		}
	}
}

func TestTallerFueraDelEvento_ElMensajeDiceLasFechas(t *testing.T) {
	svc := NewEventService(&repoDeTalleres{}, nil)
	taller := tallerValido()
	taller.StartDateTime = time.Date(2026, 10, 26, 10, 0, 0, 0, colombia)
	taller.EndDateTime = taller.StartDateTime.Add(time.Hour)

	err := svc.CreateWorkshop(context.Background(), "evt-1", &taller)
	quiere := "el workshop tiene que estar dentro de las fechas del evento: del 04/11/2026 al 06/11/2026"
	if err == nil || err.Error() != quiere {
		t.Errorf("mensaje: %v", err)
	}
}

func TestCrearEventoConTallerFueraDeFechas(t *testing.T) {
	svc := NewEventService(&repoDeTalleres{}, nil)
	taller := tallerValido() // 5 de noviembre
	evento := &domain.Event{Title: "Summit", IsFree: true,
		StartDate: time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC), Workshops: []domain.Workshop{taller}}
	if err := svc.CreateEvent(context.Background(), evento); !errors.Is(err, domain.ErrTallerFueraDelEvento) {
		t.Errorf("se esperaba ErrTallerFueraDelEvento y llegó %v", err)
	}
}

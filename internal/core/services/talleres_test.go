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
	return &domain.Event{ID: id}, nil
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
	evento := &domain.Event{ID: "evt-1", Title: "Summit", Workshops: []domain.Workshop{tallerValido()}}
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

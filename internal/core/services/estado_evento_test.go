package services

import (
	"context"
	"errors"
	"testing"

	"applegacy/backend/internal/core/domain"
)

// La validacion del estado vive en el servicio y no en el handler porque la
// propiedad es de negocio: cualquier valor distinto de los dos conocidos deja
// el evento fuera del filtro `= 'active'` del listado, o sea invisible en la
// app, y ninguna pantalla muestra nada raro. Se escribe mal mas facil de lo que
// se nota.

type repoDeEstado struct {
	MockEventRepository
	id      string
	status  string
	llamado bool
}

func (r *repoDeEstado) UpdateEventStatus(ctx context.Context, id, status string) error {
	r.llamado = true
	r.id = id
	r.status = status
	return nil
}

func TestEstadoValido_LlegaAlRepositorio(t *testing.T) {
	for _, estado := range []string{domain.EventoActivo, domain.EventoInactivo} {
		repo := &repoDeEstado{}
		svc := NewEventService(repo, nil)

		if err := svc.UpdateEventStatus(context.Background(), "evt-1", estado); err != nil {
			t.Fatalf("%s: %v", estado, err)
		}
		if !repo.llamado {
			t.Fatalf("%s: no llego al repositorio", estado)
		}
		if repo.id != "evt-1" || repo.status != estado {
			t.Errorf("llego id=%q status=%q", repo.id, repo.status)
		}
	}
}

func TestEstadoInvalido_NoLlegaALaBase(t *testing.T) {
	for _, estado := range []string{"", "activo", "ACTIVE", "Active", "inactivo", "borrado"} {
		repo := &repoDeEstado{}
		svc := NewEventService(repo, nil)

		err := svc.UpdateEventStatus(context.Background(), "evt-1", estado)
		if !errors.Is(err, domain.ErrEstadoDeEventoInvalido) {
			t.Errorf("con %q se esperaba ErrEstadoDeEventoInvalido y llego %v", estado, err)
		}
		if repo.llamado {
			t.Errorf("con %q se escribio en la base un estado que dejaria el evento invisible", estado)
		}
	}
}

func TestEstadoDeEventoValido(t *testing.T) {
	if !domain.EstadoDeEventoValido(domain.EventoActivo) || !domain.EstadoDeEventoValido(domain.EventoInactivo) {
		t.Error("los dos estados conocidos deberian ser validos")
	}
	if domain.EstadoDeEventoValido("") {
		t.Error("la cadena vacia no es un estado: es justo lo que escribiria un UPDATE que no recibe el campo")
	}
}

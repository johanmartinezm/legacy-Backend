package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"applegacy/backend/internal/core/domain"
)

// Hasta el 2026-10-28 SubmitWorkshopRating no comprobaba nada: se podía
// calificar un taller que todavía no había ocurrido, y la segunda calificación
// de la misma persona chocaba con el UNIQUE de la tabla y salía como un 500.

type repoDeCalificaciones struct {
	MockEventRepository
	taller    *domain.Workshop
	guardadas int
	yaExiste  bool
}

func (r *repoDeCalificaciones) GetWorkshopByID(ctx context.Context, id string) (*domain.Workshop, error) {
	if r.taller == nil {
		return nil, domain.ErrNotFound
	}
	return r.taller, nil
}

func (r *repoDeCalificaciones) CreateWorkshopRating(ctx context.Context, wr *domain.WorkshopRating) error {
	if r.yaExiste {
		return fmt.Errorf("%w: workshop_ratings_workshop_id_user_id_key", domain.ErrUniqueViolation)
	}
	r.guardadas++
	return nil
}

func tallerQueTermino(hace time.Duration) *domain.Workshop {
	fin := time.Now().Add(-hace)
	return &domain.Workshop{ID: "ws-1", StartDateTime: fin.Add(-time.Hour), EndDateTime: fin}
}

func calificacion(estrellas int) *domain.WorkshopRating {
	return &domain.WorkshopRating{WorkshopID: "ws-1", UserID: "u-1", Rating: estrellas}
}

func TestCalificarTallerTerminado(t *testing.T) {
	repo := &repoDeCalificaciones{taller: tallerQueTermino(time.Hour)}
	svc := NewEventService(repo, nil)

	if err := svc.SubmitWorkshopRating(context.Background(), calificacion(5)); err != nil {
		t.Fatal(err)
	}
	if repo.guardadas != 1 {
		t.Errorf("se esperaba una calificación guardada y hay %d", repo.guardadas)
	}
}

func TestCalificarTallerQueNoHaTerminado(t *testing.T) {
	// Uno futuro y uno en curso: ninguno se puede calificar todavía.
	casos := map[string]*domain.Workshop{
		"futuro":   tallerQueTermino(-48 * time.Hour),
		"en curso": {ID: "ws-1", StartDateTime: time.Now().Add(-time.Minute), EndDateTime: time.Now().Add(time.Hour)},
	}
	for nombre, taller := range casos {
		repo := &repoDeCalificaciones{taller: taller}
		svc := NewEventService(repo, nil)

		err := svc.SubmitWorkshopRating(context.Background(), calificacion(4))
		if !errors.Is(err, ErrTallerSinTerminar) {
			t.Errorf("%s: se esperaba ErrTallerSinTerminar y llegó %v", nombre, err)
		}
		if repo.guardadas != 0 {
			t.Errorf("%s: se guardó la calificación de un taller sin terminar", nombre)
		}
	}
}

func TestCalificarDosVeces(t *testing.T) {
	repo := &repoDeCalificaciones{taller: tallerQueTermino(time.Hour), yaExiste: true}
	svc := NewEventService(repo, nil)

	err := svc.SubmitWorkshopRating(context.Background(), calificacion(3))
	if !errors.Is(err, ErrTallerYaCalificado) {
		t.Errorf("se esperaba ErrTallerYaCalificado y llegó %v", err)
	}
}

func TestCalificarTallerInexistente(t *testing.T) {
	svc := NewEventService(&repoDeCalificaciones{}, nil)

	err := svc.SubmitWorkshopRating(context.Background(), calificacion(3))
	if !errors.Is(err, ErrTallerNoEncontrado) {
		t.Errorf("se esperaba ErrTallerNoEncontrado y llegó %v", err)
	}
}

func TestCalificacionFueraDeRango(t *testing.T) {
	for _, estrellas := range []int{0, -1, 6} {
		repo := &repoDeCalificaciones{taller: tallerQueTermino(time.Hour)}
		svc := NewEventService(repo, nil)

		err := svc.SubmitWorkshopRating(context.Background(), calificacion(estrellas))
		if !errors.Is(err, ErrCalificacionFueraDeRango) {
			t.Errorf("con %d estrellas se esperaba ErrCalificacionFueraDeRango y llegó %v", estrellas, err)
		}
		if repo.guardadas != 0 {
			t.Errorf("con %d estrellas llegó a la base", estrellas)
		}
	}
}

package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"testing"
)

// La app recorre la respuesta de GET /api/events/{id}/workshop-ratings/me
// directamente: sin calificaciones tiene que llegar [], no null.
func TestGetMyWorkshopRatings_SinNingunaDevuelveListaVacia(t *testing.T) {
	mock := &MockEventRepository{}
	svc := NewEventService(mock, nil)

	ratings, err := svc.GetMyWorkshopRatings(context.Background(), "ev-1", "u-1")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if ratings == nil || len(ratings) != 0 {
		t.Fatalf("se esperaba lista vacía no nula, llegó %#v", ratings)
	}
}

func TestGetMyWorkshopRatings_PideLasDelUsuarioYElEvento(t *testing.T) {
	var gotEvento, gotUsuario string
	mock := &MockEventRepository{
		GetWorkshopRatingsByUserFunc: func(ctx context.Context, eventID, userID string) ([]domain.WorkshopRating, error) {
			gotEvento, gotUsuario = eventID, userID
			return []domain.WorkshopRating{{WorkshopID: "w-1", Rating: 4}}, nil
		},
	}
	svc := NewEventService(mock, nil)

	ratings, err := svc.GetMyWorkshopRatings(context.Background(), "ev-1", "u-1")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if gotEvento != "ev-1" || gotUsuario != "u-1" {
		t.Fatalf("consultó evento %q usuario %q", gotEvento, gotUsuario)
	}
	if len(ratings) != 1 || ratings[0].WorkshopID != "w-1" || ratings[0].Rating != 4 {
		t.Fatalf("respuesta inesperada: %#v", ratings)
	}
}

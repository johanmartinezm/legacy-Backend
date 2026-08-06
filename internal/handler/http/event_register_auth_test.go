package http

import (
	"applegacy/backend/internal/core/domain"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// stubEventService implementa lo justo de ports.EventService para ejercitar el
// handler de inscripción: guarda lo que le llega y no toca ninguna base.
type stubEventService struct {
	recibida *domain.Registration
}

func (s *stubEventService) RegisterUser(ctx context.Context, reg *domain.Registration) error {
	s.recibida = reg
	reg.ID = "reg-nueva"
	return nil
}

func (s *stubEventService) ListEvents(ctx context.Context) ([]domain.Event, error) { return nil, nil }
func (s *stubEventService) GetEventDetails(ctx context.Context, id string) (*domain.Event, error) {
	return nil, nil
}
func (s *stubEventService) ListCategories(ctx context.Context) ([]domain.EventCategory, error) {
	return nil, nil
}
func (s *stubEventService) CreateEvent(ctx context.Context, e *domain.Event) error { return nil }
func (s *stubEventService) UpdateEvent(ctx context.Context, e *domain.Event) error { return nil }
func (s *stubEventService) DeleteEvent(ctx context.Context, id string) error       { return nil }
func (s *stubEventService) SubmitWorkshopRating(ctx context.Context, r *domain.WorkshopRating) error {
	return nil
}
func (s *stubEventService) GetEventFeedback(ctx context.Context, eventID string) ([]domain.WorkshopRating, error) {
	return nil, nil
}
func (s *stubEventService) SubmitEventSurvey(ctx context.Context, sv *domain.EventSurvey) error {
	return nil
}
func (s *stubEventService) GetMyEventSurvey(ctx context.Context, eID, uID string) (*domain.EventSurvey, error) {
	return nil, nil
}
func (s *stubEventService) GetEventSurveySummary(ctx context.Context, eID string) (*domain.EventSurveySummary, error) {
	return nil, nil
}
func (s *stubEventService) GetAgenda(ctx context.Context, userID string) ([]domain.Workshop, error) {
	return nil, nil
}
func (s *stubEventService) AddToAgenda(ctx context.Context, uID, wID string) error      { return nil }
func (s *stubEventService) RemoveFromAgenda(ctx context.Context, uID, wID string) error { return nil }
func (s *stubEventService) CheckIn(ctx context.Context, qr, staff string) (*domain.CheckInResponse, error) {
	return nil, nil
}

// peticion arma un POST /api/events/{id}/register con el cuerpo, el usuario y el
// rol indicados, tal y como lo dejaría AuthMiddleware.
func peticion(cuerpo map[string]any, userID, rol string) (*http.Request, *httptest.ResponseRecorder) {
	var body []byte
	if cuerpo != nil {
		body, _ = json.Marshal(cuerpo)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/events/event-1/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "event-1")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, UserIDKey, userID)
	if rol != "" {
		ctx = context.WithValue(ctx, UserRoleKey, rol)
	}
	return req.WithContext(ctx), httptest.NewRecorder()
}

func TestRegister_CamposReservadosAAdministradores(t *testing.T) {
	t.Run("Un usuario normal no puede declararse pagado", func(t *testing.T) {
		// Fallo 9: con esto se entraba gratis a un evento de pago, con QR
		// válido y sin una sola transacción registrada.
		svc := &stubEventService{}
		req, rec := peticion(map[string]any{"paymentStatus": "paid"}, "user-1", "familia")

		NewEventHandler(svc).Register(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("esperaba 403, llegó %d", rec.Code)
		}
		if svc.recibida != nil {
			t.Error("no debería haber llegado ninguna inscripción al servicio")
		}
	})

	t.Run("Un usuario normal no puede inscribir a otro", func(t *testing.T) {
		// Fallo 10: dejaba una deuda a nombre de un tercero.
		svc := &stubEventService{}
		req, rec := peticion(map[string]any{"userID": "otra-persona"}, "user-1", "familia")

		NewEventHandler(svc).Register(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("esperaba 403, llegó %d", rec.Code)
		}
		if svc.recibida != nil {
			t.Error("no debería haber llegado ninguna inscripción al servicio")
		}
	})

	t.Run("Un administrador sí puede", func(t *testing.T) {
		svc := &stubEventService{}
		req, rec := peticion(
			map[string]any{"userID": "otra-persona", "paymentStatus": "paid"},
			"admin-1", RoleAdmin,
		)

		NewEventHandler(svc).Register(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("esperaba 201, llegó %d", rec.Code)
		}
		if svc.recibida.UserID != "otra-persona" {
			t.Errorf("esperaba inscribir a otra-persona, llegó %q", svc.recibida.UserID)
		}
		if svc.recibida.PaymentStatus != "paid" {
			t.Errorf("esperaba paid, llegó %q", svc.recibida.PaymentStatus)
		}
	})

	t.Run("Sin esos campos, un usuario normal se inscribe con normalidad", func(t *testing.T) {
		// Es lo que hace la app: no manda cuerpo. El arreglo no debe estorbarlo.
		svc := &stubEventService{}
		req, rec := peticion(nil, "user-1", "familia")

		NewEventHandler(svc).Register(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("esperaba 201, llegó %d", rec.Code)
		}
		if svc.recibida.UserID != "user-1" {
			t.Errorf("el titular debe salir del token, llegó %q", svc.recibida.UserID)
		}
	})

	t.Run("Los talleres siguen aceptándose sin ser administrador", func(t *testing.T) {
		svc := &stubEventService{}
		req, rec := peticion(map[string]any{"workshops": []string{"w-1", "w-2"}}, "user-1", "familia")

		NewEventHandler(svc).Register(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("esperaba 201, llegó %d", rec.Code)
		}
		if len(svc.recibida.Workshops) != 2 {
			t.Errorf("esperaba 2 talleres, llegaron %d", len(svc.recibida.Workshops))
		}
	})

	t.Run("Un token sin rol tampoco cuela", func(t *testing.T) {
		// Si el claim "role" falta, IsAdmin devuelve false: se deniega, no se
		// concede por omisión.
		svc := &stubEventService{}
		req, rec := peticion(map[string]any{"paymentStatus": "paid"}, "user-1", "")

		NewEventHandler(svc).Register(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("esperaba 403, llegó %d", rec.Code)
		}
	})
}

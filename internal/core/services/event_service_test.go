package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"testing"
)

// MockEventRepository for testing
type MockEventRepository struct {
	GetEventByIDFunc                  func(ctx context.Context, id string) (*domain.Event, error)
	GetRegistrationByUserAndEventFunc func(ctx context.Context, userID, eventID string) (*domain.Registration, error)
	CreateRegistrationFunc            func(ctx context.Context, registration *domain.Registration) error
	AddRegistrationWorkshopsFunc      func(ctx context.Context, regID string, workshopIDs []string) error
	ConfirmEventRegistrationFunc      func(ctx context.Context, userID, eventID string) error
	GetRegistrationsByUserFunc        func(ctx context.Context, userID string) ([]domain.UserRegistration, error)
	CreateEventSurveyFunc             func(ctx context.Context, survey *domain.EventSurvey) error
	GetEventSurveyByUserFunc          func(ctx context.Context, eventID, userID string) (*domain.EventSurvey, error)
	GetEventSurveySummaryFunc         func(ctx context.Context, eventID string) (*domain.EventSurveySummary, error)
	GetRegistrationByQRFunc           func(ctx context.Context, qr string) (*domain.Registration, *domain.CheckInResponse, error)
	RecordAttendanceFunc              func(ctx context.Context, regID, staffID string) error
	GetRegistrationsByEventFunc       func(ctx context.Context, eventID string) ([]domain.EventRegistrant, error)
}

func (m *MockEventRepository) GetRegistrationsByEvent(ctx context.Context, eID string) ([]domain.EventRegistrant, error) {
	if m.GetRegistrationsByEventFunc == nil {
		return nil, nil
	}
	return m.GetRegistrationsByEventFunc(ctx, eID)
}

func (m *MockEventRepository) GetEvents(ctx context.Context) ([]domain.Event, error) { return nil, nil }
func (m *MockEventRepository) GetEventByID(ctx context.Context, id string) (*domain.Event, error) {
	if m.GetEventByIDFunc == nil {
		return nil, domain.ErrNotFound
	}
	return m.GetEventByIDFunc(ctx, id)
}
func (m *MockEventRepository) GetWorkshopsByEventID(ctx context.Context, id string) ([]domain.Workshop, error) {
	return nil, nil
}
func (m *MockEventRepository) ListCategories(ctx context.Context) ([]domain.EventCategory, error) {
	return nil, nil
}
func (m *MockEventRepository) CreateEvent(ctx context.Context, event *domain.Event) error { return nil }
func (m *MockEventRepository) UpdateEvent(ctx context.Context, event *domain.Event) error { return nil }
func (m *MockEventRepository) DeleteEvent(ctx context.Context, id string) error           { return nil }
func (m *MockEventRepository) CreateWorkshop(ctx context.Context, w *domain.Workshop) error {
	return nil
}
func (m *MockEventRepository) DeleteWorkshopsByEventID(ctx context.Context, id string) error {
	return nil
}
func (m *MockEventRepository) CreateRegistration(ctx context.Context, r *domain.Registration) error {
	return m.CreateRegistrationFunc(ctx, r)
}
func (m *MockEventRepository) AddRegistrationWorkshops(ctx context.Context, id string, wIDs []string) error {
	return m.AddRegistrationWorkshopsFunc(ctx, id, wIDs)
}
// Sin gancho fijado devuelve "no hay inscripcion" en vez de reventar.
//
// Antes llamaba al campo a secas y cualquier test que no lo fijara moria con un
// nil pointer en cuanto el codigo bajo prueba consultara la inscripcion —le paso
// al webhook el 2026-08-18, cuando el servicio de pagos empezo a mirar el estado
// previo para no repetir el correo—. Un mock compartido que revienta por un
// gancho sin fijar es una trampa para el siguiente que escriba un test.
func (m *MockEventRepository) GetRegistrationByUserAndEvent(ctx context.Context, uID, eID string) (*domain.Registration, error) {
	if m.GetRegistrationByUserAndEventFunc == nil {
		return nil, nil
	}
	return m.GetRegistrationByUserAndEventFunc(ctx, uID, eID)
}
func (m *MockEventRepository) CreateWorkshopRating(ctx context.Context, r *domain.WorkshopRating) error {
	return nil
}
func (m *MockEventRepository) GetRatingsByEventID(ctx context.Context, id string) ([]domain.WorkshopRating, error) {
	return nil, nil
}
func (m *MockEventRepository) GetAgenda(ctx context.Context, uID string) ([]domain.Workshop, error) {
	return nil, nil
}
func (m *MockEventRepository) AddToAgenda(ctx context.Context, uID, wID string) error { return nil }
func (m *MockEventRepository) RemoveFromAgenda(ctx context.Context, uID, wID string) error {
	return nil
}
func (m *MockEventRepository) GetRegistrationByQR(ctx context.Context, qr string) (*domain.Registration, *domain.CheckInResponse, error) {
	if m.GetRegistrationByQRFunc == nil {
		return nil, nil, nil
	}
	return m.GetRegistrationByQRFunc(ctx, qr)
}
func (m *MockEventRepository) RecordAttendance(ctx context.Context, rID, sID string) error {
	if m.RecordAttendanceFunc == nil {
		return nil
	}
	return m.RecordAttendanceFunc(ctx, rID, sID)
}
func (m *MockEventRepository) GetWorkshopsByRegistrationID(ctx context.Context, rID string) ([]domain.Workshop, error) {
	return nil, nil
}
func (m *MockEventRepository) GetRegistrationsByUser(ctx context.Context, uID string) ([]domain.UserRegistration, error) {
	if m.GetRegistrationsByUserFunc == nil {
		return nil, nil
	}
	return m.GetRegistrationsByUserFunc(ctx, uID)
}
func (m *MockEventRepository) ConfirmEventRegistration(ctx context.Context, uID, eID string) error {
	if m.ConfirmEventRegistrationFunc == nil {
		return nil
	}
	return m.ConfirmEventRegistrationFunc(ctx, uID, eID)
}
func (m *MockEventRepository) CreateEventSurvey(ctx context.Context, s *domain.EventSurvey) error {
	if m.CreateEventSurveyFunc == nil {
		return nil
	}
	return m.CreateEventSurveyFunc(ctx, s)
}
func (m *MockEventRepository) GetEventSurveyByUser(ctx context.Context, eID, uID string) (*domain.EventSurvey, error) {
	if m.GetEventSurveyByUserFunc == nil {
		return nil, nil
	}
	return m.GetEventSurveyByUserFunc(ctx, eID, uID)
}
func (m *MockEventRepository) GetEventSurveySummary(ctx context.Context, eID string) (*domain.EventSurveySummary, error) {
	if m.GetEventSurveySummaryFunc == nil {
		return nil, nil
	}
	return m.GetEventSurveySummaryFunc(ctx, eID)
}

func TestEventService_RegisterUser(t *testing.T) {
	ctx := context.Background()

	t.Run("Should register a user with PAID status if forced", func(t *testing.T) {
		mockRepo := &MockEventRepository{
			GetEventByIDFunc: func(ctx context.Context, id string) (*domain.Event, error) {
				return &domain.Event{ID: id, IsFree: false, Price: 100}, nil
			},
			GetRegistrationByUserAndEventFunc: func(ctx context.Context, uID, eID string) (*domain.Registration, error) {
				return nil, nil // Not registered yet
			},
			CreateRegistrationFunc: func(ctx context.Context, r *domain.Registration) error {
				if r.PaymentStatus != "paid" {
					t.Errorf("Expected status paid, got %s", r.PaymentStatus)
				}
				r.ID = "new-reg-id"
				return nil
			},
			AddRegistrationWorkshopsFunc: func(ctx context.Context, id string, wIDs []string) error {
				return nil
			},
		}

		service := NewEventService(mockRepo, nil)
		reg := &domain.Registration{
			EventID:       "event-1",
			UserID:        "user-1",
			PaymentStatus: "paid", // Forced by admin
		}

		err := service.RegisterUser(ctx, reg)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})

	t.Run("Should register a user with PENDING status for non-free events by default", func(t *testing.T) {
		mockRepo := &MockEventRepository{
			GetEventByIDFunc: func(ctx context.Context, id string) (*domain.Event, error) {
				return &domain.Event{ID: id, IsFree: false, Price: 100}, nil
			},
			GetRegistrationByUserAndEventFunc: func(ctx context.Context, uID, eID string) (*domain.Registration, error) {
				return nil, nil
			},
			CreateRegistrationFunc: func(ctx context.Context, r *domain.Registration) error {
				if r.PaymentStatus != "pending" {
					t.Errorf("Expected status pending, got %s", r.PaymentStatus)
				}
				return nil
			},
		}

		service := NewEventService(mockRepo, nil)
		reg := &domain.Registration{
			EventID: "event-1",
			UserID:  "user-1",
		}

		err := service.RegisterUser(ctx, reg)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})
}

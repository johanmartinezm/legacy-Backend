package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"testing"
)

// surveyMock arma un repositorio con lo mínimo que SubmitEventSurvey consulta:
// el evento existe, el usuario está o no registrado, y hay o no una encuesta previa.
func surveyMock(registered bool, existing *domain.EventSurvey) *MockEventRepository {
	return &MockEventRepository{
		GetEventByIDFunc: func(ctx context.Context, id string) (*domain.Event, error) {
			return &domain.Event{ID: id}, nil
		},
		GetRegistrationByUserAndEventFunc: func(ctx context.Context, uID, eID string) (*domain.Registration, error) {
			if !registered {
				return nil, nil
			}
			return &domain.Registration{ID: "reg-1", UserID: uID, EventID: eID}, nil
		},
		GetEventSurveyByUserFunc: func(ctx context.Context, eID, uID string) (*domain.EventSurvey, error) {
			return existing, nil
		},
	}
}

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }

func TestSubmitEventSurvey(t *testing.T) {
	ctx := context.Background()

	t.Run("Guarda la encuesta de un usuario registrado", func(t *testing.T) {
		mock := surveyMock(true, nil)
		var saved *domain.EventSurvey
		mock.CreateEventSurveyFunc = func(ctx context.Context, s *domain.EventSurvey) error {
			saved = s
			s.ID = "survey-1"
			return nil
		}

		service := NewEventService(mock, nil)
		survey := &domain.EventSurvey{
			EventID:       "event-1",
			UserID:        "user-1",
			OverallRating: 5,
			ContentRating: intPtr(4),
		}

		if err := service.SubmitEventSurvey(ctx, survey); err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}
		if saved == nil {
			t.Fatal("la encuesta no llegó al repositorio")
		}
		if survey.ID != "survey-1" {
			t.Errorf("el id del repositorio no volvió al llamador: %q", survey.ID)
		}
	})

	t.Run("Rechaza con 403 a quien no está registrado", func(t *testing.T) {
		mock := surveyMock(false, nil)
		mock.CreateEventSurveyFunc = func(ctx context.Context, s *domain.EventSurvey) error {
			t.Fatal("no debería intentar guardar la encuesta de un no registrado")
			return nil
		}

		service := NewEventService(mock, nil)
		err := service.SubmitEventSurvey(ctx, &domain.EventSurvey{
			EventID: "event-1", UserID: "user-1", OverallRating: 5,
		})

		if !errors.Is(err, ErrSurveyNotRegistered) {
			t.Fatalf("esperaba ErrSurveyNotRegistered, llegó %v", err)
		}
	})

	t.Run("Rechaza una segunda encuesta del mismo usuario", func(t *testing.T) {
		previous := &domain.EventSurvey{ID: "survey-anterior", OverallRating: 3}
		mock := surveyMock(true, previous)
		mock.CreateEventSurveyFunc = func(ctx context.Context, s *domain.EventSurvey) error {
			t.Fatal("no debería guardar una segunda encuesta")
			return nil
		}

		service := NewEventService(mock, nil)
		err := service.SubmitEventSurvey(ctx, &domain.EventSurvey{
			EventID: "event-1", UserID: "user-1", OverallRating: 5,
		})

		if !errors.Is(err, ErrSurveyAlreadySent) {
			t.Fatalf("esperaba ErrSurveyAlreadySent, llegó %v", err)
		}
	})

	t.Run("Traduce la violación del UNIQUE a 'ya enviada'", func(t *testing.T) {
		// Dos peticiones simultáneas pasan la comprobación previa; solo el
		// UNIQUE de la tabla las separa. Sin la traducción sería un 500.
		mock := surveyMock(true, nil)
		mock.CreateEventSurveyFunc = func(ctx context.Context, s *domain.EventSurvey) error {
			return domain.ErrUniqueViolation
		}

		service := NewEventService(mock, nil)
		err := service.SubmitEventSurvey(ctx, &domain.EventSurvey{
			EventID: "event-1", UserID: "user-1", OverallRating: 4,
		})

		if !errors.Is(err, ErrSurveyAlreadySent) {
			t.Fatalf("esperaba ErrSurveyAlreadySent, llegó %v", err)
		}
	})

	t.Run("Distingue un evento inexistente de una avería del repositorio", func(t *testing.T) {
		mock := surveyMock(true, nil)
		mock.GetEventByIDFunc = func(ctx context.Context, id string) (*domain.Event, error) {
			return nil, domain.ErrNotFound
		}

		err := NewEventService(mock, nil).SubmitEventSurvey(ctx, &domain.EventSurvey{
			EventID: "no-existe", UserID: "user-1", OverallRating: 5,
		})

		if !errors.Is(err, ErrSurveyEventNotFound) {
			t.Fatalf("esperaba ErrSurveyEventNotFound, llegó %v", err)
		}
	})

	t.Run("Propaga la avería del repositorio en vez de disfrazarla de 'no existe'", func(t *testing.T) {
		// Traducir cualquier error a ErrSurveyEventNotFound escondía fallos de
		// base de datos detrás de un mensaje que manda a buscar donde no es.
		averia := errors.New("connection reset by peer")
		mock := surveyMock(true, nil)
		mock.GetEventByIDFunc = func(ctx context.Context, id string) (*domain.Event, error) {
			return nil, averia
		}

		err := NewEventService(mock, nil).SubmitEventSurvey(ctx, &domain.EventSurvey{
			EventID: "event-1", UserID: "user-1", OverallRating: 5,
		})

		if !errors.Is(err, averia) {
			t.Fatalf("esperaba la avería original, llegó %v", err)
		}
		if errors.Is(err, ErrSurveyEventNotFound) {
			t.Error("una avería de conexión no debe presentarse como 'evento no encontrado'")
		}
	})

	t.Run("Rechaza calificaciones fuera de rango", func(t *testing.T) {
		casos := []struct {
			nombre string
			survey domain.EventSurvey
		}{
			{"general en 0", domain.EventSurvey{OverallRating: 0}},
			{"general en 6", domain.EventSurvey{OverallRating: 6}},
			{"general negativa", domain.EventSurvey{OverallRating: -1}},
			{"organización en 6", domain.EventSurvey{OverallRating: 5, OrganizationRating: intPtr(6)}},
			{"contenido en 0", domain.EventSurvey{OverallRating: 5, ContentRating: intPtr(0)}},
			{"conferencistas en 9", domain.EventSurvey{OverallRating: 5, SpeakersRating: intPtr(9)}},
		}

		for _, caso := range casos {
			t.Run(caso.nombre, func(t *testing.T) {
				mock := surveyMock(true, nil)
				mock.CreateEventSurveyFunc = func(ctx context.Context, s *domain.EventSurvey) error {
					t.Fatal("no debería guardar una calificación inválida")
					return nil
				}

				service := NewEventService(mock, nil)
				survey := caso.survey
				survey.EventID, survey.UserID = "event-1", "user-1"

				if err := service.SubmitEventSurvey(ctx, &survey); !errors.Is(err, ErrSurveyInvalidRating) {
					t.Fatalf("esperaba ErrSurveyInvalidRating, llegó %v", err)
				}
			})
		}
	})

	t.Run("Acepta las opcionales vacías", func(t *testing.T) {
		// Solo overallRating es obligatorio: el resto son punteros justamente
		// para poder distinguir "no respondió" de un cero.
		mock := surveyMock(true, nil)
		guardada := false
		mock.CreateEventSurveyFunc = func(ctx context.Context, s *domain.EventSurvey) error {
			guardada = true
			return nil
		}

		service := NewEventService(mock, nil)
		err := service.SubmitEventSurvey(ctx, &domain.EventSurvey{
			EventID: "event-1", UserID: "user-1", OverallRating: 3,
		})

		if err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}
		if !guardada {
			t.Error("la encuesta sin opcionales no se guardó")
		}
	})

	t.Run("Un comentario en blanco se guarda como nulo", func(t *testing.T) {
		// Si no, el resumen del panel listaría comentarios vacíos.
		mock := surveyMock(true, nil)
		var saved *domain.EventSurvey
		mock.CreateEventSurveyFunc = func(ctx context.Context, s *domain.EventSurvey) error {
			saved = s
			return nil
		}

		service := NewEventService(mock, nil)
		err := service.SubmitEventSurvey(ctx, &domain.EventSurvey{
			EventID: "event-1", UserID: "user-1", OverallRating: 4,
			Comment: strPtr("   \n  "),
		})

		if err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}
		if saved.Comment != nil {
			t.Errorf("esperaba comentario nulo, llegó %q", *saved.Comment)
		}
	})

	t.Run("Un comentario con espacios sobrantes se recorta", func(t *testing.T) {
		mock := surveyMock(true, nil)
		var saved *domain.EventSurvey
		mock.CreateEventSurveyFunc = func(ctx context.Context, s *domain.EventSurvey) error {
			saved = s
			return nil
		}

		service := NewEventService(mock, nil)
		err := service.SubmitEventSurvey(ctx, &domain.EventSurvey{
			EventID: "event-1", UserID: "user-1", OverallRating: 4,
			Comment: strPtr("  Todo excelente  "),
		})

		if err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}
		if saved.Comment == nil || *saved.Comment != "Todo excelente" {
			t.Errorf("esperaba \"Todo excelente\", llegó %v", saved.Comment)
		}
	})
}

func TestGetMyEventSurvey(t *testing.T) {
	ctx := context.Background()

	t.Run("Devuelve nil sin error si aún no respondió", func(t *testing.T) {
		mock := &MockEventRepository{
			GetEventSurveyByUserFunc: func(ctx context.Context, eID, uID string) (*domain.EventSurvey, error) {
				return nil, nil
			},
		}

		survey, err := NewEventService(mock, nil).GetMyEventSurvey(ctx, "event-1", "user-1")
		if err != nil {
			t.Fatalf("no responder aún no es un error: %v", err)
		}
		if survey != nil {
			t.Errorf("esperaba nil, llegó %+v", survey)
		}
	})

	t.Run("Devuelve la encuesta ya enviada", func(t *testing.T) {
		mock := &MockEventRepository{
			GetEventSurveyByUserFunc: func(ctx context.Context, eID, uID string) (*domain.EventSurvey, error) {
				return &domain.EventSurvey{ID: "survey-1", OverallRating: 5}, nil
			},
		}

		survey, err := NewEventService(mock, nil).GetMyEventSurvey(ctx, "event-1", "user-1")
		if err != nil {
			t.Fatalf("no esperaba error, llegó %v", err)
		}
		if survey == nil || survey.ID != "survey-1" {
			t.Errorf("esperaba survey-1, llegó %+v", survey)
		}
	})
}

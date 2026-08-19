package ports

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"time"
)

type EventRepository interface {
	GetEvents(ctx context.Context) ([]domain.Event, error)
	GetEventByID(ctx context.Context, id string) (*domain.Event, error)
	GetWorkshopsByEventID(ctx context.Context, eventID string) ([]domain.Workshop, error)
	ListCategories(ctx context.Context) ([]domain.EventCategory, error)
	CreateEvent(ctx context.Context, event *domain.Event) error
	UpdateEvent(ctx context.Context, event *domain.Event) error
	DeleteEvent(ctx context.Context, id string) error
	CreateWorkshop(ctx context.Context, workshop *domain.Workshop) error
	DeleteWorkshopsByEventID(ctx context.Context, eventID string) error
	CreateRegistration(ctx context.Context, registration *domain.Registration) error
	AddRegistrationWorkshops(ctx context.Context, registrationID string, workshopIDs []string) error
	GetRegistrationByUserAndEvent(ctx context.Context, userID, eventID string) (*domain.Registration, error)
	// ConfirmEventRegistration la usa el servicio de pagos al aprobarse el
	// cobro. Devuelve domain.ErrNotFound si no había inscripción que confirmar.
	ConfirmEventRegistration(ctx context.Context, userID, eventID string) error
	GetRegistrationsByUser(ctx context.Context, userID string) ([]domain.UserRegistration, error)
	// GetRegistrationsByEvent devuelve los inscritos de un evento con sus datos
	// personales todavía cifrados: descifrar es cosa del servicio, que es quien
	// tiene el CryptoService.
	GetRegistrationsByEvent(ctx context.Context, eventID string) ([]domain.EventRegistrant, error)
	CreateWorkshopRating(ctx context.Context, rating *domain.WorkshopRating) error
	GetRatingsByEventID(ctx context.Context, eventID string) ([]domain.WorkshopRating, error)

	// Encuesta general del evento
	CreateEventSurvey(ctx context.Context, survey *domain.EventSurvey) error
	GetEventSurveyByUser(ctx context.Context, eventID, userID string) (*domain.EventSurvey, error)
	GetEventSurveySummary(ctx context.Context, eventID string) (*domain.EventSurveySummary, error)
	GetAgenda(ctx context.Context, userID string) ([]domain.Workshop, error)
	AddToAgenda(ctx context.Context, userID, workshopID string) error
	RemoveFromAgenda(ctx context.Context, userID, workshopID string) error

	// QR & Attendance
	GetRegistrationByQR(ctx context.Context, qrData string) (*domain.Registration, *domain.CheckInResponse, error)
	// RecordAttendance registra la entrada y devuelve cuándo entró esa
	// inscripción —la primera vez, no la de esta lectura— y si ya había
	// entrado antes. Es idempotente: el mismo QR dos veces deja una sola
	// asistencia.
	RecordAttendance(ctx context.Context, registrationID, staffID string) (time.Time, bool, error)
	GetWorkshopsByRegistrationID(ctx context.Context, registrationID string) ([]domain.Workshop, error)
}

type EventService interface {
	ListEvents(ctx context.Context) ([]domain.Event, error)
	GetEventDetails(ctx context.Context, id string) (*domain.Event, error)
	ListCategories(ctx context.Context) ([]domain.EventCategory, error)
	CreateEvent(ctx context.Context, event *domain.Event) error
	UpdateEvent(ctx context.Context, event *domain.Event) error
	DeleteEvent(ctx context.Context, id string) error
	RegisterUser(ctx context.Context, reg *domain.Registration) error
	// GetMyRegistrations alimenta la pantalla "Mi credencial": todos los eventos
	// en los que el usuario está inscrito, cada uno con su QR.
	GetMyRegistrations(ctx context.Context, userID string) ([]domain.UserRegistration, error)
	// GetEventRegistrants es la lista de inscritos de un evento, para quien lo
	// organiza. Va bajo AdminOnly: son datos personales de terceros.
	GetEventRegistrants(ctx context.Context, eventID string) ([]domain.EventRegistrant, error)
	SubmitWorkshopRating(ctx context.Context, rating *domain.WorkshopRating) error
	GetEventFeedback(ctx context.Context, eventID string) ([]domain.WorkshopRating, error)

	// Encuesta general del evento
	SubmitEventSurvey(ctx context.Context, survey *domain.EventSurvey) error
	GetMyEventSurvey(ctx context.Context, eventID, userID string) (*domain.EventSurvey, error)
	GetEventSurveySummary(ctx context.Context, eventID string) (*domain.EventSurveySummary, error)
	GetAgenda(ctx context.Context, userID string) ([]domain.Workshop, error)
	AddToAgenda(ctx context.Context, userID, workshopID string) error
	RemoveFromAgenda(ctx context.Context, userID, workshopID string) error

	// QR & Attendance
	CheckIn(ctx context.Context, qrData, staffID string) (*domain.CheckInResponse, error)
}

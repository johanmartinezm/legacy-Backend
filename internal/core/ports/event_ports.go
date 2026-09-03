package ports

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"time"
)

type EventRepository interface {
	GetEvents(ctx context.Context, incluirInactivos bool) ([]domain.Event, error)
	GetEventByID(ctx context.Context, id string) (*domain.Event, error)
	GetWorkshopsByEventID(ctx context.Context, eventID string) ([]domain.Workshop, error)
	ListCategories(ctx context.Context) ([]domain.EventCategory, error)
	CreateEvent(ctx context.Context, event *domain.Event) error
	UpdateEvent(ctx context.Context, event *domain.Event) error
	// UpdateEventStatus cambia solo la visibilidad. Aparte de UpdateEvent a
	// propósito: el formulario del panel no envía `status`, y meterlo en aquel
	// UPDATE lo borraría en cada guardado. Devuelve domain.ErrNotFound si el id
	// no existe.
	UpdateEventStatus(ctx context.Context, id, status string) error
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
	// tiene el CryptoService. Pagina: el orden es total (fecha e id) para que
	// dos páginas seguidas no se solapen ni se salten filas.
	GetRegistrationsByEvent(ctx context.Context, eventID string, limit, offset int) ([]domain.EventRegistrant, error)
	// CountRegistrationsByEvent es el total, para saber cuántas páginas hay.
	CountRegistrationsByEvent(ctx context.Context, eventID string) (int, error)
	// GetRegistrationsSinCredencial devuelve las inscripciones de un evento a
	// las que les falta el código de acceso. Con `ids` vacío son todas las del
	// evento; con ids, solo esas —es la misma consulta para la acción en bloque
	// y para la de una sola persona—.
	//
	// El estado es `qr_data IS NULL`, no una columna aparte: no hay dos sitios
	// que puedan decir cosas distintas.
	GetRegistrationsSinCredencial(ctx context.Context, eventID string, ids []string) ([]domain.Registration, error)
	// SetRegistrationQR escribe el código que faltaba. No pisa uno existente:
	// cambiarle el QR a quien ya lo tiene invalidaría el que lleva encima.
	SetRegistrationQR(ctx context.Context, registrationID, qrData string) error
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
	ListEvents(ctx context.Context, incluirInactivos bool) ([]domain.Event, error)
	GetEventDetails(ctx context.Context, id string) (*domain.Event, error)
	ListCategories(ctx context.Context) ([]domain.EventCategory, error)
	CreateEvent(ctx context.Context, event *domain.Event) error
	UpdateEvent(ctx context.Context, event *domain.Event) error
	// UpdateEventStatus oculta o vuelve a mostrar un evento en la app. Solo
	// acepta domain.EventoActivo y domain.EventoInactivo.
	UpdateEventStatus(ctx context.Context, id, status string) error
	DeleteEvent(ctx context.Context, id string) error
	RegisterUser(ctx context.Context, reg *domain.Registration) error
	// GetMyRegistrations alimenta la pantalla "Mi credencial": todos los eventos
	// en los que el usuario está inscrito, cada uno con su QR.
	GetMyRegistrations(ctx context.Context, userID string) ([]domain.UserRegistration, error)
	// GetEventRegistrants es la lista de inscritos de un evento, para quien lo
	// organiza. Va bajo AdminOnly: son datos personales de terceros.
	//
	// Devuelve también el total de inscritos, que no es el largo de la página:
	// el panel lo necesita para pintar el paginador.
	GetEventRegistrants(ctx context.Context, eventID string, limit, offset int) ([]domain.EventRegistrant, int, error)
	// GenerarCredenciales rellena el código que falta y, si se pide, manda el
	// correo con el QR. Es la vuelta del interruptor de la carga masiva: quien
	// se importó sin credencial no pasa el check-in hasta pasar por aquí.
	//
	// Con `registrationIDs` vacío alcanza a todos los inscritos del evento a
	// los que les falta; con ids, solo a esos. Devuelve cuántas generó.
	GenerarCredenciales(ctx context.Context, eventID string, registrationIDs []string, avisarPorCorreo bool) (int, error)
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

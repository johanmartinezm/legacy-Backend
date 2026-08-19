package domain

import (
	"time"
)

type EventCategory struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	OrderIndex  int       `json:"orderIndex" db:"order_index"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type Event struct {
	ID             string     `json:"id" db:"id"`
	CategoryID     string     `json:"category_id" db:"category_id"`
	Category       string     `json:"category" db:"-"` // Added for frontend convenience
	Title          string     `json:"title" db:"title"`
	Description    *string    `json:"description" db:"description"`
	ImageUrl       *string    `json:"imageUrl" db:"image_url"`
	Location       *string    `json:"location" db:"location"`
	// IsVirtual decide qué recibe quien se inscribe: los presenciales dan QR de
	// acceso; los virtuales, el enlace de la sesión. Añadido el 2026-08-18
	// (scripts/20260818_modalidad_y_enlace_evento.sql); antes se emitía QR para
	// todo, también para una masterclass virtual, donde no sirve de nada.
	IsVirtual bool `json:"isVirtual" db:"is_virtual"`
	// AccessURL es el enlace de la sesión. NULL en los presenciales, y **solo se
	// entrega a inscripciones confirmadas**: verlo equivale a poder entrar.
	AccessURL   *string `json:"accessUrl" db:"access_url"`
	SpeakerMain *string `json:"speaker" db:"speaker_main"`
	StartDate      time.Time  `json:"date" db:"start_date"`
	EndDate        *time.Time `json:"end_date" db:"end_date"`
	Price          float64    `json:"price" db:"price"`
	PriceLabel     string     `json:"priceLabel" db:"-"`
	IsFree         bool       `json:"isFree" db:"is_free"`
	ActionStatus   string     `json:"actionStatus" db:"action_status"`
	ButtonText     string     `json:"buttonText" db:"button_text"`
	AttendeesLimit *int       `json:"attendees_limit" db:"attendees_limit"`
	Includes       *string    `json:"includes" db:"includes"`
	CategoryOrder  int        `json:"categoryOrder" db:"-"`
	Workshops      []Workshop `json:"workshops" db:"-"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

type Workshop struct {
	ID            string    `json:"id" db:"id"`
	EventID       string    `json:"eventId" db:"event_id"`
	EventTitle    string    `json:"eventTitle" db:"-"`
	Name          string    `json:"name" db:"name"`
	Description   string    `json:"description" db:"description"`
	Room          string    `json:"room" db:"room"`
	Speaker       string    `json:"speaker" db:"speaker"`
	ImageUrl      string    `json:"imageUrl" db:"image_url"`
	StartDateTime time.Time `json:"startDateTime" db:"start_date_time"`
	EndDateTime   time.Time `json:"endDateTime" db:"end_date_time"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// Estados de una inscripción. Se corresponden con el CHECK de
// events.registrations.registration_status.
const (
	// RegistrationConfirmed: el evento es gratuito, o el de pago ya se pagó.
	RegistrationConfirmed = "confirmed"
	// RegistrationPendingPayment: evento de pago sin pagar todavía. La
	// inscripción existe —de ahí se sabe quién intentó comprar— pero no da
	// derecho a entrar.
	RegistrationPendingPayment = "pending_payment"
)

type Registration struct {
	ID                  string     `json:"id" db:"id"`
	UserID              string     `json:"user_id" db:"user_id"`
	EventID             string     `json:"event_id" db:"event_id"`
	PaymentStatus       string     `json:"payment_status" db:"payment_status"`
	RegistrationStatus  string     `json:"registration_status" db:"registration_status"`
	RegistrationDate    time.Time  `json:"registration_date" db:"registration_date"`
	QRData              string     `json:"qr_data" db:"qr_data"`
	TotalPaid           float64    `json:"total_paid" db:"total_paid"`
	AttendanceConfirmed bool       `json:"attendance_confirmed" db:"attendance_confirmed"`
	Workshops           []Workshop `json:"workshops" db:"-"`

	// Contacto de quien asiste, para este evento concreto. Puede diferir del
	// perfil —un correo de trabajo, otro teléfono— y es lo que usa quien
	// organiza si alguien no aparece.
	//
	// **No cambian de quién es la entrada:** el titular sigue siendo UserID.
	// Vacíos significan "usa los del perfil". Se guardan cifrados.
	ParticipantName  string `json:"participant_name" db:"participant_name"`
	ParticipantEmail string `json:"participant_email" db:"participant_email"`
	ParticipantPhone string `json:"participant_phone" db:"participant_phone"`
}

// IsPendingPayment indica si la inscripción está a la espera del pago. Se usa
// para no dar por buena una entrada que nadie ha pagado.
func (r *Registration) IsPendingPayment() bool {
	return r.RegistrationStatus == RegistrationPendingPayment
}

// UserRegistration es una inscripción con los datos del evento que hacen falta
// para pintarla en la credencial del usuario, sin obligar a pedir cada evento
// por separado.
// CorreoInscripcion es lo que necesita la plantilla del correo de confirmación.
//
// EnlaceLugar guarda dos cosas según EsVirtual: el enlace de la sesión o la
// ubicación física. Van en el mismo campo porque la plantilla las pinta en el
// mismo sitio y nunca coexisten; tener dos campos invitaría a rellenar el que no
// toca. Puede ir vacío, y entonces la plantilla omite ese bloque.
type CorreoInscripcion struct {
	Para        string
	Nombre      string
	Evento      string
	Fecha       string
	EsVirtual   bool
	EnlaceLugar string
}

// CorreoPago es la confirmación que se manda cuando la pasarela aprueba el cobro
// de un evento.
//
// Es distinta de CorreoInscripcion, que sale al reservar cupo en un evento
// gratuito: esta lleva además lo que se pagó y, sobre todo, **el código de
// acceso**. Antes de existir, quien pagaba no recibía nada: ni constancia del
// cobro ni forma de entrar al evento sin abrir la app.
type CorreoPago struct {
	Para   string
	Nombre string

	// Evento y fecha de lo comprado.
	Evento string
	Fecha  string

	// Datos del pago. Referencia es el identificador de la orden en la pasarela,
	// que es por donde se rastrea un cobro con el banco.
	Importe     float64
	Moneda      string
	Referencia  string
	Metodo      string
	PagadoEl    string
	EsVirtual   bool
	EnlaceLugar string

	// QRData es el código de acceso en crudo. La plantilla lo dibuja como imagen;
	// va vacío en los eventos virtuales, que entran por enlace.
	QRData string
}

type UserRegistration struct {
	ID                  string     `json:"id"`
	EventID             string     `json:"eventId"`
	EventTitle          string     `json:"eventTitle"`
	EventLocation       *string    `json:"eventLocation"`
	EventIsVirtual      bool       `json:"eventIsVirtual"`
	EventStartDate      time.Time  `json:"eventStartDate"`
	EventEndDate        *time.Time `json:"eventEndDate"`
	EventImageUrl       *string    `json:"eventImageUrl"`
	PaymentStatus       string     `json:"paymentStatus"`
	RegistrationStatus  string     `json:"registrationStatus"`
	RegistrationDate    time.Time  `json:"registrationDate"`
	// QRData va vacío en los virtuales y en las inscripciones sin confirmar.
	QRData string `json:"qrData"`
	// AccessURL es el enlace de la sesión virtual. Vacío en los presenciales y
	// en cualquier inscripción sin confirmar: verlo equivale a poder entrar, así
	// que sigue exactamente la misma regla que el QR.
	AccessURL string `json:"accessUrl"`
	TotalPaid           float64    `json:"totalPaid"`
	AttendanceConfirmed bool       `json:"attendanceConfirmed"`
}

// EventRegistrant es una inscripción vista desde la organización del evento:
// quién es la persona, en qué estado está su pago y si ya pasó por la puerta.
//
// No lleva el qr_data. Quien organiza necesita saber quién viene y quién debe,
// no el código de entrada de cada asistente; dejarlo fuera evita que una lista
// que se comparte por correo o se exporta acabe repartiendo credenciales.
type EventRegistrant struct {
	RegistrationID string `json:"registrationId"`
	UserID         string `json:"userId"`
	// FirstName y LastName no salen al JSON: son el paso intermedio entre el
	// repositorio, que los lee cifrados y por separado, y el servicio, que los
	// descifra y compone FullName. Concatenarlos antes de descifrar produciría
	// un texto que ya no se puede abrir.
	FirstName           string    `json:"-"`
	LastName            string    `json:"-"`
	FullName            string    `json:"fullName"`
	Email               string    `json:"email"`
	Phone               string    `json:"phone"`
	PaymentStatus       string    `json:"paymentStatus"`
	RegistrationStatus  string    `json:"registrationStatus"`
	RegistrationDate    time.Time `json:"registrationDate"`
	TotalPaid           float64   `json:"totalPaid"`
	AttendanceConfirmed bool      `json:"attendanceConfirmed"`
}

type WorkshopRating struct {
	ID           string    `json:"id" db:"id"`
	WorkshopID   string    `json:"workshopId" db:"workshop_id"`
	WorkshopName string    `json:"workshopName" db:"-"`
	UserID       string    `json:"userId" db:"user_id"`
	Rating       int       `json:"rating" db:"rating"`
	Comment      string    `json:"comment" db:"comment"`
	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
}

// EventSurvey es la encuesta general de un evento: una respuesta por usuario y
// evento. Distinta de WorkshopRating, que califica una charla suelta.
//
// Solo OverallRating es obligatorio; el resto son punteros porque el usuario
// puede dejarlos en blanco y un 0 no es lo mismo que "sin responder".
type EventSurvey struct {
	ID                 string    `json:"id" db:"id"`
	EventID            string    `json:"eventId" db:"event_id"`
	UserID             string    `json:"userId" db:"user_id"`
	OverallRating      int       `json:"overallRating" db:"overall_rating"`
	OrganizationRating *int      `json:"organizationRating" db:"organization_rating"`
	ContentRating      *int      `json:"contentRating" db:"content_rating"`
	SpeakersRating     *int      `json:"speakersRating" db:"speakers_rating"`
	WouldRecommend     *bool     `json:"wouldRecommend" db:"would_recommend"`
	Comment            *string   `json:"comment" db:"comment"`
	CreatedAt          time.Time `json:"createdAt" db:"created_at"`
}

// EventSurveyComment es un comentario suelto dentro del resumen, sin el usuario
// que lo escribió: el panel muestra la opinión, no quién la firmó.
type EventSurveyComment struct {
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
}

// EventSurveySummary es lo que lee el panel. Los promedios son punteros porque
// las preguntas opcionales pueden no tener ni una sola respuesta, y en ese caso
// un 0 se leería como "pésimo" en vez de "sin datos".
type EventSurveySummary struct {
	EventID             string               `json:"eventId"`
	Responses           int                  `json:"responses"`
	OverallAverage      *float64             `json:"overallAverage"`
	OrganizationAverage *float64             `json:"organizationAverage"`
	ContentAverage      *float64             `json:"contentAverage"`
	SpeakersAverage     *float64             `json:"speakersAverage"`
	RecommendRate       *float64             `json:"recommendRate"`
	Comments            []EventSurveyComment `json:"comments"`
}

type CheckInResponse struct {
	RegistrationID string `json:"registrationId"`
	// FirstName y LastName no salen al JSON, igual que en EventRegistrant: son
	// el paso intermedio entre el repositorio, que los lee cifrados y por
	// separado, y el servicio, que los descifra y compone UserName. El panel
	// sigue recibiendo userName y userEmail, solo que legibles.
	FirstName   string     `json:"-"`
	LastName    string     `json:"-"`
	UserName    string     `json:"userName"`
	UserEmail   string     `json:"userEmail"`
	EventTitle  string     `json:"eventTitle"`
	CheckInTime time.Time  `json:"checkInTime"`
	Workshops   []Workshop `json:"workshops"`
}

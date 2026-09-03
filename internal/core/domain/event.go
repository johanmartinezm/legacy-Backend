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
	ID          string  `json:"id" db:"id"`
	CategoryID  string  `json:"category_id" db:"category_id"`
	Category    string  `json:"category" db:"-"` // Added for frontend convenience
	Title       string  `json:"title" db:"title"`
	Description *string `json:"description" db:"description"`
	ImageUrl    *string `json:"imageUrl" db:"image_url"`
	Location    *string `json:"location" db:"location"`
	// IsVirtual decide qué recibe quien se inscribe: los presenciales dan QR de
	// acceso; los virtuales, el enlace de la sesión. Añadido el 2026-08-18
	// (scripts/20260818_modalidad_y_enlace_evento.sql); antes se emitía QR para
	// todo, también para una masterclass virtual, donde no sirve de nada.
	IsVirtual bool `json:"isVirtual" db:"is_virtual"`
	// AccessURL es el enlace de la sesión. NULL en los presenciales, y **solo se
	// entrega a inscripciones confirmadas**: verlo equivale a poder entrar.
	AccessURL    *string    `json:"accessUrl" db:"access_url"`
	SpeakerMain  *string    `json:"speaker" db:"speaker_main"`
	StartDate    time.Time  `json:"date" db:"start_date"`
	EndDate      *time.Time `json:"end_date" db:"end_date"`
	Price        float64    `json:"price" db:"price"`
	PriceLabel   string     `json:"priceLabel" db:"-"`
	IsFree       bool       `json:"isFree" db:"is_free"`
	ActionStatus string     `json:"actionStatus" db:"action_status"`
	// Status decide si el evento se ve en la app: solo salen los `active`
	// (event_repository.go:66). Se lee aquí para que el panel pueda mostrar
	// cuál está oculto, pero **no se escribe por UpdateEvent**: el formulario
	// del panel no lo envía, así que incluirlo en el UPDATE lo dejaría vacío en
	// cada guardado y el evento desaparecería de la app al editarlo. Cambiarlo
	// es una acción aparte, PUT /api/events/{id}/status.
	Status         string     `json:"status" db:"status"`
	ButtonText     string     `json:"buttonText" db:"button_text"`
	AttendeesLimit *int       `json:"attendees_limit" db:"attendees_limit"`
	Includes       *string    `json:"includes" db:"includes"`
	CategoryOrder  int        `json:"categoryOrder" db:"-"`
	Workshops      []Workshop `json:"workshops" db:"-"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// OcultarEnlaceDeAcceso borra el enlace de la sesión virtual de este evento.
//
// El enlace **equivale a poder entrar**, así que solo debe verlo quien tiene una
// inscripción confirmada —eso lo resuelve GetMyRegistrations— y el panel, que
// necesita el valor actual para no borrarlo al editar.
//
// Hasta el 2026-08-20 salía en `GET /api/events` y en el detalle, que son rutas
// públicas: cualquiera sin sesión podía sacar la URL de una masterclass de pago.
// F12.15 no lo detectó porque comprobaba el otro camino, el de las
// inscripciones, donde la protección sí existía.
func (e *Event) OcultarEnlaceDeAcceso() {
	e.AccessURL = nil
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

// Estados de un evento. Se corresponden con el CHECK de events.events.status.
const (
	// EventoActivo: se ve en la app. Es el valor por defecto de la columna, así
	// que todo evento creado desde el panel nace activo.
	EventoActivo = "active"
	// EventoInactivo: sigue existiendo y quien ya está inscrito conserva su
	// credencial —GetEventByID no filtra—, pero desaparece del listado público.
	EventoInactivo = "inactive"
)

// EstadoDeEventoValido rechaza cualquier otro valor antes de que llegue a la
// base. Sin esta guarda, un estado escrito a mano ("activo", "") no lo atrapa
// nadie: no pasaría el filtro `= 'active'` y el evento se quedaría oculto sin
// que la pantalla mostrara nada raro.
func EstadoDeEventoValido(estado string) bool {
	return estado == EventoActivo || estado == EventoInactivo
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

	// Importacion son las decisiones que solo toma una carga masiva. No se
	// persiste ni entra por el JSON: la app deja este campo en su valor cero y
	// entonces RegisterUser se comporta exactamente como siempre.
	Importacion AltaImportada `json:"-" db:"-"`

	// YaEstabaInscrito lo rellena RegisterUser al salir: dice si encontró la
	// inscripción hecha en vez de crearla. Es lo que permite que el informe de
	// una carga distinga «inscritas» de «ya inscritas» sin consultar el evento
	// fila por fila. No se persiste ni sale al JSON.
	YaEstabaInscrito bool `json:"-" db:"-"`
}

// AvisoDeAlta dice qué correo sale al crear una inscripción confirmada.
//
// Existe porque la carga masiva necesita las tres respuestas —el de siempre,
// ninguno, o el de credencial— y la app solo la primera, que es el valor cero.
type AvisoDeAlta string

const (
	// AvisoPorDefecto es lo que hace la app: el correo de inscripción de
	// siempre, que remite a la credencial y nunca lleva el QR.
	AvisoPorDefecto AvisoDeAlta = ""
	// AvisoNinguno no manda nada. Sin él, importar trescientas personas
	// dispararía trescientos correos que nadie pidió.
	AvisoNinguno AvisoDeAlta = "ninguno"
	// AvisoCredencial manda el correo con el QR dibujado. Solo sale de una
	// carga o de la acción «Generar credenciales», nunca de la app.
	AvisoCredencial AvisoDeAlta = "credencial"
)

// AltaImportada agrupa lo que la carga masiva decide por inscripción
// (reports/20260826_plan_carga_masiva.md §4.1). Va junto y no como cuatro
// campos sueltos para que se lea de un vistazo qué es de la importación y qué
// no.
type AltaImportada struct {
	// EsCarga distingue una inscripción venida del importador de una hecha
	// desde la app, que deja toda esta estructura en su valor cero.
	//
	// Hace falta para una regla que no se puede leer de los otros campos: en un
	// evento virtual una carga **nunca** genera el código, se pida o no. Allí no
	// hay puerta que cruzar y el QR no se muestra jamás, así que crearlo sería
	// escribir un valor que nadie va a mirar. La app sigue como estaba.
	EsCarga bool

	// SinCredencial pide que **no** se genere el código. Entonces qr_data queda
	// en NULL, y ese es todo el estado: no hay ninguna columna que pueda decir
	// otra cosa. La consecuencia es que esa persona no pasa el check-in hasta
	// que se le genere, y por eso existe la acción «Generar credenciales».
	SinCredencial bool
	Aviso         AvisoDeAlta

	// Usuario y Contrasena son las credenciales de acceso que se escriben en el
	// correo de credencial. Solo se rellenan para una cuenta **recién creada**
	// por la carga —su contraseña es su número de documento y no hay otro sitio
	// donde se entere—. Para una cuenta que ya existía van vacías: esa persona
	// tiene su propia contraseña y decirle otra cosa sería mentirle.
	Usuario    string
	Contrasena string
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

// CorreoCredencial entrega el código de acceso a quien ya está inscrito.
//
// Sale de un solo sitio —generar la credencial— se dispare desde la carga
// masiva o desde la acción «Generar credenciales» de la pantalla de inscritos:
// crear el código y avisar de él son la misma cosa.
//
// **Contradice a propósito** el comentario de SendEventRegistrationEmail
// —«nunca se manda el QR por correo»—. El de pago ya lo contradecía con el
// mismo criterio: quien ya tiene su entrada recibe su acceso. Lo amortigua que
// el código es aleatorio, así que tener uno no permite fabricar el de otro, y
// que CheckIn es idempotente: un QR reenviado no cuela a dos personas.
// Ver reports/20260826_plan_carga_masiva.md §4.3.
type CorreoCredencial struct {
	Para   string
	Nombre string

	Evento string
	Fecha  string

	EsVirtual   bool
	EnlaceLugar string

	// QRData es el código en crudo; la plantilla lo dibuja. Vacío en un evento
	// virtual, que entra por enlace.
	QRData string

	// Usuario y Contrasena solo se rellenan cuando la cuenta acaba de crearse
	// en una carga masiva: entonces este correo es **el único sitio** donde la
	// persona se entera de cómo entrar. Vacíos, la plantilla omite ese bloque.
	Usuario    string
	Contrasena string
}

type UserRegistration struct {
	ID                 string     `json:"id"`
	EventID            string     `json:"eventId"`
	EventTitle         string     `json:"eventTitle"`
	EventLocation      *string    `json:"eventLocation"`
	EventIsVirtual     bool       `json:"eventIsVirtual"`
	EventStartDate     time.Time  `json:"eventStartDate"`
	EventEndDate       *time.Time `json:"eventEndDate"`
	EventImageUrl      *string    `json:"eventImageUrl"`
	PaymentStatus      string     `json:"paymentStatus"`
	RegistrationStatus string     `json:"registrationStatus"`
	RegistrationDate   time.Time  `json:"registrationDate"`
	// QRData va vacío en los virtuales y en las inscripciones sin confirmar.
	QRData string `json:"qrData"`
	// AccessURL es el enlace de la sesión virtual. Vacío en los presenciales y
	// en cualquier inscripción sin confirmar: verlo equivale a poder entrar, así
	// que sigue exactamente la misma regla que el QR.
	AccessURL           string  `json:"accessUrl"`
	TotalPaid           float64 `json:"totalPaid"`
	AttendanceConfirmed bool    `json:"attendanceConfirmed"`
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
	// TieneCredencial dice si esa inscripción tiene código de acceso, no cuál
	// es: sigue sin salir el qr_data, por lo mismo de siempre. Lo necesita el
	// panel para marcar en la lista a quién le falta, porque si no el que se
	// entera es quien está en la puerta el día del evento.
	TieneCredencial bool `json:"tieneCredencial"`
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
	FirstName  string `json:"-"`
	LastName   string `json:"-"`
	UserName   string `json:"userName"`
	UserEmail  string `json:"userEmail"`
	EventTitle string `json:"eventTitle"`
	// CheckInTime es la hora de la PRIMERA entrada de esa inscripción, no la de
	// la lectura que se acaba de hacer. En una relectura es lo que interesa en
	// la puerta: a qué hora entró esta persona.
	CheckInTime time.Time `json:"checkInTime"`
	// AlreadyCheckedIn avisa de que ese QR ya se había usado. Hasta el
	// 2026-08-19 las dos respuestas eran idénticas —y además quedaban dos
	// asistencias registradas—, así que una entrada repetida no se distinguía
	// de una nueva.
	AlreadyCheckedIn bool       `json:"alreadyCheckedIn"`
	Workshops        []Workshop `json:"workshops"`
}

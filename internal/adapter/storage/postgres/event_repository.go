package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type EventRepository struct {
	db *pgxpool.Pool
}

func NewEventRepository(db *pgxpool.Pool) *EventRepository {
	return &EventRepository{db: db}
}

func (r *EventRepository) ListCategories(ctx context.Context) ([]domain.EventCategory, error) {
	query := `SELECT id, name, description, order_index, created_at FROM events.categories ORDER BY order_index ASC, name ASC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []domain.EventCategory
	for rows.Next() {
		var c domain.EventCategory
		err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.OrderIndex, &c.CreatedAt)
		if err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

// GetEvents devuelve el listado completo de eventos.
//
// LEFT JOIN y no JOIN: events.category_id es anulable, y con un JOIN un evento
// sin categoría —o con una categoría borrada— desaparecía del listado sin ningún
// error. Nadie se enteraba: la respuesta era un 200 con un evento de menos.
//
// Los COALESCE son la contrapartida obligatoria del LEFT JOIN: domain.Event
// declara CategoryID, Category y CategoryOrder como valores no anulables, así que
// un nulo rompería el Scan. El 9999 del orden manda los eventos sin categoría al
// final de la lista, que es donde estorban menos.
// El filtro por `status` existía en la tabla desde el principio y esta consulta
// lo ignoraba, así que **todo** evento salía en el listado público sin importar
// su estado. No había forma de retirar un evento de la app sin borrarlo: los de
// verificación interna se veían igual que los reales.
//
// `incluirInactivos` lo decide quien llama: el panel necesita verlos todos para
// poder reactivarlos, y consume este mismo endpoint.
//
// Ojo: GetEventByID **no** filtra a propósito. Lo usan los pagos, las
// inscripciones y las encuestas; si un evento inactivo dejara de resolverse por
// id, quien ya estuviera inscrito perdería su credencial y la confirmación de
// pago fallaría.
func (r *EventRepository) GetEvents(ctx context.Context, incluirInactivos bool) ([]domain.Event, error) {
	filtro := "WHERE e.status = 'active'"
	if incluirInactivos {
		filtro = ""
	}
	query := `
		SELECT e.id, COALESCE(e.category_id::text, '') AS category_id,
		       COALESCE(c.name, '') AS category, e.title, e.description, e.image_url,
		       e.location, e.is_virtual, e.access_url, e.speaker_main, e.start_date, e.end_date, e.price, e.is_free,
		       e.action_status, e.button_text, e.attendees_limit, e.includes,
		       COALESCE(e.status, '') AS status,
		       COALESCE(c.order_index, 9999) AS order_index
		FROM events.events e
		LEFT JOIN events.categories c ON e.category_id = c.id
		` + filtro + `
		ORDER BY COALESCE(c.order_index, 9999) ASC, e.start_date ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		var e domain.Event
		err := rows.Scan(
			&e.ID, &e.CategoryID, &e.Category, &e.Title, &e.Description, &e.ImageUrl,
			&e.Location, &e.IsVirtual, &e.AccessURL, &e.SpeakerMain, &e.StartDate, &e.EndDate, &e.Price, &e.IsFree,
			&e.ActionStatus, &e.ButtonText, &e.AttendeesLimit, &e.Includes, &e.Status, &e.CategoryOrder,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

// GetEventByID busca un evento concreto.
//
// Mismo LEFT JOIN que GetEvents, y por un motivo peor: con el JOIN, pedir el
// detalle de un evento sin categoría devolvía ErrNotFound, o sea un 404 sobre un
// evento que existe. Quien tuviera el enlace veía "no encontrado".
func (r *EventRepository) GetEventByID(ctx context.Context, id string) (*domain.Event, error) {
	query := `
		SELECT e.id, COALESCE(e.category_id::text, '') AS category_id,
		       COALESCE(c.name, '') AS category, e.title, e.description, e.image_url,
		       e.location, e.is_virtual, e.access_url, e.speaker_main, e.start_date, e.end_date, e.price, e.is_free,
		       e.action_status, e.button_text, e.attendees_limit, e.includes,
		       COALESCE(e.status, '') AS status,
		       COALESCE(c.order_index, 9999) AS order_index
		FROM events.events e
		LEFT JOIN events.categories c ON e.category_id = c.id
		WHERE e.id = $1
	`
	var e domain.Event
	err := r.db.QueryRow(ctx, query, id).Scan(
		&e.ID, &e.CategoryID, &e.Category, &e.Title, &e.Description, &e.ImageUrl,
		&e.Location, &e.IsVirtual, &e.AccessURL, &e.SpeakerMain, &e.StartDate, &e.EndDate, &e.Price, &e.IsFree,
		&e.ActionStatus, &e.ButtonText, &e.AttendeesLimit, &e.Includes, &e.Status, &e.CategoryOrder,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &e, nil
}

func (r *EventRepository) GetWorkshopsByEventID(ctx context.Context, eventID string) ([]domain.Workshop, error) {
	query := `
		SELECT 
			id, event_id, '' as event_title, name, 
			COALESCE(description, '') as description, 
			COALESCE(room, '') as room, 
			COALESCE(speaker, '') as speaker, 
			COALESCE(image_url, '') as image_url, 
			start_date_time, end_date_time
		FROM events.workshops
		WHERE event_id = $1::uuid
		ORDER BY start_date_time ASC
	`
	rows, err := r.db.Query(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workshops []domain.Workshop
	for rows.Next() {
		var w domain.Workshop
		err := rows.Scan(
			&w.ID, &w.EventID, &w.EventTitle, &w.Name, &w.Description, &w.Room, &w.Speaker, &w.ImageUrl,
			&w.StartDateTime, &w.EndDateTime,
		)
		if err != nil {
			return nil, err
		}
		workshops = append(workshops, w)
	}
	return workshops, nil
}

func (r *EventRepository) CreateRegistration(ctx context.Context, reg *domain.Registration) error {
	query := `
		INSERT INTO events.registrations (
			user_id, event_id, payment_status, registration_status, qr_data, total_paid,
			participant_name, participant_email, participant_phone
		)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''))
		RETURNING id, registration_date
	`
	// NULLIF sobre qr_data, y no es cosmético: `registrations_qr_data_key` es
	// UNIQUE (20260805_qr_data_impredecible.sql) y la cadena vacía SÍ colisiona
	// consigo misma, mientras que NULL no. Desde que la carga masiva puede
	// crear inscripciones sin credencial, la segunda de ellas violaría la
	// restricción. Es exactamente el mismo caso que el alias en core.users.
	return r.db.QueryRow(ctx, query,
		reg.UserID, reg.EventID, reg.PaymentStatus, reg.RegistrationStatus, reg.QRData, reg.TotalPaid,
		reg.ParticipantName, reg.ParticipantEmail, reg.ParticipantPhone,
	).Scan(&reg.ID, &reg.RegistrationDate)
}

// ConfirmEventRegistration pasa la inscripción a pagada y confirmada. La llama
// el servicio de pagos cuando la pasarela aprueba la transacción.
//
// Devuelve domain.ErrNotFound si no había inscripción que confirmar: eso
// significa que se pagó sin haberse inscrito antes, y es una incoherencia que
// el llamador debe poder distinguir de un fallo de base de datos.
func (r *EventRepository) ConfirmEventRegistration(ctx context.Context, userID, eventID string) error {
	query := `
		UPDATE events.registrations
		   SET payment_status = 'paid',
		       registration_status = $1
		 WHERE user_id = $2 AND event_id = $3
	`
	tag, err := r.db.Exec(ctx, query, domain.RegistrationConfirmed, userID, eventID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// GetRegistrationsByEvent devuelve los inscritos de un evento para el panel.
//
// Los nombres y el correo salen tal como están en la base, es decir cifrados:
// quien descifra es EventService, que es el que tiene el CryptoService. Por eso
// tampoco se concatena el nombre en SQL —un `first_name || ' ' || last_name`
// junta dos textos cifrados en uno que ya no se puede descifrar por partes, que
// es justo lo que le pasa hoy a GetRegistrationByQR.
//
// Pagina desde el 2026-08-26. Antes traía todas las filas de una vez: un evento
// con miles de inscritos devolvía miles de filas, y **cada una se descifra
// después en el servicio**, así que el coste no era solo la consulta.
//
// El orden es `registration_date DESC, id DESC`. El id desempata a propósito:
// con solo la fecha, dos inscripciones del mismo instante —que las hay, porque
// la columna guarda hasta el microsegundo pero un lote de altas puede repetir—
// pueden salir en distinto orden en dos consultas, y entonces la página 2 se
// salta una fila o repite otra. Un orden total es lo que hace que paginar sea
// correcto, no solo rápido.
func (r *EventRepository) GetRegistrationsByEvent(ctx context.Context, eventID string, limit, offset int) ([]domain.EventRegistrant, error) {
	query := `
		SELECT r.id, r.user_id,
		       COALESCE(u.first_name, ''), COALESCE(u.last_name, ''),
		       COALESCE(u.email_encrypted, ''), COALESCE(u.phone, ''),
		       COALESCE(r.payment_status, ''), COALESCE(r.registration_status, ''),
		       r.registration_date, r.total_paid, r.attendance_confirmed,
		       COALESCE(r.qr_data, '') <> ''
		FROM events.registrations r
		JOIN core.users u ON r.user_id = u.id
		WHERE r.event_id = $1
		ORDER BY r.registration_date DESC, r.id DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, eventID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Se inicializa vacío y no nil para que un evento sin inscritos serialice
	// como [] y no como null: el panel recorre la lista sin comprobar nada.
	registrants := []domain.EventRegistrant{}
	for rows.Next() {
		var reg domain.EventRegistrant
		if err := rows.Scan(
			&reg.RegistrationID, &reg.UserID,
			&reg.FirstName, &reg.LastName,
			&reg.Email, &reg.Phone,
			&reg.PaymentStatus, &reg.RegistrationStatus, &reg.RegistrationDate,
			&reg.TotalPaid, &reg.AttendanceConfirmed, &reg.TieneCredencial,
		); err != nil {
			return nil, err
		}
		registrants = append(registrants, reg)
	}
	return registrants, rows.Err()
}

// CountRegistrationsByEvent es el total de inscritos, para que el panel sepa
// cuántas páginas hay.
//
// Va en consulta aparte y no como `COUNT(*) OVER ()` dentro del listado porque
// con OFFSET más allá de la última fila el listado no devuelve ninguna fila, y
// entonces tampoco devolvería el total: la pantalla se quedaría sin saber a
// dónde volver.
func (r *EventRepository) CountRegistrationsByEvent(ctx context.Context, eventID string) (int, error) {
	var total int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM events.registrations WHERE event_id = $1`, eventID).Scan(&total)
	return total, err
}

// GetRegistrationsSinCredencial trae las inscripciones a las que les falta el
// código de acceso, que es lo que deja una carga masiva con el interruptor de
// credencial apagado.
//
// El estado se lee de la propia fila —`qr_data IS NULL`— y no de una columna
// «credencial_entregada» que se valoró y se descartó: dos sitios que puedan
// decir cosas distintas acaban diciéndolas.
//
// `ids` vacío significa «todas las del evento». Es la misma consulta para la
// acción en bloque y para la de una sola persona, así que no hay dos caminos
// que puedan divergir.
func (r *EventRepository) GetRegistrationsSinCredencial(ctx context.Context, eventID string, ids []string) ([]domain.Registration, error) {
	query := `
		SELECT id, user_id, event_id,
		       COALESCE(payment_status, ''), COALESCE(registration_status, ''),
		       registration_date, total_paid, attendance_confirmed,
		       COALESCE(participant_name, ''), COALESCE(participant_email, ''),
		       COALESCE(participant_phone, '')
		FROM events.registrations
		WHERE event_id = $1
		  AND COALESCE(qr_data, '') = ''
		  AND (cardinality($2::text[]) = 0 OR id::text = ANY($2::text[]))
		ORDER BY registration_date, id
	`
	// El array vacío se manda como tal y la condición lo neutraliza, en vez de
	// componer dos SQL distintos según haya ids o no.
	if ids == nil {
		ids = []string{}
	}

	// La comparación va sobre `id::text` y no con un `::uuid[]`, por dos motivos
	// que se comprobaron contra la base:
	//
	//   - `[]string` de Go se codifica de forma natural como `text[]`; pedirle a
	//     pgx un `uuid[]` es depender de una conversión que aquí no hace falta.
	//   - un id mal formado en la lista hunde la consulta entera con un error de
	//     casteo si se pide `::uuid[]`, mientras que como texto simplemente no
	//     casa con nada. Los ids vienen del panel, que los sacó de esta misma
	//     API, pero el modo de fallar barato es el que se elige.

	rows, err := r.db.Query(ctx, query, eventID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	registrations := []domain.Registration{}
	for rows.Next() {
		var reg domain.Registration
		if err := rows.Scan(
			&reg.ID, &reg.UserID, &reg.EventID,
			&reg.PaymentStatus, &reg.RegistrationStatus,
			&reg.RegistrationDate, &reg.TotalPaid, &reg.AttendanceConfirmed,
			&reg.ParticipantName, &reg.ParticipantEmail, &reg.ParticipantPhone,
		); err != nil {
			return nil, err
		}
		registrations = append(registrations, reg)
	}
	return registrations, rows.Err()
}

// SetRegistrationQR escribe el código que faltaba.
//
// La condición sobre qr_data no es decorativa: si dos personas pulsan «Generar
// credenciales» a la vez, la segunda no le cambia el código a nadie —el QR ya
// enviado por correo seguiría siendo el bueno—. Devuelve domain.ErrNotFound si
// no había nada que rellenar.
func (r *EventRepository) SetRegistrationQR(ctx context.Context, registrationID, qrData string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE events.registrations
		   SET qr_data = $1
		 WHERE id = $2 AND COALESCE(qr_data, '') = ''
	`, qrData, registrationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// GetRegistrationsByUser devuelve las inscripciones del usuario con los datos
// del evento ya incorporados, para que la credencial se pinte con una sola
// llamada.
//
// Usa LEFT JOIN contra events.categories a propósito: GetEvents hace JOIN y por
// eso un evento con la categoría nula desaparece del listado sin error. Aquí eso
// significaría que al usuario le falta una entrada que sí compró.
func (r *EventRepository) GetRegistrationsByUser(ctx context.Context, userID string) ([]domain.UserRegistration, error) {
	query := `
		SELECT r.id, r.event_id, e.title, e.location, e.is_virtual, e.start_date, e.end_date, e.image_url,
		       r.payment_status, r.registration_status, r.registration_date,
		       r.qr_data, e.access_url, r.total_paid, r.attendance_confirmed
		FROM events.registrations r
		JOIN events.events e ON r.event_id = e.id
		WHERE r.user_id = $1
		ORDER BY e.start_date DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	registrations := []domain.UserRegistration{}
	for rows.Next() {
		var reg domain.UserRegistration
		var qrData *string
		var accessURL *string
		if err := rows.Scan(
			&reg.ID, &reg.EventID, &reg.EventTitle, &reg.EventLocation, &reg.EventIsVirtual,
			&reg.EventStartDate, &reg.EventEndDate, &reg.EventImageUrl,
			&reg.PaymentStatus, &reg.RegistrationStatus, &reg.RegistrationDate,
			&qrData, &accessURL, &reg.TotalPaid, &reg.AttendanceConfirmed,
		); err != nil {
			return nil, err
		}
		if qrData != nil {
			reg.QRData = *qrData
		}
		if accessURL != nil {
			reg.AccessURL = *accessURL
		}
		registrations = append(registrations, reg)
	}
	return registrations, rows.Err()
}

func (r *EventRepository) AddRegistrationWorkshops(ctx context.Context, registrationID string, workshopIDs []string) error {
	if len(workshopIDs) == 0 {
		return nil
	}

	query := `INSERT INTO events.registration_workshops (registration_id, workshop_id) VALUES ($1, $2)`
	for _, workshopID := range workshopIDs {
		_, err := r.db.Exec(ctx, query, registrationID, workshopID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *EventRepository) GetRegistrationByUserAndEvent(ctx context.Context, userID, eventID string) (*domain.Registration, error) {
	// COALESCE sobre qr_data porque desde la carga masiva hay inscripciones sin
	// credencial, y ahí la columna es NULL de verdad. Sin esto el Scan sobre un
	// string revienta con "cannot scan NULL", y este método lo llama lo primero
	// RegisterUser: quien se importó sin credencial no habría podido ni abrir
	// la pantalla del evento en la app.
	query := `
		SELECT id, user_id, event_id, payment_status, registration_status, registration_date,
		       COALESCE(qr_data, ''), total_paid, attendance_confirmed
		FROM events.registrations
		WHERE user_id = $1 AND event_id = $2
	`
	var reg domain.Registration
	err := r.db.QueryRow(ctx, query, userID, eventID).Scan(
		&reg.ID, &reg.UserID, &reg.EventID, &reg.PaymentStatus, &reg.RegistrationStatus, &reg.RegistrationDate,
		&reg.QRData, &reg.TotalPaid, &reg.AttendanceConfirmed,
	)
	if err != nil {
		// pgx v4 devuelve pgx.ErrNoRows, no sql.ErrNoRows: comparar con este
		// ultimo nunca casaba y "no hay registro" salia como error. El unico
		// llamador (event_service.go:99) descarta el error, asi que no rompia
		// nada, pero dejaba la rama muerta.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &reg, nil
}

func (r *EventRepository) CreateEvent(ctx context.Context, e *domain.Event) error {
	query := `
		INSERT INTO events.events (category_id, title, description, image_url, location, is_virtual, access_url, speaker_main, start_date, end_date, price, is_free, action_status, button_text, attendees_limit, includes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		e.CategoryID, e.Title, e.Description, e.ImageUrl, e.Location, e.IsVirtual, e.AccessURL, e.SpeakerMain, e.StartDate, e.EndDate, e.Price, e.IsFree, e.ActionStatus, e.ButtonText, e.AttendeesLimit, e.Includes,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
}

func (r *EventRepository) UpdateEvent(ctx context.Context, e *domain.Event) error {
	// Fallback: If CategoryID is empty but Category (name) is present, resolve it
	if (e.CategoryID == "" || e.CategoryID == "00000000-0000-0000-0000-000000000000") && e.Category != "" {
		var id string
		err := r.db.QueryRow(ctx, "SELECT id FROM events.categories WHERE name = $1", e.Category).Scan(&id)
		if err == nil {
			e.CategoryID = id
		}
	}

	query := `
		UPDATE events.events 
		SET category_id = $1, title = $2, description = $3, image_url = $4, location = $5, is_virtual = $6,
		    access_url = $7, speaker_main = $8,
		    start_date = $9, end_date = $10, price = $11, is_free = $12, action_status = $13, button_text = $14,
		    attendees_limit = $15, includes = $16, updated_at = NOW()
		WHERE id = $17
	`
	_, err := r.db.Exec(ctx, query,
		e.CategoryID, e.Title, e.Description, e.ImageUrl, e.Location, e.IsVirtual, e.AccessURL, e.SpeakerMain, e.StartDate, e.EndDate, e.Price, e.IsFree, e.ActionStatus, e.ButtonText, e.AttendeesLimit, e.Includes,
		e.ID,
	)
	return err
}

// UpdateEventStatus cambia solo la visibilidad del evento.
//
// Va aparte de UpdateEvent —y no como una columna más de aquel UPDATE— porque
// el formulario del panel no envía `status`: incluirlo allí escribiría la
// cadena vacía en cada guardado, y como el listado filtra por `= 'active'`, el
// evento desaparecería de la app al editarlo sin que nadie lo hubiera ocultado.
// Ocultar un evento es una acción deliberada, no un efecto secundario de
// guardar el formulario.
//
// Devuelve domain.ErrNotFound si el id no existe: sin esa comprobación, el
// panel recibiría un 200 sobre un evento que no está, que es justo el silencio
// que tenía antes el PUT genérico al ignorar el campo.
func (r *EventRepository) UpdateEventStatus(ctx context.Context, id, status string) error {
	query := `UPDATE events.events SET status = $1, updated_at = NOW() WHERE id = $2`
	tag, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *EventRepository) DeleteEvent(ctx context.Context, id string) error {
	query := `DELETE FROM events.events WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

func (r *EventRepository) CreateWorkshop(ctx context.Context, w *domain.Workshop) error {
	query := `
		INSERT INTO events.workshops (event_id, name, description, room, speaker, image_url, start_date_time, end_date_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, query,
		w.EventID, w.Name, w.Description, w.Room, w.Speaker, w.ImageUrl, w.StartDateTime, w.EndDateTime,
	).Scan(&w.ID, &w.CreatedAt)
}

func (r *EventRepository) DeleteWorkshopsByEventID(ctx context.Context, eventID string) error {
	query := `DELETE FROM events.workshops WHERE event_id = $1`
	_, err := r.db.Exec(ctx, query, eventID)
	return err
}

func (r *EventRepository) CreateWorkshopRating(ctx context.Context, wr *domain.WorkshopRating) error {
	query := `
		INSERT INTO events.workshop_ratings (workshop_id, user_id, rating, comment)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, query, wr.WorkshopID, wr.UserID, wr.Rating, wr.Comment).Scan(&wr.ID, &wr.CreatedAt)
}

func (r *EventRepository) GetRatingsByEventID(ctx context.Context, eventID string) ([]domain.WorkshopRating, error) {
	query := `
		SELECT wr.id, wr.workshop_id, w.name as workshop_name, wr.user_id, wr.rating, wr.comment, wr.created_at
		FROM events.workshop_ratings wr
		JOIN events.workshops w ON wr.workshop_id = w.id
		WHERE w.event_id = $1
		ORDER BY wr.created_at DESC
	`
	rows, err := r.db.Query(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ratings []domain.WorkshopRating
	for rows.Next() {
		var wr domain.WorkshopRating
		err := rows.Scan(&wr.ID, &wr.WorkshopID, &wr.WorkshopName, &wr.UserID, &wr.Rating, &wr.Comment, &wr.CreatedAt)
		if err != nil {
			return nil, err
		}
		ratings = append(ratings, wr)
	}
	return ratings, nil
}

// CreateEventSurvey guarda la encuesta general. El UNIQUE (event_id, user_id)
// de events.event_surveys hace de guardia definitiva contra el envio duplicado:
// el servicio ya comprueba antes, pero dos peticiones simultaneas pasan esa
// comprobacion a la vez y solo la base de datos las separa.
func (r *EventRepository) CreateEventSurvey(ctx context.Context, s *domain.EventSurvey) error {
	query := `
		INSERT INTO events.event_surveys (
			event_id, user_id, overall_rating, organization_rating,
			content_rating, speakers_rating, would_recommend, comment
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, query,
		s.EventID, s.UserID, s.OverallRating, s.OrganizationRating,
		s.ContentRating, s.SpeakersRating, s.WouldRecommend, s.Comment,
	).Scan(&s.ID, &s.CreatedAt)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("%w: %s", domain.ErrUniqueViolation, pgErr.ConstraintName)
	}
	return err
}

// GetEventSurveyByUser devuelve (nil, nil) si el usuario aun no ha respondido:
// no haber contestado no es un error.
func (r *EventRepository) GetEventSurveyByUser(ctx context.Context, eventID, userID string) (*domain.EventSurvey, error) {
	query := `
		SELECT id, event_id, user_id, overall_rating, organization_rating,
		       content_rating, speakers_rating, would_recommend, comment, created_at
		FROM events.event_surveys
		WHERE event_id = $1 AND user_id = $2
	`
	var s domain.EventSurvey
	err := r.db.QueryRow(ctx, query, eventID, userID).Scan(
		&s.ID, &s.EventID, &s.UserID, &s.OverallRating, &s.OrganizationRating,
		&s.ContentRating, &s.SpeakersRating, &s.WouldRecommend, &s.Comment, &s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// GetEventSurveySummary agrega las respuestas para el panel. Los promedios son
// punteros: avg() ignora los NULL, asi que una pregunta opcional que nadie
// respondio devuelve NULL, y eso es "sin datos", no un cero.
func (r *EventRepository) GetEventSurveySummary(ctx context.Context, eventID string) (*domain.EventSurveySummary, error) {
	query := `
		SELECT
			count(*),
			avg(overall_rating),
			avg(organization_rating),
			avg(content_rating),
			avg(speakers_rating),
			avg(CASE
			      WHEN would_recommend IS NULL THEN NULL
			      WHEN would_recommend THEN 1.0
			      ELSE 0.0
			    END)
		FROM events.event_surveys
		WHERE event_id = $1
	`
	summary := domain.EventSurveySummary{
		EventID:  eventID,
		Comments: []domain.EventSurveyComment{},
	}
	err := r.db.QueryRow(ctx, query, eventID).Scan(
		&summary.Responses, &summary.OverallAverage, &summary.OrganizationAverage,
		&summary.ContentAverage, &summary.SpeakersAverage, &summary.RecommendRate,
	)
	if err != nil {
		return nil, err
	}

	if summary.Responses == 0 {
		return &summary, nil
	}

	commentsQuery := `
		SELECT comment, created_at
		FROM events.event_surveys
		WHERE event_id = $1 AND comment IS NOT NULL AND btrim(comment) <> ''
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, commentsQuery, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var c domain.EventSurveyComment
		if err := rows.Scan(&c.Comment, &c.CreatedAt); err != nil {
			return nil, err
		}
		summary.Comments = append(summary.Comments, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &summary, nil
}

func (r *EventRepository) GetAgenda(ctx context.Context, userID string) ([]domain.Workshop, error) {
	query := `
		SELECT 
			w.id, w.event_id, e.title as event_title, w.name, 
			COALESCE(w.description, '') as description, 
			COALESCE(w.room, '') as room, 
			COALESCE(w.speaker, '') as speaker, 
			COALESCE(w.image_url, '') as image_url, 
			w.start_date_time, w.end_date_time
		FROM events.workshops w
		JOIN events.user_agenda ua ON w.id = ua.workshop_id
		JOIN events.events e ON w.event_id = e.id
		WHERE ua.user_id = $1::uuid
		ORDER BY w.start_date_time ASC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workshops []domain.Workshop
	for rows.Next() {
		var w domain.Workshop
		err := rows.Scan(
			&w.ID, &w.EventID, &w.EventTitle, &w.Name, &w.Description, &w.Room, &w.Speaker, &w.ImageUrl,
			&w.StartDateTime, &w.EndDateTime,
		)
		if err != nil {
			return nil, err
		}
		workshops = append(workshops, w)
	}
	return workshops, nil
}

func (r *EventRepository) AddToAgenda(ctx context.Context, userID, workshopID string) error {
	query := `
		INSERT INTO events.user_agenda (user_id, workshop_id)
		VALUES ($1::uuid, $2::uuid)
		ON CONFLICT ON CONSTRAINT user_agenda_pkey DO NOTHING
	`
	_, err := r.db.Exec(ctx, query, userID, workshopID)
	return err
}

func (r *EventRepository) RemoveFromAgenda(ctx context.Context, userID, workshopID string) error {
	query := `DELETE FROM events.user_agenda WHERE user_id = $1::uuid AND workshop_id = $2::uuid`
	_, err := r.db.Exec(ctx, query, userID, workshopID)
	return err
}

// GetRegistrationByQR busca la inscripción de un código QR para el check-in.
//
// El nombre y el apellido salen en columnas separadas y sin concatenar. Antes
// la consulta hacía `u.first_name || ' ' || u.last_name`, y como ambos están
// cifrados el resultado era la unión de dos bloques AES independientes: una
// cadena que ya no se puede descifrar ni por partes. El efecto era que el
// escáner del panel mostraba el nombre del asistente en texto cifrado. Quien
// descifra es EventService.CheckIn, que tiene el CryptoService.
func (r *EventRepository) GetRegistrationByQR(ctx context.Context, qrData string) (*domain.Registration, *domain.CheckInResponse, error) {
	query := `
		SELECT 
			r.id, r.user_id, r.event_id, r.payment_status, r.registration_status, r.registration_date, r.qr_data, r.total_paid, r.attendance_confirmed,
			COALESCE(u.first_name, ''), COALESCE(u.last_name, ''), COALESCE(u.email_encrypted, ''),
			e.title as event_title
		FROM events.registrations r
		JOIN core.users u ON r.user_id = u.id
		JOIN events.events e ON r.event_id = e.id
		WHERE r.qr_data = $1
	`
	var reg domain.Registration
	var resp domain.CheckInResponse

	err := r.db.QueryRow(ctx, query, qrData).Scan(
		&reg.ID, &reg.UserID, &reg.EventID, &reg.PaymentStatus, &reg.RegistrationStatus, &reg.RegistrationDate,
		&reg.QRData, &reg.TotalPaid, &reg.AttendanceConfirmed,
		&resp.FirstName, &resp.LastName, &resp.UserEmail, &resp.EventTitle,
	)
	if err != nil {
		return nil, nil, err
	}
	resp.RegistrationID = reg.ID
	// CheckInTime lo fija RecordAttendance con la hora que quedó guardada: aquí
	// todavía no se sabe si esta lectura es la primera o una repetida.

	return &reg, &resp, nil
}

// RecordAttendance deja constancia de la entrada y devuelve el momento en que
// esa inscripción entró —el PRIMERO, no el de esta lectura— junto con si ya
// había entrado antes.
//
// El ON CONFLICT es lo que hace la operación idempotente: hasta el 2026-08-19
// se insertaba siempre, así que pasar dos veces el mismo QR dejaba dos filas y
// el evento contaba un asistente de más por cada relectura. La restricción
// UNIQUE que lo respalda la crea
// scripts/20260819_attendance_logs_una_por_inscripcion.sql.
func (r *EventRepository) RecordAttendance(ctx context.Context, registrationID, staffID string) (time.Time, bool, error) {
	// 1. Update registration status
	updateQuery := `UPDATE events.registrations SET attendance_confirmed = true WHERE id = $1::uuid`
	_, err := r.db.Exec(ctx, updateQuery, registrationID)
	if err != nil {
		return time.Time{}, false, err
	}

	// 2. Insert attendance log
	logQuery := `
		INSERT INTO events.attendance_logs (registration_id, staff_user_id)
		VALUES ($1::uuid, $2::uuid)
		ON CONFLICT (registration_id) DO NOTHING
	`
	tag, err := r.db.Exec(ctx, logQuery, registrationID, staffID)
	if err != nil {
		return time.Time{}, false, err
	}
	yaHabiaEntrado := tag.RowsAffected() == 0

	// 3. La hora sale siempre de la fila que quedó guardada, no del reloj de
	// ahora: en una relectura lo que interesa en la puerta es cuándo entró esta
	// persona la primera vez.
	var entrada time.Time
	err = r.db.QueryRow(ctx,
		`SELECT check_in_time FROM events.attendance_logs WHERE registration_id = $1::uuid`,
		registrationID).Scan(&entrada)
	if err != nil {
		return time.Time{}, yaHabiaEntrado, err
	}

	return entrada, yaHabiaEntrado, nil
}

func (r *EventRepository) GetWorkshopsByRegistrationID(ctx context.Context, registrationID string) ([]domain.Workshop, error) {
	query := `
		SELECT w.id, w.event_id, e.title as event_title, w.name, 
			COALESCE(w.description, '') as description, 
			COALESCE(w.room, '') as room, 
			COALESCE(w.speaker, '') as speaker, 
			COALESCE(w.image_url, '') as image_url, 
			w.start_date_time, w.end_date_time
		FROM events.workshops w
		JOIN events.registration_workshops rw ON w.id = rw.workshop_id
		JOIN events.events e ON w.event_id = e.id
		WHERE rw.registration_id = $1::uuid
		ORDER BY w.start_date_time ASC
	`
	rows, err := r.db.Query(ctx, query, registrationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workshops []domain.Workshop
	for rows.Next() {
		var w domain.Workshop
		err := rows.Scan(
			&w.ID, &w.EventID, &w.EventTitle, &w.Name, &w.Description, &w.Room, &w.Speaker, &w.ImageUrl,
			&w.StartDateTime, &w.EndDateTime,
		)
		if err != nil {
			return nil, err
		}
		workshops = append(workshops, w)
	}
	return workshops, nil
}

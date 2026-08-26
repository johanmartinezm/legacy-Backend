package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

type ContactoRepository struct {
	db *pgxpool.Pool
}

func NewContactoRepository(db *pgxpool.Pool) *ContactoRepository {
	return &ContactoRepository{db: db}
}

// Guardar escribe el mensaje tal como llega: el servicio ya lo ha cifrado.
func (r *ContactoRepository) Guardar(ctx context.Context, m *domain.MensajeDeContacto) (string, error) {
	sql := `
		INSERT INTO core.contact_messages (user_id, subject, body)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	var id string
	err := r.db.QueryRow(ctx, sql, m.UserID, m.Asunto, m.Mensaje).Scan(&id)
	return id, err
}

func (r *ContactoRepository) MarcarEnviado(ctx context.Context, id string) error {
	sql := `UPDATE core.contact_messages SET email_enviado = true, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(ctx, sql, id)
	return err
}

// Listar trae los mensajes con el nombre y el correo de quien escribio. Todo
// sale cifrado de la base —el asunto y el cuerpo de esta tabla, y el nombre, el
// apellido y el correo de core.users— y lo descifra el servicio, que es quien
// tiene la clave.
//
// El nombre y el apellido se devuelven por separado a proposito: van cifrados
// cada uno por su cuenta, asi que unirlos aqui daria una cadena imposible de
// descifrar despues.
// Listar pagina desde el 2026-08-26. Es una bandeja: crece con cada mensaje que
// escribe cualquiera y nadie la vacía.
//
// El id desempata el orden por la misma razón que en el resto: sin un orden
// total, dos mensajes del mismo instante pueden cambiar de sitio entre consultas
// y una página se salta uno o lo repite.
func (r *ContactoRepository) Listar(ctx context.Context, estado string, limit, offset int) ([]*domain.MensajeDeContacto, error) {
	sql := `
		SELECT m.id, m.user_id, m.subject, m.body, m.status, m.email_enviado,
		       m.created_at, m.updated_at,
		       COALESCE(u.first_name, ''), COALESCE(u.last_name, ''), COALESCE(u.email_encrypted, '')
		FROM core.contact_messages m
		LEFT JOIN core.users u ON u.id = m.user_id
		WHERE ($1 = '' OR m.status = $1)
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, sql, estado, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mensajes := []*domain.MensajeDeContacto{}
	for rows.Next() {
		var m domain.MensajeDeContacto
		if err := rows.Scan(&m.ID, &m.UserID, &m.Asunto, &m.Mensaje, &m.Estado, &m.EmailEnviado,
			&m.CreatedAt, &m.UpdatedAt, &m.RemitenteNombre, &m.RemitenteApellido, &m.RemitenteEmail); err != nil {
			return nil, err
		}
		mensajes = append(mensajes, &m)
	}
	return mensajes, rows.Err()
}

// Contar es el total con el mismo filtro de estado que Listar. Si cambia el
// WHERE de allí, tiene que cambiar aquí: si no, el paginador ofrece páginas
// vacías.
func (r *ContactoRepository) Contar(ctx context.Context, estado string) (int, error) {
	var total int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM core.contact_messages m WHERE ($1 = '' OR m.status = $1)`,
		estado).Scan(&total)
	return total, err
}

func (r *ContactoRepository) CambiarEstado(ctx context.Context, id, estado string) error {
	sql := `UPDATE core.contact_messages SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(ctx, sql, estado, id)
	return err
}

func (r *ContactoRepository) ContarDesde(ctx context.Context, userID string, desde time.Time) (int, error) {
	sql := `SELECT count(*) FROM core.contact_messages WHERE user_id = $1 AND created_at >= $2`
	var cuantos int
	err := r.db.QueryRow(ctx, sql, userID, desde).Scan(&cuantos)
	return cuantos, err
}

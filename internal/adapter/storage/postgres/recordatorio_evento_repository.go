package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

// RecordatorioEventoRepository guarda qué eventos ya tuvieron su aviso de la
// mañana (events.event_reminders, migración 20261021_event_reminders.sql).
type RecordatorioEventoRepository struct {
	db *pgxpool.Pool
}

func NewRecordatorioEventoRepository(db *pgxpool.Pool) *RecordatorioEventoRepository {
	return &RecordatorioEventoRepository{db: db}
}

func (r *RecordatorioEventoRepository) EventosSinRecordar(ctx context.Context, dia string) ([]*domain.EventoPorRecordar, error) {
	sql := `
		SELECT e.id, e.title, e.location, e.is_virtual
		  FROM events.events e
		 WHERE e.start_date = $1::date
		   AND e.status = 'active'
		   AND NOT EXISTS (
		         SELECT 1 FROM events.event_reminders er
		          WHERE er.event_id = e.id AND er.fecha = $1::date
		       )
		 ORDER BY e.id
	`
	rows, err := r.db.Query(ctx, sql, dia)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var eventos []*domain.EventoPorRecordar
	for rows.Next() {
		e := &domain.EventoPorRecordar{}
		if err := rows.Scan(&e.ID, &e.Title, &e.Location, &e.IsVirtual); err != nil {
			return nil, err
		}
		eventos = append(eventos, e)
	}
	return eventos, rows.Err()
}

func (r *RecordatorioEventoRepository) MarcarRecordado(ctx context.Context, eventID, dia string) (bool, error) {
	// La clave primaria (event_id, fecha) es la que decide: si otra vuelta ya
	// insertó la fila, esta no inserta nada y no se avisa otra vez.
	sql := `
		INSERT INTO events.event_reminders (event_id, fecha)
		VALUES ($1, $2::date)
		ON CONFLICT (event_id, fecha) DO NOTHING
	`
	tag, err := r.db.Exec(ctx, sql, eventID, dia)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *RecordatorioEventoRepository) InscritosConfirmados(ctx context.Context, eventID string) ([]string, error) {
	sql := `
		SELECT DISTINCT user_id
		  FROM events.registrations
		 WHERE event_id = $1
		   AND registration_status = 'confirmed'
	`
	rows, err := r.db.Query(ctx, sql, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usuarios []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		usuarios = append(usuarios, id)
	}
	return usuarios, rows.Err()
}

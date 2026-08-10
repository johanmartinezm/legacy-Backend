package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
)

type BlockRepository struct {
	db *pgxpool.Pool
}

func NewBlockRepository(db *pgxpool.Pool) *BlockRepository {
	return &BlockRepository{db: db}
}

func (r *BlockRepository) Block(ctx context.Context, blockerID, blockedID string) error {
	// ON CONFLICT DO NOTHING: bloquear a quien ya está bloqueado no es un error,
	// es la misma situación. Devolver fallo obligaría a la app a distinguir dos
	// casos que para la persona son uno.
	sql := `
		INSERT INTO core.user_blocks (blocker_id, blocked_id)
		VALUES ($1, $2)
		ON CONFLICT (blocker_id, blocked_id) DO NOTHING
	`
	_, err := r.db.Exec(ctx, sql, blockerID, blockedID)
	if err != nil {
		// La restricción user_blocks_no_auto rechaza bloquearse a uno mismo. El
		// servicio ya lo corta antes; esto es la red de seguridad.
		if strings.Contains(err.Error(), "user_blocks_no_auto") {
			return errors.New("no puedes bloquearte a ti mismo")
		}
		return err
	}
	return nil
}

func (r *BlockRepository) Unblock(ctx context.Context, blockerID, blockedID string) error {
	sql := `DELETE FROM core.user_blocks WHERE blocker_id = $1 AND blocked_id = $2`
	_, err := r.db.Exec(ctx, sql, blockerID, blockedID)
	return err
}

func (r *BlockRepository) ListBlocked(ctx context.Context, blockerID string) ([]*domain.BlockedUser, error) {
	// Los nombres salen cifrados de la base; los descifra el servicio, que es
	// quien tiene la clave.
	sql := `
		SELECT u.id, u.first_name, u.last_name, u.alias,
		       COALESCE(u.profile_image_url, ''), b.created_at
		FROM core.user_blocks b
		JOIN core.users u ON u.id = b.blocked_id
		WHERE b.blocker_id = $1
		ORDER BY b.created_at DESC
	`
	rows, err := r.db.Query(ctx, sql, blockerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bloqueados := []*domain.BlockedUser{}
	for rows.Next() {
		var b domain.BlockedUser
		if err := rows.Scan(&b.UserID, &b.FirstName, &b.LastName, &b.Alias,
			&b.ProfileImageUrl, &b.BlockedAt); err != nil {
			return nil, err
		}
		bloqueados = append(bloqueados, &b)
	}
	return bloqueados, rows.Err()
}

// AreBlocked mira las dos direcciones: da igual quién bloqueó a quién, el efecto
// es el mismo para ambos.
func (r *BlockRepository) AreBlocked(ctx context.Context, userA, userB string) (bool, error) {
	sql := `
		SELECT EXISTS (
			SELECT 1 FROM core.user_blocks
			WHERE (blocker_id = $1 AND blocked_id = $2)
			   OR (blocker_id = $2 AND blocked_id = $1)
		)
	`
	var hay bool
	err := r.db.QueryRow(ctx, sql, userA, userB).Scan(&hay)
	return hay, err
}

func (r *BlockRepository) Report(ctx context.Context, report *domain.UserReport) error {
	sql := `
		INSERT INTO core.user_reports (reporter_id, reported_id, message_id, reason)
		VALUES ($1, $2, $3, $4)
		RETURNING id, status, created_at
	`
	return r.db.QueryRow(ctx, sql, report.ReporterID, report.ReportedID,
		report.MessageID, report.Reason).
		Scan(&report.ID, &report.Status, &report.CreatedAt)
}

// ListReports filtra por estado; con status vacío devuelve todos.
func (r *BlockRepository) ListReports(ctx context.Context, status string) ([]*domain.UserReport, error) {
	// Los nombres salen cifrados; los descifra el servicio, que tiene la clave.
	sql := `
		SELECT r.id, r.reporter_id, r.reported_id, r.message_id, r.reason,
		       r.status, r.created_at,
		       COALESCE(quien.first_name, ''), COALESCE(quien.last_name, ''),
		       COALESCE(sobre.first_name, ''), COALESCE(sobre.last_name, '')
		FROM core.user_reports r
		LEFT JOIN core.users quien ON quien.id = r.reporter_id
		LEFT JOIN core.users sobre ON sobre.id = r.reported_id
		WHERE ($1 = '' OR r.status = $1)
		ORDER BY r.created_at DESC
	`
	rows, err := r.db.Query(ctx, sql, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reportes := []*domain.UserReport{}
	for rows.Next() {
		var rep domain.UserReport
		if err := rows.Scan(&rep.ID, &rep.ReporterID, &rep.ReportedID, &rep.MessageID,
			&rep.Reason, &rep.Status, &rep.CreatedAt,
			&rep.ReporterFirstName, &rep.ReporterLastName,
			&rep.ReportedFirstName, &rep.ReportedLastName); err != nil {
			return nil, err
		}
		reportes = append(reportes, &rep)
	}
	return reportes, rows.Err()
}

func (r *BlockRepository) UpdateReportStatus(ctx context.Context, reportID, status string) error {
	sql := `UPDATE core.user_reports SET status = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, sql, status, reportID)
	return err
}

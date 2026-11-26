package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type BoardEnvioRepository struct {
	db *pgxpool.Pool
}

func NewBoardEnvioRepository(db *pgxpool.Pool) *BoardEnvioRepository {
	return &BoardEnvioRepository{db: db}
}

const camposEnvioBoard = `id, formulario, user_id, respuestas, datos_cifrados, foto_url, acepta_terminos,
	estado, estado_motivo, estado_cambiado_en, creado_en`

func escanearEnvioBoard(row pgx.Row, e *domain.EnvioBoard) error {
	var respuestas []byte
	if err := row.Scan(&e.ID, &e.Formulario, &e.UserID, &respuestas, &e.DatosCifrados, &e.FotoURL,
		&e.AceptaTerminos, &e.Estado, &e.EstadoMotivo, &e.EstadoCambiadoEn, &e.CreadoEn); err != nil {
		return err
	}
	e.Respuestas = map[string]any{}
	return json.Unmarshal(respuestas, &e.Respuestas)
}

func (r *BoardEnvioRepository) Create(ctx context.Context, e *domain.EnvioBoard) error {
	respuestas, err := json.Marshal(e.Respuestas)
	if err != nil {
		return err
	}
	sql := `INSERT INTO board.envios (formulario, user_id, respuestas, datos_cifrados, foto_url, acepta_terminos)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING ` + camposEnvioBoard
	err = escanearEnvioBoard(r.db.QueryRow(ctx, sql, e.Formulario, e.UserID, respuestas, e.DatosCifrados,
		e.FotoURL, e.AceptaTerminos), e)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("%w: %s", domain.ErrUniqueViolation, pgErr.ConstraintName)
	}
	return err
}

// Actual: del A cuenta cualquiera (solo hay uno por persona); del B, solo el
// pendiente, porque después de una respuesta se puede pedir otra búsqueda.
func (r *BoardEnvioRepository) Actual(ctx context.Context, userID, formulario string) (*domain.EnvioBoard, error) {
	if !idValido(userID) {
		return nil, domain.ErrNotFound
	}
	sql := `SELECT ` + camposEnvioBoard + ` FROM board.envios
		WHERE user_id = $1 AND formulario = $2 AND (formulario = 'A' OR estado = 'pendiente')
		ORDER BY creado_en DESC, id DESC
		LIMIT 1`
	var e domain.EnvioBoard
	err := escanearEnvioBoard(r.db.QueryRow(ctx, sql, userID, formulario), &e)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// filtroEnviosSQL arma el WHERE de la bandeja. Todo va parametrizado: el campo
// de la opción viaja como parámetro del jsonb_build_object, no pegado al SQL.
func filtroEnviosSQL(f domain.FiltroEnviosBoard) (string, []any) {
	where := []string{"TRUE"}
	args := []any{}
	agregar := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.Formulario != "" {
		agregar("e.formulario = $%d", f.Formulario)
	}
	if f.Estado != "" {
		agregar("e.estado = $%d", f.Estado)
	}
	if f.Desde != nil {
		agregar("e.creado_en >= $%d", *f.Desde)
	}
	if f.Hasta != nil {
		agregar("e.creado_en < $%d", *f.Hasta)
	}
	if f.Campo != "" && f.Opcion != "" {
		// El índice GIN jsonb_path_ops de la migración sirve a @>.
		args = append(args, f.Campo, f.Opcion)
		if f.Multiple {
			where = append(where, fmt.Sprintf("e.respuestas @> jsonb_build_object($%d::text, jsonb_build_array($%d::text))", len(args)-1, len(args)))
		} else {
			where = append(where, fmt.Sprintf("e.respuestas @> jsonb_build_object($%d::text, $%d::text)", len(args)-1, len(args)))
		}
	}
	return strings.Join(where, " AND "), args
}

const camposEnvioBoardPanel = `e.id, e.formulario, e.user_id, e.respuestas, e.datos_cifrados, e.foto_url, e.acepta_terminos,
	e.estado, e.estado_motivo, e.estado_cambiado_en, e.creado_en, e.aviso_enviado, COALESCE(a.email, '')`

func escanearEnvioBoardPanel(row pgx.Row, e *domain.EnvioBoard) error {
	var respuestas []byte
	if err := row.Scan(&e.ID, &e.Formulario, &e.UserID, &respuestas, &e.DatosCifrados, &e.FotoURL,
		&e.AceptaTerminos, &e.Estado, &e.EstadoMotivo, &e.EstadoCambiadoEn, &e.CreadoEn, &e.AvisoEnviado,
		&e.EstadoCambiadoPorStr); err != nil {
		return err
	}
	e.Respuestas = map[string]any{}
	return json.Unmarshal(respuestas, &e.Respuestas)
}

func (r *BoardEnvioRepository) Listar(ctx context.Context, f domain.FiltroEnviosBoard, limit, offset int) ([]*domain.EnvioBoard, error) {
	where, args := filtroEnviosSQL(f)
	args = append(args, limit, offset)
	// Orden total: el id desempata dos envíos del mismo instante.
	sql := `SELECT ` + camposEnvioBoardPanel + `
		FROM board.envios e LEFT JOIN core.admin_users a ON a.id = e.estado_cambiado_por
		WHERE ` + where + fmt.Sprintf(`
		ORDER BY e.creado_en DESC, e.id DESC
		LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var envios []*domain.EnvioBoard
	for rows.Next() {
		var e domain.EnvioBoard
		if err := escanearEnvioBoardPanel(rows, &e); err != nil {
			return nil, err
		}
		envios = append(envios, &e)
	}
	return envios, rows.Err()
}

func (r *BoardEnvioRepository) Contar(ctx context.Context, f domain.FiltroEnviosBoard) (int, error) {
	where, args := filtroEnviosSQL(f)
	var total int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM board.envios e WHERE `+where, args...).Scan(&total)
	return total, err
}

func (r *BoardEnvioRepository) FindByID(ctx context.Context, id string) (*domain.EnvioBoard, error) {
	if !idValido(id) {
		return nil, domain.ErrNotFound
	}
	var e domain.EnvioBoard
	err := escanearEnvioBoardPanel(r.db.QueryRow(ctx, `SELECT `+camposEnvioBoardPanel+`
		FROM board.envios e LEFT JOIN core.admin_users a ON a.id = e.estado_cambiado_por
		WHERE e.id = $1`, id), &e)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *BoardEnvioRepository) CambiarEstado(ctx context.Context, id, estado, motivo, adminID string) error {
	if !idValido(id) {
		return domain.ErrNotFound
	}
	// Un admin que ya no existe (o un id que no es uuid) deja el «quién» vacío
	// en vez de tumbar el cambio.
	var por any
	if idValido(adminID) {
		por = adminID
	}
	tag, err := r.db.Exec(ctx, `UPDATE board.envios
		SET estado = $2, estado_motivo = $3,
		    estado_cambiado_por = (SELECT id FROM core.admin_users WHERE id = $4::uuid),
		    estado_cambiado_en = CURRENT_TIMESTAMP, actualizado_en = CURRENT_TIMESTAMP
		WHERE id = $1`, id, estado, motivo, por)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w: %s", domain.ErrUniqueViolation, pgErr.ConstraintName)
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *BoardEnvioRepository) MarcarAvisoEnviado(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE board.envios SET aviso_enviado = true WHERE id = $1`, id)
	return err
}

// Destinatarios no pagina: son unos pocos buzones.
func (r *BoardEnvioRepository) Destinatarios(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT correo FROM board.destinatarios_aviso ORDER BY creado_en, correo LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	correos := []string{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		correos = append(correos, c)
	}
	return correos, rows.Err()
}

func (r *BoardEnvioRepository) ReemplazarDestinatarios(ctx context.Context, correos []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Se conservan los que siguen (y su fecha de alta) y se borran los que no.
	if _, err := tx.Exec(ctx, `DELETE FROM board.destinatarios_aviso WHERE NOT (correo = ANY($1))`, correos); err != nil {
		return err
	}
	for _, c := range correos {
		if _, err := tx.Exec(ctx, `INSERT INTO board.destinatarios_aviso (correo) VALUES ($1) ON CONFLICT DO NOTHING`, c); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

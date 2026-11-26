package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

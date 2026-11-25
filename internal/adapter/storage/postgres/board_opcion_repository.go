package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type BoardOpcionRepository struct {
	db *pgxpool.Pool
}

func NewBoardOpcionRepository(db *pgxpool.Pool) *BoardOpcionRepository {
	return &BoardOpcionRepository{db: db}
}

const camposOpcionBoard = `id, catalogo, codigo, etiqueta, orden, activo, creado_en, actualizado_en`

// limiteOpcionesBoard es un techo de seguridad, no una página: la siembra son
// 140 opciones en 18 catálogos y la app pide todas de una vez para pintar el
// formulario. Si algún día se acerca, hay que paginar por catálogo.
const limiteOpcionesBoard = 2000

func escanearOpcionBoard(row pgx.Row, o *domain.OpcionBoard) error {
	return row.Scan(&o.ID, &o.Catalogo, &o.Codigo, &o.Etiqueta, &o.Orden, &o.Activo, &o.CreadoEn, &o.ActualizadoEn)
}

func (r *BoardOpcionRepository) List(ctx context.Context, catalogo string, soloActivas bool) ([]*domain.OpcionBoard, error) {
	// El id de desempate deja el orden total aunque dos opciones compartan número.
	sql := `SELECT ` + camposOpcionBoard + ` FROM board.opciones
		WHERE ($1 = '' OR catalogo = $1) AND (NOT $2 OR activo)
		ORDER BY catalogo ASC, orden ASC, id ASC
		LIMIT $3`
	rows, err := r.db.Query(ctx, sql, catalogo, soloActivas, limiteOpcionesBoard)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var opciones []*domain.OpcionBoard
	for rows.Next() {
		var o domain.OpcionBoard
		if err := escanearOpcionBoard(rows, &o); err != nil {
			return nil, err
		}
		opciones = append(opciones, &o)
	}
	return opciones, rows.Err()
}

// FindByID devuelve domain.ErrNotFound si el id no existe o no es un uuid.
func (r *BoardOpcionRepository) FindByID(ctx context.Context, id string) (*domain.OpcionBoard, error) {
	if !idValido(id) {
		return nil, domain.ErrNotFound
	}
	var o domain.OpcionBoard
	err := escanearOpcionBoard(r.db.QueryRow(ctx, `SELECT `+camposOpcionBoard+` FROM board.opciones WHERE id = $1`, id), &o)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *BoardOpcionRepository) Create(ctx context.Context, o *domain.OpcionBoard) error {
	sql := `INSERT INTO board.opciones (catalogo, codigo, etiqueta, orden, activo)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + camposOpcionBoard
	return escanearOpcionBoard(r.db.QueryRow(ctx, sql, o.Catalogo, o.Codigo, o.Etiqueta, o.Orden, o.Activo), o)
}

// Update devuelve domain.ErrNotFound si el id no existe o no es un uuid.
func (r *BoardOpcionRepository) Update(ctx context.Context, o *domain.OpcionBoard) error {
	if !idValido(o.ID) {
		return domain.ErrNotFound
	}
	sql := `UPDATE board.opciones
		SET etiqueta = $1, orden = $2, activo = $3, actualizado_en = CURRENT_TIMESTAMP
		WHERE id = $4
		RETURNING ` + camposOpcionBoard
	err := escanearOpcionBoard(r.db.QueryRow(ctx, sql, o.Etiqueta, o.Orden, o.Activo, o.ID), o)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

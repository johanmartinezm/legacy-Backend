package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type LegacyPlusItemRepository struct {
	db *pgxpool.Pool
}

func NewLegacyPlusItemRepository(db *pgxpool.Pool) *LegacyPlusItemRepository {
	return &LegacyPlusItemRepository{db: db}
}

const camposLegacyPlusItem = `id, grupo, titulo, texto, orden, activo, creado_en, actualizado_en`

func escanearLegacyPlusItem(row pgx.Row, i *domain.LegacyPlusItem) error {
	return row.Scan(&i.ID, &i.Grupo, &i.Titulo, &i.Texto, &i.Orden, &i.Activo, &i.CreadoEn, &i.ActualizadoEn)
}

// List no pagina: son los puntos de una pantalla, unos pocos por grupo.
func (r *LegacyPlusItemRepository) List(ctx context.Context, soloActivos bool) ([]*domain.LegacyPlusItem, error) {
	sql := `SELECT ` + camposLegacyPlusItem + ` FROM core.legacy_plus_items`
	if soloActivos {
		sql += ` WHERE activo`
	}
	sql += ` ORDER BY grupo ASC, orden ASC, creado_en ASC, id ASC`

	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.LegacyPlusItem
	for rows.Next() {
		var i domain.LegacyPlusItem
		if err := escanearLegacyPlusItem(rows, &i); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	return items, rows.Err()
}

func (r *LegacyPlusItemRepository) Create(ctx context.Context, i *domain.LegacyPlusItem) error {
	sql := `
		INSERT INTO core.legacy_plus_items (grupo, titulo, texto, orden, activo)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + camposLegacyPlusItem
	return escanearLegacyPlusItem(r.db.QueryRow(ctx, sql, i.Grupo, i.Titulo, i.Texto, i.Orden, i.Activo), i)
}

// Update devuelve domain.ErrNotFound si el id no existe o no es un uuid.
func (r *LegacyPlusItemRepository) Update(ctx context.Context, i *domain.LegacyPlusItem) error {
	if !idValido(i.ID) {
		return domain.ErrNotFound
	}
	sql := `
		UPDATE core.legacy_plus_items
		SET grupo = $1, titulo = $2, texto = $3, orden = $4, activo = $5,
		    actualizado_en = CURRENT_TIMESTAMP
		WHERE id = $6
		RETURNING ` + camposLegacyPlusItem
	err := escanearLegacyPlusItem(r.db.QueryRow(ctx, sql, i.Grupo, i.Titulo, i.Texto, i.Orden, i.Activo, i.ID), i)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// Delete devuelve domain.ErrNotFound si el id no existe o no es un uuid.
func (r *LegacyPlusItemRepository) Delete(ctx context.Context, id string) error {
	if !idValido(id) {
		return domain.ErrNotFound
	}
	tag, err := r.db.Exec(ctx, `DELETE FROM core.legacy_plus_items WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type BeneficioRepository struct {
	db *pgxpool.Pool
}

func NewBeneficioRepository(db *pgxpool.Pool) *BeneficioRepository {
	return &BeneficioRepository{db: db}
}

const camposBeneficio = `id, tipo, titulo, descripcion, imagen_url, enlace_url, orden, activo, creado_en, actualizado_en`

func escanearBeneficio(row pgx.Row, b *domain.Beneficio) error {
	return row.Scan(&b.ID, &b.Tipo, &b.Titulo, &b.Descripcion, &b.ImagenURL, &b.EnlaceURL, &b.Orden, &b.Activo, &b.CreadoEn, &b.ActualizadoEn)
}

// List no pagina: son las tarjetas de un slider, unas pocas.
func (r *BeneficioRepository) List(ctx context.Context, soloActivos bool) ([]*domain.Beneficio, error) {
	sql := `SELECT ` + camposBeneficio + ` FROM core.beneficios`
	if soloActivos {
		sql += ` WHERE activo`
	}
	sql += ` ORDER BY tipo ASC, orden ASC, creado_en ASC, id ASC`

	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var beneficios []*domain.Beneficio
	for rows.Next() {
		var b domain.Beneficio
		if err := escanearBeneficio(rows, &b); err != nil {
			return nil, err
		}
		beneficios = append(beneficios, &b)
	}
	return beneficios, rows.Err()
}

func (r *BeneficioRepository) Create(ctx context.Context, b *domain.Beneficio) error {
	sql := `
		INSERT INTO core.beneficios (tipo, titulo, descripcion, imagen_url, enlace_url, orden, activo)
		VALUES ($7, $1, $2, $3, $4, $5, $6)
		RETURNING ` + camposBeneficio
	return escanearBeneficio(r.db.QueryRow(ctx, sql, b.Titulo, b.Descripcion, b.ImagenURL, b.EnlaceURL, b.Orden, b.Activo, b.Tipo), b)
}

// Update devuelve domain.ErrNotFound si el id no existe o no es un uuid.
func (r *BeneficioRepository) Update(ctx context.Context, b *domain.Beneficio) error {
	if !idValido(b.ID) {
		return domain.ErrNotFound
	}
	sql := `
		UPDATE core.beneficios
		SET titulo = $1, descripcion = $2, imagen_url = $3, enlace_url = $4, orden = $5, activo = $6,
		    tipo = $8, actualizado_en = CURRENT_TIMESTAMP
		WHERE id = $7
		RETURNING ` + camposBeneficio
	err := escanearBeneficio(r.db.QueryRow(ctx, sql, b.Titulo, b.Descripcion, b.ImagenURL, b.EnlaceURL, b.Orden, b.Activo, b.ID, b.Tipo), b)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// Delete devuelve domain.ErrNotFound si el id no existe o no es un uuid.
func (r *BeneficioRepository) Delete(ctx context.Context, id string) error {
	if !idValido(id) {
		return domain.ErrNotFound
	}
	tag, err := r.db.Exec(ctx, `DELETE FROM core.beneficios WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

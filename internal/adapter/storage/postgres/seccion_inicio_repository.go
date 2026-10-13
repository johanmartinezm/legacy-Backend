package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type SeccionInicioRepository struct {
	db *pgxpool.Pool
}

func NewSeccionInicioRepository(db *pgxpool.Pool) *SeccionInicioRepository {
	return &SeccionInicioRepository{db: db}
}

const camposSeccionInicio = `slug, nombre, orden, imagen_url, actualizada_en`

// ListAll no pagina: las secciones las siembra una migración, una por tarjeta
// del Inicio, así que son una docena.
func (r *SeccionInicioRepository) ListAll(ctx context.Context) ([]*domain.SeccionInicio, error) {
	sql := `SELECT ` + camposSeccionInicio + ` FROM core.secciones_inicio ORDER BY orden ASC, slug ASC`

	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var secciones []*domain.SeccionInicio
	for rows.Next() {
		var s domain.SeccionInicio
		if err := rows.Scan(&s.Slug, &s.Nombre, &s.Orden, &s.ImagenURL, &s.ActualizadaEn); err != nil {
			return nil, err
		}
		secciones = append(secciones, &s)
	}
	return secciones, rows.Err()
}

// ActualizarImagen no crea la fila si falta: el slug lo decide una migración,
// no el panel. Devuelve domain.ErrNotFound para que el handler responda 404.
func (r *SeccionInicioRepository) ActualizarImagen(ctx context.Context, slug, imagenURL string) (*domain.SeccionInicio, error) {
	sql := `
		UPDATE core.secciones_inicio
		SET imagen_url = $1, actualizada_en = CURRENT_TIMESTAMP
		WHERE slug = $2
		RETURNING ` + camposSeccionInicio

	var s domain.SeccionInicio
	err := r.db.QueryRow(ctx, sql, imagenURL, slug).Scan(
		&s.Slug, &s.Nombre, &s.Orden, &s.ImagenURL, &s.ActualizadaEn,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

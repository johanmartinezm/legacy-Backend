package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type AsesoriaUnidadRepository struct {
	db *pgxpool.Pool
}

func NewAsesoriaUnidadRepository(db *pgxpool.Pool) *AsesoriaUnidadRepository {
	return &AsesoriaUnidadRepository{db: db}
}

const camposAsesoriaUnidad = `id, titulo, subtitulo, icono, imagen_url, necesidad, orden, activa, creada_en, actualizada_en`

func escanearUnidad(row pgx.Row, u *domain.AsesoriaUnidad) error {
	return row.Scan(&u.ID, &u.Titulo, &u.Subtitulo, &u.Icono, &u.ImagenURL, &u.Necesidad, &u.Orden, &u.Activa, &u.CreadaEn, &u.ActualizadaEn)
}

// List no pagina: son las unidades de asesoría, una por empresa del grupo, y
// no pasan de unas pocas. soloActivas es lo que pide la app.
func (r *AsesoriaUnidadRepository) List(ctx context.Context, soloActivas bool) ([]*domain.AsesoriaUnidad, error) {
	sql := `SELECT ` + camposAsesoriaUnidad + ` FROM core.asesoria_unidades`
	if soloActivas {
		sql += ` WHERE activa`
	}
	sql += ` ORDER BY orden ASC, creada_en ASC, id ASC`

	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var unidades []*domain.AsesoriaUnidad
	for rows.Next() {
		var u domain.AsesoriaUnidad
		if err := escanearUnidad(rows, &u); err != nil {
			return nil, err
		}
		unidades = append(unidades, &u)
	}
	return unidades, rows.Err()
}

func (r *AsesoriaUnidadRepository) Create(ctx context.Context, u *domain.AsesoriaUnidad) error {
	sql := `
		INSERT INTO core.asesoria_unidades (titulo, subtitulo, icono, imagen_url, necesidad, orden, activa)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING ` + camposAsesoriaUnidad
	return escanearUnidad(r.db.QueryRow(ctx, sql, u.Titulo, u.Subtitulo, u.Icono, u.ImagenURL, u.Necesidad, u.Orden, u.Activa), u)
}

// idValido evita mandar a Postgres un id que no es uuid: la columna lo
// rechazaría con un error de tipo (500) cuando lo que pasa es que no existe.
func idValido(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}

// Update devuelve domain.ErrNotFound si el id no existe.
func (r *AsesoriaUnidadRepository) Update(ctx context.Context, u *domain.AsesoriaUnidad) error {
	if !idValido(u.ID) {
		return domain.ErrNotFound
	}
	sql := `
		UPDATE core.asesoria_unidades
		SET titulo = $1, subtitulo = $2, icono = $3, imagen_url = $4, necesidad = $5, orden = $6, activa = $7,
		    actualizada_en = CURRENT_TIMESTAMP
		WHERE id = $8
		RETURNING ` + camposAsesoriaUnidad
	err := escanearUnidad(r.db.QueryRow(ctx, sql, u.Titulo, u.Subtitulo, u.Icono, u.ImagenURL, u.Necesidad, u.Orden, u.Activa, u.ID), u)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// Delete devuelve domain.ErrNotFound si el id no existe.
func (r *AsesoriaUnidadRepository) Delete(ctx context.Context, id string) error {
	if !idValido(id) {
		return domain.ErrNotFound
	}
	tag, err := r.db.Exec(ctx, `DELETE FROM core.asesoria_unidades WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type PaginaRepository struct {
	db *pgxpool.Pool
}

func NewPaginaRepository(db *pgxpool.Pool) *PaginaRepository {
	return &PaginaRepository{db: db}
}

const camposPagina = `slug, titulo, subtitulo, imagen_url, cuerpo, publicada, actualizada_en`

func (r *PaginaRepository) GetBySlug(ctx context.Context, slug string) (*domain.PaginaInformativa, error) {
	sql := `SELECT ` + camposPagina + ` FROM core.paginas_informativas WHERE slug = $1`

	var p domain.PaginaInformativa
	err := r.db.QueryRow(ctx, sql, slug).Scan(
		&p.Slug, &p.Titulo, &p.Subtitulo, &p.ImagenURL, &p.Cuerpo, &p.Publicada, &p.ActualizadaEn,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// ListAll no pagina: las páginas las siembra una migración, una por pantalla de
// la app, así que la tabla se cuenta con los dedos de una mano. Si algún día
// dejan de ser un puñado, esta consulta necesita LIMIT y OFFSET como el resto.
func (r *PaginaRepository) ListAll(ctx context.Context) ([]*domain.PaginaInformativa, error) {
	sql := `SELECT ` + camposPagina + ` FROM core.paginas_informativas ORDER BY titulo ASC, slug ASC`

	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paginas []*domain.PaginaInformativa
	for rows.Next() {
		var p domain.PaginaInformativa
		if err := rows.Scan(
			&p.Slug, &p.Titulo, &p.Subtitulo, &p.ImagenURL, &p.Cuerpo, &p.Publicada, &p.ActualizadaEn,
		); err != nil {
			return nil, err
		}
		paginas = append(paginas, &p)
	}
	return paginas, rows.Err()
}

// Update no crea la fila si falta: el slug lo decide una migración, no el panel.
// Devuelve domain.ErrNotFound para que el handler pueda responder 404 en vez de
// tragarse en silencio una edición que no escribió nada.
func (r *PaginaRepository) Update(ctx context.Context, pagina *domain.PaginaInformativa) error {
	sql := `
		UPDATE core.paginas_informativas
		SET titulo = $1, subtitulo = $2, imagen_url = $3, cuerpo = $4, publicada = $5,
		    actualizada_en = CURRENT_TIMESTAMP
		WHERE slug = $6
		RETURNING ` + camposPagina + `
	`
	err := r.db.QueryRow(ctx, sql,
		pagina.Titulo, pagina.Subtitulo, pagina.ImagenURL, pagina.Cuerpo, pagina.Publicada, pagina.Slug,
	).Scan(
		&pagina.Slug, &pagina.Titulo, &pagina.Subtitulo, &pagina.ImagenURL,
		&pagina.Cuerpo, &pagina.Publicada, &pagina.ActualizadaEn,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

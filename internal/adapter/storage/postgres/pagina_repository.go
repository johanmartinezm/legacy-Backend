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

const camposPagina = `slug, titulo, subtitulo, imagen_url, cuerpo, publicada, actualizada_en,
	pregunta_acciones, boton_principal, boton_secundario`

func escanearPagina(row pgx.Row, p *domain.PaginaInformativa) error {
	return row.Scan(
		&p.Slug, &p.Titulo, &p.Subtitulo, &p.ImagenURL, &p.Cuerpo, &p.Publicada, &p.ActualizadaEn,
		&p.PreguntaAcciones, &p.BotonPrincipal, &p.BotonSecundario,
	)
}

func (r *PaginaRepository) GetBySlug(ctx context.Context, slug string) (*domain.PaginaInformativa, error) {
	sql := `SELECT ` + camposPagina + ` FROM core.paginas_informativas WHERE slug = $1`

	var p domain.PaginaInformativa
	err := escanearPagina(r.db.QueryRow(ctx, sql, slug), &p)
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
		if err := escanearPagina(rows, &p); err != nil {
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
		    pregunta_acciones = $6, boton_principal = $7, boton_secundario = $8,
		    actualizada_en = CURRENT_TIMESTAMP
		WHERE slug = $9
		RETURNING ` + camposPagina + `
	`
	err := escanearPagina(r.db.QueryRow(ctx, sql,
		pagina.Titulo, pagina.Subtitulo, pagina.ImagenURL, pagina.Cuerpo, pagina.Publicada,
		pagina.PreguntaAcciones, pagina.BotonPrincipal, pagina.BotonSecundario, pagina.Slug,
	), pagina)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

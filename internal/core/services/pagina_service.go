package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"
	"strings"
)

// ErrTituloVacio protege a la app: la pantalla pinta el título como encabezado,
// y una página sin título se ve como un error de la aplicación, no como una
// página a medio escribir.
var ErrTituloVacio = errors.New("el título de la página no puede estar vacío")

// ErrCuerpoDemasiadoLargo es del formulario, no de la base: merece un 400 y no
// el 500 genérico, para que el panel pueda decir qué pasó.
var ErrCuerpoDemasiadoLargo = errors.New("el contenido de la página es demasiado largo")

// Tope del cuerpo. No es una restricción de la base —la columna es text— sino
// del lado de la app: la pantalla lo pinta entero, sin paginar.
const maxCuerpoPagina = 20000

type PaginaService struct {
	repo ports.PaginaRepository
}

func NewPaginaService(repo ports.PaginaRepository) *PaginaService {
	return &PaginaService{repo: repo}
}

// GetPaginaPublicada es la que atiende a la app. Una página despublicada se
// trata como inexistente: si respondiera con el contenido, apagar la casilla
// del panel no serviría de nada.
func (s *PaginaService) GetPaginaPublicada(ctx context.Context, slug string) (*domain.PaginaInformativa, error) {
	pagina, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if !pagina.Publicada {
		return nil, domain.ErrNotFound
	}
	return pagina, nil
}

func (s *PaginaService) ListPaginas(ctx context.Context) ([]*domain.PaginaInformativa, error) {
	return s.repo.ListAll(ctx)
}

func (s *PaginaService) ActualizarPagina(ctx context.Context, pagina *domain.PaginaInformativa) error {
	pagina.Titulo = strings.TrimSpace(pagina.Titulo)
	pagina.Subtitulo = strings.TrimSpace(pagina.Subtitulo)
	pagina.ImagenURL = strings.TrimSpace(pagina.ImagenURL)
	pagina.Cuerpo = strings.TrimSpace(pagina.Cuerpo)

	if pagina.Titulo == "" {
		return ErrTituloVacio
	}
	if len(pagina.Cuerpo) > maxCuerpoPagina {
		return ErrCuerpoDemasiadoLargo
	}

	return s.repo.Update(ctx, pagina)
}

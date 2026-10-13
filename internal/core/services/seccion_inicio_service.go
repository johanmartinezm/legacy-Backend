package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ErrImagenNoEsURL: la app carga la imagen tal cual llega. Un valor que no sea
// una URL http(s) dejaría la tarjeta con una imagen rota en vez de su icono.
var ErrImagenNoEsURL = errors.New("la imagen debe ser una URL http(s) o quedar vacía")

// Las tarjetas del Inicio son de ancho fijo: un título o un texto más largo que
// esto se corta con puntos suspensivos y deja de leerse.
const (
	maxTituloSeccion = 60
	maxTextoSeccion  = 200
)

var (
	ErrTituloSeccionLargo = fmt.Errorf("el título admite hasta %d caracteres", maxTituloSeccion)
	ErrTextoSeccionLargo  = fmt.Errorf("el texto admite hasta %d caracteres", maxTextoSeccion)
)

type SeccionInicioService struct {
	repo ports.SeccionInicioRepository
}

func NewSeccionInicioService(repo ports.SeccionInicioRepository) *SeccionInicioService {
	return &SeccionInicioService{repo: repo}
}

func (s *SeccionInicioService) ListSecciones(ctx context.Context) ([]*domain.SeccionInicio, error) {
	return s.repo.ListAll(ctx)
}

// Actualizar admite cadena vacía en los tres campos: es como el panel quita la
// imagen o vuelve al título y al texto de siempre. Un campo nil no se toca.
func (s *SeccionInicioService) Actualizar(ctx context.Context, slug string, c domain.CambiosSeccionInicio) (*domain.SeccionInicio, error) {
	if c.ImagenURL != nil {
		imagenURL := strings.TrimSpace(*c.ImagenURL)
		if imagenURL != "" && !strings.HasPrefix(imagenURL, "https://") && !strings.HasPrefix(imagenURL, "http://") {
			return nil, ErrImagenNoEsURL
		}
		c.ImagenURL = &imagenURL
	}
	if c.Titulo != nil {
		titulo := strings.TrimSpace(*c.Titulo)
		if utf8.RuneCountInString(titulo) > maxTituloSeccion {
			return nil, ErrTituloSeccionLargo
		}
		c.Titulo = &titulo
	}
	if c.Texto != nil {
		texto := strings.TrimSpace(*c.Texto)
		if utf8.RuneCountInString(texto) > maxTextoSeccion {
			return nil, ErrTextoSeccionLargo
		}
		c.Texto = &texto
	}
	return s.repo.Actualizar(ctx, slug, c)
}

package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"
	"strings"
)

// ErrImagenNoEsURL: la app carga la imagen tal cual llega. Un valor que no sea
// una URL http(s) dejaría la tarjeta con una imagen rota en vez de su icono.
var ErrImagenNoEsURL = errors.New("la imagen debe ser una URL http(s) o quedar vacía")

type SeccionInicioService struct {
	repo ports.SeccionInicioRepository
}

func NewSeccionInicioService(repo ports.SeccionInicioRepository) *SeccionInicioService {
	return &SeccionInicioService{repo: repo}
}

func (s *SeccionInicioService) ListSecciones(ctx context.Context) ([]*domain.SeccionInicio, error) {
	return s.repo.ListAll(ctx)
}

// CambiarImagen admite cadena vacía, que es como el panel quita la imagen.
func (s *SeccionInicioService) CambiarImagen(ctx context.Context, slug, imagenURL string) (*domain.SeccionInicio, error) {
	imagenURL = strings.TrimSpace(imagenURL)
	if imagenURL != "" && !strings.HasPrefix(imagenURL, "https://") && !strings.HasPrefix(imagenURL, "http://") {
		return nil, ErrImagenNoEsURL
	}
	return s.repo.ActualizarImagen(ctx, slug, imagenURL)
}

package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// La tarjeta de una unidad en la app es de ancho fijo: título a una línea y
// subtítulo a dos. Por encima de esto se corta con puntos suspensivos.
const (
	maxTituloUnidad    = 60
	maxSubtituloUnidad = 120
)

var (
	ErrUnidadSinTitulo      = errors.New("el título de la unidad es obligatorio")
	ErrUnidadTituloLargo    = fmt.Errorf("el título admite hasta %d caracteres", maxTituloUnidad)
	ErrUnidadSubtituloLargo = fmt.Errorf("el subtítulo admite hasta %d caracteres", maxSubtituloUnidad)
	ErrUnidadNecesidad      = errors.New("la necesidad no es una de las opciones del formulario")
	ErrUnidadIcono          = errors.New("el icono no es uno de los disponibles")
	ErrUnidadImagen         = errors.New("la imagen debe ser una dirección http:// o https://")
)

// EsErrorDeValidacionUnidad dice si el error es del formulario (400) y no del
// servidor (500).
func EsErrorDeValidacionUnidad(err error) bool {
	for _, e := range []error{ErrUnidadSinTitulo, ErrUnidadTituloLargo, ErrUnidadSubtituloLargo, ErrUnidadNecesidad, ErrUnidadIcono, ErrUnidadImagen} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

type AsesoriaUnidadService struct {
	repo ports.AsesoriaUnidadRepository
}

func NewAsesoriaUnidadService(repo ports.AsesoriaUnidadRepository) *AsesoriaUnidadService {
	return &AsesoriaUnidadService{repo: repo}
}

func (s *AsesoriaUnidadService) ListActivas(ctx context.Context) ([]*domain.AsesoriaUnidad, error) {
	return s.repo.List(ctx, true)
}

func (s *AsesoriaUnidadService) ListTodas(ctx context.Context) ([]*domain.AsesoriaUnidad, error) {
	return s.repo.List(ctx, false)
}

func (s *AsesoriaUnidadService) Crear(ctx context.Context, u *domain.AsesoriaUnidad) error {
	if err := validarUnidad(u); err != nil {
		return err
	}
	return s.repo.Create(ctx, u)
}

func (s *AsesoriaUnidadService) Actualizar(ctx context.Context, u *domain.AsesoriaUnidad) error {
	if err := validarUnidad(u); err != nil {
		return err
	}
	return s.repo.Update(ctx, u)
}

func (s *AsesoriaUnidadService) Borrar(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// validarUnidad recorta los textos y comprueba lo que la app necesita para
// pintar la tarjeta y preseleccionar la necesidad del formulario.
func validarUnidad(u *domain.AsesoriaUnidad) error {
	u.Titulo = strings.TrimSpace(u.Titulo)
	u.Subtitulo = strings.TrimSpace(u.Subtitulo)
	u.Icono = strings.TrimSpace(u.Icono)
	u.Necesidad = strings.TrimSpace(u.Necesidad)
	u.ImagenURL = strings.TrimSpace(u.ImagenURL)

	if u.Titulo == "" {
		return ErrUnidadSinTitulo
	}
	if utf8.RuneCountInString(u.Titulo) > maxTituloUnidad {
		return ErrUnidadTituloLargo
	}
	if utf8.RuneCountInString(u.Subtitulo) > maxSubtituloUnidad {
		return ErrUnidadSubtituloLargo
	}
	if !slices.Contains(domain.NecesidadesAsesoria, u.Necesidad) {
		return ErrUnidadNecesidad
	}
	if u.Icono == "" {
		u.Icono = "negocio"
	}
	if !slices.Contains(domain.IconosAsesoria, u.Icono) {
		return ErrUnidadIcono
	}
	// La app la pinta tal cual con una petición de red: cualquier otra cosa
	// (una ruta suelta, un «javascript:») no se puede cargar.
	if u.ImagenURL != "" && !strings.HasPrefix(u.ImagenURL, "https://") && !strings.HasPrefix(u.ImagenURL, "http://") {
		return ErrUnidadImagen
	}
	return nil
}

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

const (
	maxTituloBeneficio      = 80
	maxDescripcionBeneficio = 500
)

var (
	ErrBeneficioSinTitulo        = errors.New("el título del beneficio es obligatorio")
	ErrBeneficioTituloLargo      = fmt.Errorf("el título admite hasta %d caracteres", maxTituloBeneficio)
	ErrBeneficioDescripcionLarga = fmt.Errorf("la descripción admite hasta %d caracteres", maxDescripcionBeneficio)
	ErrBeneficioImagenNoEsURL    = errors.New("la imagen debe ser una URL http(s) o quedar vacía")
	ErrBeneficioEnlaceNoEsURL    = errors.New("el enlace debe ser una URL http(s) o quedar vacío")
	ErrBeneficioTipo             = errors.New("el tipo debe ser «beneficio» o «alianza»")
)

// EsErrorDeValidacionBeneficio dice si el error es del formulario (400).
func EsErrorDeValidacionBeneficio(err error) bool {
	for _, e := range []error{ErrBeneficioSinTitulo, ErrBeneficioTituloLargo, ErrBeneficioDescripcionLarga,
		ErrBeneficioImagenNoEsURL, ErrBeneficioEnlaceNoEsURL, ErrBeneficioTipo} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

type BeneficioService struct {
	repo ports.BeneficioRepository
}

func NewBeneficioService(repo ports.BeneficioRepository) *BeneficioService {
	return &BeneficioService{repo: repo}
}

func (s *BeneficioService) ListActivos(ctx context.Context) ([]*domain.Beneficio, error) {
	return s.repo.List(ctx, true)
}

func (s *BeneficioService) ListTodos(ctx context.Context) ([]*domain.Beneficio, error) {
	return s.repo.List(ctx, false)
}

func (s *BeneficioService) Crear(ctx context.Context, b *domain.Beneficio) error {
	if err := validarBeneficio(b); err != nil {
		return err
	}
	return s.repo.Create(ctx, b)
}

func (s *BeneficioService) Actualizar(ctx context.Context, b *domain.Beneficio) error {
	if err := validarBeneficio(b); err != nil {
		return err
	}
	return s.repo.Update(ctx, b)
}

func (s *BeneficioService) Borrar(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func esURLoVacio(v string) bool {
	return v == "" || strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "http://")
}

// validarBeneficio recorta los textos y comprueba lo que la app necesita: la
// imagen la carga tal cual y el enlace lo abre en el navegador, así que deben
// ser URL http(s).
func validarBeneficio(b *domain.Beneficio) error {
	b.Tipo = strings.TrimSpace(b.Tipo)
	if b.Tipo == "" {
		b.Tipo = "beneficio"
	}
	b.Titulo = strings.TrimSpace(b.Titulo)
	b.Descripcion = strings.TrimSpace(b.Descripcion)
	b.ImagenURL = strings.TrimSpace(b.ImagenURL)
	b.EnlaceURL = strings.TrimSpace(b.EnlaceURL)

	switch {
	case !slices.Contains(domain.TiposBeneficio, b.Tipo):
		return ErrBeneficioTipo
	case b.Titulo == "":
		return ErrBeneficioSinTitulo
	case utf8.RuneCountInString(b.Titulo) > maxTituloBeneficio:
		return ErrBeneficioTituloLargo
	case utf8.RuneCountInString(b.Descripcion) > maxDescripcionBeneficio:
		return ErrBeneficioDescripcionLarga
	case !esURLoVacio(b.ImagenURL):
		return ErrBeneficioImagenNoEsURL
	case !esURLoVacio(b.EnlaceURL):
		return ErrBeneficioEnlaceNoEsURL
	}
	return nil
}

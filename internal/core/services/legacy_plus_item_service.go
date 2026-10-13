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
	maxTituloLegacyPlus = 60
	maxTextoLegacyPlus  = 500
)

var (
	ErrLegacyPlusSinTitulo   = errors.New("el título del punto es obligatorio")
	ErrLegacyPlusTituloLargo = fmt.Errorf("el título admite hasta %d caracteres", maxTituloLegacyPlus)
	ErrLegacyPlusTextoLargo  = fmt.Errorf("el texto admite hasta %d caracteres", maxTextoLegacyPlus)
	ErrLegacyPlusGrupo       = errors.New("el grupo debe ser «incluye» o «beneficios»")
)

// EsErrorDeValidacionLegacyPlus dice si el error es del formulario (400).
func EsErrorDeValidacionLegacyPlus(err error) bool {
	for _, e := range []error{ErrLegacyPlusSinTitulo, ErrLegacyPlusTituloLargo, ErrLegacyPlusTextoLargo, ErrLegacyPlusGrupo} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

type LegacyPlusItemService struct {
	repo ports.LegacyPlusItemRepository
}

func NewLegacyPlusItemService(repo ports.LegacyPlusItemRepository) *LegacyPlusItemService {
	return &LegacyPlusItemService{repo: repo}
}

func (s *LegacyPlusItemService) ListActivos(ctx context.Context) ([]*domain.LegacyPlusItem, error) {
	return s.repo.List(ctx, true)
}

func (s *LegacyPlusItemService) ListTodos(ctx context.Context) ([]*domain.LegacyPlusItem, error) {
	return s.repo.List(ctx, false)
}

func (s *LegacyPlusItemService) Crear(ctx context.Context, i *domain.LegacyPlusItem) error {
	if err := validarLegacyPlusItem(i); err != nil {
		return err
	}
	return s.repo.Create(ctx, i)
}

func (s *LegacyPlusItemService) Actualizar(ctx context.Context, i *domain.LegacyPlusItem) error {
	if err := validarLegacyPlusItem(i); err != nil {
		return err
	}
	return s.repo.Update(ctx, i)
}

func (s *LegacyPlusItemService) Borrar(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func validarLegacyPlusItem(i *domain.LegacyPlusItem) error {
	i.Grupo = strings.TrimSpace(i.Grupo)
	i.Titulo = strings.TrimSpace(i.Titulo)
	i.Texto = strings.TrimSpace(i.Texto)

	if !slices.Contains(domain.GruposLegacyPlus, i.Grupo) {
		return ErrLegacyPlusGrupo
	}
	if i.Titulo == "" {
		return ErrLegacyPlusSinTitulo
	}
	if utf8.RuneCountInString(i.Titulo) > maxTituloLegacyPlus {
		return ErrLegacyPlusTituloLargo
	}
	if utf8.RuneCountInString(i.Texto) > maxTextoLegacyPlus {
		return ErrLegacyPlusTextoLargo
	}
	return nil
}

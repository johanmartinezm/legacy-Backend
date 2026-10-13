package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"strings"
	"testing"
)

type repoLegacyPlusFake struct {
	guardado *domain.LegacyPlusItem
}

func (r *repoLegacyPlusFake) List(ctx context.Context, soloActivos bool) ([]*domain.LegacyPlusItem, error) {
	return nil, nil
}
func (r *repoLegacyPlusFake) Create(ctx context.Context, i *domain.LegacyPlusItem) error {
	r.guardado = i
	return nil
}
func (r *repoLegacyPlusFake) Update(ctx context.Context, i *domain.LegacyPlusItem) error {
	r.guardado = i
	return nil
}
func (r *repoLegacyPlusFake) Delete(ctx context.Context, id string) error { return nil }

func TestValidarPuntoLegacyPlus(t *testing.T) {
	valido := func() *domain.LegacyPlusItem {
		return &domain.LegacyPlusItem{Grupo: " beneficios ", Titulo: " Aliados ", Texto: " Vitalea "}
	}

	t.Run("un punto válido se guarda recortado", func(t *testing.T) {
		repo := &repoLegacyPlusFake{}
		if err := NewLegacyPlusItemService(repo).Crear(context.Background(), valido()); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		g := repo.guardado
		if g.Grupo != "beneficios" || g.Titulo != "Aliados" || g.Texto != "Vitalea" {
			t.Fatalf("no recortó: %+v", g)
		}
	})

	casos := []struct {
		nombre string
		cambia func(*domain.LegacyPlusItem)
		err    error
	}{
		{"grupo inventado", func(i *domain.LegacyPlusItem) { i.Grupo = "otros" }, ErrLegacyPlusGrupo},
		{"sin título", func(i *domain.LegacyPlusItem) { i.Titulo = " " }, ErrLegacyPlusSinTitulo},
		{"título largo", func(i *domain.LegacyPlusItem) { i.Titulo = strings.Repeat("a", 61) }, ErrLegacyPlusTituloLargo},
		{"texto largo", func(i *domain.LegacyPlusItem) { i.Texto = strings.Repeat("a", 501) }, ErrLegacyPlusTextoLargo},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			repo := &repoLegacyPlusFake{}
			i := valido()
			c.cambia(i)
			err := NewLegacyPlusItemService(repo).Actualizar(context.Background(), i)
			if !errors.Is(err, c.err) || !EsErrorDeValidacionLegacyPlus(err) {
				t.Fatalf("error = %v, se esperaba %v (de validación)", err, c.err)
			}
			if repo.guardado != nil {
				t.Fatal("con error no debía escribir nada")
			}
		})
	}
}

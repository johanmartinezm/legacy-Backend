package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"strings"
	"testing"
)

type repoBeneficiosFake struct {
	guardado *domain.Beneficio
}

func (r *repoBeneficiosFake) List(ctx context.Context, soloActivos bool) ([]*domain.Beneficio, error) {
	return nil, nil
}
func (r *repoBeneficiosFake) Create(ctx context.Context, b *domain.Beneficio) error {
	r.guardado = b
	return nil
}
func (r *repoBeneficiosFake) Update(ctx context.Context, b *domain.Beneficio) error {
	r.guardado = b
	return nil
}
func (r *repoBeneficiosFake) Delete(ctx context.Context, id string) error { return nil }

func TestValidarBeneficio(t *testing.T) {
	valido := func() *domain.Beneficio {
		return &domain.Beneficio{
			Titulo: " Vitalea ", Descripcion: " 20% ",
			ImagenURL: " https://x.co/v.png ", EnlaceURL: "https://vitalea.co",
		}
	}

	t.Run("válido, recortado; imagen y enlace opcionales", func(t *testing.T) {
		repo := &repoBeneficiosFake{}
		if err := NewBeneficioService(repo).Crear(context.Background(), valido()); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if repo.guardado.Titulo != "Vitalea" || repo.guardado.ImagenURL != "https://x.co/v.png" || repo.guardado.Tipo != "beneficio" {
			t.Fatalf("no recortó: %+v", repo.guardado)
		}
		sinNada := &domain.Beneficio{Titulo: "Solo título"}
		if err := NewBeneficioService(repo).Crear(context.Background(), sinNada); err != nil {
			t.Fatalf("sin imagen ni enlace debía valer: %v", err)
		}
	})

	casos := []struct {
		nombre string
		cambia func(*domain.Beneficio)
		err    error
	}{
		{"sin título", func(b *domain.Beneficio) { b.Titulo = "" }, ErrBeneficioSinTitulo},
		{"título largo", func(b *domain.Beneficio) { b.Titulo = strings.Repeat("a", 81) }, ErrBeneficioTituloLargo},
		{"descripción larga", func(b *domain.Beneficio) { b.Descripcion = strings.Repeat("a", 501) }, ErrBeneficioDescripcionLarga},
		{"imagen sin esquema", func(b *domain.Beneficio) { b.ImagenURL = "x.co/v.png" }, ErrBeneficioImagenNoEsURL},
		{"enlace javascript", func(b *domain.Beneficio) { b.EnlaceURL = "javascript:alert(1)" }, ErrBeneficioEnlaceNoEsURL},
		{"tipo inventado", func(b *domain.Beneficio) { b.Tipo = "promo" }, ErrBeneficioTipo},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			repo := &repoBeneficiosFake{}
			b := valido()
			c.cambia(b)
			err := NewBeneficioService(repo).Actualizar(context.Background(), b)
			if !errors.Is(err, c.err) || !EsErrorDeValidacionBeneficio(err) {
				t.Fatalf("error = %v, se esperaba %v", err, c.err)
			}
			if repo.guardado != nil {
				t.Fatal("con error no debía escribir nada")
			}
		})
	}
}

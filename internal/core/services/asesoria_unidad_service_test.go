package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"strings"
	"testing"
)

type repoUnidadesFake struct {
	creada *domain.AsesoriaUnidad
}

func (r *repoUnidadesFake) List(ctx context.Context, soloActivas bool) ([]*domain.AsesoriaUnidad, error) {
	return nil, nil
}
func (r *repoUnidadesFake) Create(ctx context.Context, u *domain.AsesoriaUnidad) error {
	r.creada = u
	return nil
}
func (r *repoUnidadesFake) Update(ctx context.Context, u *domain.AsesoriaUnidad) error {
	r.creada = u
	return nil
}
func (r *repoUnidadesFake) Delete(ctx context.Context, id string) error { return nil }

func TestValidarUnidadAsesoria(t *testing.T) {
	valida := func() *domain.AsesoriaUnidad {
		return &domain.AsesoriaUnidad{Titulo: "  Aurum ", Subtitulo: " Finanzas ", Icono: "crecimiento", Necesidad: "Patrimonio"}
	}

	t.Run("una unidad válida se guarda recortada", func(t *testing.T) {
		repo := &repoUnidadesFake{}
		if err := NewAsesoriaUnidadService(repo).Crear(context.Background(), valida()); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if repo.creada.Titulo != "Aurum" || repo.creada.Subtitulo != "Finanzas" {
			t.Fatalf("no recortó: %+v", repo.creada)
		}
	})

	t.Run("sin icono toma el genérico", func(t *testing.T) {
		repo := &repoUnidadesFake{}
		u := valida()
		u.Icono = ""
		if err := NewAsesoriaUnidadService(repo).Crear(context.Background(), u); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if repo.creada.Icono != "negocio" {
			t.Fatalf("icono = %q, se esperaba «negocio»", repo.creada.Icono)
		}
	})

	t.Run("la imagen se guarda recortada y vacía vale", func(t *testing.T) {
		repo := &repoUnidadesFake{}
		u := valida()
		u.ImagenURL = "  https://app.legacynetworkco.com/api/images/x.png "
		if err := NewAsesoriaUnidadService(repo).Crear(context.Background(), u); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if repo.creada.ImagenURL != "https://app.legacynetworkco.com/api/images/x.png" {
			t.Fatalf("imagen = %q", repo.creada.ImagenURL)
		}
		if err := NewAsesoriaUnidadService(repo).Crear(context.Background(), valida()); err != nil {
			t.Fatalf("sin imagen debía valer: %v", err)
		}
	})

	casos := []struct {
		nombre string
		cambia func(*domain.AsesoriaUnidad)
		err    error
	}{
		{"sin título", func(u *domain.AsesoriaUnidad) { u.Titulo = "   " }, ErrUnidadSinTitulo},
		{"título largo", func(u *domain.AsesoriaUnidad) { u.Titulo = strings.Repeat("a", 61) }, ErrUnidadTituloLargo},
		{"subtítulo largo", func(u *domain.AsesoriaUnidad) { u.Subtitulo = strings.Repeat("a", 121) }, ErrUnidadSubtituloLargo},
		{"necesidad inventada", func(u *domain.AsesoriaUnidad) { u.Necesidad = "Cripto" }, ErrUnidadNecesidad},
		{"icono inventado", func(u *domain.AsesoriaUnidad) { u.Icono = "cohete" }, ErrUnidadIcono},
		{"imagen sin http", func(u *domain.AsesoriaUnidad) { u.ImagenURL = "logo.png" }, ErrUnidadImagen},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			repo := &repoUnidadesFake{}
			u := valida()
			c.cambia(u)
			err := NewAsesoriaUnidadService(repo).Actualizar(context.Background(), u)
			if !errors.Is(err, c.err) {
				t.Fatalf("error = %v, se esperaba %v", err, c.err)
			}
			if !EsErrorDeValidacionUnidad(err) {
				t.Fatal("debería contar como error de validación (400)")
			}
			if repo.creada != nil {
				t.Fatal("con error no debía escribir nada")
			}
		})
	}
}

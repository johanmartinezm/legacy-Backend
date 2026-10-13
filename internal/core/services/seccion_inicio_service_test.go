package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"strings"
	"testing"
)

type repoSeccionesFake struct {
	guardado *domain.CambiosSeccionInicio
}

func (r *repoSeccionesFake) ListAll(ctx context.Context) ([]*domain.SeccionInicio, error) {
	return nil, nil
}

func (r *repoSeccionesFake) Actualizar(ctx context.Context, slug string, c domain.CambiosSeccionInicio) (*domain.SeccionInicio, error) {
	r.guardado = &c
	return &domain.SeccionInicio{Slug: slug}, nil
}

func ptr(s string) *string { return &s }

func TestCambiarImagenSeccionInicio(t *testing.T) {
	casos := []struct {
		nombre   string
		entrada  string
		guardado string
		err      error
	}{
		{"URL https se guarda recortada", "  https://app.legacynetworkco.com/api/images/a.jpg ", "https://app.legacynetworkco.com/api/images/a.jpg", nil},
		{"vacía quita la imagen", "   ", "", nil},
		{"sin esquema se rechaza", "app.legacynetworkco.com/a.jpg", "", ErrImagenNoEsURL},
		{"javascript: se rechaza", "javascript:alert(1)", "", ErrImagenNoEsURL},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			repo := &repoSeccionesFake{}
			_, err := NewSeccionInicioService(repo).Actualizar(context.Background(), "eventos",
				domain.CambiosSeccionInicio{ImagenURL: ptr(c.entrada)})

			if !errors.Is(err, c.err) {
				t.Fatalf("error = %v, se esperaba %v", err, c.err)
			}
			if c.err != nil {
				if repo.guardado != nil {
					t.Fatalf("con error no debía escribir nada, escribió %+v", *repo.guardado)
				}
				return
			}
			if repo.guardado == nil || repo.guardado.ImagenURL == nil || *repo.guardado.ImagenURL != c.guardado {
				t.Fatalf("guardó %+v, se esperaba %q", repo.guardado, c.guardado)
			}
		})
	}
}

func TestCambiarTextosSeccionInicio(t *testing.T) {
	t.Run("título y texto se guardan recortados", func(t *testing.T) {
		repo := &repoSeccionesFake{}
		_, err := NewSeccionInicioService(repo).Actualizar(context.Background(), "eventos",
			domain.CambiosSeccionInicio{Titulo: ptr("  Eventos 2026 "), Texto: ptr(" Agenda y talleres  ")})
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if *repo.guardado.Titulo != "Eventos 2026" || *repo.guardado.Texto != "Agenda y talleres" {
			t.Fatalf("guardó título %q y texto %q", *repo.guardado.Titulo, *repo.guardado.Texto)
		}
	})

	t.Run("lo que no viene no se toca", func(t *testing.T) {
		repo := &repoSeccionesFake{}
		_, err := NewSeccionInicioService(repo).Actualizar(context.Background(), "eventos",
			domain.CambiosSeccionInicio{ImagenURL: ptr("https://a.co/x.jpg")})
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if repo.guardado.Titulo != nil || repo.guardado.Texto != nil {
			t.Fatalf("un cliente que solo manda la imagen no debe tocar título ni texto: %+v", *repo.guardado)
		}
	})

	t.Run("el largo se cuenta en caracteres, no en bytes", func(t *testing.T) {
		repo := &repoSeccionesFake{}
		// 60 «ñ» son 120 bytes: deben caber.
		_, err := NewSeccionInicioService(repo).Actualizar(context.Background(), "eventos",
			domain.CambiosSeccionInicio{Titulo: ptr(strings.Repeat("ñ", maxTituloSeccion))})
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
	})

	casos := []struct {
		nombre  string
		cambios domain.CambiosSeccionInicio
		err     error
	}{
		{"título demasiado largo", domain.CambiosSeccionInicio{Titulo: ptr(strings.Repeat("a", maxTituloSeccion+1))}, ErrTituloSeccionLargo},
		{"texto demasiado largo", domain.CambiosSeccionInicio{Texto: ptr(strings.Repeat("a", maxTextoSeccion+1))}, ErrTextoSeccionLargo},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			repo := &repoSeccionesFake{}
			_, err := NewSeccionInicioService(repo).Actualizar(context.Background(), "eventos", c.cambios)
			if !errors.Is(err, c.err) {
				t.Fatalf("error = %v, se esperaba %v", err, c.err)
			}
			if repo.guardado != nil {
				t.Fatal("con error no debía escribir nada")
			}
		})
	}
}

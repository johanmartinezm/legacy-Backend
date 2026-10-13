package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"testing"
)

type repoSeccionesFake struct {
	guardado *string
}

func (r *repoSeccionesFake) ListAll(ctx context.Context) ([]*domain.SeccionInicio, error) {
	return nil, nil
}

func (r *repoSeccionesFake) ActualizarImagen(ctx context.Context, slug, imagenURL string) (*domain.SeccionInicio, error) {
	r.guardado = &imagenURL
	return &domain.SeccionInicio{Slug: slug, ImagenURL: imagenURL}, nil
}

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
			_, err := NewSeccionInicioService(repo).CambiarImagen(context.Background(), "eventos", c.entrada)

			if !errors.Is(err, c.err) {
				t.Fatalf("error = %v, se esperaba %v", err, c.err)
			}
			if c.err != nil {
				if repo.guardado != nil {
					t.Fatalf("con error no debía escribir nada, escribió %q", *repo.guardado)
				}
				return
			}
			if repo.guardado == nil || *repo.guardado != c.guardado {
				t.Fatalf("guardó %v, se esperaba %q", repo.guardado, c.guardado)
			}
		})
	}
}

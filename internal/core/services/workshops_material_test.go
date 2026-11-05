package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"applegacy/backend/internal/core/domain"
)

// Material de cada workshop (parte A del plan de workshops).

func TestEnlacesDeVideo(t *testing.T) {
	validos := []string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ",
		"https://m.youtube.com/watch?v=dQw4w9WgXcQ&t=30",
		"https://www.youtube.com/shorts/abc123",
		"https://www.youtube.com/live/abc123",
		"https://vimeo.com/123456789",
		"https://vimeo.com/123456789/a1b2c3d4e5", // no listado, con su código
		"https://player.vimeo.com/video/123456789?h=a1b2c3",
	}
	for _, v := range validos {
		if err := validarEnlaceDeVideo(v); err != nil {
			t.Errorf("%s: se rechazó (%v)", v, err)
		}
	}

	invalidos := map[string]string{
		"http://www.youtube.com/watch?v=x": "https://",
		"https://www.dailymotion.com/v/x1": "YouTube o de Vimeo",
		"https://www.youtube.com/":         "no apunta a un video",
		"https://www.youtube.com/watch":    "no apunta a un video",
		"https://vimeo.com/canal/algo":     "no apunta a un video",
		"no es un enlace":                  "no es una dirección válida",
	}
	for v, motivo := range invalidos {
		err := validarEnlaceDeVideo(v)
		var e *ErrorMaterial
		if !errors.As(err, &e) || !strings.Contains(e.Motivo, motivo) {
			t.Errorf("%s: err = %v, se esperaba un motivo con %q", v, err, motivo)
		}
	}
}

const docSubido = "0f8fad5b-d9cb-469f-a165-70867728950e_Presentacion-final.pdf"

func TestNormalizarMaterial(t *testing.T) {
	t.Run("completo", func(t *testing.T) {
		m, err := normalizarMaterial(domain.MaterialDeWorkshop{
			ImageUrl: " foto.jpg ", FileName: docSubido, FileSize: 1024,
			VideoUrl: "https://youtu.be/abc",
		})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if m.ImageUrl != "foto.jpg" || m.FileOriginalName != "Presentacion-final.pdf" {
			t.Errorf("material = %+v", m)
		}
	})

	t.Run("vacío es válido: es quitar el material", func(t *testing.T) {
		m, err := normalizarMaterial(domain.MaterialDeWorkshop{FileOriginalName: "suelto.pdf", FileSize: 9})
		if err != nil || !m.Vacio() || m.FileSize != 0 {
			t.Fatalf("m = %+v, err = %v", m, err)
		}
	})

	casos := map[string]domain.MaterialDeWorkshop{
		"subido desde el panel": {FileName: "../../config.yaml", FileSize: 10},
		"como mucho 25 MB":      {FileName: docSubido, FileSize: MaxTamanoDocumento + 1},
		"YouTube o de Vimeo":    {VideoUrl: "https://example.com/video"},
	}
	for motivo, m := range casos {
		_, err := normalizarMaterial(m)
		var e *ErrorMaterial
		if !errors.As(err, &e) || !strings.Contains(e.Motivo, motivo) {
			t.Errorf("%s: err = %v", motivo, err)
		}
	}
}

type repoDeMaterial struct {
	MockEventRepository
	guardado *domain.MaterialDeWorkshop
}

func (r *repoDeMaterial) UpdateWorkshopMaterial(ctx context.Context, id string, m domain.MaterialDeWorkshop) error {
	if id != "ws-1" {
		return domain.ErrNotFound
	}
	r.guardado = &m
	return nil
}

func (r *repoDeMaterial) GetWorkshopByID(ctx context.Context, id string) (*domain.Workshop, error) {
	return &domain.Workshop{ID: id, Material: *r.guardado}, nil
}

func TestActualizarMaterialWorkshop(t *testing.T) {
	repo := &repoDeMaterial{}
	svc := NewEventService(repo, nil)

	w, err := svc.ActualizarMaterialWorkshop(context.Background(), "ws-1",
		domain.MaterialDeWorkshop{VideoUrl: "https://vimeo.com/123"})
	if err != nil || w.Material.VideoUrl != "https://vimeo.com/123" {
		t.Fatalf("w = %+v, err = %v", w, err)
	}

	// Un material inválido no llega al repositorio.
	repo.guardado = nil
	if _, err := svc.ActualizarMaterialWorkshop(context.Background(), "ws-1",
		domain.MaterialDeWorkshop{VideoUrl: "https://example.com"}); err == nil || repo.guardado != nil {
		t.Fatalf("se guardó un material inválido: err = %v", err)
	}

	if _, err := svc.ActualizarMaterialWorkshop(context.Background(), "otro",
		domain.MaterialDeWorkshop{}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, se esperaba ErrNotFound", err)
	}
}

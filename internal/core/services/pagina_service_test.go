package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"strings"
	"testing"
)

type repoPaginasFake struct {
	guardada *domain.PaginaInformativa
}

func (r *repoPaginasFake) GetBySlug(ctx context.Context, slug string) (*domain.PaginaInformativa, error) {
	return nil, domain.ErrNotFound
}
func (r *repoPaginasFake) ListAll(ctx context.Context) ([]*domain.PaginaInformativa, error) {
	return nil, nil
}
func (r *repoPaginasFake) Update(ctx context.Context, p *domain.PaginaInformativa) error {
	r.guardada = p
	return nil
}

// Los textos de los botones de Legacy Board se recortan y tienen tope; vacíos
// valen, porque la app pone entonces los de siempre.
func TestActualizarPaginaBotones(t *testing.T) {
	base := func() *domain.PaginaInformativa {
		return &domain.PaginaInformativa{Slug: "legacy-board", Titulo: "Legacy Board"}
	}

	t.Run("se recortan y vacíos valen", func(t *testing.T) {
		repo := &repoPaginasFake{}
		p := base()
		p.PreguntaAcciones = "  ¿Cómo quiere participar? "
		p.BotonPrincipal = " Quiero ser parte "
		if err := NewPaginaService(repo).ActualizarPagina(context.Background(), p); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		g := repo.guardada
		if g.PreguntaAcciones != "¿Cómo quiere participar?" || g.BotonPrincipal != "Quiero ser parte" || g.BotonSecundario != "" {
			t.Fatalf("no recortó: %+v", g)
		}
	})

	// 40 letras con tilde caben: el tope cuenta letras, no bytes.
	t.Run("el tope cuenta letras", func(t *testing.T) {
		p := base()
		p.BotonPrincipal = strings.Repeat("á", 40)
		if err := NewPaginaService(&repoPaginasFake{}).ActualizarPagina(context.Background(), p); err != nil {
			t.Fatalf("40 letras debían caber: %v", err)
		}
	})

	largos := map[string]func(*domain.PaginaInformativa){
		"pregunta":   func(p *domain.PaginaInformativa) { p.PreguntaAcciones = strings.Repeat("a", 81) },
		"principal":  func(p *domain.PaginaInformativa) { p.BotonPrincipal = strings.Repeat("a", 41) },
		"secundario": func(p *domain.PaginaInformativa) { p.BotonSecundario = strings.Repeat("a", 41) },
	}
	for nombre, cambia := range largos {
		t.Run(nombre+" largo", func(t *testing.T) {
			repo := &repoPaginasFake{}
			p := base()
			cambia(p)
			err := NewPaginaService(repo).ActualizarPagina(context.Background(), p)
			if !errors.Is(err, ErrTextoBotonLargo) {
				t.Fatalf("error = %v, se esperaba ErrTextoBotonLargo", err)
			}
			if repo.guardada != nil {
				t.Fatal("con error no debía escribir nada")
			}
		})
	}
}

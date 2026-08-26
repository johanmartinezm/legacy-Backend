package services

import (
	"context"
	"testing"
	"time"

	"applegacy/backend/internal/core/domain"
)

// `published_at` la ponia quien llamaba, es decir nadie: el formulario del
// panel no tiene ese campo. Los dos contenidos publicados en produccion la
// tenian vacia, y la app la usa para la fecha que muestra bajo el titulo
// (custom_content_model.dart:63), asi que el video salia como «0 vistas • »
// con la fecha en blanco.

type repoDeContenido struct {
	guardado  *domain.CustomContent
	existente *domain.CustomContent
}

func (r *repoDeContenido) Create(ctx context.Context, c *domain.CustomContent) error {
	r.guardado = c
	return nil
}

func (r *repoDeContenido) Update(ctx context.Context, c *domain.CustomContent) error {
	r.guardado = c
	return nil
}

func (r *repoDeContenido) Delete(ctx context.Context, id string) error { return nil }

func (r *repoDeContenido) List(ctx context.Context, categorySlug string, onlyPublished bool) ([]*domain.CustomContent, error) {
	return nil, nil
}

func (r *repoDeContenido) GetByID(ctx context.Context, id string) (*domain.CustomContent, error) {
	return r.existente, nil
}

func servicioDeContenido(repo *repoDeContenido) *ContentService {
	return NewContentService(nil, repo)
}

func TestPublicar_PoneLaFecha(t *testing.T) {
	repo := &repoDeContenido{}
	svc := servicioDeContenido(repo)

	// Tal cual lo manda el panel: sin published_at, porque su formulario no lo
	// tiene.
	err := svc.CreateContent(context.Background(), &domain.CustomContent{
		Title:       "Claves para la Sucesión",
		IsPublished: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.guardado.PublishedAt == nil {
		t.Fatal("se guardo publicado y sin fecha: es lo que dejaba la fecha en blanco en la app")
	}
	if time.Since(*repo.guardado.PublishedAt) > time.Minute {
		t.Errorf("la fecha no es de ahora: %v", repo.guardado.PublishedAt)
	}
}

func TestBorrador_NoLlevaFecha(t *testing.T) {
	repo := &repoDeContenido{}
	svc := servicioDeContenido(repo)

	if err := svc.CreateContent(context.Background(), &domain.CustomContent{
		Title:       "Borrador",
		IsPublished: false,
	}); err != nil {
		t.Fatal(err)
	}
	if repo.guardado.PublishedAt != nil {
		t.Error("un borrador no se ha publicado, no deberia tener fecha de publicacion")
	}
}

func TestGuardarDeNuevo_ConservaLaFechaOriginal(t *testing.T) {
	original := time.Date(2026, 2, 27, 10, 0, 0, 0, time.UTC)
	repo := &repoDeContenido{existente: &domain.CustomContent{
		ID:          "c1",
		IsPublished: true,
		PublishedAt: &original,
	}}
	svc := servicioDeContenido(repo)

	// Segundo guardado desde el panel: tampoco trae published_at.
	if err := svc.UpdateContent(context.Background(), &domain.CustomContent{
		ID:          "c1",
		Title:       "Claves para la Sucesión (corregido)",
		IsPublished: true,
	}); err != nil {
		t.Fatal(err)
	}
	if repo.guardado.PublishedAt == nil {
		t.Fatal("se perdio la fecha al guardar")
	}
	if !repo.guardado.PublishedAt.Equal(original) {
		t.Errorf("la fecha se reescribio: era %v y quedo %v. Es la fecha en que se publico, no la del ultimo guardado", original, repo.guardado.PublishedAt)
	}
}

func TestPublicarUnBorradorViejo_LePoneLaFechaDeAhora(t *testing.T) {
	repo := &repoDeContenido{existente: &domain.CustomContent{
		ID:          "c1",
		IsPublished: false,
	}}
	svc := servicioDeContenido(repo)

	if err := svc.UpdateContent(context.Background(), &domain.CustomContent{
		ID:          "c1",
		IsPublished: true,
	}); err != nil {
		t.Fatal(err)
	}
	if repo.guardado.PublishedAt == nil {
		t.Fatal("al publicar un borrador hay que sellar la fecha")
	}
}

func TestDespublicar_NoBorraLaFecha(t *testing.T) {
	// Retirar algo de la app no significa que nunca se publicara. Si se vuelve
	// a publicar, la fecha buena sigue siendo la primera.
	original := time.Date(2026, 2, 27, 10, 0, 0, 0, time.UTC)
	repo := &repoDeContenido{existente: &domain.CustomContent{
		ID:          "c1",
		IsPublished: true,
		PublishedAt: &original,
	}}
	svc := servicioDeContenido(repo)

	if err := svc.UpdateContent(context.Background(), &domain.CustomContent{
		ID:          "c1",
		IsPublished: false,
	}); err != nil {
		t.Fatal(err)
	}
	if repo.guardado.PublishedAt == nil || !repo.guardado.PublishedAt.Equal(original) {
		t.Errorf("se perdio la fecha al despublicar: %v", repo.guardado.PublishedAt)
	}
}

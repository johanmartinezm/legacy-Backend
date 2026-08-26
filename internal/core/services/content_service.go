package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"time"
)

type ContentService struct {
	catRepo     ports.ContentCategoryRepository
	contentRepo ports.CustomContentRepository
}

func NewContentService(catRepo ports.ContentCategoryRepository, contentRepo ports.CustomContentRepository) *ContentService {
	return &ContentService{
		catRepo:     catRepo,
		contentRepo: contentRepo,
	}
}

// Category methods
func (s *ContentService) ListCategories(ctx context.Context) ([]*domain.ContentCategory, error) {
	return s.catRepo.ListAll(ctx)
}

func (s *ContentService) CreateCategory(ctx context.Context, cat *domain.ContentCategory) error {
	return s.catRepo.Create(ctx, cat)
}

func (s *ContentService) UpdateCategory(ctx context.Context, cat *domain.ContentCategory) error {
	return s.catRepo.Update(ctx, cat)
}

func (s *ContentService) DeleteCategory(ctx context.Context, id string) error {
	return s.catRepo.Delete(ctx, id)
}

// Content methods
func (s *ContentService) ListContent(ctx context.Context, categorySlug string, onlyPublished bool) ([]*domain.CustomContent, error) {
	return s.contentRepo.List(ctx, categorySlug, onlyPublished)
}

func (s *ContentService) GetContentByID(ctx context.Context, id string) (*domain.CustomContent, error) {
	return s.contentRepo.GetByID(ctx, id)
}

func (s *ContentService) CreateContent(ctx context.Context, c *domain.CustomContent) error {
	sellarPublicacion(c)
	return s.contentRepo.Create(ctx, c)
}

func (s *ContentService) UpdateContent(ctx context.Context, c *domain.CustomContent) error {
	// El formulario del panel no envía `published_at`, así que aquí llega nulo
	// aunque el contenido ya estuviera publicado con su fecha. Se recupera la
	// guardada antes de sellar: sin esto, cada guardado la reescribiría con la
	// del momento y dejaría de ser la fecha en que se publicó.
	if c.PublishedAt == nil {
		if existente, err := s.contentRepo.GetByID(ctx, c.ID); err == nil && existente != nil {
			c.PublishedAt = existente.PublishedAt
		}
	}
	sellarPublicacion(c)
	return s.contentRepo.Update(ctx, c)
}

// sellarPublicacion pone la fecha de publicación cuando el contenido está
// publicado y todavía no la tiene.
//
// La ponía quien llamaba, es decir nadie: el formulario del panel no tiene ese
// campo, así que `published_at` llegaba siempre nulo y el UPDATE lo escribía
// nulo. Los dos contenidos publicados en producción lo tenían vacío, y como la
// app lo usa para la fecha que muestra bajo el título
// (custom_content_model.dart:63), el vídeo se veía como «0 vistas • » con la
// fecha en blanco.
//
// Va en el servicio y no en el cliente porque es un dato del servidor: la hora
// del navegador de quien administra no tiene por qué ser la buena, y así vale
// para cualquier cliente que llame a la API.
//
// **No se borra al despublicar** y no se pisa si ya existe: es la fecha en que
// se publicó por primera vez, no la del último guardado. Volver a publicar algo
// que ya tenía fecha conserva la original.
func sellarPublicacion(c *domain.CustomContent) {
	if c.IsPublished && c.PublishedAt == nil {
		ahora := time.Now()
		c.PublishedAt = &ahora
	}
}

func (s *ContentService) DeleteContent(ctx context.Context, id string) error {
	return s.contentRepo.Delete(ctx, id)
}

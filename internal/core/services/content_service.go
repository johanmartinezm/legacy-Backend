package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
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
	return s.contentRepo.Create(ctx, c)
}

func (s *ContentService) UpdateContent(ctx context.Context, c *domain.CustomContent) error {
	return s.contentRepo.Update(ctx, c)
}

func (s *ContentService) DeleteContent(ctx context.Context, id string) error {
	return s.contentRepo.Delete(ctx, id)
}

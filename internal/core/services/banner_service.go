package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
)

type BannerService struct {
	repo ports.BannerRepository
}

func NewBannerService(repo ports.BannerRepository) *BannerService {
	return &BannerService{repo: repo}
}

func (s *BannerService) CreateBanner(ctx context.Context, banner *domain.Banner) error {
	return s.repo.Create(ctx, banner)
}

func (s *BannerService) UpdateBanner(ctx context.Context, banner *domain.Banner) error {
	return s.repo.Update(ctx, banner)
}

func (s *BannerService) DeleteBanner(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *BannerService) GetActiveBanners(ctx context.Context, category string) ([]*domain.Banner, error) {
	return s.repo.ListActive(ctx, category)
}

func (s *BannerService) ListAllBanners(ctx context.Context) ([]*domain.Banner, error) {
	return s.repo.ListAll(ctx)
}

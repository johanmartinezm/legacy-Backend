package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
)

type LikeService struct {
	repo ports.LikeRepository
}

func NewLikeService(repo ports.LikeRepository) *LikeService {
	return &LikeService{repo: repo}
}

func (s *LikeService) ToggleLike(ctx context.Context, userID, postID string) (*domain.LikeStatus, error) {
	_, err := s.repo.ToggleLike(ctx, userID, postID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetLikeStatus(ctx, userID, postID)
}

func (s *LikeService) GetLikeStatus(ctx context.Context, userID, postID string) (*domain.LikeStatus, error) {
	return s.repo.GetLikeStatus(ctx, userID, postID)
}

func (s *LikeService) RecordView(ctx context.Context, userID, postID, title string) error {
	return s.repo.RecordView(ctx, userID, postID, title)
}

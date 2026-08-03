package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"applegacy/backend/internal/security"
	"context"
)

type SynergyService struct {
	repo   ports.SynergyRepository
	crypto *security.CryptoService
}

func NewSynergyService(repo ports.SynergyRepository, crypto *security.CryptoService) *SynergyService {
	return &SynergyService{
		repo:   repo,
		crypto: crypto,
	}
}

func (s *SynergyService) ProposeSynergy(ctx context.Context, synergy *domain.Synergy) error {
	if synergy.Status == "" {
		synergy.Status = domain.SynergyStatusActive
	}
	return s.repo.CreateSynergy(ctx, synergy)
}

func (s *SynergyService) GetSynergy(ctx context.Context, id string, userID string) (*domain.Synergy, error) {
	synergy, err := s.repo.GetSynergyByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if synergy.Author != nil {
		s.decryptUser(synergy.Author)
	}

	// Decrypt commenters
	comments, _ := s.repo.GetCommentsBySynergyID(ctx, id)
	for i := range comments {
		if comments[i].User != nil {
			s.decryptUser(comments[i].User)
		}
	}
	synergy.Comments = comments

	// Increment views (fire and forget or ignore error)
	_ = s.repo.IncrementViews(ctx, id)

	return synergy, nil
}

func (s *SynergyService) ListSynergies(ctx context.Context, category string, status string, search string, page, pageSize int) ([]domain.Synergy, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	synergies, err := s.repo.ListSynergies(ctx, category, status, search, offset, pageSize)
	if err != nil {
		return nil, err
	}

	for i := range synergies {
		if synergies[i].Author != nil {
			s.decryptUser(synergies[i].Author)
		}
	}

	return synergies, nil
}

func (s *SynergyService) decryptUser(user *domain.User) {
	if user == nil {
		return
	}
	// Decrypt only name fields for synergy display
	user.FirstName, _ = s.crypto.Decrypt(user.FirstName)
	user.LastName, _ = s.crypto.Decrypt(user.LastName)
}

func (s *SynergyService) CommentSynergy(ctx context.Context, comment *domain.SynergyComment) error {
	return s.repo.CreateComment(ctx, comment)
}

func (s *SynergyService) ToggleLike(ctx context.Context, synergyID, userID string) (bool, error) {
	isLiked, err := s.repo.IsLikedByUser(ctx, synergyID, userID)
	if err != nil {
		return false, err
	}

	if isLiked {
		err = s.repo.RemoveLike(ctx, synergyID, userID)
		return false, err
	} else {
		err = s.repo.AddLike(ctx, synergyID, userID)
		return true, err
	}
}

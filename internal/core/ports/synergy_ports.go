package ports

import (
	"applegacy/backend/internal/core/domain"
	"context"
)

type SynergyRepository interface {
	CreateSynergy(ctx context.Context, synergy *domain.Synergy) error
	GetSynergyByID(ctx context.Context, id string) (*domain.Synergy, error)
	ListSynergies(ctx context.Context, category string, status string, search string, offset, limit int) ([]domain.Synergy, error)
	UpdateSynergy(ctx context.Context, synergy *domain.Synergy) error
	DeleteSynergy(ctx context.Context, id string) error

	CreateComment(ctx context.Context, comment *domain.SynergyComment) error
	GetCommentsBySynergyID(ctx context.Context, synergyID string) ([]domain.SynergyComment, error)

	AddLike(ctx context.Context, synergyID, userID string) error
	RemoveLike(ctx context.Context, synergyID, userID string) error
	IsLikedByUser(ctx context.Context, synergyID, userID string) (bool, error)

	IncrementViews(ctx context.Context, synergyID string) error
}

type SynergyService interface {
	ProposeSynergy(ctx context.Context, synergy *domain.Synergy) error
	GetSynergy(ctx context.Context, id string, userID string) (*domain.Synergy, error)
	ListSynergies(ctx context.Context, category string, status string, search string, page, pageSize int) ([]domain.Synergy, error)
	CommentSynergy(ctx context.Context, comment *domain.SynergyComment) error
	ToggleLike(ctx context.Context, synergyID, userID string) (bool, error)
}

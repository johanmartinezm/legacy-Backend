package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"
)

type GroupService struct {
	repo ports.GroupRepository
}

func NewGroupService(repo ports.GroupRepository) *GroupService {
	return &GroupService{repo: repo}
}

func (s *GroupService) CreateGroup(ctx context.Context, name, description string) (*domain.CustomGroup, error) {
	if name == "" {
		return nil, errors.New("el nombre del grupo no puede estar vacío")
	}

	group := &domain.CustomGroup{
		Name:        name,
		Description: description,
	}

	err := s.repo.Create(ctx, group)
	if err != nil {
		return nil, err
	}

	return group, nil
}

func (s *GroupService) ListGroups(ctx context.Context) ([]*domain.CustomGroup, error) {
	return s.repo.List(ctx)
}

func (s *GroupService) DeleteGroup(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("el ID del grupo es requerido")
	}
	return s.repo.Delete(ctx, id)
}

func (s *GroupService) GetGroupByID(ctx context.Context, id string) (*domain.CustomGroup, error) {
	if id == "" {
		return nil, errors.New("el ID del grupo es requerido")
	}
	return s.repo.GetByID(ctx, id)
}

func (s *GroupService) GetMembers(ctx context.Context, groupID string) ([]string, error) {
	if groupID == "" {
		return nil, errors.New("el ID del grupo es requerido")
	}
	return s.repo.GetMembers(ctx, groupID)
}

func (s *GroupService) ReplaceMembers(ctx context.Context, groupID string, userIDs []string) error {
	if groupID == "" {
		return errors.New("el ID del grupo es requerido")
	}
	return s.repo.ReplaceMembers(ctx, groupID, userIDs)
}

var _ ports.GroupService = (*GroupService)(nil)

package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"
)

type forumService struct {
	repo     ports.ForumRepository
	userRepo ports.UserRepository
}

func NewForumService(repo ports.ForumRepository, userRepo ports.UserRepository) ports.ForumService {
	return &forumService{repo: repo, userRepo: userRepo}
}

// Admin Methods
func (s *forumService) CreateForum(ctx context.Context, forum *domain.Forum) error {
	forum.Status = domain.ForumStatusActive
	forum.CreatedByAdmin = true
	return s.repo.CreateForum(ctx, forum)
}

func (s *forumService) UpdateForum(ctx context.Context, forum *domain.Forum) error {
	existing, err := s.repo.GetForumByID(ctx, forum.ID)
	if err != nil {
		return err
	}
	existing.Title = forum.Title
	existing.Description = forum.Description
	if forum.CoverURL != "" {
		existing.CoverURL = forum.CoverURL
	}
	if forum.Status != "" {
		existing.Status = forum.Status
	}
	return s.repo.UpdateForum(ctx, existing)
}

func (s *forumService) DeleteForum(ctx context.Context, forumID string) error {
	return s.repo.SoftDeleteForum(ctx, forumID)
}

func (s *forumService) LockForum(ctx context.Context, forumID string) error {
	return s.repo.LockForum(ctx, forumID)
}

func (s *forumService) UnlockForum(ctx context.Context, forumID string) error {
	return s.repo.UnlockForum(ctx, forumID)
}

func (s *forumService) ListAllForums(ctx context.Context) ([]*domain.Forum, error) {
	return s.repo.ListForums(ctx, true) // include hidden
}

func (s *forumService) GetForumTree(ctx context.Context, forumID string) ([]*domain.ForumPost, error) {
	return s.repo.ListAllPostsForAdmin(ctx, forumID)
}

func (s *forumService) DeletePost(ctx context.Context, postID string) error {
	return s.repo.SoftDeletePost(ctx, postID)
}

func (s *forumService) ListFlaggedPosts(ctx context.Context) ([]*domain.ForumPost, error) {
	return s.repo.ListFlaggedPosts(ctx)
}

// App Methods
func (s *forumService) ListPublicForums(ctx context.Context) ([]*domain.Forum, error) {
	return s.repo.ListForums(ctx, false) // exclude hidden
}

func (s *forumService) CreateUserForum(ctx context.Context, userID string, forum *domain.Forum) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.Alias == "" {
		return errors.New("alias_required")
	}

	forum.Status = domain.ForumStatusActive
	forum.CreatedByUserID = &userID
	forum.CreatedByAdmin = false
	return s.repo.CreateForum(ctx, forum)
}

func (s *forumService) GetForumThread(ctx context.Context, forumID string, limit, offset int) ([]*domain.ForumPost, error) {
	return s.repo.ListPosts(ctx, forumID, limit, offset)
}

func (s *forumService) PublishPost(ctx context.Context, userID, forumID, content, imageURL string, parentID *string) (*domain.ForumPost, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Alias == "" {
		return nil, errors.New("alias_required")
	}

	// Verify forum exists and is active
	forum, err := s.repo.GetForumByID(ctx, forumID)
	if err != nil || forum.Status != domain.ForumStatusActive {
		return nil, errors.New("forum not available or locked")
	}

	if content == "" && imageURL != "" {
		content = " "
	}

	post := &domain.ForumPost{
		ForumID:  forumID,
		ParentID: parentID,
		Content:  content,
		ImageURL: imageURL,
		Status:   domain.PostStatusActive,
	}

	err = s.repo.CreatePost(ctx, post, userID)
	if err != nil {
		return nil, err
	}
	
	// Populate author alias for immediate return
	post.AuthorAlias = user.Alias
	
	return post, nil
}

func (s *forumService) ReportPost(ctx context.Context, reporterID, postID, reason string) error {
	report := &domain.ForumPostReport{
		PostID:     postID,
		ReporterID: reporterID,
		Reason:     reason,
	}
	return s.repo.ReportPost(ctx, report)
}

package ports

import (
	"applegacy/backend/internal/core/domain"
	"context"
)

// ForumRepository defines the data access methods for the forum module.
type ForumRepository interface {
	// Forums
	CreateForum(ctx context.Context, forum *domain.Forum) error
	UpdateForum(ctx context.Context, forum *domain.Forum) error
	SoftDeleteForum(ctx context.Context, forumID string) error
	LockForum(ctx context.Context, forumID string) error
	UnlockForum(ctx context.Context, forumID string) error
	ListForums(ctx context.Context, includeHidden bool) ([]*domain.Forum, error)
	GetForumByID(ctx context.Context, forumID string) (*domain.Forum, error)

	// Posts
	CreatePost(ctx context.Context, post *domain.ForumPost, userID string) error
	ListPosts(ctx context.Context, forumID string, limit, offset int) ([]*domain.ForumPost, error)
	ListAllPostsForAdmin(ctx context.Context, forumID string) ([]*domain.ForumPost, error)
	SoftDeletePost(ctx context.Context, postID string) error
	
	// Moderation
	ReportPost(ctx context.Context, report *domain.ForumPostReport) error
	ListFlaggedPosts(ctx context.Context) ([]*domain.ForumPost, error)
}

// ForumService defines the business logic for the forum module.
type ForumService interface {
	// Admin (Angular)
	CreateForum(ctx context.Context, forum *domain.Forum) error
	UpdateForum(ctx context.Context, forum *domain.Forum) error
	DeleteForum(ctx context.Context, forumID string) error
	LockForum(ctx context.Context, forumID string) error
	UnlockForum(ctx context.Context, forumID string) error
	ListAllForums(ctx context.Context) ([]*domain.Forum, error)
	GetForumTree(ctx context.Context, forumID string) ([]*domain.ForumPost, error)
	DeletePost(ctx context.Context, postID string) error
	ListFlaggedPosts(ctx context.Context) ([]*domain.ForumPost, error)

	// App (Flutter)
	ListPublicForums(ctx context.Context) ([]*domain.Forum, error)
	CreateUserForum(ctx context.Context, userID string, forum *domain.Forum) error
	GetForumThread(ctx context.Context, forumID string, limit, offset int) ([]*domain.ForumPost, error)
	PublishPost(ctx context.Context, userID, forumID, content, imageURL string, parentID *string) (*domain.ForumPost, error)
	ReportPost(ctx context.Context, reporterID, postID, reason string) error
}

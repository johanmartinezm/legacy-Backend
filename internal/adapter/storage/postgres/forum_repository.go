package postgres

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type forumRepository struct {
	db *pgxpool.Pool
}

func NewForumRepository(db *pgxpool.Pool) ports.ForumRepository {
	return &forumRepository{db: db}
}

func (r *forumRepository) CreateForum(ctx context.Context, forum *domain.Forum) error {
	query := `
		INSERT INTO core.forums (title, description, cover_url, status, created_by_user_id, created_by_admin)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		forum.Title, forum.Description, forum.CoverURL, forum.Status, forum.CreatedByUserID, forum.CreatedByAdmin,
	).Scan(&forum.ID, &forum.CreatedAt, &forum.UpdatedAt)
}

func (r *forumRepository) UpdateForum(ctx context.Context, forum *domain.Forum) error {
	query := `
		UPDATE core.forums
		SET title = $1, description = $2, cover_url = $3, status = $4, updated_at = NOW()
		WHERE id = $5
		RETURNING updated_at`
	return r.db.QueryRow(ctx, query,
		forum.Title, forum.Description, forum.CoverURL, forum.Status, forum.ID,
	).Scan(&forum.UpdatedAt)
}

func (r *forumRepository) SoftDeleteForum(ctx context.Context, forumID string) error {
	query := `UPDATE core.forums SET status = 'deleted', updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(ctx, query, forumID)
	return err
}

func (r *forumRepository) LockForum(ctx context.Context, forumID string) error {
	query := `UPDATE core.forums SET status = 'locked', updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(ctx, query, forumID)
	return err
}

func (r *forumRepository) UnlockForum(ctx context.Context, forumID string) error {
	query := `UPDATE core.forums SET status = 'active', updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(ctx, query, forumID)
	return err
}

func (r *forumRepository) ListForums(ctx context.Context, includeHidden bool) ([]*domain.Forum, error) {
	query := `
		SELECT f.id, f.title, f.description, f.cover_url, f.status, f.created_by_admin, f.created_at, f.updated_at,
		       COALESCE(u.alias, '') as author_alias,
		       (SELECT COUNT(*) FROM core.forum_posts fp WHERE fp.forum_id = f.id AND fp.status = 'active') as post_count
		FROM core.forums f
		LEFT JOIN core.users u ON f.created_by_user_id = u.id
		WHERE f.status != 'deleted'
	`
	if !includeHidden {
		query += ` AND f.status IN ('active', 'locked')`
	}
	query += ` ORDER BY f.created_at DESC`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var forums []*domain.Forum
	for rows.Next() {
		var f domain.Forum
		if err := rows.Scan(&f.ID, &f.Title, &f.Description, &f.CoverURL, &f.Status, &f.CreatedByAdmin, &f.CreatedAt, &f.UpdatedAt, &f.AuthorAlias, &f.PostCount); err != nil {
			return nil, err
		}
		forums = append(forums, &f)
	}
	return forums, nil
}

func (r *forumRepository) GetForumByID(ctx context.Context, forumID string) (*domain.Forum, error) {
	query := `
		SELECT f.id, f.title, f.description, f.cover_url, f.status, f.created_by_admin, f.created_at, f.updated_at,
		       COALESCE(u.alias, '') as author_alias,
		       (SELECT COUNT(*) FROM core.forum_posts fp WHERE fp.forum_id = f.id AND fp.status = 'active') as post_count
		FROM core.forums f
		LEFT JOIN core.users u ON f.created_by_user_id = u.id
		WHERE f.id = $1 AND f.status != 'deleted'`

	var f domain.Forum
	err := r.db.QueryRow(ctx, query, forumID).Scan(
		&f.ID, &f.Title, &f.Description, &f.CoverURL, &f.Status, &f.CreatedByAdmin, &f.CreatedAt, &f.UpdatedAt, &f.AuthorAlias, &f.PostCount,
	)
	if err == pgx.ErrNoRows {
		return nil, errors.New("forum not found")
	}
	return &f, err
}

func (r *forumRepository) CreatePost(ctx context.Context, post *domain.ForumPost, userID string) error {
	query := `
		INSERT INTO core.forum_posts (forum_id, user_id, parent_id, content, image_url, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query,
		post.ForumID, userID, post.ParentID, post.Content, post.ImageURL, post.Status,
	).Scan(&post.ID, &post.CreatedAt)
}

func (r *forumRepository) ListPosts(ctx context.Context, forumID string, limit, offset int) ([]*domain.ForumPost, error) {
	query := `
		SELECT p.id, p.forum_id, p.parent_id, p.content, p.image_url, p.status, p.created_at,
		       COALESCE(u.alias, '') as author_alias,
		       (SELECT COUNT(*) FROM core.forum_posts r WHERE r.parent_id = p.id AND r.status = 'active') as reply_count
		FROM core.forum_posts p
		JOIN core.users u ON p.user_id = u.id
		WHERE p.forum_id = $1 AND p.status = 'active' AND p.parent_id IS NULL
		ORDER BY p.created_at ASC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, forumID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*domain.ForumPost
	for rows.Next() {
		var p domain.ForumPost
		var parentID *string
		if err := rows.Scan(&p.ID, &p.ForumID, &parentID, &p.Content, &p.ImageURL, &p.Status, &p.CreatedAt, &p.AuthorAlias, &p.ReplyCount); err != nil {
			return nil, err
		}
		p.ParentID = parentID
		posts = append(posts, &p)
	}
	return posts, nil
}

func (r *forumRepository) ListAllPostsForAdmin(ctx context.Context, forumID string) ([]*domain.ForumPost, error) {
	query := `
		SELECT p.id, p.forum_id, p.parent_id, p.content, p.image_url, p.status, p.created_at,
		       COALESCE(u.alias, '') as author_alias,
		       0 as reply_count
		FROM core.forum_posts p
		JOIN core.users u ON p.user_id = u.id
		WHERE p.forum_id = $1 AND p.status != 'deleted'
		ORDER BY p.created_at ASC`

	rows, err := r.db.Query(ctx, query, forumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*domain.ForumPost
	for rows.Next() {
		var p domain.ForumPost
		var parentID *string
		if err := rows.Scan(&p.ID, &p.ForumID, &parentID, &p.Content, &p.ImageURL, &p.Status, &p.CreatedAt, &p.AuthorAlias, &p.ReplyCount); err != nil {
			return nil, err
		}
		p.ParentID = parentID
		posts = append(posts, &p)
	}
	return posts, nil
}

func (r *forumRepository) SoftDeletePost(ctx context.Context, postID string) error {
	query := `UPDATE core.forum_posts SET status = 'deleted', updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(ctx, query, postID)
	return err
}

func (r *forumRepository) ReportPost(ctx context.Context, report *domain.ForumPostReport) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	queryReport := `
		INSERT INTO core.forum_post_reports (post_id, reporter_id, reason)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`
	err = tx.QueryRow(ctx, queryReport, report.PostID, report.ReporterID, report.Reason).Scan(&report.ID, &report.CreatedAt)
	if err != nil {
		return err
	}

	queryUpdate := `UPDATE core.forum_posts SET report_count = report_count + 1 WHERE id = $1`
	if _, err := tx.Exec(ctx, queryUpdate, report.PostID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *forumRepository) ListFlaggedPosts(ctx context.Context) ([]*domain.ForumPost, error) {
	query := `
		SELECT p.id, p.forum_id, p.parent_id, p.content, p.image_url, p.status, p.created_at,
		       COALESCE(u.alias, '') as author_alias,
			   p.report_count
		FROM core.forum_posts p
		JOIN core.users u ON p.user_id = u.id
		WHERE p.report_count > 0 AND p.status != 'deleted'
		ORDER BY p.report_count DESC, p.created_at DESC`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*domain.ForumPost
	for rows.Next() {
		var p domain.ForumPost
		var parentID *string
		// note we don't have reply count here so we just scan report count
		var reportCount int
		if err := rows.Scan(&p.ID, &p.ForumID, &parentID, &p.Content, &p.ImageURL, &p.Status, &p.CreatedAt, &p.AuthorAlias, &reportCount); err != nil {
			return nil, err
		}
		p.ParentID = parentID
		posts = append(posts, &p)
	}
	return posts, nil
}

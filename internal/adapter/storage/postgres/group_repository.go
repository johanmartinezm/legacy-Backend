package postgres

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type GroupRepository struct {
	db *pgxpool.Pool
}

func NewGroupRepository(pool *pgxpool.Pool) *GroupRepository {
	return &GroupRepository{db: pool}
}

func (r *GroupRepository) Create(ctx context.Context, group *domain.CustomGroup) error {
	sql := `INSERT INTO core.custom_groups (id, name, description, created_at, updated_at)
            VALUES (gen_random_uuid(), $1, $2, now(), now())
            RETURNING id, created_at, updated_at`
	return r.db.QueryRow(ctx, sql, group.Name, group.Description).Scan(&group.ID, &group.CreatedAt, &group.UpdatedAt)
}

func (r *GroupRepository) List(ctx context.Context) ([]*domain.CustomGroup, error) {
	sql := `SELECT id, name, COALESCE(description, ''), created_at, updated_at FROM core.custom_groups ORDER BY name ASC`
	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make([]*domain.CustomGroup, 0) // Inicializar como slice vacío, no nil — nil serializa como JSON null
	for rows.Next() {
		var g domain.CustomGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, &g)
	}
	return groups, nil
}

func (r *GroupRepository) Delete(ctx context.Context, id string) error {
	sql := `DELETE FROM core.custom_groups WHERE id = $1`
	_, err := r.db.Exec(ctx, sql, id)
	return err
}

func (r *GroupRepository) GetByID(ctx context.Context, id string) (*domain.CustomGroup, error) {
	sql := `SELECT id, name, COALESCE(description, ''), created_at, updated_at FROM core.custom_groups WHERE id = $1`
	var g domain.CustomGroup
	err := r.db.QueryRow(ctx, sql, id).Scan(&g.ID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &g, nil
}

func (r *GroupRepository) AddMembers(ctx context.Context, groupID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	sql := `INSERT INTO core.custom_group_members (group_id, user_id, created_at)
            VALUES ($1, $2, now())
            ON CONFLICT (group_id, user_id) DO NOTHING`

	for _, userID := range userIDs {
		if _, err := tx.Exec(ctx, sql, groupID, userID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *GroupRepository) RemoveMember(ctx context.Context, groupID string, userID string) error {
	sql := `DELETE FROM core.custom_group_members WHERE group_id = $1 AND user_id = $2`
	_, err := r.db.Exec(ctx, sql, groupID, userID)
	return err
}

func (r *GroupRepository) GetMembers(ctx context.Context, groupID string) ([]string, error) {
	sql := `SELECT user_id FROM core.custom_group_members WHERE group_id = $1`
	rows, err := r.db.Query(ctx, sql, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	userIDs := make([]string, 0) // Inicializar como slice vacío — nil serializa como JSON null
	for rows.Next() {
		var uID string
		if err := rows.Scan(&uID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, uID)
	}
	return userIDs, nil
}

func (r *GroupRepository) ReplaceMembers(ctx context.Context, groupID string, userIDs []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Delete all existing members first
	deleteSql := `DELETE FROM core.custom_group_members WHERE group_id = $1`
	if _, err := tx.Exec(ctx, deleteSql, groupID); err != nil {
		return err
	}

	// Insert new members
	if len(userIDs) > 0 {
		insertSql := `INSERT INTO core.custom_group_members (group_id, user_id, created_at) VALUES ($1, $2, now())`
		for _, userID := range userIDs {
			if _, err := tx.Exec(ctx, insertSql, groupID, userID); err != nil {
				return err
			}
		}
	}

	// Update the updated_at timestamp of the group
	updateGroupSql := `UPDATE core.custom_groups SET updated_at = now() WHERE id = $1`
	if _, err := tx.Exec(ctx, updateGroupSql, groupID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

var _ ports.GroupRepository = (*GroupRepository)(nil)

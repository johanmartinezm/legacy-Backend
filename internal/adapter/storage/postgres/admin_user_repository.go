package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

type AdminUserRepository struct {
	db *pgxpool.Pool
}

func NewAdminUserRepository(pool *pgxpool.Pool) *AdminUserRepository {
	return &AdminUserRepository{db: pool}
}

// Create inserts a new admin user.
func (r *AdminUserRepository) Create(ctx context.Context, admin *domain.AdminUser) error {
	sql := `INSERT INTO core.admin_users (id, email, password_hash, first_name, last_name, role, created_at, updated_at)
            VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, now(), now())`
	_, err := r.db.Exec(ctx, sql, admin.Email, admin.PasswordHash, admin.FirstName, admin.LastName, admin.Role)
	return err
}

// FindByEmail returns an admin user by email.
func (r *AdminUserRepository) FindByEmail(ctx context.Context, email string) (*domain.AdminUser, error) {
	sql := `SELECT id, email, password_hash, first_name, last_name, role, created_at, updated_at FROM core.admin_users WHERE email = $1`
	var admin domain.AdminUser
	err := r.db.QueryRow(ctx, sql, email).Scan(
		&admin.ID,
		&admin.Email,
		&admin.PasswordHash,
		&admin.FirstName,
		&admin.LastName,
		&admin.Role,
		&admin.CreatedAt,
		&admin.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

// FindByID returns an admin user by id.
func (r *AdminUserRepository) FindByID(ctx context.Context, id string) (*domain.AdminUser, error) {
	sql := `SELECT id, email, password_hash, first_name, last_name, role, created_at, updated_at FROM core.admin_users WHERE id = $1`
	var admin domain.AdminUser
	err := r.db.QueryRow(ctx, sql, id).Scan(
		&admin.ID,
		&admin.Email,
		&admin.PasswordHash,
		&admin.FirstName,
		&admin.LastName,
		&admin.Role,
		&admin.CreatedAt,
		&admin.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

// List returns all admin users.
func (r *AdminUserRepository) List(ctx context.Context) ([]*domain.AdminUser, error) {
	sql := `SELECT id, email, password_hash, first_name, last_name, role, created_at, updated_at FROM core.admin_users`
	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var admins []*domain.AdminUser
	for rows.Next() {
		var a domain.AdminUser
		if err := rows.Scan(
			&a.ID,
			&a.Email,
			&a.PasswordHash,
			&a.FirstName,
			&a.LastName,
			&a.Role,
			&a.CreatedAt,
			&a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		admins = append(admins, &a)
	}
	return admins, nil
}

// Delete removes an admin user.
func (r *AdminUserRepository) Delete(ctx context.Context, id string) error {
	sql := `DELETE FROM core.admin_users WHERE id = $1`
	_, err := r.db.Exec(ctx, sql, id)
	return err
}

// UpdatePassword updates the password hash for an admin.
func (r *AdminUserRepository) UpdatePassword(ctx context.Context, id, newHash string) error {
	sql := `UPDATE core.admin_users SET password_hash = $1, updated_at = now() WHERE id = $2`
	_, err := r.db.Exec(ctx, sql, newHash, id)
	return err
}

// Update updates mutable fields of an admin (first/last name, role).
func (r *AdminUserRepository) Update(ctx context.Context, admin *domain.AdminUser) error {
	sql := `UPDATE core.admin_users SET first_name = $1, last_name = $2, role = $3, updated_at = now() WHERE id = $4`
	_, err := r.db.Exec(ctx, sql, admin.FirstName, admin.LastName, admin.Role, admin.ID)
	return err
}

// Ensure the repository satisfies a clean‑code interface (optional).
var _ interface { /* placeholder for future interface */
} = (*AdminUserRepository)(nil)

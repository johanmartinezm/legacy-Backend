package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	sql := `
		INSERT INTO core.users (
			email_blind_index, email_encrypted, password_hash, 
			first_name, last_name, birth_date, phone, location, bio, industry, profile_image_url,
			company_name, job_title, role,
			country, identification_type, identification_number, customer_status,
			generation, is_public_profile, allow_messages_from_strangers, show_activity,
			terms_accepted, data_sharing_accepted, email_verified,
			created_at, updated_at, alias
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28)
		RETURNING id
	`

	err = tx.QueryRow(ctx, sql,
		user.EmailBlindIndex,
		user.EmailEncrypted,
		user.PasswordHash,
		user.FirstName,
		user.LastName,
		user.BirthDate,
		user.Phone,
		user.Location,
		user.Bio,
		user.Industry,
		user.ProfileImageUrl,
		user.CompanyName,
		user.JobTitle,
		user.Role,
		user.Country,
		user.IdentificationType,
		user.IdentificationNumber,
		user.CustomerStatus,
		user.Generation,
		user.IsPublicProfile,
		user.AllowMessagesFromStrangers,
		user.ShowActivity,
		user.TermsAccepted,
		user.DataSharingAccepted,
		user.EmailVerified,
		user.CreatedAt,
		user.UpdatedAt,
		user.Alias,
	).Scan(&user.ID)

	if err != nil {
		if err.Error() != "" && (strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "users_alias_key")) {
			return errors.New("alias_in_use")
		}
		return err
	}

	for _, interest := range user.Interests {
		interestSql := `INSERT INTO core.user_interests (user_id, interest) VALUES ($1, $2) ON CONFLICT DO NOTHING`
		_, err = tx.Exec(ctx, interestSql, user.ID, interest)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *UserRepository) FindByEmailBlindIndex(ctx context.Context, blindIndex string) (*domain.User, error) {
	sql := `
		SELECT id, email_blind_index, email_encrypted, password_hash, role, COALESCE(email_verified, false)
		FROM core.users
		WHERE email_blind_index = $1
	`
	var user domain.User
	err := r.db.QueryRow(ctx, sql, blindIndex).Scan(
		&user.ID,
		&user.EmailBlindIndex,
		&user.EmailEncrypted,
		&user.PasswordHash,
		&user.Role,
		&user.EmailVerified,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindAll(ctx context.Context) ([]*domain.User, error) {
	sql := `
		SELECT 
			id, 
			email_blind_index, 
			COALESCE(email_encrypted, ''), 
			first_name, 
			last_name, 
			birth_date,
			COALESCE(phone, ''), 
			COALESCE(location, ''), 
			COALESCE(bio, ''), 
			COALESCE(industry, ''),
			COALESCE(profile_image_url, ''),
			COALESCE(company_name, ''), 
			COALESCE(job_title, ''), 
			role::TEXT,
			COALESCE(country, ''), 
			COALESCE(identification_type, ''), 
			COALESCE(identification_number, ''), 
			COALESCE(customer_status, ''),
			COALESCE(generation, ''),
			COALESCE(is_public_profile, false),
			COALESCE(allow_messages_from_strangers, false),
			COALESCE(show_activity, false),
			COALESCE(email_verified, false),
			COALESCE(created_at, CURRENT_TIMESTAMP),
			COALESCE(updated_at, CURRENT_TIMESTAMP),
			COALESCE(alias, '')
		FROM core.users
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var user domain.User
		err := rows.Scan(
			&user.ID,
			&user.EmailBlindIndex,
			&user.EmailEncrypted,
			&user.FirstName,
			&user.LastName,
			&user.BirthDate,
			&user.Phone,
			&user.Location,
			&user.Bio,
			&user.Industry,
			&user.ProfileImageUrl,
			&user.CompanyName,
			&user.JobTitle,
			&user.Role,
			&user.Country,
			&user.IdentificationType,
			&user.IdentificationNumber,
			&user.CustomerStatus,
			&user.Generation,
			&user.IsPublicProfile,
			&user.AllowMessagesFromStrangers,
			&user.ShowActivity,
			&user.EmailVerified,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.Alias,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, &user)
	}

	return users, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	sql := `
		SELECT 
			id, 
			email_blind_index, 
			COALESCE(email_encrypted, ''),
			first_name, 
			last_name, 
			birth_date,
			COALESCE(phone, ''), 
			COALESCE(location, ''), 
			COALESCE(bio, ''), 
			COALESCE(industry, ''),
			COALESCE(profile_image_url, ''),
			COALESCE(company_name, ''), 
			COALESCE(job_title, ''), 
			role::TEXT,
			COALESCE(country, ''), 
			COALESCE(identification_type, ''), 
			COALESCE(identification_number, ''), 
			COALESCE(customer_status, ''),
			COALESCE(generation, ''),
			COALESCE(is_public_profile, false),
			COALESCE(allow_messages_from_strangers, false),
			COALESCE(show_activity, false),
			COALESCE(email_verified, false),
			COALESCE(created_at, CURRENT_TIMESTAMP), 
			COALESCE(updated_at, CURRENT_TIMESTAMP),
			password_hash,
			COALESCE(alias, '')
		FROM core.users
		WHERE id = $1
	`

	var user domain.User
	err := r.db.QueryRow(ctx, sql, id).Scan(
		&user.ID,
		&user.EmailBlindIndex,
		&user.EmailEncrypted,
		&user.FirstName,
		&user.LastName,
		&user.BirthDate,
		&user.Phone,
		&user.Location,
		&user.Bio,
		&user.Industry,
		&user.ProfileImageUrl,
		&user.CompanyName,
		&user.JobTitle,
		&user.Role,
		&user.Country,
		&user.IdentificationType,
		&user.IdentificationNumber,
		&user.CustomerStatus,
		&user.Generation,
		&user.IsPublicProfile,
		&user.AllowMessagesFromStrangers,
		&user.ShowActivity,
		&user.EmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.PasswordHash,
		&user.Alias,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	sql := `
		UPDATE core.users SET
			email_blind_index = $1, email_encrypted = $2,
			first_name = $3, last_name = $4, birth_date = $5, phone = $6, 
			location = $7, bio = $8, industry = $9, profile_image_url = $10,
			company_name = $11, job_title = $12, role = $13,
			country = $14, identification_type = $15, 
			identification_number = $16, customer_status = $17,
			generation = $18, is_public_profile = $19,
			allow_messages_from_strangers = $20, show_activity = $21,
			updated_at = $22, alias = NULLIF($23, '')
		WHERE id = $24
	`

	_, err := r.db.Exec(ctx, sql,
		user.EmailBlindIndex,
		user.EmailEncrypted,
		user.FirstName,
		user.LastName,
		user.BirthDate,
		user.Phone,
		user.Location,
		user.Bio,
		user.Industry,
		user.ProfileImageUrl,
		user.CompanyName,
		user.JobTitle,
		user.Role,
		user.Country,
		user.IdentificationType,
		user.IdentificationNumber,
		user.CustomerStatus,
		user.Generation,
		user.IsPublicProfile,
		user.AllowMessagesFromStrangers,
		user.ShowActivity,
		user.UpdatedAt,
		user.Alias,
		user.ID,
	)

	if err != nil {
		if err.Error() != "" && (strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "users_alias_key")) {
			return errors.New("alias_in_use")
		}
	}

	return err
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	sql := "DELETE FROM core.users WHERE id = $1"
	_, err := r.db.Exec(ctx, sql, id)
	return err
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID, newHash string) error {
	sql := "UPDATE core.users SET password_hash = $1, updated_at = $2 WHERE id = $3"
	_, err := r.db.Exec(ctx, sql, newHash, time.Now(), userID)
	return err
}

func (r *UserRepository) UpdatePasswordByEmail(ctx context.Context, emailBlindIndex, newHash string) error {
	sql := "UPDATE core.users SET password_hash = $1, updated_at = $2 WHERE email_blind_index = $3"
	_, err := r.db.Exec(ctx, sql, newHash, time.Now(), emailBlindIndex)
	return err
}

func (r *UserRepository) MarkEmailAsVerified(ctx context.Context, emailBlindIndex string) error {
	sql := "UPDATE core.users SET email_verified = true, updated_at = $1 WHERE email_blind_index = $2"
	_, err := r.db.Exec(ctx, sql, time.Now(), emailBlindIndex)
	return err
}

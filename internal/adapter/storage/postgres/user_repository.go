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
			created_at, updated_at, alias,
			terms_version, terms_accepted_at, data_sharing_version, data_sharing_accepted_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, NULLIF($28, ''), $29, $30, $31, $32)
		RETURNING id
	`
	// NULLIF sobre el alias: users_alias_key es UNIQUE y la cadena vacía SÍ
	// colisiona consigo misma, mientras que NULL no. Sin esto, la segunda cuenta
	// que se registrara sin alias violaba la restricción, y como el registro no
	// pedía alias eso era literalmente la segunda cuenta de la base. El índice
	// parcial idx_users_alias ya asume NULL para "sin alias".

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
		user.TermsVersion,
		user.TermsAcceptedAt,
		user.DataSharingVersion,
		user.DataSharingAcceptedAt,
	).Scan(&user.ID)

	if err != nil {
		// Cada restricción única significa una cosa distinta. Antes cualquier
		// 23505 se traducía a "alias_in_use", así que un correo repetido —o
		// cualquier otro choque— mandaba a la persona a cambiar un alias que ni
		// siquiera había escrito.
		msg := err.Error()
		switch {
		case strings.Contains(msg, "users_alias_key"):
			return errors.New("alias_in_use")
		case strings.Contains(msg, "users_email_key"):
			return errors.New("user already exists")
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

// columnaSocial traduce el nombre del proveedor a su columna. Se hace con un
// switch y no interpolando el valor en el SQL: `provider` viene del cuerpo de
// una petición pública.
func columnaSocial(provider string) (string, error) {
	switch provider {
	case "google":
		return "google_id", nil
	case "apple":
		return "apple_id", nil
	default:
		return "", errors.New("proveedor social no soportado: " + provider)
	}
}

// FindBySocialID busca por la identidad estable del proveedor. Es la única
// forma fiable con Apple, que solo envía el correo en el primer inicio de sesión.
func (r *UserRepository) FindBySocialID(ctx context.Context, provider, socialID string) (*domain.User, error) {
	columna, err := columnaSocial(provider)
	if err != nil {
		return nil, err
	}
	if socialID == "" {
		return nil, errors.New("user not found")
	}

	sql := `
		SELECT id, email_blind_index, COALESCE(email_encrypted, ''), password_hash, role, COALESCE(email_verified, false), google_id, apple_id
		FROM core.users
		WHERE ` + columna + ` = $1
	`

	var user domain.User
	err = r.db.QueryRow(ctx, sql, socialID).Scan(
		&user.ID,
		&user.EmailBlindIndex,
		&user.EmailEncrypted,
		&user.PasswordHash,
		&user.Role,
		&user.EmailVerified,
		&user.GoogleID,
		&user.AppleID,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

// LinkSocialID deja constancia de con qué cuenta social entra alguien.
//
// El WHERE incluye que la columna esté vacía o ya sea la misma: si esa identidad
// estuviera enlazada a OTRA cuenta, el índice único lo impediría, y así el error
// llega como "no se actualizó nada" en vez de reventar la consulta.
func (r *UserRepository) LinkSocialID(ctx context.Context, userID, provider, socialID string) error {
	columna, err := columnaSocial(provider)
	if err != nil {
		return err
	}

	sql := `
		UPDATE core.users
		SET ` + columna + ` = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND (` + columna + ` IS NULL OR ` + columna + ` = $2)
	`

	tag, err := r.db.Exec(ctx, sql, userID, socialID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("no se pudo enlazar la identidad social: la cuenta ya tiene otra distinta")
	}
	return nil
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

// AnonymizeUser deja la cuenta sin ningún dato personal, pero conserva la fila.
//
// No se borra porque catorce tablas referencian core.users con ON DELETE
// CASCADE: un DELETE se llevaría por delante los mensajes de chat —incluida la
// mitad de las conversaciones de OTRAS personas—, las transacciones de eventos
// ya cobrados y las respuestas de encuestas. Y events.registrations ni siquiera
// tiene clave foránea: sus filas quedarían apuntando a un id inexistente.
//
// Qué se hace con cada cosa:
//   - El correo se libera con un valor único inventado. email_blind_index es
//     UNIQUE y NOT NULL, así que no puede quedar vacío; ponerlo derivado del id
//     garantiza que no choque y permite que esa persona vuelva a registrarse
//     mañana con el mismo correo.
//   - password_hash queda vacío. Es NOT NULL, y una cadena vacía no es un hash
//     válido de bcrypt, así que ninguna contraseña puede coincidir.
//   - first_name y last_name se guardan EN CLARO a propósito. El resto de la
//     tabla va cifrada, y los servicios descifran con el patrón "si falla, deja
//     el valor tal cual": así estos dos salen legibles como "Usuario eliminado"
//     sin necesidad de que el repositorio conozca la clave de cifrado.
//   - alias se libera por el mismo motivo que el correo: también es UNIQUE.
//   - Las preferencias de visibilidad se cierran, para que el perfil no siga
//     apareciendo en búsquedas ni acepte mensajes.
func (r *UserRepository) AnonymizeUser(ctx context.Context, id string) error {
	sql := `
		UPDATE core.users SET
			email_blind_index = 'deleted-' || id::text,
			email_encrypted = NULL,
			password_hash = '',
			first_name = 'Usuario',
			last_name = 'eliminado',
			phone = NULL,
			location = NULL,
			bio = NULL,
			industry = NULL,
			profile_image_url = NULL,
			generation = NULL,
			company_name = NULL,
			job_title = NULL,
			alias = NULL,
			identification_type = NULL,
			identification_number = NULL,
			customer_status = NULL,
			birth_date = NULL,
			refresh_token = NULL,
			is_public_profile = false,
			allow_messages_from_strangers = false,
			show_activity = false,
			deleted_at = COALESCE(deleted_at, now()),
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`
	// En transacción con el borrado de los tokens de notificación: si se
	// anonimizara la cuenta pero fallara lo segundo, esa persona seguiría
	// recibiendo push de una cuenta que cree eliminada. Un token FCM identifica
	// un dispositivo, así que es un dato personal más.
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	etiqueta, err := tx.Exec(ctx, sql, id)
	if err != nil {
		return err
	}
	// Cero filas significa que no existe o que ya estaba eliminada. Se trata
	// como "no encontrado" para que el handler no responda 200 sobre algo que
	// no ha ocurrido.
	if etiqueta.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM core.user_fcm_tokens WHERE user_id = $1`, id); err != nil {
		return err
	}

	return tx.Commit(ctx)
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

// MarkEmailAsVerified marca por id. Se excluyen las cuentas eliminadas: el
// borrado de cuenta sustituye el correo por un valor derivado del id, y un
// enlace de verificación pendiente no debe reactivar nada de una cuenta que su
// dueño dio por eliminada.
func (r *UserRepository) MarkEmailAsVerified(ctx context.Context, userID string) error {
	sql := "UPDATE core.users SET email_verified = true, updated_at = $1 WHERE id = $2 AND deleted_at IS NULL"
	_, err := r.db.Exec(ctx, sql, time.Now(), userID)
	return err
}

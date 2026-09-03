package domain

import (
	"time"
)

// UserRoles son los valores que acepta el enum core.user_role. El orden es el
// del enum. 'junta' se añadió el 2026-08-18
// (scripts/20260818_add_junta_user_role.sql); 'profesional' no lo usa ni la app
// ni el backend, pero el panel lo ofrece y puede haber cuentas con él.
//
// Un rol fuera de esta lista llega al INSERT y lo rechaza Postgres con SQLSTATE
// 22P02, así que la comprobación se hace antes de bajar a la base.
var UserRoles = []string{"familia", "empresa", "profesional", "junta"}

// RoleDefault es el rol que se asigna cuando el cliente no manda ninguno.
const RoleDefault = "familia"

// IsValidRole indica si el rol existe en el enum core.user_role.
func IsValidRole(role string) bool {
	for _, r := range UserRoles {
		if r == role {
			return true
		}
	}
	return false
}

// LongitudMinimaContrasena es el mínimo que se exige a cualquier contraseña.
//
// Hasta el 2026-08-19 esta regla vivía **solo en los clientes** —el formulario
// de registro de la app y el de restablecer del panel—, así que quien llamara a
// la API directamente se la saltaba: POST /reset-password aceptaba "ab123" con
// un 200 y la cuenta entraba después con esa contraseña.
//
// Vive en el dominio porque son cuatro los sitios que cifran una contraseña
// —registro, restablecer, cambiar y alta de administrador— y basta que uno se
// olvide para que la regla no valga nada, igual que pasó con la normalización
// del correo en BlindIndex.
const LongitudMinimaContrasena = 6

// ParsearFechaDeNacimiento acepta los tres formatos que de hecho llegan: el
// RFC3339 de la app, la fecha sola del panel ("1990-01-15") y el DD/MM/YYYY del
// registro y de los archivos de carga masiva.
//
// Vive en el dominio porque lo usan el handler de usuarios y el importador, y
// tener dos copias es cómo se acaba aceptando un formato en un sitio y no en el
// otro.
func ParsearFechaDeNacimiento(valor string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02", "02/01/2006"} {
		if t, err := time.Parse(layout, valor); err == nil {
			return t, nil
		}
	}
	return time.Time{}, ErrFechaNoReconocida
}

// ValidarContrasena aplica LongitudMinimaContrasena.
//
// Cuenta caracteres y no bytes: "contraseña" son diez caracteres, no once.
func ValidarContrasena(contrasena string) error {
	if len([]rune(contrasena)) < LongitudMinimaContrasena {
		return ErrContrasenaCorta
	}
	return nil
}

type User struct {
	ID            string `json:"id" db:"id"`
	Email         string `json:"email" db:"-"` // Input/Output only, not stored directly
	EmailVerified bool   `json:"email_verified" db:"email_verified"`

	// Internal fields for DB mapping
	EmailBlindIndex string `json:"-" db:"email_blind_index"`
	EmailEncrypted  string `json:"-" db:"email_encrypted"`

	GoogleID *string `json:"google_id" db:"google_id"`
	AppleID  *string `json:"apple_id" db:"apple_id"`

	PasswordHash    string     `json:"-" db:"password_hash"`
	FirstName       string     `json:"first_name" db:"first_name"`
	LastName        string     `json:"last_name" db:"last_name"`
	BirthDate       *time.Time `json:"birth_date" db:"birth_date"`
	Phone           string     `json:"phone" db:"phone"`
	Location        string     `json:"location" db:"location"`
	Bio             string     `json:"bio" db:"bio"`
	Industry        string     `json:"industry" db:"industry"`
	ProfileImageUrl string     `json:"profile_image_url" db:"profile_image_url"`
	CompanyName     string     `json:"company_name" db:"company_name"`
	JobTitle        string     `json:"job_title" db:"job_title"`
	Role            string     `json:"role" db:"role"`
	Country         string     `json:"country" db:"country"`

	// Sexo, departamento y dirección llegaron con la carga masiva del Summit
	// (reports/20260826_plan_carga_masiva.md §3.1). Van en la cuenta porque son
	// de la persona, no del evento, y en la app solo aparecen dentro de editar
	// perfil. `sexo` y `direccion` se guardan cifrados, como Location;
	// `departamento` en claro, como Country.
	Sexo         string `json:"sexo" db:"sexo"`
	Departamento string `json:"departamento" db:"departamento"`
	Direccion    string `json:"direccion" db:"direccion"`

	// DebeCambiarContrasena obliga a cambiarla en el primer ingreso. La pone la
	// carga masiva, que asigna como contraseña el número de documento.
	//
	// **Solo ChangePassword la baja.** No se escribe en el UPDATE general de
	// usuarios a propósito: performUpdate vuelca el JSON entero sobre el struct
	// y, sin esa exclusión, cualquiera se la quitaría mandando
	// {"debe_cambiar_contrasena": false} al editar su perfil.
	DebeCambiarContrasena bool `json:"debe_cambiar_contrasena" db:"debe_cambiar_contrasena"`

	IdentificationType         string `json:"identification_type" db:"identification_type"`
	IdentificationNumber       string `json:"identification_number" db:"identification_number"`
	CustomerStatus             string `json:"customer_status" db:"customer_status"`
	Generation                 string `json:"generation" db:"generation"`
	IsPublicProfile            bool   `json:"is_public_profile" db:"is_public_profile"`
	AllowMessagesFromStrangers bool   `json:"allow_messages_from_strangers" db:"allow_messages_from_strangers"`
	ShowActivity               bool   `json:"show_activity" db:"show_activity"`
	TermsAccepted              bool   `json:"terms_accepted" db:"terms_accepted"`
	DataSharingAccepted        bool   `json:"data_sharing_accepted" db:"data_sharing_accepted"`
	// Qué texto se aceptó y cuándo. Punteros porque las cuentas anteriores a
	// 2026-08-10 tienen el consentimiento sin versión: consta que aceptaron,
	// no qué leyeron. Ver domain/legal.go.
	TermsVersion          *string    `json:"terms_version" db:"terms_version"`
	TermsAcceptedAt       *time.Time `json:"terms_accepted_at" db:"terms_accepted_at"`
	DataSharingVersion    *string    `json:"data_sharing_version" db:"data_sharing_version"`
	DataSharingAcceptedAt *time.Time `json:"data_sharing_accepted_at" db:"data_sharing_accepted_at"`
	Interests             []string   `json:"interests" db:"-"` // Handled separately
	Alias                 string     `json:"alias" db:"alias"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

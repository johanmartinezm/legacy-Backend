package domain

import "time"

// AdminUser represents an administrator account that is separate from regular app users.
// It follows clean‑code principles: each field is explicit and the struct is purpose‑specific.
// The password is stored as a hashed string (bcrypt) in PasswordHash.
type AdminUser struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	Role         string    `json:"role"` // e.g., "admin", "superadmin"
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Roles de una cuenta del panel, guardados en core.admin_users.role.
//
// Hasta el 2026-11-09 la columna no servía para nada: AdminLogin firmaba
// siempre "admin". El formulario del panel ofrecía además "superadmin", que
// nunca dio más permisos que "admin" y se conserva por las cuentas que ya lo
// tengan.
const (
	RolAdminAdmin      = "admin"
	RolAdminSuperadmin = "superadmin"
	// RolAdminEventos solo ve eventos e inscritos y registra la asistencia.
	// Pensado para quien está en la puerta de un evento.
	RolAdminEventos = "eventos"
)

// NormalizarRolAdmin valida el rol que llega del panel. Vacío vale por
// "admin", como el DEFAULT de la columna.
func NormalizarRolAdmin(rol string) (string, error) {
	switch rol {
	case "":
		return RolAdminAdmin, nil
	case RolAdminAdmin, RolAdminSuperadmin, RolAdminEventos:
		return rol, nil
	}
	return "", ErrRolAdminInvalido
}

// RolDelToken es el claim "role" que firma AdminLogin. Solo "eventos" sale
// distinto de "admin": cualquier otro valor de la columna —"superadmin" o uno
// antiguo que no conozcamos— conserva el acceso completo que tenía.
func RolDelToken(rol string) string {
	if rol == RolAdminEventos {
		return RolAdminEventos
	}
	return RolAdminAdmin
}

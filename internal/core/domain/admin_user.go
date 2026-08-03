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

package domain

import (
	"time"
)

type User struct {
	ID    string `json:"id" db:"id"`
	Email string `json:"email" db:"-"` // Input/Output only, not stored directly
	EmailVerified bool `json:"email_verified" db:"email_verified"`

	// Internal fields for DB mapping
	EmailBlindIndex string `json:"-" db:"email_blind_index"`
	EmailEncrypted  string `json:"-" db:"email_encrypted"`
	
	GoogleID *string `json:"google_id" db:"google_id"`
	AppleID  *string `json:"apple_id" db:"apple_id"`

	PasswordHash               string     `json:"-" db:"password_hash"`
	FirstName                  string     `json:"first_name" db:"first_name"`
	LastName                   string     `json:"last_name" db:"last_name"`
	BirthDate                  *time.Time `json:"birth_date" db:"birth_date"`
	Phone                      string     `json:"phone" db:"phone"`
	Location                   string     `json:"location" db:"location"`
	Bio                        string     `json:"bio" db:"bio"`
	Industry                   string     `json:"industry" db:"industry"`
	ProfileImageUrl            string     `json:"profile_image_url" db:"profile_image_url"`
	CompanyName                string     `json:"company_name" db:"company_name"`
	JobTitle                   string     `json:"job_title" db:"job_title"`
	Role                       string     `json:"role" db:"role"`
	Country                    string     `json:"country" db:"country"`
	IdentificationType         string     `json:"identification_type" db:"identification_type"`
	IdentificationNumber       string     `json:"identification_number" db:"identification_number"`
	CustomerStatus             string     `json:"customer_status" db:"customer_status"`
	Generation                 string     `json:"generation" db:"generation"`
	IsPublicProfile            bool       `json:"is_public_profile" db:"is_public_profile"`
	AllowMessagesFromStrangers bool       `json:"allow_messages_from_strangers" db:"allow_messages_from_strangers"`
	ShowActivity               bool       `json:"show_activity" db:"show_activity"`
	TermsAccepted              bool       `json:"terms_accepted" db:"terms_accepted"`
	DataSharingAccepted        bool       `json:"data_sharing_accepted" db:"data_sharing_accepted"`
	Interests                  []string   `json:"interests" db:"-"` // Handled separately
	Alias                      string     `json:"alias" db:"alias"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

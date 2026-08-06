package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"applegacy/backend/internal/security"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"
)

type AuthService struct {
	repo          ports.UserRepository
	adminRepo     ports.AdminUserRepository
	tokenRepo      ports.PasswordResetRepository
	verifyRepo     ports.EmailVerificationRepository
	emailService   ports.EmailService
	crypto         *security.CryptoService
	jwtSecret      []byte
	tokenDuration  time.Duration
	resetURL       string
	verifyURL      string
	googleClientID string
}

func NewAuthService(
	repo ports.UserRepository,
	adminRepo ports.AdminUserRepository,
	tokenRepo ports.PasswordResetRepository,
	verifyRepo ports.EmailVerificationRepository,
	emailService ports.EmailService,
	crypto *security.CryptoService,
	jwtSecret string,
	resetURL string,
	verifyURL string,
	googleClientID string,
) *AuthService {
	return &AuthService{
		repo:           repo,
		adminRepo:      adminRepo,
		tokenRepo:      tokenRepo,
		verifyRepo:     verifyRepo,
		emailService:   emailService,
		crypto:         crypto,
		jwtSecret:      []byte(jwtSecret),
		tokenDuration:  24 * time.Hour,
		resetURL:       resetURL,
		verifyURL:      verifyURL,
		googleClientID: googleClientID,
	}
}

func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	// 1. Find user by email (blind index)
	blindIndex := s.crypto.BlindIndex(email)
	_, err := s.repo.FindByEmailBlindIndex(ctx, blindIndex)
	if err != nil {
		// For security, don't reveal if user exists. Just return nil.
		return nil
	}

	// 2. Generate token
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)

	// 3. Store token
	if err := s.tokenRepo.StoreToken(ctx, email, token); err != nil {
		return err
	}

	// 4. Send email
	resetLink := fmt.Sprintf("%s?token=%s&email=%s", s.resetURL, token, email)
	return s.emailService.SendResetPasswordEmail(email, resetLink)
}

func (s *AuthService) ResetPassword(ctx context.Context, email, token, newPassword string) error {
	// 1. Verify token
	storedToken, err := s.tokenRepo.GetToken(ctx, email)
	if err != nil {
		return errors.New("invalid or expired token")
	}

	if storedToken != token {
		return errors.New("invalid token")
	}

	// 2. Hash new password
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 3. Update user password
	blindIndex := s.crypto.BlindIndex(email)
	if err := s.repo.UpdatePasswordByEmail(ctx, blindIndex, string(hashed)); err != nil {
		return err
	}

	// 4. Delete token
	return s.tokenRepo.DeleteToken(ctx, email)
}

func (s *AuthService) Register(ctx context.Context, user *domain.User, password string) error {
	// 1. Check if user exists (using Blind Index)
	blindIndex := s.crypto.BlindIndex(user.Email)
	existing, err := s.repo.FindByEmailBlindIndex(ctx, blindIndex)
	if existing != nil {
		return errors.New("user already exists")
	}
	if err != nil && err.Error() != "user not found" {
		// If error is other than not found, return it
		// Ideally repo returns specific error for not found
		return err
	}

	// 2. Hash Password only if provided
	if password != "" {
		hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		user.PasswordHash = string(hashedBytes)
	} else if user.GoogleID == nil && user.AppleID == nil {
		// Enforce password if it's a regular registration
		return errors.New("password is required for non-social registration")
	}

	// Capture raw values before encryption
	rawEmail := user.Email
	rawFirstName := user.FirstName

	// 3. Encrypt PII
	user.EmailBlindIndex = blindIndex
	user.EmailEncrypted, _ = s.crypto.Encrypt(user.Email)
	user.FirstName, _ = s.crypto.Encrypt(user.FirstName)
	user.LastName, _ = s.crypto.Encrypt(user.LastName)
	user.Phone, _ = s.crypto.Encrypt(user.Phone)
	user.Location, _ = s.crypto.Encrypt(user.Location)
	user.Bio, _ = s.crypto.Encrypt(user.Bio)
	user.CompanyName, _ = s.crypto.Encrypt(user.CompanyName)
	user.JobTitle, _ = s.crypto.Encrypt(user.JobTitle)
	user.IdentificationNumber, _ = s.crypto.Encrypt(user.IdentificationNumber)

	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	isSocial := user.GoogleID != nil || user.AppleID != nil
	user.EmailVerified = isSocial

	// 4. Save to Repo
	err = s.repo.Create(ctx, user)
	if err != nil {
		return err
	}

	// 5. Generate Token and Send Verification Email if not verified
	if !user.EmailVerified {
		b := make([]byte, 32)
		rand.Read(b)
		token := hex.EncodeToString(b)
		
		err = s.verifyRepo.StoreToken(ctx, blindIndex, token, time.Now().Add(24*time.Hour))
		if err != nil {
			return err
		}

		verifyLink := fmt.Sprintf("%s?token=%s", s.verifyURL, token)
		go func(email, link string) {
			_ = s.emailService.SendVerificationEmail(email, link)
		}(rawEmail, verifyLink)
	} else {
		// Just send welcome email for social logins
		go func(email, name string) {
			_ = s.emailService.SendWelcomeEmail(email, name)
		}(rawEmail, rawFirstName)
	}

	return nil
}

func (s *AuthService) SocialLogin(ctx context.Context, provider, idToken string) (string, *domain.User, error) {
	var extractedEmail string
	var extractedName string

	if provider == "google" {
		payload, err := idtoken.Validate(ctx, idToken, s.googleClientID)
		if err != nil {
			return "", nil, errors.New("invalid google token: " + err.Error())
		}
		extractedEmail = payload.Claims["email"].(string)
		if name, ok := payload.Claims["name"].(string); ok {
			extractedName = name
		}
	} else if provider == "apple" {
		// Mock for now until Apple verification logic is set
		extractedEmail = "user_apple@example.com"
		extractedName = "Apple User"
	} else {
		return "", nil, errors.New("unsupported provider")
	}

	blindIndex := s.crypto.BlindIndex(extractedEmail)
	user, err := s.repo.FindByEmailBlindIndex(ctx, blindIndex)

	if err != nil || user == nil {
		// User does not exist in our DB.
		// We return a mock user so the handler knows who they are to prefill register form
		return "", &domain.User{Email: extractedEmail, FirstName: extractedName}, errors.New("user not registered")
	}

	// User exists. Link provider ID if not linked yet.
	if provider == "google" && user.GoogleID == nil {
		// Update DB with google ID (dummy update here)
	}

	// Generate JWT
	// "sub" es el claim que lee AuthMiddleware (middleware.go:52). Este token
	// solo llevaba "user_id", asi que quien entraba con Google o Apple recibia
	// un token valido con el que NINGUNA ruta privada funcionaba: respondian 401
	// "User ID not found in token". Ni inscribirse a un evento, ni la agenda, ni
	// el chat, ni la encuesta.
	//
	// Se conserva "user_id" por si algun cliente ya lo lee; lo que hacia falta
	// era anadir "sub", no sustituirlo.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":     user.ID,
		"user_id": user.ID,
		"email":   extractedEmail,
		"role":    user.Role,
		"exp":     time.Now().Add(s.tokenDuration).Unix(),
	})

	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", nil, err
	}

	return tokenString, user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	// Strictly for regular users (via Flutter app)
	blindIndex := s.crypto.BlindIndex(email)
	user, err := s.repo.FindByEmailBlindIndex(ctx, blindIndex)
	if err != nil || user == nil {
		return "", errors.New("invalid credentials")
	}

	if user.PasswordHash == "" {
		return "", errors.New("account uses social login, please sign in with Google or Apple")
	}

	// Verify email before login
	if !user.EmailVerified {
		return "", errors.New("email_not_verified")
	}

	// Verify password for regular user
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}

	// Generate JWT with role claim
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  user.ID,
		"exp":  time.Now().Add(s.tokenDuration).Unix(),
		"role": user.Role,
	})
	return token.SignedString(s.jwtSecret)
}

// RegisterAdmin creates a new admin user.
func (s *AuthService) RegisterAdmin(ctx context.Context, admin *domain.AdminUser, password string) error {
	// 1. Ensure email not already used
	if existing, _ := s.adminRepo.FindByEmail(ctx, admin.Email); existing != nil {
		return errors.New("admin already exists")
	}
	// 2. Hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	admin.PasswordHash = string(hashed)
	admin.CreatedAt = time.Now()
	admin.UpdatedAt = time.Now()
	return s.adminRepo.Create(ctx, admin)
}

// AdminLogin authenticates an admin and returns a JWT.
func (s *AuthService) AdminLogin(ctx context.Context, email, password string) (string, error) {
	admin, err := s.adminRepo.FindByEmail(ctx, email)
	if err != nil || admin == nil {
		return "", errors.New("invalid credentials")
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) != nil {
		return "", errors.New("invalid credentials")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  admin.ID,
		"exp":  time.Now().Add(s.tokenDuration).Unix(),
		"role": "admin",
	})
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) ListUsers(ctx context.Context) ([]*domain.User, error) {
	users, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		s.decryptUser(user)
	}

	return users, nil
}

// ListAdmins returns all admin users.
func (s *AuthService) ListAdmins(ctx context.Context) ([]*domain.AdminUser, error) {
	admins, err := s.adminRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	// Admin fields are not encrypted.
	return admins, nil
}

// UpdateAdmin updates admin details.
func (s *AuthService) UpdateAdmin(ctx context.Context, admin *domain.AdminUser) error {
	admin.UpdatedAt = time.Now()
	return s.adminRepo.Update(ctx, admin)
}

// DeleteAdmin removes an admin user.
func (s *AuthService) DeleteAdmin(ctx context.Context, id string) error {
	return s.adminRepo.Delete(ctx, id)
}

func (s *AuthService) GetProfile(ctx context.Context, id string) (*domain.User, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.decryptUser(user)
	return user, nil
}

func (s *AuthService) decryptUser(user *domain.User) {
	user.Email, _ = s.crypto.Decrypt(user.EmailEncrypted)
	user.FirstName, _ = s.crypto.Decrypt(user.FirstName)
	user.LastName, _ = s.crypto.Decrypt(user.LastName)
	user.Phone, _ = s.crypto.Decrypt(user.Phone)
	user.Location, _ = s.crypto.Decrypt(user.Location)
	user.Bio, _ = s.crypto.Decrypt(user.Bio)
	user.CompanyName, _ = s.crypto.Decrypt(user.CompanyName)
	user.JobTitle, _ = s.crypto.Decrypt(user.JobTitle)
	user.IdentificationNumber, _ = s.crypto.Decrypt(user.IdentificationNumber)
}

func (s *AuthService) UpdateUser(ctx context.Context, user *domain.User) error {
	if user.Alias != "" {
		validAlias := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
		if !validAlias.MatchString(user.Alias) {
			return errors.New("invalid_alias_format")
		}
	}

	// 1. Re-calculate Blind Index and Encrypt PII if they changed (or just always do it for simplicity in MVP)
	user.EmailBlindIndex = s.crypto.BlindIndex(user.Email)
	user.EmailEncrypted, _ = s.crypto.Encrypt(user.Email)
	user.FirstName, _ = s.crypto.Encrypt(user.FirstName)
	user.LastName, _ = s.crypto.Encrypt(user.LastName)
	user.Phone, _ = s.crypto.Encrypt(user.Phone)
	user.Location, _ = s.crypto.Encrypt(user.Location)
	user.Bio, _ = s.crypto.Encrypt(user.Bio)
	user.CompanyName, _ = s.crypto.Encrypt(user.CompanyName)
	user.JobTitle, _ = s.crypto.Encrypt(user.JobTitle)
	user.IdentificationNumber, _ = s.crypto.Encrypt(user.IdentificationNumber)

	user.UpdatedAt = time.Now()

	return s.repo.Update(ctx, user)
}

func (s *AuthService) DeleteUser(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *AuthService) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	// 1. Get user (raw, to get password hash)
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	// 2. Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return errors.New("invalid current password")
	}

	// Hash new password
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.repo.UpdatePassword(ctx, userID, string(hashed))
}

func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	blindIndex, err := s.verifyRepo.ValidateToken(ctx, token)
	if err != nil {
		return err
	}

	err = s.repo.MarkEmailAsVerified(ctx, blindIndex)
	if err != nil {
		return err
	}

	// Optionally delete the token after successful verification
	_ = s.verifyRepo.DeleteToken(ctx, token)
	return nil
}

func (s *AuthService) ResendVerificationEmail(ctx context.Context, email string) error {
	blindIndex := s.crypto.BlindIndex(email)
	user, err := s.repo.FindByEmailBlindIndex(ctx, blindIndex)
	if err != nil || user == nil {
		return errors.New("user not found")
	}

	if user.EmailVerified {
		return errors.New("email already verified")
	}

	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)
	
	err = s.verifyRepo.StoreToken(ctx, blindIndex, token, time.Now().Add(24*time.Hour))
	if err != nil {
		return err
	}

	verifyLink := fmt.Sprintf("%s?token=%s", s.verifyURL, token)
	go func(e, link string) {
		_ = s.emailService.SendVerificationEmail(e, link)
	}(email, verifyLink)

	return nil
}

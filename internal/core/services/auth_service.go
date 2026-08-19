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
	"log"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"
)

type AuthService struct {
	repo           ports.UserRepository
	adminRepo      ports.AdminUserRepository
	tokenRepo      ports.PasswordResetRepository
	verifyRepo     ports.EmailVerificationRepository
	emailService   ports.EmailService
	crypto         *security.CryptoService
	jwtSecret      []byte
	tokenDuration  time.Duration
	resetURL       string
	verifyURL      string
	googleClientID string
	// appleValidator verifica los tokens de Sign in with Apple. Puede ser nil si
	// falta la configuración; en ese caso el inicio de sesión con Apple se
	// rechaza en vez de dejar pasar a cualquiera, que es lo que hacía antes.
	appleValidator ports.ValidadorDeApple
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
	appleValidator ports.ValidadorDeApple,
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
		appleValidator: appleValidator,
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

// sellarConsentimiento deja constancia de QUÉ versión de cada texto legal se
// aceptó y CUÁNDO. Sin esto solo queda un booleano, que prueba que hubo
// aceptación pero no de qué, y el Decreto 1377 de 2013 exige lo segundo.
//
// La versión sale de domain, no del cuerpo de la petición: si la enviara el
// cliente, una app antigua o manipulada podría declarar una versión que nunca
// mostró. Y no se sella lo que no se aceptó — marcar una fecha sobre un
// consentimiento negado sería fabricar prueba de lo contrario de lo ocurrido.
func sellarConsentimiento(user *domain.User, cuando time.Time) {
	if user.TermsAccepted {
		v := domain.TermsVersionVigente
		user.TermsVersion = &v
		user.TermsAcceptedAt = &cuando
	}
	if user.DataSharingAccepted {
		v := domain.PrivacyVersionVigente
		user.DataSharingVersion = &v
		user.DataSharingAcceptedAt = &cuando
	}
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

	sellarConsentimiento(user, user.CreatedAt)

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

		// user.ID lo devuelve el INSERT del repositorio.
		err = s.verifyRepo.StoreToken(ctx, user.ID, token, time.Now().Add(24*time.Hour))
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

	// socialID es el identificador estable que da el proveedor (el claim `sub`).
	// Con Apple es imprescindible: solo manda el correo en el PRIMER inicio de
	// sesión de cada persona, y puede ser una dirección de retransmisión
	// privada. Buscar por correo dejaría fuera a quien vuelva a entrar.
	var socialID string

	if provider == "google" {
		payload, err := idtoken.Validate(ctx, idToken, s.googleClientID)
		if err != nil {
			return "", nil, errors.New("invalid google token: " + err.Error())
		}
		socialID = payload.Subject
		extractedEmail, _ = payload.Claims["email"].(string)
		if name, ok := payload.Claims["name"].(string); ok {
			extractedName = name
		}
	} else if provider == "apple" {
		// Esto ANTES no existía: el token se ignoraba y se devolvía siempre
		// user_apple@example.com. Cualquiera con cualquier cadena entraba, y en
		// cuanto esa cuenta existiera habría bastado para suplantarla.
		if s.appleValidator == nil {
			return "", nil, errors.New("sign in with apple no está configurado en el servidor")
		}
		identidad, err := s.appleValidator.Validar(ctx, idToken)
		if err != nil {
			return "", nil, fmt.Errorf("invalid apple token: %w", err)
		}
		socialID = identidad.Sujeto
		extractedEmail = identidad.Correo
	} else {
		return "", nil, errors.New("unsupported provider")
	}

	if socialID == "" {
		return "", nil, errors.New("el proveedor no identificó a la persona")
	}

	// 1. Por identidad social, que es la que no cambia.
	user, err := s.repo.FindBySocialID(ctx, provider, socialID)

	// 2. Si no aparece, por correo: es quien ya tenía cuenta y entra por primera
	//    vez con este proveedor. Con Apple puede no haber correo, y entonces no
	//    hay nada que enlazar: si su `sub` no está registrado, es alguien nuevo.
	if (err != nil || user == nil) && extractedEmail != "" {
		user, err = s.repo.FindByEmailBlindIndex(ctx, s.crypto.BlindIndex(extractedEmail))
	}

	if err != nil || user == nil {
		// No tiene cuenta: el handler usa estos datos para prellenar el
		// formulario de registro. El correo puede ir vacío si Apple no lo dio.
		return "", &domain.User{Email: extractedEmail, FirstName: extractedName},
			errors.New("user not registered")
	}

	// Queda constancia de con qué cuenta social entra, para reconocerla la
	// próxima vez aunque el proveedor no vuelva a mandar el correo.
	yaEnlazado := (provider == "google" && user.GoogleID != nil && *user.GoogleID == socialID) ||
		(provider == "apple" && user.AppleID != nil && *user.AppleID == socialID)
	if !yaEnlazado {
		if err := s.repo.LinkSocialID(ctx, user.ID, provider, socialID); err != nil {
			// No se corta el inicio de sesión por esto: la persona ya está
			// identificada. Se reintentará la próxima vez.
			log.Printf("[AUTH] no se pudo enlazar la identidad de %s del usuario %s: %v",
				provider, user.ID, err)
		}
	}

	// El correo del token puede ser el de retransmisión privada de Apple, que no
	// es el de la cuenta: para el JWT manda el que tenemos guardado.
	if correoGuardado, err := s.crypto.Decrypt(user.EmailEncrypted); err == nil && correoGuardado != "" {
		extractedEmail = correoGuardado
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

// Los motivos por los que un inicio de sesión no prospera están en el dominio
// (domain.ErrCredencialesInvalidas y compañía), para que el handler pueda
// distinguirlos sin importar este paquete.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	// Strictly for regular users (via Flutter app)
	blindIndex := s.crypto.BlindIndex(email)
	user, err := s.repo.FindByEmailBlindIndex(ctx, blindIndex)
	if err != nil || user == nil {
		return "", domain.ErrCredencialesInvalidas
	}

	if user.PasswordHash == "" {
		return "", domain.ErrCuentaSocial
	}

	// 🔴 La contraseña se comprueba ANTES que la verificación del correo, y el
	// orden importa: hasta el 2026-08-18 era al revés.
	//
	// Decirle a alguien "te falta verificar el correo" confirma que esa cuenta
	// existe. Si eso se responde antes de pedir la contraseña, cualquiera puede
	// ir probando correos y averiguar quién está registrado. Comprobando primero
	// la contraseña, solo se entera quien ya demostró ser el dueño, y ahí
	// explicarle qué le falta no filtra nada.
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", domain.ErrCredencialesInvalidas
	}

	if !user.EmailVerified {
		return "", domain.ErrCorreoSinVerificar
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
	userID, err := s.verifyRepo.ValidateToken(ctx, token)
	if err != nil {
		return err
	}

	err = s.repo.MarkEmailAsVerified(ctx, userID)
	if err != nil {
		return err
	}

	// Optionally delete the token after successful verification
	_ = s.verifyRepo.DeleteToken(ctx, token)
	return nil
}

func (s *AuthService) ResendVerificationEmail(ctx context.Context, email string) error {
	user, err := s.repo.FindByEmailBlindIndex(ctx, s.crypto.BlindIndex(email))
	if err != nil || user == nil {
		return errors.New("user not found")
	}

	if user.EmailVerified {
		return errors.New("email already verified")
	}

	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)

	err = s.verifyRepo.StoreToken(ctx, user.ID, token, time.Now().Add(24*time.Hour))
	if err != nil {
		return err
	}

	verifyLink := fmt.Sprintf("%s?token=%s", s.verifyURL, token)
	go func(e, link string) {
		_ = s.emailService.SendVerificationEmail(e, link)
	}(email, verifyLink)

	return nil
}

// DeleteMyAccount atiende "eliminar mi cuenta" desde la app.
//
// Es un requisito de tienda, no una mejora: Apple lo exige desde junio de 2022
// (directriz 5.1.1(v)) a toda app que permita registrarse, y Google Play
// también. Sin esto la app no se puede publicar.
//
// La cuenta se anonimiza en lugar de borrarse. El motivo está en el esquema:
// catorce tablas dependen de core.users con ON DELETE CASCADE, así que un
// borrado real destruiría los mensajes de OTRAS personas —cada conversación
// perdería su otra mitad—, las transacciones de eventos ya cobrados y las
// respuestas de encuestas. Anonimizar cumple igual con el RGPD y con las dos
// tiendas: la persona desaparece, los registros contables se quedan.
//
// Tras esto, el correo queda libre para volver a registrarse.
func (s *AuthService) DeleteMyAccount(ctx context.Context, userID string) error {
	if userID == "" {
		return errors.New("se necesita el usuario que pide el borrado")
	}

	// El usuario sale del token en el handler, nunca del cuerpo de la petición:
	// aceptarlo de fuera permitiría borrar la cuenta de cualquiera.
	if err := s.repo.AnonymizeUser(ctx, userID); err != nil {
		return err
	}

	return nil
}

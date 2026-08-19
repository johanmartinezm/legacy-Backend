package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/security"
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// 🔴 La propiedad que hay que preservar: la longitud mínima de la contraseña la
// impone el servidor, no el formulario.
//
// Hasta el 2026-08-19 la regla vivía solo en los clientes —register_screen.dart
// y reset-password.component.ts—, así que POST /reset-password aceptaba "ab123"
// con un 200 y la cuenta entraba luego con esa contraseña. Son CUATRO los
// caminos que cifran una contraseña; si alguien añade un quinto sin llamar a
// domain.ValidarContrasena, este archivo no lo detectará, pero al menos deja
// escrito que la regla es del dominio.

// repoContrasena anota la última contraseña que le mandaron guardar. Si al
// terminar sigue vacía, es que la validación cortó antes de llegar a la base.
type repoContrasena struct {
	usuariosDePrueba
	usuario  *domain.User
	guardado string
	creado   bool
}

func (r *repoContrasena) FindByID(ctx context.Context, id string) (*domain.User, error) {
	if r.usuario == nil {
		return nil, errors.New("user not found")
	}
	return r.usuario, nil
}

func (r *repoContrasena) FindByEmailBlindIndex(ctx context.Context, b string) (*domain.User, error) {
	return nil, errors.New("user not found")
}

func (r *repoContrasena) Create(ctx context.Context, user *domain.User) error {
	r.creado = true
	r.guardado = user.PasswordHash
	return nil
}

func (r *repoContrasena) UpdatePassword(ctx context.Context, id, hash string) error {
	r.guardado = hash
	return nil
}

func (r *repoContrasena) UpdatePasswordByEmail(ctx context.Context, blindIndex, hash string) error {
	r.guardado = hash
	return nil
}

// tokensDeReinicio devuelve siempre el mismo token y anota si lo borraron.
type tokensDeReinicio struct {
	token   string
	borrado bool
}

func (t *tokensDeReinicio) StoreToken(ctx context.Context, email, token string) error { return nil }
func (t *tokensDeReinicio) GetToken(ctx context.Context, email string) (string, error) {
	return t.token, nil
}
func (t *tokensDeReinicio) DeleteToken(ctx context.Context, email string) error {
	t.borrado = true
	return nil
}

func servicioContrasena(t *testing.T, repo *repoContrasena, tokens *tokensDeReinicio) *AuthService {
	t.Helper()
	crypto, err := security.NewCryptoService("12345678901234567890123456789012")
	if err != nil {
		t.Fatalf("crypto: %v", err)
	}
	return &AuthService{
		repo:         repo,
		tokenRepo:    tokens,
		verifyRepo:   &repoTokensVerificacion{},
		emailService: &correoDePrueba{},
		crypto:       crypto,
		jwtSecret:    []byte("secreto-de-prueba"),
	}
}

func TestRegister_RechazaLaContrasenaCorta(t *testing.T) {
	repo := &repoContrasena{}
	s := servicioContrasena(t, repo, &tokensDeReinicio{})

	err := s.Register(context.Background(),
		&domain.User{Email: "corta@prueba.test", Role: domain.RoleDefault}, "ab123")

	if !errors.Is(err, domain.ErrContrasenaCorta) {
		t.Fatalf("se esperaba ErrContrasenaCorta y llegó: %v", err)
	}
	if repo.creado {
		t.Fatal("la cuenta se creó igual: la validación no cortó nada")
	}
}

func TestRegister_AceptaLaContrasenaDeSeis(t *testing.T) {
	repo := &repoContrasena{}
	s := servicioContrasena(t, repo, &tokensDeReinicio{})

	if err := s.Register(context.Background(),
		&domain.User{Email: "justa@prueba.test", Role: domain.RoleDefault}, "abc123"); err != nil {
		t.Fatalf("seis caracteres son suficientes, y falló: %v", err)
	}
	if !repo.creado {
		t.Fatal("no se creó la cuenta")
	}
}

// El registro social llega sin contraseña. Exigirle seis caracteres a una
// cadena vacía dejaría fuera a quien entra con Google o Apple.
func TestRegister_ElRegistroSocialSigueSinContrasena(t *testing.T) {
	repo := &repoContrasena{}
	s := servicioContrasena(t, repo, &tokensDeReinicio{})
	google := "google-123"

	if err := s.Register(context.Background(),
		&domain.User{Email: "social@prueba.test", Role: domain.RoleDefault, GoogleID: &google}, ""); err != nil {
		t.Fatalf("el registro social no debería pedir contraseña: %v", err)
	}
	if !repo.creado {
		t.Fatal("no se creó la cuenta social")
	}
}

// 🔴 Un intento rechazado NO puede gastar el token del correo: quien se
// equivoca escribiendo la contraseña nueva tiene que poder reintentar con el
// mismo enlace, sin pedir otro.
func TestResetPassword_LaContrasenaCortaNoGastaElToken(t *testing.T) {
	repo := &repoContrasena{}
	tokens := &tokensDeReinicio{token: "token-valido"}
	s := servicioContrasena(t, repo, tokens)

	err := s.ResetPassword(context.Background(), "quien@sea.test", "token-valido", "ab123")

	if !errors.Is(err, domain.ErrContrasenaCorta) {
		t.Fatalf("se esperaba ErrContrasenaCorta y llegó: %v", err)
	}
	if repo.guardado != "" {
		t.Fatal("se guardó una contraseña de cinco caracteres")
	}
	if tokens.borrado {
		t.Fatal("el token se consumió en un intento que no prosperó")
	}
}

func TestResetPassword_ConUnaContrasenaValidaSiCambia(t *testing.T) {
	repo := &repoContrasena{}
	tokens := &tokensDeReinicio{token: "token-valido"}
	s := servicioContrasena(t, repo, tokens)

	if err := s.ResetPassword(context.Background(), "quien@sea.test", "token-valido", "NuevaQa456"); err != nil {
		t.Fatalf("restablecer con una contraseña válida falló: %v", err)
	}
	if repo.guardado == "" {
		t.Fatal("no se guardó la contraseña nueva")
	}
	if !tokens.borrado {
		t.Fatal("el token debería quedar consumido tras un cambio efectivo")
	}
}

func TestChangePassword_RechazaLaNuevaSiEsCorta(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("LaActual123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	repo := &repoContrasena{usuario: &domain.User{ID: "usuario-1", PasswordHash: string(hash)}}
	s := servicioContrasena(t, repo, &tokensDeReinicio{})

	err = s.ChangePassword(context.Background(), "usuario-1", "LaActual123", "ab123")

	if !errors.Is(err, domain.ErrContrasenaCorta) {
		t.Fatalf("se esperaba ErrContrasenaCorta y llegó: %v", err)
	}
	if repo.guardado != "" {
		t.Fatal("se guardó una contraseña de cinco caracteres")
	}
}

// "contraseña" son diez caracteres aunque ocupe once bytes: la regla cuenta
// caracteres, que es lo que cuenta quien la escribe.
func TestValidarContrasena_CuentaCaracteresYNoBytes(t *testing.T) {
	if err := domain.ValidarContrasena("añoñí6"); err != nil {
		t.Fatalf("seis caracteres con tildes deberían valer: %v", err)
	}
	if err := domain.ValidarContrasena("añoñí"); !errors.Is(err, domain.ErrContrasenaCorta) {
		t.Fatalf("cinco caracteres con tildes deberían fallar, y llegó: %v", err)
	}
}

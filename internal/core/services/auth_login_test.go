package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/security"
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// repoLogin devuelve siempre el mismo usuario, o nada si no hay ninguno.
type repoLogin struct {
	usuariosDePrueba
	user *domain.User
}

func (r *repoLogin) FindByEmailBlindIndex(ctx context.Context, b string) (*domain.User, error) {
	if r.user == nil {
		return nil, errors.New("user not found")
	}
	return r.user, nil
}

func servicioLogin(t *testing.T, user *domain.User) *AuthService {
	t.Helper()
	crypto, err := security.NewCryptoService("12345678901234567890123456789012")
	if err != nil {
		t.Fatalf("crypto: %v", err)
	}
	s := &AuthService{repo: &repoLogin{user: user}, crypto: crypto, jwtSecret: []byte("secreto-de-prueba")}
	return s
}

func usuarioCon(t *testing.T, password string, verificado bool) *domain.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return &domain.User{
		ID:            "usuario-1",
		PasswordHash:  string(hash),
		EmailVerified: verificado,
		Role:          domain.RoleDefault,
	}
}

// 🔴 La propiedad que hay que preservar: el inicio de sesión no puede servir
// para averiguar qué correos están registrados.
//
// Es lo que se rompe si alguien "mejora" los mensajes moviendo la comprobación
// del correo verificado antes del bcrypt, que es justo como estaba hasta el
// 2026-08-18.
func TestLogin_SinAcertarLaContrasenaNoRevelaSiLaCuentaExiste(t *testing.T) {
	sinCuenta := servicioLogin(t, nil)
	_, errSinCuenta := sinCuenta.Login(context.Background(), "quien@sea.test", "Prueba123")

	conCuentaSinVerificar := servicioLogin(t, usuarioCon(t, "LaBuena123", false))
	_, errMalaClave := conCuentaSinVerificar.Login(context.Background(), "quien@sea.test", "NoEsLaMia")

	if !errors.Is(errSinCuenta, domain.ErrCredencialesInvalidas) {
		t.Errorf("cuenta inexistente: se esperaba credenciales invalidas y salio %v", errSinCuenta)
	}
	if !errors.Is(errMalaClave, domain.ErrCredencialesInvalidas) {
		t.Errorf("contrasena mala: se esperaba credenciales invalidas y salio %v", errMalaClave)
	}
	if errSinCuenta.Error() != errMalaClave.Error() {
		t.Errorf("los dos errores se distinguen y no deben: %q vs %q", errSinCuenta, errMalaClave)
	}
}

// Con la contraseña correcta sí se puede explicar qué falta: quien acertó ya
// demostró ser el dueño de la cuenta.
func TestLogin_ConLaContrasenaCorrectaAvisaQueFaltaVerificar(t *testing.T) {
	s := servicioLogin(t, usuarioCon(t, "LaBuena123", false))

	_, err := s.Login(context.Background(), "quien@sea.test", "LaBuena123")

	if !errors.Is(err, domain.ErrCorreoSinVerificar) {
		t.Errorf("se esperaba correo sin verificar y salio %v", err)
	}
}

func TestLogin_CuentaVerificadaEntra(t *testing.T) {
	s := servicioLogin(t, usuarioCon(t, "LaBuena123", true))

	token, err := s.Login(context.Background(), "quien@sea.test", "LaBuena123")

	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if token == "" {
		t.Error("no devolvio token")
	}
}

// Quien se registró con Google o Apple no tiene contraseña. Decírselo no filtra
// nada que no sepa ya al pulsar el botón de su proveedor.
func TestLogin_CuentaSocialLoDice(t *testing.T) {
	user := usuarioCon(t, "loquesea", true)
	user.PasswordHash = ""
	s := servicioLogin(t, user)

	_, err := s.Login(context.Background(), "quien@sea.test", "Prueba123")

	if !errors.Is(err, domain.ErrCuentaSocial) {
		t.Errorf("se esperaba cuenta social y salio %v", err)
	}
}

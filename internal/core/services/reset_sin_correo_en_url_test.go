package services

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/security"
)

// El enlace de recuperación llevaba el correo como segundo parámetro de la URL.
// Todo lo que viaja en una URL se filtra a sitios que nadie controla: el
// historial, la cabecera Referer, los registros de los proxies del camino y
// quien reciba el enlace si se reenvía. El token basta para identificar la
// solicitud, y el correo se resuelve desde él.

// tokensEspia recuerda con qué token se preguntó, para comprobar que el correo
// no entra por ningún otro sitio.
type tokensEspia struct {
	tokenGuardado   string
	correoDelToken  string
	tokenPreguntado string
	borradoPara     string
}

func (t *tokensEspia) StoreToken(ctx context.Context, email, token string) error {
	t.tokenGuardado = token
	t.correoDelToken = email
	return nil
}

func (t *tokensEspia) GetToken(ctx context.Context, email string) (string, error) {
	return t.tokenGuardado, nil
}

func (t *tokensEspia) GetEmailByToken(ctx context.Context, token string) (string, error) {
	t.tokenPreguntado = token
	if token == "" || token != t.tokenGuardado {
		return "", errors.New("no such token")
	}
	return t.correoDelToken, nil
}

func (t *tokensEspia) DeleteToken(ctx context.Context, email string) error {
	t.borradoPara = email
	return nil
}

// correoEspia guarda el enlace que se mandó, que es lo que se quiere
// inspeccionar. Se apoya en correoDePrueba para el resto de la interfaz.
type correoEspia struct {
	correoDePrueba
	enlace string
}

func (c *correoEspia) SendResetPasswordEmail(to, resetURL string) error {
	c.enlace = resetURL
	return nil
}

// repoConCuenta es un repoContrasena cuya búsqueda por correo SÍ encuentra a
// alguien: `RequestPasswordReset` se corta en silencio si no existe la cuenta,
// y entonces no llegaría a mandar ningún enlace que inspeccionar.
type repoConCuenta struct {
	repoContrasena
}

func (r *repoConCuenta) FindByEmailBlindIndex(ctx context.Context, b string) (*domain.User, error) {
	return &domain.User{Email: "da-igual@prueba.test", Role: domain.RoleDefault}, nil
}

func servicioConEspias(t *testing.T, repo *repoConCuenta, tokens *tokensEspia, correos *correoEspia) *AuthService {
	t.Helper()
	crypto, err := security.NewCryptoService("12345678901234567890123456789012")
	if err != nil {
		t.Fatalf("crypto: %v", err)
	}
	return &AuthService{
		repo:         repo,
		tokenRepo:    tokens,
		verifyRepo:   &repoTokensVerificacion{},
		emailService: correos,
		crypto:       crypto,
		jwtSecret:    []byte("secreto-de-prueba"),
		resetURL:     "https://legacy.intelyclick.com/reset-password",
	}
}

func TestRequestPasswordReset_ElEnlaceNoLlevaElCorreo(t *testing.T) {
	repo := &repoConCuenta{}
	tokens := &tokensEspia{}
	correos := &correoEspia{}
	s := servicioConEspias(t, repo, tokens, correos)

	if err := s.RequestPasswordReset(context.Background(), "alguien@prueba.test"); err != nil {
		t.Fatalf("pedir el reinicio falló: %v", err)
	}

	if correos.enlace == "" {
		t.Fatal("no se mandó ningún enlace")
	}

	enlace, err := url.Parse(correos.enlace)
	if err != nil {
		t.Fatalf("el enlace no es una URL válida: %v", err)
	}

	if enlace.Query().Get("email") != "" {
		t.Errorf("el correo sigue viajando en la URL: %s", correos.enlace)
	}
	if enlace.Query().Get("token") == "" {
		t.Errorf("el enlace se quedó sin token: %s", correos.enlace)
	}
	// Ni disfrazado en otra parte de la URL.
	if strings.Contains(correos.enlace, "alguien@prueba.test") {
		t.Errorf("la dirección aparece en el enlace: %s", correos.enlace)
	}
	if strings.Contains(correos.enlace, "alguien%40prueba.test") {
		t.Errorf("la dirección aparece escapada en el enlace: %s", correos.enlace)
	}
}

func TestResetPassword_ResuelveElCorreoDesdeElToken(t *testing.T) {
	repo := &repoConCuenta{}
	tokens := &tokensEspia{}
	correos := &correoEspia{}
	s := servicioConEspias(t, repo, tokens, correos)

	if err := s.RequestPasswordReset(context.Background(), "duena@prueba.test"); err != nil {
		t.Fatalf("pedir el reinicio falló: %v", err)
	}

	if err := s.ResetPassword(context.Background(), tokens.tokenGuardado, "NuevaQa456"); err != nil {
		t.Fatalf("restablecer falló: %v", err)
	}

	if tokens.tokenPreguntado != tokens.tokenGuardado {
		t.Error("no se resolvió el correo a partir del token")
	}
	if repo.guardado == "" {
		t.Error("no se guardó la contraseña nueva")
	}
	// El token se consume contra el correo que salió de la propia tabla, no
	// contra uno recibido de fuera.
	if tokens.borradoPara != "duena@prueba.test" {
		t.Errorf("el token se consumió para %q", tokens.borradoPara)
	}
}

func TestResetPassword_UnTokenQueNoExisteNoCambiaNada(t *testing.T) {
	repo := &repoConCuenta{}
	tokens := &tokensEspia{}
	correos := &correoEspia{}
	s := servicioConEspias(t, repo, tokens, correos)

	if err := s.RequestPasswordReset(context.Background(), "duena@prueba.test"); err != nil {
		t.Fatalf("pedir el reinicio falló: %v", err)
	}

	err := s.ResetPassword(context.Background(), "token-inventado", "NuevaQa456")
	if err == nil {
		t.Fatal("un token inventado cambió la contraseña")
	}
	if repo.guardado != "" {
		t.Error("se guardó una contraseña con un token que no existe")
	}
	if tokens.borradoPara != "" {
		t.Error("se consumió un token que no era válido")
	}
}

func TestResetPassword_ElTokenVacioSeRechaza(t *testing.T) {
	repo := &repoConCuenta{}
	tokens := &tokensEspia{}
	s := servicioConEspias(t, repo, tokens, &correoEspia{})

	// Sin esta guarda, un cuerpo sin token consultaría la tabla con la cadena
	// vacía y dependería de que no hubiera ninguna fila así.
	err := s.ResetPassword(context.Background(), "", "NuevaQa456")
	if err == nil {
		t.Fatal("se aceptó un token vacío")
	}
	if repo.guardado != "" {
		t.Error("se guardó una contraseña sin token")
	}
}

// La contraseña se sigue validando antes de tocar el token, como ya cubría
// contrasena_minima_test.go: aquí se comprueba que ese orden aguanta ahora que
// el correo se resuelve desde la tabla.
func TestResetPassword_LaContrasenaCortaNiSiquieraConsultaElToken(t *testing.T) {
	repo := &repoConCuenta{}
	tokens := &tokensEspia{}
	s := servicioConEspias(t, repo, tokens, &correoEspia{})

	err := s.ResetPassword(context.Background(), "da-igual", "ab123")

	if !errors.Is(err, domain.ErrContrasenaCorta) {
		t.Fatalf("se esperaba ErrContrasenaCorta y llegó: %v", err)
	}
	if tokens.tokenPreguntado != "" {
		t.Error("se consultó el token antes de validar la contraseña")
	}
}

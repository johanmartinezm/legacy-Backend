package services

import (
	"context"
	"errors"
	"testing"

	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
)

// correoMudo existe solo para que Register no llame a un `nil`: el registro
// social lanza SendWelcomeEmail en una goroutine y con la interfaz vacía el
// proceso de pruebas se caía entero.
type correoMudo struct{}

func (correoMudo) SendResetPasswordEmail(to, resetURL string) error            { return nil }
func (correoMudo) SendBoardContactEmail(to, n, e, m string) error              { return nil }
func (correoMudo) SendAsesoriaEmail(to, n, e, cat, m string) error             { return nil }
func (correoMudo) SendContactoEmail(to, a, n, e, m string) error               { return nil }
func (correoMudo) SendWelcomeEmail(to, userName string) error                  { return nil }
func (correoMudo) SendVerificationEmail(to, link string) error                 { return nil }
func (correoMudo) SendEventRegistrationEmail(d domain.CorreoInscripcion) error { return nil }
func (correoMudo) SendEventPaymentEmail(d domain.CorreoPago) error             { return nil }
func (correoMudo) SendEventCredentialEmail(d domain.CorreoCredencial) error    { return nil }

// repoQueGuarda es repoSocial con un Create que sí retiene lo que se crea: es
// justo el valor guardado lo que estas pruebas miran.
type repoQueGuarda struct {
	*repoSocial
	creado *domain.User
}

func (r *repoQueGuarda) Create(ctx context.Context, u *domain.User) error {
	u.ID = "usuario-nuevo"
	r.creado = u
	return nil
}

func servicioDeRegistro(t *testing.T, validador ports.ValidadorDeApple) (*AuthService, *repoQueGuarda) {
	t.Helper()
	repo := &repoQueGuarda{repoSocial: &repoSocial{
		porSocial: map[string]*domain.User{},
		porCorreo: map[string]*domain.User{},
	}}
	svc, _ := servicioConApple(t, nil, validador)
	svc.repo = repo
	svc.emailService = correoMudo{}
	return svc, repo
}

func usuarioDeApple(token string) *domain.User {
	t := token
	return &domain.User{
		Email:     "socio@ejemplo.com",
		FirstName: "Socio",
		LastName:  "De Prueba",
		Role:      domain.RoleDefault,
		AppleID:   &t,
	}
}

// El bug del rechazo 2.1(a): la app manda el identityToken entero como
// `apple_id` y se guardaba tal cual, así que nunca coincidía con el `sub`.
func TestRegistroGuardaElSujetoDeAppleNoElToken(t *testing.T) {
	validador := &validadorFalso{identidad: &ports.IdentidadApple{
		Sujeto: "000123.abcdef.0001",
		Correo: "socio@ejemplo.com",
	}}
	svc, repo := servicioDeRegistro(t, validador)

	if err := svc.Register(context.Background(), usuarioDeApple("eyJhbGciOiJSUzI1NiJ9.cargaUtil.firma"), ""); err != nil {
		t.Fatalf("el registro con Apple debía salir bien: %v", err)
	}

	if repo.creado == nil || repo.creado.AppleID == nil {
		t.Fatal("no se guardó ninguna identidad de Apple")
	}
	if *repo.creado.AppleID != "000123.abcdef.0001" {
		t.Fatalf("apple_id guardado = %q; se esperaba el sub del token", *repo.creado.AppleID)
	}
	if validador.recibido != "eyJhbGciOiJSUzI1NiJ9.cargaUtil.firma" {
		t.Fatalf("el validador recibió %q, no el token del registro", validador.recibido)
	}
}

// La consecuencia que vio el revisor de Apple: volver a entrar. Desde la
// SEGUNDA autorización, el token de Apple llega sin correo, así que si el
// `apple_id` guardado no es el `sub` no queda absolutamente nada con lo que
// reconocer la cuenta.
func TestVolverAEntrarConAppleSinCorreoEncuentraLaCuenta(t *testing.T) {
	validador := &validadorFalso{identidad: &ports.IdentidadApple{
		Sujeto: "000123.abcdef.0001",
		Correo: "socio@ejemplo.com",
	}}
	svc, repo := servicioDeRegistro(t, validador)

	if err := svc.Register(context.Background(), usuarioDeApple("eyJhbGciOiJSUzI1NiJ9.cargaUtil.firma"), ""); err != nil {
		t.Fatalf("el registro con Apple debía salir bien: %v", err)
	}

	// La base indexa por lo que se guardó, sea lo que sea.
	repo.porSocial["apple:"+*repo.creado.AppleID] = repo.creado

	// Segundo inicio de sesión: mismo `sub`, y esta vez sin correo.
	validador.identidad = &ports.IdentidadApple{Sujeto: "000123.abcdef.0001"}

	token, _, err := svc.SocialLogin(context.Background(), "apple", "otro.token.de.apple")
	if err != nil {
		t.Fatalf("volver a entrar con Apple falló: %v", err)
	}
	if token == "" {
		t.Fatal("no se emitió ningún token de sesión")
	}
}

// `apple_id` llegaba sin comprobarse: bastaba con declarar el `sub` de otra
// persona en el formulario de registro para quedarse con su acceso social.
func TestRegistroRechazaIdentidadDeAppleNoVerificable(t *testing.T) {
	validador := &validadorFalso{err: errors.New("firma inválida")}
	svc, repo := servicioDeRegistro(t, validador)

	err := svc.Register(context.Background(), usuarioDeApple("000123.abcdef.0001"), "")

	if !errors.Is(err, domain.ErrIdentidadSocialInvalida) {
		t.Fatalf("error = %v; se esperaba ErrIdentidadSocialInvalida", err)
	}
	if repo.creado != nil {
		t.Fatal("no se puede crear la cuenta con una identidad social sin verificar")
	}
}

package services

import (
	"context"
	"errors"
	"testing"

	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"applegacy/backend/internal/security"
)

// validadorFalso evita llamar a Apple en las pruebas.
type validadorFalso struct {
	identidad *ports.IdentidadApple
	err       error
	recibido  string
}

func (v *validadorFalso) Validar(ctx context.Context, idToken string) (*ports.IdentidadApple, error) {
	v.recibido = idToken
	if v.err != nil {
		return nil, v.err
	}
	return v.identidad, nil
}

// repoSocial recuerda a quién se enlazó con qué.
type repoSocial struct {
	repoUsuarios
	porSocial  map[string]*domain.User
	porCorreo  map[string]*domain.User
	enlazados  []string
	errEnlazar error
}

func (r *repoSocial) FindBySocialID(ctx context.Context, provider, socialID string) (*domain.User, error) {
	if u, ok := r.porSocial[provider+":"+socialID]; ok {
		return u, nil
	}
	return nil, errors.New("user not found")
}

func (r *repoSocial) FindByEmailBlindIndex(ctx context.Context, blindIndex string) (*domain.User, error) {
	if u, ok := r.porCorreo[blindIndex]; ok {
		return u, nil
	}
	return nil, errors.New("user not found")
}

func (r *repoSocial) LinkSocialID(ctx context.Context, userID, provider, socialID string) error {
	if r.errEnlazar != nil {
		return r.errEnlazar
	}
	r.enlazados = append(r.enlazados, userID+":"+provider+":"+socialID)
	return nil
}

func servicioConApple(t *testing.T, repo *repoSocial, validador ports.ValidadorDeApple) (*AuthService, *security.CryptoService) {
	t.Helper()
	crypto, err := security.NewCryptoService("12345678901234567890123456789012")
	if err != nil {
		t.Fatalf("no se pudo crear el cifrador: %v", err)
	}
	return NewAuthService(repo, nil, nil, nil, nil, crypto, "secreto", "", "", "", validador), crypto
}

// Lo que motivó todo esto: el backend no miraba el token y devolvía siempre el
// mismo correo ficticio, así que cualquiera podía entrar con cualquier cadena.
func TestAppleRechazaTokenInvalido(t *testing.T) {
	repo := &repoSocial{porSocial: map[string]*domain.User{}, porCorreo: map[string]*domain.User{}}
	validador := &validadorFalso{err: errors.New("token de Apple inválido")}
	svc, _ := servicioConApple(t, repo, validador)

	_, _, err := svc.SocialLogin(context.Background(), "apple", "token-inventado")

	if err == nil {
		t.Fatal("un token que el validador rechaza NO puede iniciar sesión")
	}
	if validador.recibido != "token-inventado" {
		t.Errorf("el token debe llegar al validador, y llegó %q", validador.recibido)
	}
}

func TestAppleSinValidadorNoDejaEntrar(t *testing.T) {
	repo := &repoSocial{porSocial: map[string]*domain.User{}, porCorreo: map[string]*domain.User{}}
	svc, _ := servicioConApple(t, repo, nil)

	// Falta configuración: antes que dejar pasar a cualquiera, no se pasa.
	if _, _, err := svc.SocialLogin(context.Background(), "apple", "lo-que-sea"); err == nil {
		t.Fatal("sin validador configurado no puede iniciarse sesión con Apple")
	}
}

func TestAppleEntraPorIdentidadYNoPorCorreo(t *testing.T) {
	usuario := &domain.User{ID: "u-1", Role: "user"}
	repo := &repoSocial{
		porSocial: map[string]*domain.User{"apple:001.abc": usuario},
		porCorreo: map[string]*domain.User{},
	}
	// Sin correo, que es lo que pasa a partir del segundo inicio de sesión.
	validador := &validadorFalso{identidad: &ports.IdentidadApple{Sujeto: "001.abc"}}
	svc, _ := servicioConApple(t, repo, validador)

	token, u, err := svc.SocialLogin(context.Background(), "apple", "token-bueno")
	if err != nil {
		t.Fatalf("debía entrar por su identidad de Apple: %v", err)
	}
	if token == "" || u == nil || u.ID != "u-1" {
		t.Errorf("sesión incorrecta: token vacío=%t, usuario=%v", token == "", u)
	}
}

func TestAppleEnlazaLaPrimeraVez(t *testing.T) {
	crypto, _ := security.NewCryptoService("12345678901234567890123456789012")
	indice := crypto.BlindIndex("persona@ejemplo.test")

	usuario := &domain.User{ID: "u-7", Role: "user"}
	repo := &repoSocial{
		porSocial: map[string]*domain.User{},
		porCorreo: map[string]*domain.User{indice: usuario},
	}
	validador := &validadorFalso{identidad: &ports.IdentidadApple{
		Sujeto: "001.nuevo",
		Correo: "persona@ejemplo.test",
	}}
	svc, _ := servicioConApple(t, repo, validador)

	if _, _, err := svc.SocialLogin(context.Background(), "apple", "token-bueno"); err != nil {
		t.Fatalf("quien ya tenía cuenta debe poder entrar con Apple: %v", err)
	}

	// Sin este enlace, la próxima vez —cuando Apple ya no mande el correo— no
	// habría forma de reconocerla.
	if len(repo.enlazados) != 1 || repo.enlazados[0] != "u-7:apple:001.nuevo" {
		t.Errorf("debía quedar enlazada la identidad, y quedó %v", repo.enlazados)
	}
}

func TestAppleSinCuentaMandaARegistro(t *testing.T) {
	repo := &repoSocial{porSocial: map[string]*domain.User{}, porCorreo: map[string]*domain.User{}}
	validador := &validadorFalso{identidad: &ports.IdentidadApple{
		Sujeto: "001.desconocido",
		Correo: "nueva@ejemplo.test",
	}}
	svc, _ := servicioConApple(t, repo, validador)

	token, u, err := svc.SocialLogin(context.Background(), "apple", "token-bueno")

	if err == nil || token != "" {
		t.Fatal("quien no tiene cuenta no recibe sesión")
	}
	// El correo real viaja de vuelta para prellenar el registro. Antes aquí
	// llegaba siempre user_apple@example.com.
	if u == nil || u.Email != "nueva@ejemplo.test" {
		t.Errorf("debía devolver el correo del token para el registro, y devolvió %v", u)
	}
}

func TestFalloAlEnlazarNoImpideEntrar(t *testing.T) {
	usuario := &domain.User{ID: "u-9", Role: "user"}
	repo := &repoSocial{
		porSocial:  map[string]*domain.User{"apple:001.xyz": usuario},
		porCorreo:  map[string]*domain.User{},
		errEnlazar: errors.New("la base no responde"),
	}
	validador := &validadorFalso{identidad: &ports.IdentidadApple{Sujeto: "001.xyz"}}
	svc, _ := servicioConApple(t, repo, validador)

	// La persona ya está identificada: un fallo al dejar constancia no debe
	// dejarla fuera.
	if _, _, err := svc.SocialLogin(context.Background(), "apple", "token-bueno"); err != nil {
		t.Fatalf("un fallo al enlazar no debe impedir el inicio de sesión: %v", err)
	}
}

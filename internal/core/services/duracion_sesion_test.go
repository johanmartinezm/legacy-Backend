package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

// 🔴 La propiedad que hay que preservar: el token de la app dura 30 días y el
// del panel 24 horas.
//
// Si el de la app vuelve a 24 horas, «Recordarme» deja de recordar al día
// siguiente sin que nada falle a la vista. Si el del panel se alarga por
// arrastre —eran el mismo campo hasta el 2026-11-13—, una cuenta que puede
// borrar contenido queda abierta un mes.

func vencimiento(t *testing.T, token string) time.Duration {
	t.Helper()
	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (interface{}, error) {
		return []byte("secreto-de-prueba"), nil
	}); err != nil {
		t.Fatalf("token ilegible: %v", err)
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		t.Fatal("token sin exp")
	}
	return time.Until(time.Unix(int64(exp), 0))
}

func TestLogin_LaSesionDeLaAppDuraTreintaDias(t *testing.T) {
	s := servicioLogin(t, usuarioCon(t, "LaBuena123", true))

	token, err := s.Login(context.Background(), "quien@sea.test", "LaBuena123")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if d := vencimiento(t, token); d < DuracionSesionApp-time.Minute || d > DuracionSesionApp {
		t.Errorf("vence en %v, se esperaba %v", d, DuracionSesionApp)
	}
}

type adminsDePrueba struct {
	ports.AdminUserRepository
	admin *domain.AdminUser
}

func (r *adminsDePrueba) FindByEmail(ctx context.Context, email string) (*domain.AdminUser, error) {
	return r.admin, nil
}

func TestAdminLogin_LaSesionDelPanelDuraUnDia(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("LaBuena123"), bcrypt.MinCost)
	s := &AuthService{
		adminRepo: &adminsDePrueba{admin: &domain.AdminUser{ID: "admin-1", PasswordHash: string(hash), Role: "admin"}},
		jwtSecret: []byte("secreto-de-prueba"),
	}

	token, err := s.AdminLogin(context.Background(), "admin@sea.test", "LaBuena123")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if d := vencimiento(t, token); d < DuracionSesionPanel-time.Minute || d > DuracionSesionPanel {
		t.Errorf("vence en %v, se esperaba %v", d, DuracionSesionPanel)
	}
}

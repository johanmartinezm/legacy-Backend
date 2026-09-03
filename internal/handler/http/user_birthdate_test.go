package http

import (
	"applegacy/backend/internal/core/domain"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// stubAuthServiceEdicion solo implementa lo que performUpdate necesita:
// GetProfile para cargar el usuario base y UpdateUser para capturar lo que le
// llega. El resto del contrato de ports.AuthService no lo toca este handler.
type stubAuthServiceEdicion struct {
	usuario   *domain.User
	recibido  *domain.User
	errUpdate error
}

func (s *stubAuthServiceEdicion) Register(ctx context.Context, user *domain.User, password string) error {
	return nil
}
func (s *stubAuthServiceEdicion) RegistrarImportado(ctx context.Context, user *domain.User, password string) error {
	return nil
}
func (s *stubAuthServiceEdicion) IDDeCuentaConCorreo(ctx context.Context, email string) (string, error) {
	return "", nil
}
func (s *stubAuthServiceEdicion) Login(ctx context.Context, email, password string) (string, error) {
	return "", nil
}
func (s *stubAuthServiceEdicion) SocialLogin(ctx context.Context, provider, idToken string) (string, *domain.User, error) {
	return "", nil, nil
}
func (s *stubAuthServiceEdicion) RegisterAdmin(ctx context.Context, admin *domain.AdminUser, password string) error {
	return nil
}
func (s *stubAuthServiceEdicion) AdminLogin(ctx context.Context, email, password string) (string, error) {
	return "", nil
}
func (s *stubAuthServiceEdicion) ListAdmins(ctx context.Context) ([]*domain.AdminUser, error) {
	return nil, nil
}
func (s *stubAuthServiceEdicion) UpdateAdmin(ctx context.Context, admin *domain.AdminUser) error {
	return nil
}
func (s *stubAuthServiceEdicion) DeleteAdmin(ctx context.Context, id string) error { return nil }
func (s *stubAuthServiceEdicion) ListUsers(ctx context.Context, limit, offset int) ([]*domain.User, int, error) {
	return nil, 0, nil
}
func (s *stubAuthServiceEdicion) UpdateUser(ctx context.Context, user *domain.User) error {
	s.recibido = user
	return s.errUpdate
}
func (s *stubAuthServiceEdicion) DeleteUser(ctx context.Context, id string) error { return nil }
func (s *stubAuthServiceEdicion) GetProfile(ctx context.Context, id string) (*domain.User, error) {
	return s.usuario, nil
}
func (s *stubAuthServiceEdicion) DeleteMyAccount(ctx context.Context, userID string) error {
	return nil
}
func (s *stubAuthServiceEdicion) ChangePassword(ctx context.Context, id string, oldPassword, newPassword string) error {
	return nil
}
func (s *stubAuthServiceEdicion) RequestPasswordReset(ctx context.Context, email string) error {
	return nil
}
func (s *stubAuthServiceEdicion) ResetPassword(ctx context.Context, token, newPassword string) error {
	return nil
}
func (s *stubAuthServiceEdicion) VerifyEmail(ctx context.Context, token string) error { return nil }
func (s *stubAuthServiceEdicion) ResendVerificationEmail(ctx context.Context, email string) error {
	return nil
}

// El panel manda birth_date como fecha sola ("2006-01-02"), no como RFC3339:
// hasta este fix, decodificar el body directo sobre *time.Time hacía fallar
// el Decode entero con 400, y como el body también trae name/role válidos el
// panel no tenía forma de saber qué falló. Ver reports/20260820_ruta_pruebas_manuales.html.
func TestPerformUpdate_AceptaFechaSolaDelPanel(t *testing.T) {
	stub := &stubAuthServiceEdicion{
		usuario: &domain.User{ID: "u1", Role: "junta", FirstName: "QA"},
	}
	h := NewUserHandler(stub)

	body, _ := json.Marshal(map[string]any{
		"first_name": "QA",
		"role":       "junta",
		"birth_date": "1990-01-15",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/users/u1", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.performUpdate(rec, req, "u1")

	if rec.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d (%s)", rec.Code, rec.Body.String())
	}
	if stub.recibido == nil || stub.recibido.BirthDate == nil {
		t.Fatal("la fecha de nacimiento no llegó a UpdateUser")
	}
	esperado := time.Date(1990, 1, 15, 0, 0, 0, 0, time.UTC)
	if !stub.recibido.BirthDate.Equal(esperado) {
		t.Errorf("fecha esperada %v, llegó %v", esperado, *stub.recibido.BirthDate)
	}
}

// El registro desde la app sigue mandando RFC3339 en otros flujos y el propio
// registro manda DD/MM/YYYY: performUpdate debe seguir aceptando ambos.
func TestPerformUpdate_AceptaRFC3339YFormatoDDMMYYYY(t *testing.T) {
	casos := []struct {
		nombre string
		valor  string
	}{
		{"RFC3339", "1990-01-15T00:00:00Z"},
		{"DD/MM/YYYY", "15/01/1990"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			stub := &stubAuthServiceEdicion{usuario: &domain.User{ID: "u1", Role: "familia"}}
			h := NewUserHandler(stub)

			body, _ := json.Marshal(map[string]any{"role": "familia", "birth_date": c.valor})
			req := httptest.NewRequest(http.MethodPut, "/api/users/u1", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			h.performUpdate(rec, req, "u1")

			if rec.Code != http.StatusOK {
				t.Fatalf("se esperaba 200, se obtuvo %d (%s)", rec.Code, rec.Body.String())
			}
			if stub.recibido == nil || stub.recibido.BirthDate == nil {
				t.Fatal("la fecha de nacimiento no llegó a UpdateUser")
			}
		})
	}
}

// Guardar sin tocar el rol -F20.5- no debe borrar la fecha que ya tenía la
// cuenta: si el cliente no manda birth_date, se conserva la que cargó GetProfile.
func TestPerformUpdate_SinBirthDateConservaElQueYaTenia(t *testing.T) {
	previa := time.Date(1985, 6, 1, 0, 0, 0, 0, time.UTC)
	stub := &stubAuthServiceEdicion{
		usuario: &domain.User{ID: "u1", Role: "junta", BirthDate: &previa},
	}
	h := NewUserHandler(stub)

	body, _ := json.Marshal(map[string]any{"role": "junta"})
	req := httptest.NewRequest(http.MethodPut, "/api/users/u1", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.performUpdate(rec, req, "u1")

	if rec.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d (%s)", rec.Code, rec.Body.String())
	}
	if stub.recibido == nil || stub.recibido.BirthDate == nil || !stub.recibido.BirthDate.Equal(previa) {
		t.Errorf("la fecha previa no se conservó: %+v", stub.recibido)
	}
}

func TestPerformUpdate_FechaDeNacimientoInvalidaDevuelve400(t *testing.T) {
	stub := &stubAuthServiceEdicion{usuario: &domain.User{ID: "u1", Role: "familia"}}
	h := NewUserHandler(stub)

	body, _ := json.Marshal(map[string]any{"role": "familia", "birth_date": "no es una fecha"})
	req := httptest.NewRequest(http.MethodPut, "/api/users/u1", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.performUpdate(rec, req, "u1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("se esperaba 400, se obtuvo %d (%s)", rec.Code, rec.Body.String())
	}
}

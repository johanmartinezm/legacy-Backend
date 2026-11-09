package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"applegacy/backend/internal/core/domain"

	"github.com/golang-jwt/jwt/v4"
)

// La cuenta de rol "eventos" (2026-11-09) solo ve eventos e inscritos y valida
// asistencia. Lo que se fija aqui es la frontera: StaffEventos la deja pasar,
// AdminOnly no, y un administrador sigue pasando por los dos.

var secretoDePrueba = []byte("secreto-de-prueba")

func tokenConRol(t *testing.T, rol string) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  "cuenta-1",
		"exp":  time.Now().Add(time.Hour).Unix(),
		"role": rol,
	}).SignedString(secretoDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func codigoTras(t *testing.T, mw func(http.Handler) http.Handler, rol string) int {
	t.Helper()
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenConRol(t, rol))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestRolEventos_FronteraDeLosMiddlewares(t *testing.T) {
	casos := []struct {
		mw     string
		rol    string
		espera int
	}{
		{"AdminOnly", "admin", http.StatusOK},
		{"AdminOnly", "eventos", http.StatusForbidden},
		{"AdminOnly", "familia", http.StatusForbidden},
		{"StaffEventos", "admin", http.StatusOK},
		{"StaffEventos", "eventos", http.StatusOK},
		{"StaffEventos", "familia", http.StatusForbidden},
		{"StaffEventos", "", http.StatusForbidden},
	}
	for _, c := range casos {
		mw := AdminOnly(secretoDePrueba)
		if c.mw == "StaffEventos" {
			mw = StaffEventos(secretoDePrueba)
		}
		if got := codigoTras(t, mw, c.rol); got != c.espera {
			t.Errorf("%s con rol %q: llego %d, se esperaba %d", c.mw, c.rol, got, c.espera)
		}
	}
}

func TestRolEventos_VeLosEventosOcultosComoElPanel(t *testing.T) {
	if !pedirListado(t, RoleEventos).incluirInactivosRecibido {
		t.Error("quien valida asistencia no ve los eventos ocultos, y puede tener que validar en uno")
	}
}

func TestRolEventos_ElTokenSoloCambiaParaEventos(t *testing.T) {
	casos := map[string]string{
		"eventos":    "eventos",
		"admin":      "admin",
		"superadmin": "admin", // nunca tuvo mas permisos: no puede perder ninguno
		"":           "admin",
		"otro-viejo": "admin",
	}
	for columna, claim := range casos {
		if got := domain.RolDelToken(columna); got != claim {
			t.Errorf("rol %q firma %q, se esperaba %q", columna, got, claim)
		}
	}
}

func TestRolEventos_SoloSeGuardanRolesConocidos(t *testing.T) {
	if r, err := domain.NormalizarRolAdmin(""); err != nil || r != "admin" {
		t.Errorf("vacio deberia valer por admin: %q, %v", r, err)
	}
	for _, r := range []string{"admin", "superadmin", "eventos"} {
		if got, err := domain.NormalizarRolAdmin(r); err != nil || got != r {
			t.Errorf("%q deberia aceptarse: %q, %v", r, got, err)
		}
	}
	if _, err := domain.NormalizarRolAdmin("root"); err == nil {
		t.Error("un rol desconocido se acepto")
	}
}

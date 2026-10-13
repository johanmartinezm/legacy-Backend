package http

import (
	"applegacy/backend/internal/core/domain"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// --- Autoedición: PUT /api/me no puede cambiar es_plus, customer_status ni role.
// Ver reports/20261013_plan_usuario_plus.md (repositorio raíz).

func TestPerformUpdate_ElUsuarioNoSePoneNiPlusNiClienteNiRol(t *testing.T) {
	stub := &stubAuthServiceEdicion{usuario: &domain.User{
		ID: "u1", Role: "familia", CustomerStatus: "Quiero ser cliente", EsPlus: false,
	}}
	h := NewUserHandler(stub)

	body, _ := json.Marshal(map[string]any{
		"first_name":      "Ana",
		"es_plus":         true,
		"customer_status": "Ya soy cliente",
		"role":            "junta",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/me", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.performUpdate(rec, req, "u1", true)

	if rec.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d (%s)", rec.Code, rec.Body.String())
	}
	g := stub.recibido
	if g.EsPlus || g.CustomerStatus != "Quiero ser cliente" || g.Role != "familia" {
		t.Fatalf("el usuario se cambió a sí mismo el acceso: plus=%v estado=%q rol=%q", g.EsPlus, g.CustomerStatus, g.Role)
	}
	if g.FirstName != "Ana" {
		t.Error("el resto del perfil sí debía guardarse")
	}
}

func TestPerformUpdate_ElPanelCambiaClienteYRolPeroNoPlus(t *testing.T) {
	stub := &stubAuthServiceEdicion{usuario: &domain.User{
		ID: "u1", Role: "familia", CustomerStatus: "Quiero ser cliente", EsPlus: true,
	}}
	h := NewUserHandler(stub)

	body, _ := json.Marshal(map[string]any{
		"es_plus":         false,
		"customer_status": "Ya soy cliente",
		"role":            "junta",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/users/u1", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.performUpdate(rec, req, "u1", false)

	g := stub.recibido
	if g.CustomerStatus != "Ya soy cliente" || g.Role != "junta" {
		t.Fatalf("el panel no pudo cambiar estado y rol: %q %q", g.CustomerStatus, g.Role)
	}
	if !g.EsPlus {
		t.Error("el plus solo se cambia con PUT /api/admin/users/{id}/plus, no con el PUT general")
	}
}

// --- RequierePlus

type plusFake struct {
	plus      map[string]bool
	err       error
	cambiados map[string]bool
}

func (p *plusFake) EsPlus(ctx context.Context, userID string) (bool, error) {
	return p.plus[userID], p.err
}

func (p *plusFake) CambiarPlus(ctx context.Context, userID string, esPlus bool) error {
	if userID == "no-existe" {
		return domain.ErrNotFound
	}
	if p.cambiados == nil {
		p.cambiados = map[string]bool{}
	}
	p.cambiados[userID] = esPlus
	return nil
}

func TestRequierePlus(t *testing.T) {
	svc := &plusFake{plus: map[string]bool{"plus": true, "normal": false}}
	protegido := RequierePlus(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	casos := []struct {
		nombre  string
		usuario string
		codigo  int
	}{
		{"sin sesión", "", http.StatusUnauthorized},
		{"sin plus", "normal", http.StatusForbidden},
		{"desconocido", "otro", http.StatusForbidden},
		{"con plus", "plus", http.StatusTeapot},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/forums", nil)
			if c.usuario != "" {
				req = req.WithContext(context.WithValue(req.Context(), UserIDKey, c.usuario))
			}
			rec := httptest.NewRecorder()
			protegido.ServeHTTP(rec, req)
			if rec.Code != c.codigo {
				t.Fatalf("código %d, se esperaba %d", rec.Code, c.codigo)
			}
		})
	}

	t.Run("si la base falla, 500 y no pasa", func(t *testing.T) {
		roto := RequierePlus(&plusFake{err: errors.New("db caída")})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("no debía pasar")
		}))
		req := httptest.NewRequest(http.MethodGet, "/api/forums", nil)
		req = req.WithContext(context.WithValue(req.Context(), UserIDKey, "plus"))
		rec := httptest.NewRecorder()
		roto.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("código %d", rec.Code)
		}
	})
}

// --- PUT /api/admin/users/{id}/plus

func TestAdminCambiarPlus(t *testing.T) {
	svc := &plusFake{}
	h := NewUsuarioPlusHandler(svc)
	r := chi.NewRouter()
	r.Put("/api/admin/users/{id}/plus", h.AdminCambiar)

	pedir := func(id, cuerpo string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/api/admin/users/"+id+"/plus", bytes.NewBufferString(cuerpo))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	if rec := pedir("u1", `{"es_plus": true}`); rec.Code != http.StatusOK || !svc.cambiados["u1"] {
		t.Fatalf("no lo marcó: %d %v", rec.Code, svc.cambiados)
	}
	if rec := pedir("u1", `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("sin es_plus debía ser 400, fue %d", rec.Code)
	}
	if rec := pedir("no-existe", `{"es_plus": false}`); rec.Code != http.StatusNotFound {
		t.Fatalf("usuario inexistente debía ser 404, fue %d", rec.Code)
	}
}

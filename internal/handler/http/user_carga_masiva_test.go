package http

import (
	"applegacy/backend/internal/core/domain"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// La bandera de "cambia tu contraseña" la pone la carga masiva y solo la baja
// ChangePassword. performUpdate vuelca el JSON entero sobre el usuario cargado,
// así que sin la exclusión bastaba con mandarla en false al editar el perfil
// para quitársela. Ver reports/20260826_plan_carga_masiva.md §2.5.
func TestPerformUpdate_NoDejaQuitarseLaObligacionDeCambiarContrasena(t *testing.T) {
	stub := &stubAuthServiceEdicion{
		usuario: &domain.User{
			ID:                    "u1",
			Role:                  "profesional",
			DebeCambiarContrasena: true,
		},
	}
	h := NewUserHandler(stub)

	body, _ := json.Marshal(map[string]any{
		"role":                    "profesional",
		"first_name":              "Importado",
		"debe_cambiar_contrasena": false,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/users/u1", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.performUpdate(rec, req, "u1")

	if rec.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d (%s)", rec.Code, rec.Body.String())
	}
	if stub.recibido == nil {
		t.Fatal("la actualización no llegó al servicio")
	}
	if !stub.recibido.DebeCambiarContrasena {
		t.Error("el PUT bajó debe_cambiar_contrasena; solo puede bajarla ChangePassword")
	}

	// Y la respuesta tampoco puede decir lo contrario, o la app se creería
	// liberada hasta el siguiente GET /api/me.
	var devuelto domain.User
	if err := json.Unmarshal(rec.Body.Bytes(), &devuelto); err != nil {
		t.Fatalf("respuesta ilegible: %v", err)
	}
	if !devuelto.DebeCambiarContrasena {
		t.Error("la respuesta devolvió la bandera en false")
	}
}

// Quien no la tenía puesta no la gana por editar su perfil.
func TestPerformUpdate_NoInventaLaObligacionDeCambiarContrasena(t *testing.T) {
	stub := &stubAuthServiceEdicion{
		usuario: &domain.User{ID: "u1", Role: "familia", DebeCambiarContrasena: false},
	}
	h := NewUserHandler(stub)

	body, _ := json.Marshal(map[string]any{
		"role":                    "familia",
		"debe_cambiar_contrasena": true,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/users/u1", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.performUpdate(rec, req, "u1")

	if stub.recibido == nil || stub.recibido.DebeCambiarContrasena {
		t.Error("el PUT puso debe_cambiar_contrasena en true")
	}
}

// Los tres campos nuevos sí se editan por el mismo PUT: el plan los quiere en
// el formulario de editar perfil, y el handler no necesita nada especial.
func TestPerformUpdate_AceptaSexoDepartamentoYDireccion(t *testing.T) {
	stub := &stubAuthServiceEdicion{
		usuario: &domain.User{ID: "u1", Role: "profesional"},
	}
	h := NewUserHandler(stub)

	body, _ := json.Marshal(map[string]any{
		"role":         "profesional",
		"sexo":         "Femenino",
		"departamento": "Antioquia",
		"direccion":    "Calle 10 # 43-21",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/users/u1", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.performUpdate(rec, req, "u1")

	if rec.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d (%s)", rec.Code, rec.Body.String())
	}
	if stub.recibido.Sexo != "Femenino" {
		t.Errorf("sexo no llegó: %q", stub.recibido.Sexo)
	}
	if stub.recibido.Departamento != "Antioquia" {
		t.Errorf("departamento no llegó: %q", stub.recibido.Departamento)
	}
	if stub.recibido.Direccion != "Calle 10 # 43-21" {
		t.Errorf("dirección no llegó: %q", stub.recibido.Direccion)
	}
}

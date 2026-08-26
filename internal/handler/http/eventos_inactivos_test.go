package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"applegacy/backend/internal/core/domain"
)

// La tabla events.events tiene una columna `status` desde el principio, y el
// listado publico la ignoraba: TODO evento salia en la app sin importar su
// estado. No habia forma de retirar uno sin borrarlo, asi que los eventos de
// verificacion interna se veian igual que los reales.
//
// La propiedad que se fija aqui: el listado publico pide solo los activos, y el
// panel —que consume este mismo endpoint y necesita verlos todos para poder
// reactivarlos— los pide completos.

type eventosSegunQuienPregunta struct {
	stubEventService
	incluirInactivosRecibido bool
	llamado                  bool
}

func (e *eventosSegunQuienPregunta) ListEvents(ctx context.Context, incluirInactivos bool) ([]domain.Event, error) {
	e.llamado = true
	e.incluirInactivosRecibido = incluirInactivos
	return []domain.Event{{ID: "1", Title: "Un evento"}}, nil
}

func pedirListado(t *testing.T, rol string) *eventosSegunQuienPregunta {
	t.Helper()
	svc := &eventosSegunQuienPregunta{}
	h := NewEventHandler(svc, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	if rol != "" {
		req = req.WithContext(context.WithValue(req.Context(), UserRoleKey, rol))
	}

	rec := httptest.NewRecorder()
	h.ListEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 y llego %d", rec.Code)
	}
	if !svc.llamado {
		t.Fatal("no se llego a consultar el listado")
	}
	return svc
}

func TestListado_SinSesionSoloEventosActivos(t *testing.T) {
	svc := pedirListado(t, "")
	if svc.incluirInactivosRecibido {
		t.Error("a quien no ha iniciado sesion se le estan ofreciendo eventos inactivos")
	}
}

func TestListado_UnUsuarioNormalSoloEventosActivos(t *testing.T) {
	// El rol de una persona es su tipo de cuenta, no "admin".
	for _, rol := range []string{"familia", "empresa", "profesional", "junta"} {
		svc := pedirListado(t, rol)
		if svc.incluirInactivosRecibido {
			t.Errorf("el rol %q recibe eventos inactivos", rol)
		}
	}
}

func TestListado_ElPanelLosVeTodos(t *testing.T) {
	// Sin esto, un evento marcado como inactivo desapareceria tambien del panel
	// y no habria forma de volver a activarlo desde ninguna pantalla.
	svc := pedirListado(t, RoleAdmin)
	if !svc.incluirInactivosRecibido {
		t.Error("el panel no esta recibiendo los eventos inactivos")
	}
}

func TestListado_DevuelveLoQueTraeElServicio(t *testing.T) {
	svc := &eventosSegunQuienPregunta{}
	h := NewEventHandler(svc, nil)
	rec := httptest.NewRecorder()
	h.ListEvents(rec, httptest.NewRequest(http.MethodGet, "/api/events", nil))

	var eventos []domain.Event
	if err := json.NewDecoder(rec.Body).Decode(&eventos); err != nil {
		t.Fatalf("la respuesta no es JSON valido: %v", err)
	}
	if len(eventos) != 1 || eventos[0].Title != "Un evento" {
		t.Errorf("la respuesta no trae el evento esperado: %+v", eventos)
	}
}

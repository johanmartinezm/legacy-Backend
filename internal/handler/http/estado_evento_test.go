package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"applegacy/backend/internal/core/domain"

	"github.com/go-chi/chi/v5"
)

// Hasta el 2026-08-26 no habia forma de reactivar un evento desde el panel. La
// columna `status` decide si se ve en la app, pero no estaba en el dominio, ni
// en el UPDATE del repositorio, ni en el formulario: mandar `"status":"active"`
// en el PUT del evento devolvia **200 sin cambiar nada**. Los tres eventos que
// se marcaron inactivos en produccion solo se podian recuperar por SQL.
//
// Las propiedades que se fijan aqui:
//  1. la ruta propia si cambia el estado, y solo acepta los dos conocidos;
//  2. el PUT del evento sigue **sin** tocar `status`, que es lo que impide que
//     un guardado normal del formulario —que no envia el campo— oculte el
//     evento de la app sin que nadie lo haya pedido.

type estadoRecibido struct {
	stubEventService
	id      string
	status  string
	llamado bool
	fallo   error
}

func (e *estadoRecibido) UpdateEventStatus(ctx context.Context, id, status string) error {
	e.llamado = true
	e.id = id
	e.status = status
	return e.fallo
}

func pedirCambioDeEstado(t *testing.T, svc *estadoRecibido, id, cuerpo string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewEventHandler(svc, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/events/"+id+"/status", bytes.NewBufferString(cuerpo))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.UpdateEventStatus(rec, req)
	return rec
}

func TestEstado_ReactivarUnEventoOculto(t *testing.T) {
	svc := &estadoRecibido{}
	rec := pedirCambioDeEstado(t, svc, "evt-1", `{"status":"active"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 y llego %d: %s", rec.Code, rec.Body.String())
	}
	if !svc.llamado {
		t.Fatal("no se llego a cambiar el estado: es el fallo que tenia el PUT del evento, que respondia 200 sin hacer nada")
	}
	if svc.id != "evt-1" || svc.status != domain.EventoActivo {
		t.Errorf("llego id=%q status=%q; se esperaba evt-1/active", svc.id, svc.status)
	}

	var cuerpo map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &cuerpo); err != nil {
		t.Fatalf("la respuesta no es JSON: %v", err)
	}
	if cuerpo["status"] != domain.EventoActivo {
		t.Errorf("la respuesta deberia confirmar el estado nuevo, y trae %q", cuerpo["status"])
	}
}

func TestEstado_OcultarUnEvento(t *testing.T) {
	svc := &estadoRecibido{}
	rec := pedirCambioDeEstado(t, svc, "evt-1", `{"status":"inactive"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 y llego %d", rec.Code)
	}
	if svc.status != domain.EventoInactivo {
		t.Errorf("llego %q en vez de inactive", svc.status)
	}
}

// Un estado escrito a mano no lo atrapa nadie mas abajo: no pasa el filtro
// `= 'active'` del listado, asi que el evento se queda oculto sin que ninguna
// pantalla muestre nada raro.
func TestEstado_UnValorDesconocidoSeRechaza(t *testing.T) {
	for _, valor := range []string{`"activo"`, `"ACTIVE"`, `""`, `"borrado"`} {
		svc := &estadoRecibido{fallo: domain.ErrEstadoDeEventoInvalido}
		rec := pedirCambioDeEstado(t, svc, "evt-1", `{"status":`+valor+`}`)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("con status=%s se esperaba 400 y llego %d", valor, rec.Code)
		}
	}
}

func TestEstado_EventoQueNoExiste(t *testing.T) {
	svc := &estadoRecibido{fallo: domain.ErrNotFound}
	rec := pedirCambioDeEstado(t, svc, "evt-fantasma", `{"status":"active"}`)

	if rec.Code != http.StatusNotFound {
		t.Errorf("se esperaba 404 sobre un evento que no existe y llego %d", rec.Code)
	}
}

func TestEstado_CuerpoQueNoEsJSON(t *testing.T) {
	svc := &estadoRecibido{}
	rec := pedirCambioDeEstado(t, svc, "evt-1", `no soy json`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("se esperaba 400 y llego %d", rec.Code)
	}
	if svc.llamado {
		t.Error("no deberia haberse tocado el estado con un cuerpo ilegible")
	}
}

// El formulario del panel no envia `status`. Si el PUT del evento lo escribiera,
// cada guardado normal lo dejaria vacio y el evento desapareceria de la app al
// editarlo. Por eso el estado viaja por su propia ruta y este PUT lo ignora.
type eventoActualizado struct {
	stubEventService
	recibido *domain.Event
}

func (e *eventoActualizado) UpdateEvent(ctx context.Context, ev *domain.Event) error {
	e.recibido = ev
	return nil
}

func TestGuardarElFormularioNoCambiaLaVisibilidad(t *testing.T) {
	svc := &eventoActualizado{}
	h := NewEventHandler(svc, nil)

	// Tal cual lo manda el panel: sin `status`, porque su modelo no lo tiene.
	cuerpo := `{"title":"Legacy Summit","category_id":"cat-1"}`
	req := httptest.NewRequest(http.MethodPut, "/api/events/evt-1", bytes.NewBufferString(cuerpo))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "evt-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.UpdateEvent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 y llego %d", rec.Code)
	}
	if svc.recibido == nil {
		t.Fatal("no se llego a guardar el evento")
	}
	if svc.recibido.Status != "" {
		t.Errorf("el PUT del formulario no deberia traer estado, y trae %q", svc.recibido.Status)
	}
}

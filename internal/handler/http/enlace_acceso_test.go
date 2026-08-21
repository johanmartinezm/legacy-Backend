package http

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// 🔴 La propiedad que hay que preservar: el enlace de una sesión virtual
// **equivale a poder entrar**, así que no sale por las rutas públicas.
//
// Hasta el 2026-08-20 `GET /api/events` y el detalle devolvían el evento entero
// —enlace incluido— sin pedir sesión: cualquiera podía sacar la URL de una
// masterclass de pago. La regla estaba escrita solo en GetMyRegistrations, que
// es el otro camino, y por eso F12.15 la dio por buena.
//
// El panel sí lo necesita: carga el valor actual para no borrarlo al editar.

// Se apoya en el stub que ya tiene el paquete y solo cambia los dos métodos que
// importan aquí: los demás siguen devolviendo lo de siempre.
type eventosDePrueba struct {
	*stubEventService
	evento domain.Event
}

func (e *eventosDePrueba) ListEvents(ctx context.Context) ([]domain.Event, error) {
	return []domain.Event{e.evento}, nil
}

func (e *eventosDePrueba) GetEventDetails(ctx context.Context, id string) (*domain.Event, error) {
	copia := e.evento
	return &copia, nil
}

func eventoVirtual() domain.Event {
	enlace := "https://meet.example.com/sesion-de-pago"
	return domain.Event{
		ID:        "evento-1",
		Title:     "Masterclass virtual de pago",
		IsVirtual: true,
		AccessURL: &enlace,
		StartDate: time.Now().Add(48 * time.Hour),
	}
}

func pedir(t *testing.T, ruta string, comoAdmin bool, servir func(http.ResponseWriter, *http.Request)) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, ruta, nil)
	if comoAdmin {
		req = req.WithContext(context.WithValue(req.Context(), UserRoleKey, RoleAdmin))
	}
	rec := httptest.NewRecorder()
	servir(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, llegó %d", rec.Code)
	}
	return rec.Body.String()
}

func TestListEvents_SinSesionNoEntregaElEnlaceDeAcceso(t *testing.T) {
	h := NewEventHandler(&eventosDePrueba{stubEventService: &stubEventService{}, evento: eventoVirtual()}, nil)

	cuerpo := pedir(t, "/api/events", false, h.ListEvents)

	if strings.Contains(cuerpo, "meet.example.com") {
		t.Fatalf("el enlace salió por una ruta pública: %s", cuerpo)
	}
	// El resto del evento sí tiene que salir: la lista es pública a propósito.
	if !strings.Contains(cuerpo, "Masterclass virtual de pago") {
		t.Error("el evento debe seguir apareciendo, solo sin el enlace")
	}
	var eventos []map[string]any
	if err := json.Unmarshal([]byte(cuerpo), &eventos); err != nil {
		t.Fatalf("respuesta ilegible: %v", err)
	}
	if eventos[0]["isVirtual"] != true {
		t.Error("que es virtual sí se puede decir; lo que no se da es la puerta")
	}
}

func TestGetEventDetails_SinSesionTampoco(t *testing.T) {
	h := NewEventHandler(&eventosDePrueba{stubEventService: &stubEventService{}, evento: eventoVirtual()}, nil)

	cuerpo := pedir(t, "/api/events/evento-1", false, h.GetEventDetails)

	if strings.Contains(cuerpo, "meet.example.com") {
		t.Fatalf("el enlace salió por el detalle público: %s", cuerpo)
	}
}

// El panel consume estas mismas rutas y necesita el valor actual: si no lo
// recibiera, editar un evento virtual borraría su enlace sin decir nada.
func TestListEvents_ElAdministradorSiRecibeElEnlace(t *testing.T) {
	h := NewEventHandler(&eventosDePrueba{stubEventService: &stubEventService{}, evento: eventoVirtual()}, nil)

	cuerpo := pedir(t, "/api/events", true, h.ListEvents)

	if !strings.Contains(cuerpo, "meet.example.com") {
		t.Fatalf("el panel se quedó sin el enlace al editar: %s", cuerpo)
	}
}

func TestGetEventDetails_ElAdministradorSiRecibeElEnlace(t *testing.T) {
	h := NewEventHandler(&eventosDePrueba{stubEventService: &stubEventService{}, evento: eventoVirtual()}, nil)

	cuerpo := pedir(t, "/api/events/evento-1", true, h.GetEventDetails)

	if !strings.Contains(cuerpo, "meet.example.com") {
		t.Fatalf("el panel se quedó sin el enlace al editar: %s", cuerpo)
	}
}

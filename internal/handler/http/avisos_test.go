package http

import (
	"applegacy/backend/internal/core/domain"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// notificadorFalso guarda lo enviado en vez de llamar a FCM.
type notificadorFalso struct {
	envios []envio
	err    error
}

type envio struct {
	adminID    string
	titulo     string
	cuerpo     string
	targetType string
	datos      map[string]string
}

func (n *notificadorFalso) RegisterToken(ctx context.Context, userID, token, deviceType string) error {
	return nil
}

func (n *notificadorFalso) GetHistory(ctx context.Context, limit, offset int) ([]*domain.NotificationHistory, error) {
	return nil, nil
}

func (n *notificadorFalso) SubscribeAllToTopic(ctx context.Context) (int, error) { return 0, nil }

func (n *notificadorFalso) SendNotification(ctx context.Context, adminID, title, body, targetType, targetValue string, data map[string]string) error {
	n.envios = append(n.envios, envio{adminID: adminID, titulo: title, cuerpo: body, targetType: targetType, datos: data})
	return n.err
}

// --- Eventos ---

func TestCrearEvento_AvisaALosUsuarios(t *testing.T) {
	notificador := &notificadorFalso{}
	svc := &stubEventService{}
	h := NewEventHandler(svc, notificador)

	descripcion := "Un dia completo de charlas"
	cuerpo, _ := json.Marshal(map[string]any{"title": "LEGACY SUMMIT", "description": descripcion})
	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(cuerpo))
	req = req.WithContext(context.WithValue(req.Context(), UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.CreateEvent(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("se esperaba 201, llegó %d", rec.Code)
	}
	if len(notificador.envios) != 1 {
		t.Fatalf("se esperaba 1 aviso, hubo %d", len(notificador.envios))
	}
	e := notificador.envios[0]
	if e.titulo != "Nuevo evento: LEGACY SUMMIT" {
		t.Errorf("título inesperado: %q", e.titulo)
	}
	if e.targetType != "all" {
		t.Errorf("el aviso va al topic all, llegó %q", e.targetType)
	}
	// El admin sale del token para que el historial diga quién publicó.
	if e.adminID != "admin-1" {
		t.Errorf("adminID esperado admin-1, llegó %q", e.adminID)
	}
	if e.datos["type"] != "event" {
		t.Errorf("los datos deben identificar el tipo, llegó %v", e.datos)
	}
}

func TestCrearEvento_UnFalloDelAvisoNoTumbaLaCreacion(t *testing.T) {
	// El aviso es un efecto secundario. Si FCM está caído —o corre en modo mock
	// por falta de credenciales, que es como está producción hoy—, el evento
	// tiene que crearse igual.
	notificador := &notificadorFalso{err: errors.New("FCM no disponible")}
	h := NewEventHandler(&stubEventService{}, notificador)

	cuerpo, _ := json.Marshal(map[string]any{"title": "LEGACY SUMMIT"})
	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(cuerpo))
	rec := httptest.NewRecorder()

	h.CreateEvent(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("el evento debe crearse aunque el aviso falle, llegó %d", rec.Code)
	}
}

func TestCrearEvento_SinNotificadorSigueFuncionando(t *testing.T) {
	h := NewEventHandler(&stubEventService{}, nil)

	cuerpo, _ := json.Marshal(map[string]any{"title": "LEGACY SUMMIT"})
	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(cuerpo))
	rec := httptest.NewRecorder()

	h.CreateEvent(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("se esperaba 201, llegó %d", rec.Code)
	}
}

// --- Recorte del cuerpo ---

func TestRecortar(t *testing.T) {
	casos := []struct {
		nombre  string
		entrada string
		maximo  int
		quiero  string
	}{
		{"corto se deja igual", "Hola", 10, "Hola"},
		{"se recorta por el espacio", "uno dos tres cuatro cinco", 12, "uno dos…"},
		{"sin espacios utiles corta seco", strings.Repeat("a", 20), 10, strings.Repeat("a", 10) + "…"},
		{"espacios sobrantes fuera", "   Hola   ", 10, "Hola"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := recortar(c.entrada, c.maximo); got != c.quiero {
				t.Errorf("recortar(%q,%d) = %q, se esperaba %q", c.entrada, c.maximo, got, c.quiero)
			}
		})
	}
}

func TestNotificarNovedad_SinTituloNoEnvia(t *testing.T) {
	// SendNotification rechaza un título vacío, y avisar de algo sin nombre no
	// le sirve a nadie: se corta antes y queda en el log.
	notificador := &notificadorFalso{}

	notificarNovedad(context.Background(), notificador, "admin-1", "   ", "cuerpo", nil)

	if len(notificador.envios) != 0 {
		t.Errorf("no debe enviarse nada sin título, hubo %d envíos", len(notificador.envios))
	}
}

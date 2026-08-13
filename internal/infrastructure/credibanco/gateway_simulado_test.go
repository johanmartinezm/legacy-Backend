package credibanco

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"applegacy/backend/internal/core/domain"
)

// Monta la pasarela de mentira con sus rutas, como hace main.go.
func servidorSimulado(t *testing.T) (*GatewaySimulado, *httptest.Server) {
	t.Helper()

	r := chi.NewRouter()
	g := NuevoGatewaySimulado("")
	g.RegistrarRutas(r)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// La URL de la pantalla tiene que apuntar al servidor de prueba.
	g.baseURL = srv.URL
	return g, srv
}

func TestSimuladoNaceEnPendiente(t *testing.T) {
	g, _ := servidorSimulado(t)

	orderID, formURL, err := g.CreatePaymentIntent(context.Background(), 250000, "tx-1", "legacyapp://app/payment-callback")
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if !strings.HasPrefix(orderID, "SIM-") {
		t.Errorf("el id debe delatar que es simulado, y es %q", orderID)
	}
	if !strings.Contains(formURL, orderID) {
		t.Errorf("la url de la pantalla debe llevar la orden: %q", formURL)
	}

	estado, err := g.GetPaymentStatus(context.Background(), orderID)
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if estado != domain.TxStatusPending {
		t.Errorf("una orden recién creada está pendiente, y estaba %s", estado)
	}
}

// El recorrido entero: crear, abrir la pantalla, decidir y volver por donde la
// app espera. Es justo lo que la pasarela real no deja probar.
func TestSimuladoRecorridoCompleto(t *testing.T) {
	casos := []struct {
		resultado string
		esperado  domain.TransactionStatus
	}{
		{"aprobado", domain.TxStatusApproved},
		{"rechazado", domain.TxStatusDeclined},
		{"pendiente", domain.TxStatusPending},
	}

	for _, c := range casos {
		t.Run(c.resultado, func(t *testing.T) {
			g, srv := servidorSimulado(t)
			retorno := "legacyapp://app/payment-callback?tx_id=abc"

			orderID, formURL, _ := g.CreatePaymentIntent(context.Background(), 1000, "tx-1", retorno)

			// La pantalla se puede abrir.
			resp, err := srv.Client().Get(formURL)
			if err != nil {
				t.Fatalf("no se pudo abrir la pantalla: %v", err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("la pantalla respondió %d", resp.StatusCode)
			}

			// Se decide, y debe devolver a la app sin seguir la redirección:
			// legacyapp:// no lo entiende un cliente HTTP, igual que en el
			// navegador del teléfono es el sistema quien la recoge.
			cliente := &http.Client{
				CheckRedirect: func(*http.Request, []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
			resp, err = cliente.Get(formURL + "/resolver?resultado=" + c.resultado)
			if err != nil {
				t.Fatalf("no se pudo resolver: %v", err)
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusFound {
				t.Errorf("debe redirigir con 302, y respondió %d", resp.StatusCode)
			}
			if destino := resp.Header.Get("Location"); destino != retorno {
				t.Errorf("debe volver a %q, y vuelve a %q", retorno, destino)
			}

			estado, _ := g.GetPaymentStatus(context.Background(), orderID)
			if estado != c.esperado {
				t.Errorf("tras %q el estado debe ser %s, y es %s", c.resultado, c.esperado, estado)
			}
		})
	}
}

// La salvaguarda que impide arrancar en modo simulado contra la pasarela real.
func TestSimuladoEsPeligroso(t *testing.T) {
	casos := []struct {
		url     string
		peligro bool
		porque  string
	}{
		{"https://eco.credibanco.com/payment/rest/", true, "es la pasarela real"},
		{"https://ecouat.credibanco.com/payment/rest/", false, "es UAT"},
		{"http://localhost:8080/", false, "es local"},
		{"http://127.0.0.1:8080/", false, "es local"},
		{"", false, "sin URL no hay a quién cobrar"},
		// Lo que salva la lista blanca: un dominio nuevo del banco seguiría
		// bloqueado aunque nadie actualice esta lista.
		{"https://pagos.credibanco.com/api/", true, "dominio desconocido del banco"},
		{"https://otra-pasarela.com/", true, "cualquier destino real"},
	}

	for _, c := range casos {
		if got := SimuladoEsPeligroso(c.url); got != c.peligro {
			t.Errorf("SimuladoEsPeligroso(%q) = %t, se esperaba %t porque %s",
				c.url, got, c.peligro, c.porque)
		}
	}
}

func TestSimuladoOrdenDesconocida(t *testing.T) {
	g, srv := servidorSimulado(t)

	if _, err := g.GetPaymentStatus(context.Background(), "SIM-inventada"); err == nil {
		t.Error("una orden que no existe debe dar error, como en la pasarela real")
	}

	resp, err := srv.Client().Get(srv.URL + "/api/payments/simulado/SIM-inventada")
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("la pantalla de una orden inventada debe dar 404, y dio %d", resp.StatusCode)
	}
}

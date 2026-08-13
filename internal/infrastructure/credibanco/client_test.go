package credibanco

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"applegacy/backend/internal/config"
	"applegacy/backend/internal/core/domain"
)

func TestCreatePaymentIntent(t *testing.T) {
	// Mock server for register.do
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/register.do" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"orderId": "test-order-123", "formUrl": "https://test.form.url", "errorCode": "0"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.CredibanCo.BaseURL = server.URL + "/"

	client := NewCredibancoClient(cfg)
	orderID, formUrl, err := client.CreatePaymentIntent(context.Background(), 1000.50, "txn-1", "https://return.url")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if orderID != "test-order-123" {
		t.Errorf("Expected orderId test-order-123, got %s", orderID)
	}
	if formUrl != "https://test.form.url" {
		t.Errorf("Expected formUrl https://test.form.url, got %s", formUrl)
	}
}

// Captura el formulario que se envía a register.do.
func formularioEnviado(t *testing.T, importe float64) url.Values {
	t.Helper()

	var recibido url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		recibido = r.PostForm
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"orderId": "o-1", "formUrl": "https://form", "errorCode": "0"}`))
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.CredibanCo.BaseURL = server.URL + "/"
	cfg.CredibanCo.Username = "usuario-api"
	cfg.CredibanCo.Password = "clave"
	cfg.CredibanCo.Terminal = "000BHWZS"
	cfg.CredibanCo.Merchant = "092461418"

	_, _, err := NewCredibancoClient(cfg).
		CreatePaymentIntent(context.Background(), importe, "tx-1", "https://retorno")
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	return recibido
}

// Los tres parámetros que provocaban "acceso denegado". El plugin de WooCommerce
// del mismo comercio no manda ninguno, y con ellos la pasarela responde error 5.
func TestRegisterNoEnviaTerminalMerchantNiCurrency(t *testing.T) {
	enviado := formularioEnviado(t, 1000)

	for _, prohibido := range []string{"terminal", "merchant", "currency"} {
		if enviado.Has(prohibido) {
			t.Errorf("no debe enviarse %q, y se envió %q", prohibido, enviado.Get(prohibido))
		}
	}
}

func TestRegisterEnviaLoQueEsperaLaPasarela(t *testing.T) {
	enviado := formularioEnviado(t, 1000)

	esperado := map[string]string{
		"userName":    "usuario-api",
		"password":    "clave",
		"orderNumber": "tx-1",
		"returnUrl":   "https://retorno",
		"language":    "es",
	}
	for clave, valor := range esperado {
		if enviado.Get(clave) != valor {
			t.Errorf("%s = %q, se esperaba %q", clave, enviado.Get(clave), valor)
		}
	}
	if enviado.Get("jsonParams") == "" {
		t.Error("falta jsonParams, que es lo que identifica a la app en los registros del banco")
	}
}

func TestImporteEnCentavos(t *testing.T) {
	casos := []struct {
		importe  float64
		esperado string
	}{
		{1000, "100000"},
		{250000, "25000000"},
		// Por la vía anterior —formatear a texto y quitarle el punto— este
		// llegaba como 2554: 25.55 en coma flotante es 25.549999…
		{25.55, "2555"},
		{0.1, "10"},
		{0, "0"},
	}

	for _, c := range casos {
		if got := formularioEnviado(t, c.importe).Get("amount"); got != c.esperado {
			t.Errorf("importe %.2f → %q, se esperaba %q", c.importe, got, c.esperado)
		}
	}
}

func TestPreautorizadoNoCuentaComoPagado(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"orderStatus": 1, "errorCode": "0"}`))
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.CredibanCo.BaseURL = server.URL + "/"

	status, err := NewCredibancoClient(cfg).GetPaymentStatus(context.Background(), "o-1")
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	// Preautorizado es dinero retenido, no cobrado: inscribir a alguien de quien
	// no se ha cobrado es peor que hacerle esperar.
	if status != domain.TxStatusPending {
		t.Errorf("orderStatus 1 debe quedar pendiente, y quedó %s", status)
	}
}

func TestRechazadoSeDistingueDeFallo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"orderStatus": 6, "errorCode": "0"}`))
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.CredibanCo.BaseURL = server.URL + "/"

	status, _ := NewCredibancoClient(cfg).GetPaymentStatus(context.Background(), "o-1")
	if status != domain.TxStatusDeclined {
		t.Errorf("orderStatus 6 debe ser rechazado, y fue %s", status)
	}
}

func TestGetPaymentStatus(t *testing.T) {
	// Mock server for getOrderStatusExtended.do
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/getOrderStatusExtended.do" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"orderStatus": 2, "errorCode": "0"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.CredibanCo.BaseURL = server.URL + "/"

	client := NewCredibancoClient(cfg)
	status, err := client.GetPaymentStatus(context.Background(), "test-order-123")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if status != domain.TxStatusApproved {
		t.Errorf("Expected status APPROVED, got %s", status)
	}
}

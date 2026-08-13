package http

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/services"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// stubPaymentService registra con qué referencia se llamó al servicio: lo que
// se comprueba aquí es la extracción del parámetro y la traducción a códigos
// HTTP, no la lógica de pago, que se prueba en el paquete services.
type stubPaymentService struct {
	referencia string
	llamadas   int
	err        error
}

func (s *stubPaymentService) InitiatePayment(ctx context.Context, userID uuid.UUID, refType domain.ReferenceType, refID uuid.UUID, amount float64, returnUrl string, paymentMethod string) (string, error) {
	return "", nil
}

func (s *stubPaymentService) VerifyPayment(ctx context.Context, txID uuid.UUID) (*domain.Transaction, error) {
	return nil, nil
}

func (s *stubPaymentService) ProcessGatewayNotification(ctx context.Context, referencia string) (*domain.Transaction, error) {
	s.referencia = referencia
	s.llamadas++
	if s.err != nil {
		return nil, s.err
	}
	return &domain.Transaction{ID: uuid.New(), Status: domain.TxStatusApproved}, nil
}

func TestCallback_ExtraeLaReferenciaDeCadaFormato(t *testing.T) {
	// No está confirmado con qué nombre enviará CredibanCo el identificador, así
	// que se aceptan los de las integraciones conocidas de esta plataforma.
	casos := []struct {
		nombre     string
		url        string
		esperada   string
		metodoPost bool
		cuerpo     string
	}{
		{nombre: "orderNumber en GET", url: "/api/payments/credibanco/callback?orderNumber=tx-1", esperada: "tx-1"},
		{nombre: "mdOrder en GET", url: "/api/payments/credibanco/callback?mdOrder=orden-abc", esperada: "orden-abc"},
		{nombre: "tx_id en GET", url: "/api/payments/credibanco/callback?tx_id=tx-2", esperada: "tx-2"},
		{nombre: "orderId en GET", url: "/api/payments/credibanco/callback?orderId=orden-def", esperada: "orden-def"},
		{
			nombre: "POST con formulario", url: "/api/payments/credibanco/callback",
			metodoPost: true, cuerpo: "mdOrder=orden-post", esperada: "orden-post",
		},
		{
			// orderNumber es nuestro id y manda sobre el de la pasarela: si vienen
			// los dos, resolver por el nuestro evita una consulta extra.
			nombre: "orderNumber gana a mdOrder",
			url:    "/api/payments/credibanco/callback?mdOrder=orden-abc&orderNumber=tx-1", esperada: "tx-1",
		},
		{
			// Un parámetro presente pero vacío no cuenta como referencia.
			nombre: "vacio se ignora y se sigue buscando",
			url:    "/api/payments/credibanco/callback?orderNumber=&mdOrder=orden-abc", esperada: "orden-abc",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			svc := &stubPaymentService{}
			h := NewPaymentHandler(svc)

			var req *http.Request
			if c.metodoPost {
				req = httptest.NewRequest(http.MethodPost, c.url, strings.NewReader(c.cuerpo))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			} else {
				req = httptest.NewRequest(http.MethodGet, c.url, nil)
			}
			rec := httptest.NewRecorder()

			h.CredibancoCallback(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("se esperaba 200, llegó %d", rec.Code)
			}
			if svc.referencia != c.esperada {
				t.Errorf("referencia esperada %q, llegó %q", c.esperada, svc.referencia)
			}
		})
	}
}

func TestCallback_SinReferencia(t *testing.T) {
	svc := &stubPaymentService{err: services.ErrPaymentNotificationEmpty}
	h := NewPaymentHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/payments/credibanco/callback", nil)
	rec := httptest.NewRecorder()
	h.CredibancoCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("se esperaba 400, llegó %d", rec.Code)
	}
}

func TestCallback_ReferenciaDesconocidaResponde200(t *testing.T) {
	// A propósito: un error haría que la pasarela reintentara en bucle algo que
	// nunca va a existir. Puede ser una prueba suya, un reenvío viejo o alguien
	// tanteando la URL. Queda registrado en el log y se acusa recibo.
	svc := &stubPaymentService{err: services.ErrPaymentNotificationUnknown}
	h := NewPaymentHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/payments/credibanco/callback?mdOrder=no-existe", nil)
	rec := httptest.NewRecorder()
	h.CredibancoCallback(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("se esperaba 200, llegó %d", rec.Code)
	}
}

func TestCallback_ErrorPropioResponde500(t *testing.T) {
	// Aquí sí interesa que reintenten: la avería es nuestra.
	svc := &stubPaymentService{err: context.DeadlineExceeded}
	h := NewPaymentHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/payments/credibanco/callback?mdOrder=orden-abc", nil)
	rec := httptest.NewRecorder()
	h.CredibancoCallback(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("se esperaba 500, llegó %d", rec.Code)
	}
}

func TestCallback_NoFiltraDatosDeLaTransaccion(t *testing.T) {
	// Quien llama no está autenticado: la respuesta no debe contar nada que no
	// supiera ya. Un acuse mínimo basta.
	svc := &stubPaymentService{}
	h := NewPaymentHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/payments/credibanco/callback?orderNumber=tx-1", nil)
	rec := httptest.NewRecorder()
	h.CredibancoCallback(rec, req)

	cuerpo := rec.Body.String()
	if cuerpo != "OK" {
		t.Errorf("el acuse debe ser mínimo, llegó %q", cuerpo)
	}
	for _, prohibido := range []string{"amount", "user_id", "status", "reference"} {
		if strings.Contains(strings.ToLower(cuerpo), prohibido) {
			t.Errorf("la respuesta no debe incluir %q", prohibido)
		}
	}
}

package http

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/services"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Reservar cupo en un evento de pago crea la inscripción antes de salir a la
// pasarela, con su QR ya generado. La respuesta de esa reserva no puede
// entregar ese código: sería una credencial de un evento impago.

func TestRegister_NoDevuelveElQRDeUnaReservaSinPagar(t *testing.T) {
	svc := &stubEventService{alRegistrar: func(reg *domain.Registration) {
		reg.PaymentStatus = "pending"
		reg.RegistrationStatus = domain.RegistrationPendingPayment
		reg.QRData = "REG-9f1c3a2e"
		reg.TotalPaid = 250000
	}}
	h := NewEventHandler(svc)

	req, rec := peticion(nil, "user-1", "familia")
	h.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("se esperaba 201, se obtuvo %d", rec.Code)
	}
	var cuerpo map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &cuerpo); err != nil {
		t.Fatalf("respuesta ilegible: %v", err)
	}
	if qr := cuerpo["qr_data"]; qr != "" {
		t.Errorf("el QR no debe viajar en la reserva impaga, llegó %q", qr)
	}
	// El resto de la respuesta sí tiene que llegar: el usuario necesita ver que
	// su cupo quedó reservado y cuánto debe.
	if cuerpo["registration_status"] != domain.RegistrationPendingPayment {
		t.Errorf("estado esperado %q, llegó %v", domain.RegistrationPendingPayment, cuerpo["registration_status"])
	}
	if cuerpo["total_paid"] != 250000.0 {
		t.Errorf("total_paid esperado 250000, llegó %v", cuerpo["total_paid"])
	}
	// Vaciarlo en la respuesta no puede borrarlo de la fila recién escrita.
	if svc.recibida.QRData != "REG-9f1c3a2e" {
		t.Errorf("la inscripción guardada perdió su QR: %q", svc.recibida.QRData)
	}
}

func TestRegister_ElEventoGratuitoSiEntregaSuQR(t *testing.T) {
	// Un evento gratuito queda confirmado en el acto: ahí el QR es la entrada y
	// no hay ningún pago que esperar.
	svc := &stubEventService{alRegistrar: func(reg *domain.Registration) {
		reg.PaymentStatus = "free"
		reg.RegistrationStatus = domain.RegistrationConfirmed
		reg.QRData = "REG-4b8d17aa"
	}}
	h := NewEventHandler(svc)

	req, rec := peticion(nil, "user-1", "familia")
	h.Register(rec, req)

	var cuerpo map[string]any
	json.Unmarshal(rec.Body.Bytes(), &cuerpo)
	if cuerpo["qr_data"] != "REG-4b8d17aa" {
		t.Errorf("el evento gratuito debe entregar su QR, llegó %v", cuerpo["qr_data"])
	}
}

func TestCheckIn_CodigosDeRespuesta(t *testing.T) {
	casos := []struct {
		nombre   string
		err      error
		esperado int
	}{
		// 402 y no 404: quien está en la puerta tiene delante a un asistente
		// real al que hay que cobrarle, no un código inventado.
		{"pendiente de pago", services.ErrCheckInPendingPayment, http.StatusPaymentRequired},
		{"QR inexistente", services.ErrCheckInNotFound, http.StatusNotFound},
		{"válido", nil, http.StatusOK},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			h := NewEventHandler(&stubEventService{errCheckIn: c.err})

			body, _ := json.Marshal(map[string]string{"qrData": "REG-9f1c3a2e"})
			req := httptest.NewRequest(http.MethodPost, "/api/events/checkin", bytes.NewReader(body))
			req = req.WithContext(context.WithValue(req.Context(), UserIDKey, "staff-1"))
			rec := httptest.NewRecorder()

			h.CheckIn(rec, req)

			if rec.Code != c.esperado {
				t.Errorf("se esperaba %d, se obtuvo %d (%s)", c.esperado, rec.Code, rec.Body.String())
			}
		})
	}
}

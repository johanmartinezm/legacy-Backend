package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"
	"testing"
)

// El QR se genera al reservar el cupo, antes de pasar por la pasarela, así que
// una inscripción impaga tiene código desde el primer momento. Estos casos
// fijan que ese código identifique la reserva pero no abra la puerta.

func servicioConQR(reg *domain.Registration, errRepo error) (*EventService, *bool) {
	svc, asistencia, _ := servicioConQRYDatos(reg, errRepo, nil)
	return svc, asistencia
}

// servicioConQRYDatos permite fijar los datos personales que devuelve el
// repositorio —tal como los guarda la base, es decir cifrados— para comprobar
// que el servicio los abre.
func servicioConQRYDatos(reg *domain.Registration, errRepo error, datos *domain.CheckInResponse) (*EventService, *bool, ports.CryptoService) {
	asistenciaRegistrada := false
	var crypto ports.CryptoService
	if datos != nil {
		crypto = cryptoFalso{}
	}
	repo := &MockEventRepository{
		GetRegistrationByQRFunc: func(ctx context.Context, qr string) (*domain.Registration, *domain.CheckInResponse, error) {
			if errRepo != nil {
				return nil, nil, errRepo
			}
			if datos != nil {
				copia := *datos
				copia.RegistrationID = reg.ID
				return reg, &copia, nil
			}
			return reg, &domain.CheckInResponse{RegistrationID: reg.ID, EventTitle: "LEGACY SUMMIT"}, nil
		},
		RecordAttendanceFunc: func(ctx context.Context, regID, staffID string) error {
			asistenciaRegistrada = true
			return nil
		},
	}
	return NewEventService(repo, crypto), &asistenciaRegistrada, crypto
}

func TestCheckIn_RechazaInscripcionPendienteDePago(t *testing.T) {
	reg := &domain.Registration{
		ID:                 "reg-1",
		PaymentStatus:      "pending",
		RegistrationStatus: domain.RegistrationPendingPayment,
		QRData:             "REG-9f1c3a2e",
	}
	svc, asistencia := servicioConQR(reg, nil)

	resp, err := svc.CheckIn(context.Background(), reg.QRData, "staff-1")

	if !errors.Is(err, ErrCheckInPendingPayment) {
		t.Fatalf("se esperaba ErrCheckInPendingPayment, se obtuvo %v", err)
	}
	if resp != nil {
		t.Errorf("no debe devolverse respuesta de check-in: %+v", resp)
	}
	// Lo importante no es solo el error: la asistencia no puede quedar
	// registrada. Si se marcara igual, el informe de aforo contaría a alguien
	// que no entró y el rechazo sería decorativo.
	if *asistencia {
		t.Error("no debe registrarse la asistencia de una inscripción sin pagar")
	}
}

func TestCheckIn_AceptaInscripcionConfirmada(t *testing.T) {
	for _, estadoPago := range []string{"paid", "free"} {
		t.Run(estadoPago, func(t *testing.T) {
			reg := &domain.Registration{
				ID:                 "reg-2",
				PaymentStatus:      estadoPago,
				RegistrationStatus: domain.RegistrationConfirmed,
				QRData:             "REG-4b8d17aa",
			}
			svc, asistencia := servicioConQR(reg, nil)

			resp, err := svc.CheckIn(context.Background(), reg.QRData, "staff-1")

			if err != nil {
				t.Fatalf("una inscripción confirmada debe entrar: %v", err)
			}
			if resp == nil || resp.EventTitle != "LEGACY SUMMIT" {
				t.Errorf("respuesta incompleta: %+v", resp)
			}
			if !*asistencia {
				t.Error("debe registrarse la asistencia")
			}
		})
	}
}

func TestCheckIn_DescifraElNombreYElCorreoDelAsistente(t *testing.T) {
	// Lo que ve el personal de la puerta. Hasta ahora la consulta concatenaba
	// `first_name || ' ' || last_name`, uniendo dos bloques cifrados en una
	// cadena que ya no se podía abrir, y el escáner mostraba texto cifrado.
	reg := &domain.Registration{
		ID:                 "reg-3",
		PaymentStatus:      "paid",
		RegistrationStatus: domain.RegistrationConfirmed,
		QRData:             "REG-4b8d17aa",
	}
	datos := &domain.CheckInResponse{
		FirstName:  prefijoCifrado + "Ana",
		LastName:   prefijoCifrado + "Restrepo",
		UserEmail:  prefijoCifrado + "ana@example.com",
		EventTitle: "LEGACY SUMMIT",
	}
	svc, _, _ := servicioConQRYDatos(reg, nil, datos)

	resp, err := svc.CheckIn(context.Background(), reg.QRData, "staff-1")

	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if resp.UserName != "Ana Restrepo" {
		t.Errorf("nombre esperado \"Ana Restrepo\", llegó %q", resp.UserName)
	}
	if resp.UserEmail != "ana@example.com" {
		t.Errorf("correo esperado descifrado, llegó %q", resp.UserEmail)
	}
}

func TestCheckIn_NombreEnClaroSeConservaTalCual(t *testing.T) {
	// Filas anteriores al cifrado: el descifrado falla y hay que quedarse con
	// el valor original, o el asistente aparecería sin nombre en la puerta.
	reg := &domain.Registration{
		ID:                 "reg-4",
		PaymentStatus:      "free",
		RegistrationStatus: domain.RegistrationConfirmed,
		QRData:             "REG-antiguo",
	}
	datos := &domain.CheckInResponse{
		FirstName: "Carlos",
		LastName:  "Mejía",
		UserEmail: "carlos@example.com",
	}
	svc, _, _ := servicioConQRYDatos(reg, nil, datos)

	resp, _ := svc.CheckIn(context.Background(), reg.QRData, "staff-1")

	if resp.UserName != "Carlos Mejía" {
		t.Errorf("nombre esperado \"Carlos Mejía\", llegó %q", resp.UserName)
	}
	if resp.UserEmail != "carlos@example.com" {
		t.Errorf("correo esperado intacto, llegó %q", resp.UserEmail)
	}
}

func TestCheckIn_QRInexistente(t *testing.T) {
	svc, asistencia := servicioConQR(nil, errors.New("no rows in result set"))

	_, err := svc.CheckIn(context.Background(), "REG-inventado", "staff-1")

	// Distinto centinela que el de pendiente de pago: en la puerta son dos
	// situaciones diferentes y el handler las traduce a 404 y 402.
	if !errors.Is(err, ErrCheckInNotFound) {
		t.Fatalf("se esperaba ErrCheckInNotFound, se obtuvo %v", err)
	}
	if *asistencia {
		t.Error("no debe registrarse asistencia de un QR inexistente")
	}
}

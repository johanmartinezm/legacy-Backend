package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"testing"
	"time"
)

// 🔴 La propiedad que hay que preservar: pasar dos veces el mismo QR no crea una
// asistencia nueva, y quien está en la puerta se entera de que ese código ya
// había entrado.
//
// Hasta el 2026-08-19 RecordAttendance insertaba siempre, así que una relectura
// dejaba dos filas en events.attendance_logs —el aforo del evento subía con
// cada escaneo— y las dos respuestas eran idénticas. Encontrado con F12.8 del
// plan de pruebas.

// servicioCheckInConMemoria imita el comportamiento del repositorio ya
// corregido: la primera lectura registra, las siguientes devuelven la hora de
// aquella.
func servicioCheckInConMemoria(reg *domain.Registration, entrada time.Time) (*EventService, *int) {
	inserciones := 0
	repo := &MockEventRepository{
		GetRegistrationByQRFunc: func(ctx context.Context, qr string) (*domain.Registration, *domain.CheckInResponse, error) {
			return reg, &domain.CheckInResponse{RegistrationID: reg.ID, EventTitle: "LEGACY SUMMIT"}, nil
		},
		RecordAttendanceFunc: func(ctx context.Context, regID, staffID string) (time.Time, bool, error) {
			if inserciones == 0 {
				inserciones++
				return entrada, false, nil
			}
			return entrada, true, nil
		},
	}
	return NewEventService(repo, nil), &inserciones
}

func inscripcionConfirmada() *domain.Registration {
	return &domain.Registration{
		ID:                 "reg-1",
		PaymentStatus:      "paid",
		RegistrationStatus: domain.RegistrationConfirmed,
		QRData:             "REG-9f1c3a2e",
	}
}

func TestCheckIn_LaPrimeraLecturaNoSeMarcaComoRepetida(t *testing.T) {
	entrada := time.Date(2026, 9, 15, 9, 30, 0, 0, time.UTC)
	svc, inserciones := servicioCheckInConMemoria(inscripcionConfirmada(), entrada)

	resp, err := svc.CheckIn(context.Background(), "REG-9f1c3a2e", "staff-1")
	if err != nil {
		t.Fatalf("check-in válido falló: %v", err)
	}
	if resp.AlreadyCheckedIn {
		t.Error("la primera entrada no puede salir marcada como repetida")
	}
	if !resp.CheckInTime.Equal(entrada) {
		t.Errorf("la hora debe ser la que guardó la base: %v, se obtuvo %v", entrada, resp.CheckInTime)
	}
	if *inserciones != 1 {
		t.Errorf("se esperaba una asistencia registrada, hubo %d", *inserciones)
	}
}

func TestCheckIn_LaSegundaLecturaAvisaYNoCuentaOtraVez(t *testing.T) {
	entrada := time.Date(2026, 9, 15, 9, 30, 0, 0, time.UTC)
	svc, inserciones := servicioCheckInConMemoria(inscripcionConfirmada(), entrada)

	if _, err := svc.CheckIn(context.Background(), "REG-9f1c3a2e", "staff-1"); err != nil {
		t.Fatalf("primer check-in: %v", err)
	}
	resp, err := svc.CheckIn(context.Background(), "REG-9f1c3a2e", "staff-2")
	if err != nil {
		t.Fatalf("la relectura no es un error: sigue siendo un asistente válido, y falló con %v", err)
	}

	if !resp.AlreadyCheckedIn {
		t.Error("la segunda lectura tiene que salir marcada: sin esto, en la puerta no se distingue")
	}
	// La hora es la de la PRIMERA entrada, no la de ahora: es el dato que sirve
	// para decidir si el QR se está compartiendo.
	if !resp.CheckInTime.Equal(entrada) {
		t.Errorf("se esperaba la hora de la primera entrada %v, se obtuvo %v", entrada, resp.CheckInTime)
	}
	if *inserciones != 1 {
		t.Errorf("la relectura no puede registrar otra asistencia: hubo %d", *inserciones)
	}
	// Los datos del asistente siguen llegando: la puerta necesita verlos aunque
	// el código ya se hubiera usado.
	if resp.EventTitle == "" {
		t.Error("la respuesta de una relectura debe traer los datos del asistente")
	}
}

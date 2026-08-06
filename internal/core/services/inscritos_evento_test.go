package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// cryptoFalso imita al CryptoService: "cifra" anteponiendo un prefijo, de modo
// que un valor sin él se comporta como un dato antiguo guardado en claro y hace
// fallar el descifrado, igual que en la base real.
type cryptoFalso struct{}

const prefijoCifrado = "enc:"

func (cryptoFalso) Encrypt(texto string) (string, error) { return prefijoCifrado + texto, nil }
func (cryptoFalso) BlindIndex(texto string) string       { return texto }
func (cryptoFalso) Decrypt(texto string) (string, error) {
	if !strings.HasPrefix(texto, prefijoCifrado) {
		return "", errors.New("no está cifrado")
	}
	return strings.TrimPrefix(texto, prefijoCifrado), nil
}

func repoConInscritos(inscritos []domain.EventRegistrant) *MockEventRepository {
	return &MockEventRepository{
		GetRegistrationsByEventFunc: func(ctx context.Context, eID string) ([]domain.EventRegistrant, error) {
			return inscritos, nil
		},
	}
}

func TestGetEventRegistrants_DescifraNombreYCorreo(t *testing.T) {
	repo := repoConInscritos([]domain.EventRegistrant{{
		RegistrationID:     "reg-1",
		FirstName:          prefijoCifrado + "Ana",
		LastName:           prefijoCifrado + "Restrepo",
		Email:              prefijoCifrado + "ana@example.com",
		Phone:              "+57 300 000 0000",
		PaymentStatus:      "paid",
		RegistrationStatus: domain.RegistrationConfirmed,
	}})

	inscritos, err := NewEventService(repo, cryptoFalso{}).GetEventRegistrants(context.Background(), "event-1")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(inscritos) != 1 {
		t.Fatalf("se esperaba 1 inscrito, llegaron %d", len(inscritos))
	}
	if inscritos[0].FullName != "Ana Restrepo" {
		t.Errorf("nombre esperado \"Ana Restrepo\", llegó %q", inscritos[0].FullName)
	}
	if inscritos[0].Email != "ana@example.com" {
		t.Errorf("correo esperado descifrado, llegó %q", inscritos[0].Email)
	}
	// El teléfono no está cifrado en la base: tiene que pasar intacto.
	if inscritos[0].Phone != "+57 300 000 0000" {
		t.Errorf("el teléfono no debe tocarse, llegó %q", inscritos[0].Phone)
	}
}

func TestGetEventRegistrants_DejaEnPazLoQueNoEstaCifrado(t *testing.T) {
	// Las filas anteriores al cifrado están en claro. Perderlas sería peor que
	// mostrarlas, así que un descifrado fallido conserva el valor original.
	repo := repoConInscritos([]domain.EventRegistrant{{
		RegistrationID: "reg-vieja",
		FirstName:      "Carlos",
		LastName:       "Mejía",
		Email:          "carlos@example.com",
	}})

	inscritos, err := NewEventService(repo, cryptoFalso{}).GetEventRegistrants(context.Background(), "event-1")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if inscritos[0].FullName != "Carlos Mejía" {
		t.Errorf("nombre esperado \"Carlos Mejía\", llegó %q", inscritos[0].FullName)
	}
	if inscritos[0].Email != "carlos@example.com" {
		t.Errorf("correo esperado intacto, llegó %q", inscritos[0].Email)
	}
}

func TestGetEventRegistrants_OrdenAlfabetico(t *testing.T) {
	// En la base los nombres están cifrados, así que el ORDER BY de la consulta
	// no puede ordenarlos: el orden se decide aquí, ya en claro.
	ahora := time.Now()
	ayer := ahora.Add(-24 * time.Hour)
	repo := repoConInscritos([]domain.EventRegistrant{
		{RegistrationID: "r3", FirstName: prefijoCifrado + "Zulema", LastName: prefijoCifrado + "Ariza"},
		{RegistrationID: "r1", FirstName: prefijoCifrado + "ana", LastName: prefijoCifrado + "Beltrán", RegistrationDate: ahora},
		{RegistrationID: "r2", FirstName: prefijoCifrado + "Bruno", LastName: prefijoCifrado + "Cano"},
		// Mismo nombre que r1 pero inscrita antes: desempata la fecha, así el
		// orden no depende de cómo llegaran las filas.
		{RegistrationID: "r4", FirstName: prefijoCifrado + "ana", LastName: prefijoCifrado + "Beltrán", RegistrationDate: ayer},
	})

	inscritos, _ := NewEventService(repo, cryptoFalso{}).GetEventRegistrants(context.Background(), "event-1")

	// "ana" en minúscula va primero: se ordena sin distinguir mayúsculas, o
	// quedaría detrás de "Zulema" por el orden de los bytes.
	esperado := []string{"r4", "r1", "r2", "r3"}
	for i, id := range esperado {
		if inscritos[i].RegistrationID != id {
			t.Errorf("posición %d: se esperaba %s, llegó %s", i, id, inscritos[i].RegistrationID)
		}
	}
}

func TestGetEventRegistrants_SinInscritos(t *testing.T) {
	inscritos, err := NewEventService(repoConInscritos([]domain.EventRegistrant{}), cryptoFalso{}).
		GetEventRegistrants(context.Background(), "event-1")

	if err != nil {
		t.Fatalf("un evento sin inscritos no es un error: %v", err)
	}
	if len(inscritos) != 0 {
		t.Errorf("se esperaba lista vacía, llegaron %d", len(inscritos))
	}
}

func TestGetEventRegistrants_ErrorDelRepositorio(t *testing.T) {
	repo := &MockEventRepository{
		GetRegistrationsByEventFunc: func(ctx context.Context, eID string) ([]domain.EventRegistrant, error) {
			return nil, errors.New("fallo de conexión")
		},
	}

	if _, err := NewEventService(repo, cryptoFalso{}).GetEventRegistrants(context.Background(), "event-1"); err == nil {
		t.Error("el error del repositorio debe propagarse")
	}
}

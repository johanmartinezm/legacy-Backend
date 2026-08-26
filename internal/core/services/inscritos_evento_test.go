package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"strings"
	"testing"
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
		GetRegistrationsByEventFunc: func(ctx context.Context, eID string, limit, offset int) ([]domain.EventRegistrant, error) {
			return inscritos, nil
		},
		CountRegistrationsByEventFunc: func(ctx context.Context, eID string) (int, error) {
			return len(inscritos), nil
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

	inscritos, _, err := NewEventService(repo, cryptoFalso{}).GetEventRegistrants(context.Background(), "event-1", 50, 0)
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

	inscritos, _, err := NewEventService(repo, cryptoFalso{}).GetEventRegistrants(context.Background(), "event-1", 50, 0)
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

// Hasta el 2026-08-26 este servicio ordenaba la lista por nombre, ya en claro,
// porque en la base los nombres estan cifrados y un ORDER BY los ordenaria por
// su texto cifrado. **Ese orden se retiro al paginar, y no es un descuido.**
//
// Ordenar en Go solo alcanza a las filas que ya se trajeron: con paginas
// quedaria una lista alfabetica que vuelve a empezar en cada pagina —la A
// despues de la Z—, que parece un orden y no lo es. Ahora manda el ORDER BY de
// la consulta (fecha de inscripcion descendente, con el id de desempate), que
// ademas es el orden util para quien organiza.
func TestGetEventRegistrants_RespetaElOrdenDelRepositorio(t *testing.T) {
	repo := repoConInscritos([]domain.EventRegistrant{
		{RegistrationID: "r3", FirstName: prefijoCifrado + "Zulema", LastName: prefijoCifrado + "Ariza"},
		{RegistrationID: "r1", FirstName: prefijoCifrado + "ana", LastName: prefijoCifrado + "Beltrán"},
		{RegistrationID: "r2", FirstName: prefijoCifrado + "Bruno", LastName: prefijoCifrado + "Cano"},
	})

	inscritos, _, _ := NewEventService(repo, cryptoFalso{}).GetEventRegistrants(context.Background(), "event-1", 50, 0)

	// Tal cual llegaron. Si alguien vuelve a ordenar aqui, este test lo avisa:
	// reordenar la pagina rompe la paginacion sin dar ningun sintoma.
	esperado := []string{"r3", "r1", "r2"}
	for i, id := range esperado {
		if inscritos[i].RegistrationID != id {
			t.Errorf("posición %d: se esperaba %s, llegó %s", i, id, inscritos[i].RegistrationID)
		}
	}
}

func TestGetEventRegistrants_PasaLaPaginacionAlRepositorio(t *testing.T) {
	var limitRecibido, offsetRecibido int
	repo := &MockEventRepository{
		GetRegistrationsByEventFunc: func(ctx context.Context, eID string, limit, offset int) ([]domain.EventRegistrant, error) {
			limitRecibido, offsetRecibido = limit, offset
			return nil, nil
		},
		CountRegistrationsByEventFunc: func(ctx context.Context, eID string) (int, error) { return 1234, nil },
	}

	_, total, err := NewEventService(repo, cryptoFalso{}).GetEventRegistrants(context.Background(), "event-1", 25, 75)
	if err != nil {
		t.Fatal(err)
	}
	if limitRecibido != 25 || offsetRecibido != 75 {
		t.Errorf("llegó limit=%d offset=%d; se esperaba 25/75", limitRecibido, offsetRecibido)
	}
	// El total es el de la tabla, no el largo de la pagina: es lo que el panel
	// necesita para saber cuantas paginas hay.
	if total != 1234 {
		t.Errorf("el total debe venir del conteo, y llegó %d", total)
	}
}

func TestGetEventRegistrants_ErrorAlContar(t *testing.T) {
	// Si el conteo falla no se sigue: una pagina sin total deja al panel
	// pintando un paginador inventado.
	repo := &MockEventRepository{
		CountRegistrationsByEventFunc: func(ctx context.Context, eID string) (int, error) {
			return 0, errors.New("fallo de conexión")
		},
	}

	if _, _, err := NewEventService(repo, cryptoFalso{}).GetEventRegistrants(context.Background(), "event-1", 50, 0); err == nil {
		t.Error("el error del conteo debe propagarse")
	}
}

func TestGetEventRegistrants_SinInscritos(t *testing.T) {
	inscritos, total, err := NewEventService(repoConInscritos([]domain.EventRegistrant{}), cryptoFalso{}).
		GetEventRegistrants(context.Background(), "event-1", 50, 0)

	if err != nil {
		t.Fatalf("un evento sin inscritos no es un error: %v", err)
	}
	if len(inscritos) != 0 {
		t.Errorf("se esperaba lista vacía, llegaron %d", len(inscritos))
	}
	if total != 0 {
		t.Errorf("el total de un evento sin inscritos es 0, llegó %d", total)
	}
}

func TestGetEventRegistrants_ErrorDelRepositorio(t *testing.T) {
	repo := &MockEventRepository{
		GetRegistrationsByEventFunc: func(ctx context.Context, eID string, limit, offset int) ([]domain.EventRegistrant, error) {
			return nil, errors.New("fallo de conexión")
		},
	}

	if _, _, err := NewEventService(repo, cryptoFalso{}).GetEventRegistrants(context.Background(), "event-1", 50, 0); err == nil {
		t.Error("el error del repositorio debe propagarse")
	}
}

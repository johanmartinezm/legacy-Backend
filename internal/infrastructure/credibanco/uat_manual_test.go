//go:build uat

// Prueba manual contra el entorno UAT de CredibanCo. NO se ejecuta con
// `go test ./...`: hace falta pedirla a mano.
//
//	go test -tags uat -count=1 -v ./internal/infrastructure/credibanco/ -run TestUATRegistroReal
//
// **Una llamada por sesión, y no más.** Tras tres intentos seguidos el 2026-08-06
// la pasarela empezó a devolver 403 en el borde; repetir puede bloquear la IP del
// servidor o el usuario de API. Por eso vive tras una etiqueta de compilación y
// no como un test normal.
//
// Lee las credenciales de config.yaml, que no está versionado.
package credibanco

import (
	"context"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"applegacy/backend/internal/config"
)

func TestUATRegistroReal(t *testing.T) {
	datos, err := os.ReadFile("../../../config.yaml")
	if err != nil {
		t.Fatalf("no se pudo leer config.yaml: %v", err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(datos, &cfg); err != nil {
		t.Fatalf("no se pudo interpretar config.yaml: %v", err)
	}

	// Salvaguarda: esta prueba nunca debe salir contra producción.
	if !strings.Contains(cfg.CredibanCo.BaseURL, "ecouat") {
		t.Fatalf("config.yaml no apunta a UAT, apunta a %q", cfg.CredibanCo.BaseURL)
	}

	t.Logf("entorno: %s", cfg.CredibanCo.BaseURL)
	t.Logf("usuario: %d caracteres, termina en -api: %t",
		len(cfg.CredibanCo.Username), strings.HasSuffix(cfg.CredibanCo.Username, "-api"))

	// Importe simbólico: en UAT no se cobra, pero no hay motivo para pedir más.
	orderID, formURL, err := NewCredibancoClient(&cfg).CreatePaymentIntent(
		context.Background(),
		1000,
		"prueba-uat-payload-corregido",
		"https://legacy.intelyclick.com/api/payments/credibanco/callback",
	)

	if err != nil {
		t.Fatalf("RESULTADO: la pasarela rechazó el registro: %v", err)
	}

	t.Logf("RESULTADO: registro aceptado. orderId=%s", orderID)
	t.Logf("formUrl=%s", formURL)
}

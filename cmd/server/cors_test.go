package main

import "testing"

// El criterio de origen es lo unico que separa "solo el panel puede llamar" de
// "cualquier pagina de internet puede llamar con el token del visitante". Los
// casos negativos importan mas que los positivos.
func TestOrigenPermitido(t *testing.T) {
	casos := []struct {
		origen  string
		permite bool
		porque  string
	}{
		{"https://legacy.intelyclick.com", true, "el panel en produccion"},
		{"http://localhost:4200", true, "ng serve"},
		{"http://localhost:51234", true, "flutter run -d chrome usa un puerto aleatorio"},
		{"http://127.0.0.1:8080", true, "misma maquina por IP"},

		{"https://malicioso.com", false, "un dominio cualquiera"},
		// El clasico: un atacante registra un dominio que EMPIEZA igual. Con una
		// comparacion por prefijo esto pasaria.
		{"https://legacy.intelyclick.com.malicioso.com", false, "sufijo anadido"},
		{"https://malicioso-legacy.intelyclick.com", false, "prefijo anadido al host"},
		// Mismo dominio por http: degradar a texto plano no puede valer.
		{"http://legacy.intelyclick.com", false, "el mismo dominio sin TLS"},
		{"https://localhost.malicioso.com", false, "localhost como subdominio ajeno"},
		{"", false, "sin origen"},
		{"no-es-una-url", false, "cadena sin esquema ni host"},
	}

	for _, c := range casos {
		t.Run(c.origen, func(t *testing.T) {
			if got := origenPermitido(nil, c.origen); got != c.permite {
				t.Errorf("origenPermitido(%q) = %v, se esperaba %v (%s)", c.origen, got, c.permite, c.porque)
			}
		})
	}
}

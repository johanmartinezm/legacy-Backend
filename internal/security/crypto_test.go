package security

import "testing"

// La clave tiene que medir 32 caracteres exactos: NewCryptoService lo exige.
const claveDePrueba = "12345678901234567890123456789012"

func nuevo(t *testing.T) *CryptoService {
	t.Helper()
	s, err := NewCryptoService(claveDePrueba)
	if err != nil {
		t.Fatalf("no se pudo crear el servicio: %v", err)
	}
	return s
}

// El índice ciego es por donde se busca a la persona al iniciar sesión. Si no
// normaliza, la misma cuenta escrita con otra caja no se encuentra —y además se
// puede registrar dos veces, porque la comprobación de duplicados usa este mismo
// índice—. Pasó hasta el 2026-08-18.
func TestBlindIndexNormalizaElCorreo(t *testing.T) {
	s := nuevo(t)
	base := s.BlindIndex("juan@mail.com")

	casos := map[string]string{
		"tal cual":              "juan@mail.com",
		"primera en mayuscula":  "Juan@mail.com",
		"todo en mayusculas":    "JUAN@MAIL.COM",
		"dominio en mayusculas": "juan@MAIL.com",
		"con espacios delante":  "   juan@mail.com",
		"con espacios detras":   "juan@mail.com   ",
		"rodeado de espacios":   "  Juan@Mail.Com  ",
	}

	for nombre, entrada := range casos {
		if got := s.BlindIndex(entrada); got != base {
			t.Errorf("%s: el indice difiere del de la forma normalizada", nombre)
		}
	}
}

// Normalizar no puede volverse tan agresivo como para confundir dos cuentas que
// de verdad son distintas.
func TestBlindIndexDistingueCorreosDistintos(t *testing.T) {
	s := nuevo(t)

	distintos := []string{
		"juan@mail.com",
		"juana@mail.com",
		"juan@otromail.com",
		"juan.perez@mail.com",
	}

	vistos := map[string]string{}
	for _, correo := range distintos {
		idx := s.BlindIndex(correo)
		if antes, repetido := vistos[idx]; repetido {
			t.Errorf("%q y %q comparten indice y no deberian", antes, correo)
		}
		vistos[idx] = correo
	}
}

// Un correo vacío no tiene índice: devolver un hash lo haría buscable, y todas
// las filas sin correo colisionarían entre sí contra la restricción UNIQUE.
func TestBlindIndexVacio(t *testing.T) {
	s := nuevo(t)
	for _, entrada := range []string{"", "   ", "\t"} {
		if got := s.BlindIndex(entrada); got != "" {
			t.Errorf("con %q se esperaba cadena vacia y salio %q", entrada, got)
		}
	}
}

func TestNormalizarCorreo(t *testing.T) {
	casos := []struct{ entrada, esperado string }{
		{"juan@mail.com", "juan@mail.com"},
		{"Juan@Mail.Com", "juan@mail.com"},
		{"  JUAN@MAIL.COM  ", "juan@mail.com"},
		{"", ""},
		{"   ", ""},
	}
	for _, c := range casos {
		if got := NormalizarCorreo(c.entrada); got != c.esperado {
			t.Errorf("NormalizarCorreo(%q) = %q, se esperaba %q", c.entrada, got, c.esperado)
		}
	}
}

// El cifrado conserva el correo tal como se escribió: normalizar es cosa del
// índice de búsqueda, no de lo que se le muestra a la persona.
func TestElCifradoConservaLaCajaOriginal(t *testing.T) {
	s := nuevo(t)
	original := "Juan.Perez@Mail.com"

	cifrado, err := s.Encrypt(original)
	if err != nil {
		t.Fatalf("no se pudo cifrar: %v", err)
	}
	claro, err := s.Decrypt(cifrado)
	if err != nil {
		t.Fatalf("no se pudo descifrar: %v", err)
	}
	if claro != original {
		t.Errorf("se esperaba %q y salio %q", original, claro)
	}
}

package apple

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const bundleDePrueba = "co.legacynetwork.legacyapp"

// servidorDeClaves imita a appleid.apple.com/auth/keys con una clave nuestra,
// para poder firmar tokens y comprobar qué acepta el validador.
func servidorDeClaves(t *testing.T) (*rsa.PrivateKey, string, *Validador) {
	t.Helper()

	clave, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("no se pudo generar la clave: %v", err)
	}
	const kid = "clave-de-prueba"

	jwks := map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA",
			"kid": kid,
			"n":   base64.RawURLEncoding.EncodeToString(clave.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(clave.E)).Bytes()),
		}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)

	v := NuevoValidador(bundleDePrueba)
	// Se apunta al servidor de prueba sustituyendo el transporte, porque la URL
	// de Apple es una constante a propósito.
	v.cliente = srv.Client()
	v.cliente.Transport = redirigirA(srv.URL)

	return clave, kid, v
}

type redirigir struct {
	destino string
	base    http.RoundTripper
}

func redirigirA(destino string) http.RoundTripper {
	return redirigir{destino: destino, base: http.DefaultTransport}
}

func (r redirigir) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	nuevo, _ := http.NewRequestWithContext(req.Context(), req.Method, r.destino, nil)
	nuevo.URL.Path = u.Path
	return r.base.RoundTrip(nuevo)
}

func firmar(t *testing.T, clave *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	firmado, err := token.SignedString(clave)
	if err != nil {
		t.Fatalf("no se pudo firmar: %v", err)
	}
	return firmado
}

func claimsValidos() jwt.MapClaims {
	return jwt.MapClaims{
		"iss":            emisor,
		"aud":            bundleDePrueba,
		"sub":            "001234.abcdef.0000",
		"exp":            time.Now().Add(10 * time.Minute).Unix(),
		"email":          "persona@ejemplo.test",
		"email_verified": "true",
	}
}

func TestTokenLegitimo(t *testing.T) {
	clave, kid, v := servidorDeClaves(t)

	identidad, err := v.Validar(context.Background(), firmar(t, clave, kid, claimsValidos()))
	if err != nil {
		t.Fatalf("un token bien firmado debe valer: %v", err)
	}
	if identidad.Sujeto != "001234.abcdef.0000" {
		t.Errorf("sujeto = %q", identidad.Sujeto)
	}
	if identidad.Correo != "persona@ejemplo.test" {
		t.Errorf("correo = %q", identidad.Correo)
	}
	// Apple manda este campo unas veces como cadena y otras como booleano.
	if !identidad.CorreoVerificado {
		t.Error(`email_verified llegó como "true" y debe leerse como verdadero`)
	}
}

// Lo que de verdad importa: lo que NO debe pasar por bueno. Antes de este
// paquete, cualquiera de estos casos entraba —el token ni se miraba—.
func TestTokensQueDebenRechazarse(t *testing.T) {
	clave, kid, v := servidorDeClaves(t)
	otraClave, _ := rsa.GenerateKey(rand.Reader, 2048)

	sinAudiencia := claimsValidos()
	delete(sinAudiencia, "aud")

	otraApp := claimsValidos()
	otraApp["aud"] = "com.otra.aplicacion"

	otroEmisor := claimsValidos()
	otroEmisor["iss"] = "https://falso.example.com"

	caducado := claimsValidos()
	caducado["exp"] = time.Now().Add(-time.Minute).Unix()

	sinCaducidad := claimsValidos()
	delete(sinCaducidad, "exp")

	sinSujeto := claimsValidos()
	delete(sinSujeto, "sub")

	casos := []struct {
		nombre string
		token  string
	}{
		{"cadena que no es un token", "no-soy-un-token"},
		{"vacío", ""},
		{"firmado con otra clave", firmar(t, otraClave, kid, claimsValidos())},
		{"sin audiencia", firmar(t, clave, kid, sinAudiencia)},
		{"emitido para otra app", firmar(t, clave, kid, otraApp)},
		{"emisor que no es Apple", firmar(t, clave, kid, otroEmisor)},
		{"caducado", firmar(t, clave, kid, caducado)},
		{"sin caducidad", firmar(t, clave, kid, sinCaducidad)},
		{"sin identificador de usuario", firmar(t, clave, kid, sinSujeto)},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if _, err := v.Validar(context.Background(), c.token); err == nil {
				t.Error("debía rechazarse y se aceptó")
			}
		})
	}
}

func TestSinBundleNoValidaNada(t *testing.T) {
	clave, kid, v := servidorDeClaves(t)
	v.bundleID = ""

	// Sin audiencia esperada no hay forma de saber para qué app es el token, así
	// que se rechaza en vez de aceptarlo a ciegas.
	if _, err := v.Validar(context.Background(), firmar(t, clave, kid, claimsValidos())); err == nil {
		t.Error("sin bundle_id configurado no debe validar nada")
	}
}

func TestCorreoOpcional(t *testing.T) {
	clave, kid, v := servidorDeClaves(t)

	sinCorreo := claimsValidos()
	delete(sinCorreo, "email")

	// Apple solo manda el correo en el primer inicio de sesión: que falte es
	// normal y no invalida el token.
	identidad, err := v.Validar(context.Background(), firmar(t, clave, kid, sinCorreo))
	if err != nil {
		t.Fatalf("un token sin correo sigue siendo válido: %v", err)
	}
	if identidad.Correo != "" {
		t.Errorf("correo = %q, se esperaba vacío", identidad.Correo)
	}
	if identidad.Sujeto == "" {
		t.Error("el sujeto es lo que identifica a la persona y debe venir")
	}
}

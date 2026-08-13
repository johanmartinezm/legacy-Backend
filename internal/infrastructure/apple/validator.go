// Package apple verifica los tokens de identidad de Sign in with Apple.
//
// Antes de esto, el backend **no los validaba**: aceptaba cualquier cadena como
// token y devolvía siempre el mismo correo ficticio. Eso convertía el botón de
// Apple en un inicio de sesión que no identificaba a nadie y, en cuanto esa
// cuenta existiera, en una vía para entrar en ella sin credenciales.
package apple

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"applegacy/backend/internal/core/ports"
)

const (
	// urlClaves publica las claves con las que Apple firma los tokens.
	urlClaves = "https://appleid.apple.com/auth/keys"

	// emisor es el único `iss` aceptable.
	emisor = "https://appleid.apple.com"

	// vidaCache: Apple rota estas claves muy de tarde en tarde, pero cuando lo
	// hace, un caché eterno dejaría de validar tokens legítimos.
	vidaCache = 6 * time.Hour
)

var (
	ErrTokenInvalido = errors.New("token de Apple inválido")
	ErrSinSujeto     = errors.New("el token de Apple no trae identificador de usuario")
)

// Identidad es lo que un token de Apple dice de quien inicia sesión.
type Identidad struct {
	// Sujeto es el claim `sub`: el **único** identificador estable. El correo
	// puede faltar o ser una dirección de retransmisión privada.
	Sujeto string

	// Correo puede venir vacío: Apple solo lo envía en el primer inicio de
	// sesión de cada persona con cada app.
	Correo string

	// CorreoVerificado lo afirma Apple. Llega como bool o como cadena "true",
	// según el caso, y por eso se normaliza aquí.
	CorreoVerificado bool

	// EsCorreoPrivado indica una dirección @privaterelay.appleid.com.
	EsCorreoPrivado bool
}

// Validador comprueba tokens de Apple contra las claves públicas del emisor.
type Validador struct {
	bundleID string
	cliente  *http.Client

	mu      sync.RWMutex
	claves  map[string]*rsa.PublicKey
	caducan time.Time
}

// NuevoValidador construye el verificador. `bundleID` es la audiencia esperada:
// el identificador de la app, `co.legacynetwork.legacyapp`. Sin comprobarlo, un
// token emitido para **otra** aplicación cualquiera valdría aquí.
func NuevoValidador(bundleID string) *Validador {
	return &Validador{
		bundleID: bundleID,
		cliente:  &http.Client{Timeout: 10 * time.Second},
		claves:   make(map[string]*rsa.PublicKey),
	}
}

// Validar comprueba firma, emisor, audiencia y caducidad, y devuelve quién es.
func (v *Validador) Validar(ctx context.Context, idToken string) (*Identidad, error) {
	if v.bundleID == "" {
		// Sin audiencia esperada no se puede comprobar para qué app se emitió
		// el token, que es justo lo que evita que valga el de otra aplicación.
		return nil, errors.New("falta apple.bundle_id en la configuración")
	}

	token, err := jwt.Parse(idToken, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			// Apple firma con RS256. Aceptar otro algoritmo abriría la puerta
			// al clásico "alg: none" y a tokens firmados con HMAC usando la
			// propia clave pública.
			return nil, fmt.Errorf("algoritmo inesperado: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("el token no indica con qué clave se firmó")
		}
		return v.clavePara(ctx, kid)
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTokenInvalido, err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalido
	}

	// Las tres comprobaciones van explícitas y con `req = true`: un token al que
	// le falte el emisor, la audiencia o la caducidad **no** se da por bueno.
	// Sin exigir la audiencia, el token de cualquier otra app de Apple entraría
	// aquí; sin el emisor, cualquiera que firme un JWT con una clave que Apple
	// publique.
	ahora := time.Now().Unix()
	if !claims.VerifyIssuer(emisor, true) {
		return nil, fmt.Errorf("%w: el emisor no es %s", ErrTokenInvalido, emisor)
	}
	if !claims.VerifyAudience(v.bundleID, true) {
		return nil, fmt.Errorf("%w: el token no se emitió para %s", ErrTokenInvalido, v.bundleID)
	}
	if !claims.VerifyExpiresAt(ahora, true) {
		return nil, fmt.Errorf("%w: caducado", ErrTokenInvalido)
	}

	sujeto, _ := claims["sub"].(string)
	if sujeto == "" {
		return nil, ErrSinSujeto
	}

	correo, _ := claims["email"].(string)

	return &Identidad{
		Sujeto:           sujeto,
		Correo:           correo,
		CorreoVerificado: comoBool(claims["email_verified"]),
		EsCorreoPrivado:  comoBool(claims["is_private_email"]),
	}, nil
}

// ValidadorParaPuerto adapta este verificador a lo que espera el servicio de
// autenticación, que no debe conocer los detalles de Apple.
type ValidadorParaPuerto struct {
	*Validador
}

func (v ValidadorParaPuerto) Validar(ctx context.Context, idToken string) (*ports.IdentidadApple, error) {
	identidad, err := v.Validador.Validar(ctx, idToken)
	if err != nil {
		return nil, err
	}
	return &ports.IdentidadApple{Sujeto: identidad.Sujeto, Correo: identidad.Correo}, nil
}

// comoBool normaliza los campos que Apple manda unas veces como booleano y
// otras como cadena.
func comoBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true"
	default:
		return false
	}
}

// clavePara devuelve la clave pública del `kid`, descargándolas si hace falta.
func (v *Validador) clavePara(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	clave, ok := v.claves[kid]
	fresco := time.Now().Before(v.caducan)
	v.mu.RUnlock()

	if ok && fresco {
		return clave, nil
	}

	// Se recargan también cuando el kid no está: Apple pudo haber rotado.
	if err := v.recargarClaves(ctx); err != nil {
		// Si la recarga falla pero teníamos la clave, se usa aunque esté vieja:
		// mejor eso que rechazar inicios de sesión legítimos porque Apple no
		// respondió en ese instante.
		if ok {
			return clave, nil
		}
		return nil, err
	}

	v.mu.RLock()
	defer v.mu.RUnlock()
	if clave, ok := v.claves[kid]; ok {
		return clave, nil
	}
	return nil, fmt.Errorf("Apple no publica ninguna clave con kid %q", kid)
}

type respuestaClaves struct {
	Keys []struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func (v *Validador) recargarClaves(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlClaves, nil)
	if err != nil {
		return err
	}

	resp, err := v.cliente.Do(req)
	if err != nil {
		return fmt.Errorf("no se pudieron descargar las claves de Apple: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Apple devolvió %d al pedir sus claves", resp.StatusCode)
	}

	var cuerpo respuestaClaves
	if err := json.NewDecoder(resp.Body).Decode(&cuerpo); err != nil {
		return fmt.Errorf("no se pudieron interpretar las claves de Apple: %w", err)
	}

	nuevas := make(map[string]*rsa.PublicKey, len(cuerpo.Keys))
	for _, k := range cuerpo.Keys {
		if k.Kty != "RSA" {
			continue
		}
		clave, err := clavePublica(k.N, k.E)
		if err != nil {
			continue
		}
		nuevas[k.Kid] = clave
	}

	if len(nuevas) == 0 {
		return errors.New("Apple no devolvió ninguna clave utilizable")
	}

	v.mu.Lock()
	v.claves = nuevas
	v.caducan = time.Now().Add(vidaCache)
	v.mu.Unlock()

	return nil
}

// clavePublica arma la clave RSA a partir del módulo y el exponente en
// base64url, que es como viaja un JWK.
func clavePublica(nB64, eB64 string) (*rsa.PublicKey, error) {
	n, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	e, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}

	exponente := 0
	for _, b := range e {
		exponente = exponente<<8 | int(b)
	}
	if exponente == 0 {
		return nil, errors.New("exponente vacío")
	}

	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponente}, nil
}

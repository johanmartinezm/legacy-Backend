package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

type CryptoService struct {
	key []byte
}

func NewCryptoService(keyString string) (*CryptoService, error) {
	// Ensure key is 32 bytes for AES-256
	if len(keyString) != 32 {
		return nil, errors.New("encryption key must be exactly 32 bytes")
	}
	return &CryptoService{key: []byte(keyString)}, nil
}

// Encrypt encrypts plain text using AES-GCM and returns a base64 encoded string
func (s *CryptoService) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64 encoded ciphertext
func (s *CryptoService) Decrypt(encodedCiphertext string) (string, error) {
	if encodedCiphertext == "" {
		return "", nil
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// BlindIndex creates a deterministic hash for searching (HMAC-SHA256)
//
// Normaliza antes de aplicar el HMAC. Hasta el 2026-08-18 hasheaba el texto tal
// cual y eso rompía por los dos lados: `Juan@Mail.com` y `juan@mail.com` daban
// índices distintos, así que **la misma persona podía registrarse dos veces** y
// **no podía iniciar sesión** si escribía su correo con otra caja —cosa normal
// en un teclado móvil, que pone la primera letra en mayúscula—.
//
// La normalización vive aquí y no en quien llama a propósito: son cuatro puntos
// —registro, inicio de sesión, solicitud de restablecimiento y restablecimiento—
// y basta que uno se olvide para que el índice no cuadre con los otros tres.
//
// Solo afecta al índice de búsqueda. El correo que se muestra sale de
// `email_encrypted`, que conserva lo que la persona escribió.
func (s *CryptoService) BlindIndex(input string) string {
	normalizado := NormalizarCorreo(input)
	if normalizado == "" {
		return ""
	}
	h := hmac.New(sha256.New, s.key)
	h.Write([]byte(normalizado))
	return hex.EncodeToString(h.Sum(nil))
}

// NormalizarCorreo deja el correo en la forma con la que se indexa: sin espacios
// alrededor y en minúsculas.
//
// El dominio es insensible a mayúsculas por norma, y la parte local lo es en la
// práctica en todos los proveedores grandes. Es una función aparte para que el
// comando que reindexa la base use exactamente la misma regla que el servidor:
// si se separaran, la base quedaría indexada de una forma y consultada de otra.
func NormalizarCorreo(correo string) string {
	return strings.ToLower(strings.TrimSpace(correo))
}

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
func (s *CryptoService) BlindIndex(input string) string {
    if input == "" {
        return ""
    }
	h := hmac.New(sha256.New, s.key)
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}

// Package secretbox provides authenticated symmetric encryption (AES-256-GCM)
// for secrets stored at rest, keyed by a server master key (KOVA_SECRET_ENC_KEY).
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

type Box struct{ gcm cipher.AEAD }

// New derives a 256-bit key from the given secret (any length) and returns a Box.
func New(key string) (*Box, error) {
	if key == "" {
		return nil, errors.New("secretbox: empty key (set KOVA_SECRET_ENC_KEY)")
	}
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{gcm: gcm}, nil
}

// Encrypt returns base64(nonce || ciphertext). Empty input returns "".
func (b *Box) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(crand.Reader, nonce); err != nil {
		return "", err
	}
	ct := b.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt reverses Encrypt. Empty input returns "".
func (b *Box) Decrypt(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	ns := b.gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("secretbox: ciphertext too short")
	}
	pt, err := b.gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

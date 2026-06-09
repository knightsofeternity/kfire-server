// Package crypto encrypts secrets (OAuth tokens) at rest with AES-256-GCM,
// using the instance master key (KFIRE_MASTER_KEY).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// Cipher seals and opens values with a single AES-256-GCM key.
type Cipher struct {
	aead cipher.AEAD
}

// New builds a Cipher from a base64-encoded 32-byte master key
// (generate with: openssl rand -base64 32).
func New(masterKeyB64 string) (*Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(masterKeyB64)
	if err != nil {
		return nil, fmt.Errorf("master key must be base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("master key must decode to 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Seal encrypts plaintext, returning nonce || ciphertext || tag.
func (c *Cipher) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// SealString is Seal for strings.
func (c *Cipher) SealString(s string) ([]byte, error) { return c.Seal([]byte(s)) }

// Open decrypts a value produced by Seal.
func (c *Cipher) Open(data []byte) ([]byte, error) {
	ns := c.aead.NonceSize()
	if len(data) < ns {
		return nil, errors.New("ciphertext too short")
	}
	return c.aead.Open(nil, data[:ns], data[ns:], nil)
}

// OpenString is Open returning a string.
func (c *Cipher) OpenString(data []byte) (string, error) {
	b, err := c.Open(data)
	return string(b), err
}

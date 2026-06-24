package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const encryptedPrefix = "v1"

type ChunkCipher struct {
	aead cipher.AEAD
}

func NewChunkCipher(base64Key string) (*ChunkCipher, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	return &ChunkCipher{aead: aead}, nil
}

func (c *ChunkCipher) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("create nonce: %w", err)
	}

	ciphertext := c.aead.Seal(nil, nonce, []byte(plaintext), nil)

	return fmt.Sprintf(
		"%s:%s:%s",
		encryptedPrefix,
		base64.RawURLEncoding.EncodeToString(nonce),
		base64.RawURLEncoding.EncodeToString(ciphertext),
	), nil
}

func (c *ChunkCipher) Decrypt(value string) (string, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 || parts[0] != encryptedPrefix {
		// Optional backwards compatibility while old plaintext chunks still exist.
		return value, nil
	}

	nonce, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	if len(nonce) != c.aead.NonceSize() {
		return "", errors.New("invalid nonce size")
	}

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt chunk: %w", err)
	}

	return string(plaintext), nil
}

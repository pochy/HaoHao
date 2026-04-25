package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var ErrInvalidSecretBoxKey = errors.New("invalid secret box key")

type SecretBox struct {
	aead       cipher.AEAD
	keyVersion int
}

func NewSecretBox(rawKey string, keyVersion int) (*SecretBox, error) {
	key, err := decodeSecretBoxKey(rawKey)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create secret cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create secret gcm: %w", err)
	}
	if keyVersion <= 0 {
		keyVersion = 1
	}
	return &SecretBox{aead: aead, keyVersion: keyVersion}, nil
}

func (b *SecretBox) KeyVersion() int {
	if b == nil || b.keyVersion <= 0 {
		return 1
	}
	return b.keyVersion
}

func (b *SecretBox) Seal(plaintext string) (string, error) {
	if b == nil || b.aead == nil {
		return "", ErrInvalidSecretBoxKey
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := b.aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, ciphertext...)
	return fmt.Sprintf("v%d:%s", b.KeyVersion(), base64.RawURLEncoding.EncodeToString(payload)), nil
}

func (b *SecretBox) Open(ciphertext string) (string, error) {
	if b == nil || b.aead == nil {
		return "", ErrInvalidSecretBoxKey
	}
	parts := strings.SplitN(strings.TrimSpace(ciphertext), ":", 2)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "v") {
		return "", ErrInvalidSecretBoxKey
	}
	if _, err := strconv.Atoi(strings.TrimPrefix(parts[0], "v")); err != nil {
		return "", ErrInvalidSecretBoxKey
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ErrInvalidSecretBoxKey
	}
	nonceSize := b.aead.NonceSize()
	if len(payload) <= nonceSize {
		return "", ErrInvalidSecretBoxKey
	}
	plaintext, err := b.aead.Open(nil, payload[:nonceSize], payload[nonceSize:], nil)
	if err != nil {
		return "", ErrInvalidSecretBoxKey
	}
	return string(plaintext), nil
}

func decodeSecretBoxKey(rawKey string) ([]byte, error) {
	trimmed := strings.TrimSpace(rawKey)
	if trimmed == "" {
		return nil, ErrInvalidSecretBoxKey
	}
	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(trimmed); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(trimmed); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(trimmed); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if len([]byte(trimmed)) == 32 {
		return []byte(trimmed), nil
	}
	return nil, ErrInvalidSecretBoxKey
}

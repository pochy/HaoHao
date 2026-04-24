package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

var (
	ErrTokenEncryptionKeyNotConfigured = errors.New("token encryption key is not configured")
	ErrInvalidTokenEncryptionKey       = errors.New("invalid token encryption key")
	ErrUnsupportedTokenKeyVersion      = errors.New("unsupported token key version")
)

type RefreshTokenStore struct {
	aead       cipher.AEAD
	keyVersion int32
}

func NewRefreshTokenStore(encodedKey string, keyVersion int) (*RefreshTokenStore, error) {
	if encodedKey == "" {
		return nil, ErrTokenEncryptionKeyNotConfigured
	}
	if keyVersion < 1 {
		return nil, fmt.Errorf("%w: key version must be positive", ErrInvalidTokenEncryptionKey)
	}

	key, err := decodeTokenEncryptionKey(encodedKey)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTokenEncryptionKey, err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTokenEncryptionKey, err)
	}

	return &RefreshTokenStore{
		aead:       aead,
		keyVersion: int32(keyVersion),
	}, nil
}

func (s *RefreshTokenStore) KeyVersion() int32 {
	return s.keyVersion
}

func (s *RefreshTokenStore) Encrypt(plaintext string) ([]byte, int32, error) {
	if s == nil || s.aead == nil {
		return nil, 0, ErrTokenEncryptionKeyNotConfigured
	}
	if plaintext == "" {
		return nil, 0, errors.New("refresh token is empty")
	}

	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, 0, fmt.Errorf("generate token nonce: %w", err)
	}

	ciphertext := s.aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := make([]byte, 0, len(nonce)+len(ciphertext))
	payload = append(payload, nonce...)
	payload = append(payload, ciphertext...)

	return payload, s.keyVersion, nil
}

func (s *RefreshTokenStore) Decrypt(ciphertext []byte, keyVersion int32) (string, error) {
	if s == nil || s.aead == nil {
		return "", ErrTokenEncryptionKeyNotConfigured
	}
	if keyVersion != s.keyVersion {
		return "", ErrUnsupportedTokenKeyVersion
	}
	if len(ciphertext) <= s.aead.NonceSize() {
		return "", errors.New("refresh token ciphertext is malformed")
	}

	nonce := ciphertext[:s.aead.NonceSize()]
	payload := ciphertext[s.aead.NonceSize():]

	plaintext, err := s.aead.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt refresh token: %w", err)
	}

	return string(plaintext), nil
}

func decodeTokenEncryptionKey(encoded string) ([]byte, error) {
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}

	var decoded []byte
	var err error
	for _, encoding := range encodings {
		decoded, err = encoding.DecodeString(encoded)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("%w: expected base64", ErrInvalidTokenEncryptionKey)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("%w: expected 32 bytes, got %d", ErrInvalidTokenEncryptionKey, len(decoded))
	}

	return decoded, nil
}

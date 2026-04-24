package auth

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"
)

func TestRefreshTokenStoreEncryptDecrypt(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	store, err := NewRefreshTokenStore(key, 3)
	if err != nil {
		t.Fatalf("NewRefreshTokenStore() error = %v", err)
	}

	ciphertext, keyVersion, err := store.Encrypt("refresh-token")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if keyVersion != 3 {
		t.Fatalf("Encrypt() keyVersion = %d, want 3", keyVersion)
	}
	if bytes.Contains(ciphertext, []byte("refresh-token")) {
		t.Fatal("ciphertext contains plaintext token")
	}

	plaintext, err := store.Decrypt(ciphertext, keyVersion)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plaintext != "refresh-token" {
		t.Fatalf("Decrypt() = %q, want refresh-token", plaintext)
	}
}

func TestRefreshTokenStoreRejectsWrongKeyVersion(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))
	store, err := NewRefreshTokenStore(key, 1)
	if err != nil {
		t.Fatalf("NewRefreshTokenStore() error = %v", err)
	}

	ciphertext, _, err := store.Encrypt("refresh-token")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	_, err = store.Decrypt(ciphertext, 2)
	if !errors.Is(err, ErrUnsupportedTokenKeyVersion) {
		t.Fatalf("Decrypt() error = %v, want ErrUnsupportedTokenKeyVersion", err)
	}
}

func TestRefreshTokenStoreRejectsInvalidKey(t *testing.T) {
	_, err := NewRefreshTokenStore(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 31)), 1)
	if !errors.Is(err, ErrInvalidTokenEncryptionKey) {
		t.Fatalf("NewRefreshTokenStore() error = %v, want ErrInvalidTokenEncryptionKey", err)
	}
}

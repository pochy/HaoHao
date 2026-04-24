package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionRecord struct {
	UserID              int64  `json:"userId"`
	CSRFToken           string `json:"csrfToken"`
	ProviderIDTokenHint string `json:"providerIdTokenHint,omitempty"`
}

type SessionStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func NewSessionStore(client *redis.Client, ttl time.Duration) *SessionStore {
	return &SessionStore{
		client: client,
		prefix: "session:",
		ttl:    ttl,
	}
}

func (s *SessionStore) Create(ctx context.Context, userID int64) (string, string, error) {
	return s.CreateWithProviderHint(ctx, userID, "")
}

func (s *SessionStore) CreateWithProviderHint(ctx context.Context, userID int64, providerIDTokenHint string) (string, string, error) {
	sessionID, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	csrfToken, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	record := SessionRecord{
		UserID:              userID,
		CSRFToken:           csrfToken,
		ProviderIDTokenHint: providerIDTokenHint,
	}
	if err := s.save(ctx, sessionID, record, s.ttl); err != nil {
		return "", "", err
	}

	return sessionID, csrfToken, nil
}

func (s *SessionStore) Get(ctx context.Context, sessionID string) (SessionRecord, error) {
	record, _, err := s.loadWithTTL(ctx, sessionID)
	return record, err
}

func (s *SessionStore) Delete(ctx context.Context, sessionID string) error {
	if err := s.client.Del(ctx, s.key(sessionID)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *SessionStore) ReissueCSRF(ctx context.Context, sessionID string) (string, error) {
	record, ttl, err := s.loadWithTTL(ctx, sessionID)
	if err != nil {
		return "", err
	}

	csrfToken, err := randomToken(32)
	if err != nil {
		return "", err
	}

	record.CSRFToken = csrfToken
	if err := s.save(ctx, sessionID, record, ttl); err != nil {
		return "", err
	}

	return csrfToken, nil
}

func (s *SessionStore) Rotate(ctx context.Context, sessionID string) (string, string, error) {
	record, _, err := s.loadWithTTL(ctx, sessionID)
	if err != nil {
		return "", "", err
	}

	newSessionID, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	newCSRFToken, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	record.CSRFToken = newCSRFToken
	if err := s.save(ctx, newSessionID, record, s.ttl); err != nil {
		return "", "", err
	}
	if err := s.Delete(ctx, sessionID); err != nil {
		return "", "", err
	}

	return newSessionID, newCSRFToken, nil
}

func (s *SessionStore) key(sessionID string) string {
	return s.prefix + sessionID
}

func (s *SessionStore) loadWithTTL(ctx context.Context, sessionID string) (SessionRecord, time.Duration, error) {
	raw, err := s.client.Get(ctx, s.key(sessionID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return SessionRecord{}, 0, ErrSessionNotFound
	}
	if err != nil {
		return SessionRecord{}, 0, fmt.Errorf("get session: %w", err)
	}

	var record SessionRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return SessionRecord{}, 0, fmt.Errorf("decode session: %w", err)
	}

	ttl, err := s.client.TTL(ctx, s.key(sessionID)).Result()
	if err != nil {
		return SessionRecord{}, 0, fmt.Errorf("get session ttl: %w", err)
	}
	if ttl <= 0 {
		ttl = s.ttl
	}

	return record, ttl, nil
}

func (s *SessionStore) save(ctx context.Context, sessionID string, record SessionRecord, ttl time.Duration) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}

	if err := s.client.Set(ctx, s.key(sessionID), payload, ttl).Err(); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	return nil
}

func randomToken(numBytes int) (string, error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

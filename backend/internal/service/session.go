package service

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

type SessionSnapshot struct {
	Authenticated bool
	AuthMode      string
	APISurface    string
}

type SessionRecord struct {
	SessionID string
	Session   StoredSession
}

type SessionPrincipal struct {
	UserID         int64
	ZitadelSubject string
	Roles          []string
	CSRFSecret     string
}

type StoredSession struct {
	UserID         int64     `json:"user_id"`
	ZitadelSubject string    `json:"zitadel_subject"`
	Roles          []string  `json:"roles"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	CSRFSecret     string    `json:"csrf_secret"`
}

type SessionStore interface {
	Save(ctx context.Context, sessionID string, session StoredSession, ttl time.Duration) error
	Get(ctx context.Context, sessionID string) (StoredSession, error)
	Delete(ctx context.Context, sessionID string) error
}

var (
	ErrSessionNotFound         = errors.New("session not found")
	ErrSessionStoreUnavailable = errors.New("session store is not configured")
)

const (
	defaultSessionTTL     = 8 * time.Hour
	sessionRedisKeyPrefix = "haohao:session:"
)

type SessionService struct {
	store SessionStore
	ttl   time.Duration
	now   func() time.Time
}

type RedisSessionStore struct {
	client *redis.Client
}

func NewSessionService(store SessionStore, ttl time.Duration) *SessionService {
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}

	return &SessionService{
		store: store,
		ttl:   ttl,
		now:   time.Now,
	}
}

func NewRedisSessionStore(redisURL string) (*RedisSessionStore, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	return &RedisSessionStore{
		client: redis.NewClient(options),
	}, nil
}

func (s *SessionService) Save(ctx context.Context, sessionID string, principal SessionPrincipal) (StoredSession, error) {
	if s.store == nil {
		return StoredSession{}, ErrSessionStoreUnavailable
	}
	if sessionID == "" {
		return StoredSession{}, errors.New("session id is required")
	}
	if principal.UserID == 0 {
		return StoredSession{}, errors.New("user id is required")
	}
	if principal.ZitadelSubject == "" {
		return StoredSession{}, errors.New("zitadel subject is required")
	}

	now := s.now().UTC()
	csrfSecret := principal.CSRFSecret
	if csrfSecret == "" {
		csrfSecret = s.NewCSRFCookieValue()
	}

	session := StoredSession{
		UserID:         principal.UserID,
		ZitadelSubject: principal.ZitadelSubject,
		Roles:          append([]string(nil), principal.Roles...),
		CreatedAt:      now,
		ExpiresAt:      now.Add(s.ttl),
		CSRFSecret:     csrfSecret,
	}

	if err := s.store.Save(ctx, sessionID, session, s.ttl); err != nil {
		return StoredSession{}, err
	}

	return session, nil
}

func (s *SessionService) Create(ctx context.Context, principal SessionPrincipal) (SessionRecord, error) {
	sessionID := s.NewSessionID()
	if sessionID == "" {
		return SessionRecord{}, errors.New("generate session id")
	}

	session, err := s.Save(ctx, sessionID, principal)
	if err != nil {
		return SessionRecord{}, err
	}

	return SessionRecord{
		SessionID: sessionID,
		Session:   session,
	}, nil
}

func (s *SessionService) Get(ctx context.Context, sessionID string) (StoredSession, error) {
	if s.store == nil {
		return StoredSession{}, ErrSessionStoreUnavailable
	}

	return s.store.Get(ctx, sessionID)
}

func (s *SessionService) Delete(ctx context.Context, sessionID string) error {
	if s.store == nil {
		return ErrSessionStoreUnavailable
	}

	return s.store.Delete(ctx, sessionID)
}

func (s *SessionService) Rotate(ctx context.Context, sessionID string) (SessionRecord, error) {
	if s.store == nil {
		return SessionRecord{}, ErrSessionStoreUnavailable
	}
	if sessionID == "" {
		return SessionRecord{}, errors.New("session id is required")
	}

	current, err := s.store.Get(ctx, sessionID)
	if err != nil {
		return SessionRecord{}, err
	}

	now := s.now().UTC()
	remaining := current.ExpiresAt.Sub(now)
	if remaining <= 0 {
		_ = s.store.Delete(ctx, sessionID)
		return SessionRecord{}, ErrSessionNotFound
	}

	newSessionID := s.NewSessionID()
	if newSessionID == "" {
		return SessionRecord{}, errors.New("generate session id")
	}

	if err := s.store.Save(ctx, newSessionID, current, remaining); err != nil {
		return SessionRecord{}, err
	}
	if err := s.store.Delete(ctx, sessionID); err != nil {
		_ = s.store.Delete(ctx, newSessionID)
		return SessionRecord{}, err
	}

	return SessionRecord{
		SessionID: newSessionID,
		Session:   current,
	}, nil
}

func (s *RedisSessionStore) Save(ctx context.Context, sessionID string, session StoredSession, ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("session ttl must be positive")
	}

	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	if err := s.client.Set(ctx, sessionRedisKey(sessionID), payload, ttl).Err(); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	return nil
}

func (s *RedisSessionStore) Get(ctx context.Context, sessionID string) (StoredSession, error) {
	payload, err := s.client.Get(ctx, sessionRedisKey(sessionID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return StoredSession{}, ErrSessionNotFound
	}
	if err != nil {
		return StoredSession{}, fmt.Errorf("get session: %w", err)
	}

	var session StoredSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return StoredSession{}, fmt.Errorf("unmarshal session: %w", err)
	}

	return session, nil
}

func (s *RedisSessionStore) Delete(ctx context.Context, sessionID string) error {
	if err := s.client.Del(ctx, sessionRedisKey(sessionID)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

func (s *SessionService) Snapshot(_ context.Context) SessionSnapshot {
	return SessionSnapshot{
		Authenticated: false,
		AuthMode:      "stub",
		APISurface:    "browser",
	}
}

func (s *SessionService) NewCSRFCookieValue() string {
	return newRandomHex(16, "csrf-placeholder")
}

func (s *SessionService) NewSessionID() string {
	return newRandomHex(32, "")
}

func newRandomHex(size int, fallback string) string {
	if size <= 0 {
		return fallback
	}

	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return fallback
	}

	return hex.EncodeToString(buf)
}

func sessionRedisKey(sessionID string) string {
	return sessionRedisKeyPrefix + sessionID
}

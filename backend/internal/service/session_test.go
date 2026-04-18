package service

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

type memorySessionStore struct {
	sessions map[string]StoredSession
	ttls     map[string]time.Duration
}

func newMemorySessionStore() *memorySessionStore {
	return &memorySessionStore{
		sessions: make(map[string]StoredSession),
		ttls:     make(map[string]time.Duration),
	}
}

func (s *memorySessionStore) Save(_ context.Context, sessionID string, session StoredSession, ttl time.Duration) error {
	s.sessions[sessionID] = session
	s.ttls[sessionID] = ttl
	return nil
}

func (s *memorySessionStore) Get(_ context.Context, sessionID string) (StoredSession, error) {
	session, ok := s.sessions[sessionID]
	if !ok {
		return StoredSession{}, ErrSessionNotFound
	}

	return session, nil
}

func (s *memorySessionStore) Delete(_ context.Context, sessionID string) error {
	delete(s.sessions, sessionID)
	delete(s.ttls, sessionID)
	return nil
}

func TestSessionServiceSaveGetDelete(t *testing.T) {
	store := newMemorySessionStore()
	svc := NewSessionService(store, 8*time.Hour)
	fixedNow := time.Date(2026, time.April, 18, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixedNow }

	session, err := svc.Save(context.Background(), "session-1", SessionPrincipal{
		UserID:         42,
		ZitadelSubject: "zitadel-user-42",
		Roles:          []string{"app:user", "docs:read"},
	})
	if err != nil {
		t.Fatalf("save session: %v", err)
	}

	if session.UserID != 42 {
		t.Fatalf("user_id = %d, want %d", session.UserID, 42)
	}
	if session.ZitadelSubject != "zitadel-user-42" {
		t.Fatalf("zitadel_subject = %q, want %q", session.ZitadelSubject, "zitadel-user-42")
	}
	if len(session.Roles) != 2 {
		t.Fatalf("roles = %#v, want two roles", session.Roles)
	}
	if session.CreatedAt != fixedNow {
		t.Fatalf("created_at = %v, want %v", session.CreatedAt, fixedNow)
	}
	if session.ExpiresAt != fixedNow.Add(8*time.Hour) {
		t.Fatalf("expires_at = %v, want %v", session.ExpiresAt, fixedNow.Add(8*time.Hour))
	}
	if session.CSRFSecret == "" {
		t.Fatal("csrf_secret = empty, want random secret")
	}
	if store.ttls["session-1"] != 8*time.Hour {
		t.Fatalf("ttl = %v, want %v", store.ttls["session-1"], 8*time.Hour)
	}

	got, err := svc.Get(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if !reflect.DeepEqual(got, session) {
		t.Fatalf("got session = %#v, want %#v", got, session)
	}

	if err := svc.Delete(context.Background(), "session-1"); err != nil {
		t.Fatalf("delete session: %v", err)
	}

	_, err = svc.Get(context.Background(), "session-1")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("get deleted session error = %v, want %v", err, ErrSessionNotFound)
	}
}

func TestRedisSessionStoreRoundTrip(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(server.Close)

	store, err := NewRedisSessionStore("redis://" + server.Addr() + "/0")
	if err != nil {
		t.Fatalf("new redis session store: %v", err)
	}

	session := StoredSession{
		UserID:         7,
		ZitadelSubject: "zitadel-sub-7",
		Roles:          []string{"app:user"},
		CreatedAt:      time.Date(2026, time.April, 18, 11, 0, 0, 0, time.UTC),
		ExpiresAt:      time.Date(2026, time.April, 18, 19, 0, 0, 0, time.UTC),
		CSRFSecret:     "csrf-secret",
	}

	if err := store.Save(context.Background(), "session-redis", session, 8*time.Hour); err != nil {
		t.Fatalf("save session: %v", err)
	}

	raw, err := server.Get("haohao:session:session-redis")
	if err != nil {
		t.Fatalf("read raw redis key: %v", err)
	}
	if !strings.Contains(raw, "\"zitadel_subject\":\"zitadel-sub-7\"") {
		t.Fatalf("raw payload = %q, want zitadel_subject field", raw)
	}

	got, err := store.Get(context.Background(), "session-redis")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if !reflect.DeepEqual(got, session) {
		t.Fatalf("got session = %#v, want %#v", got, session)
	}

	server.FastForward(8*time.Hour + time.Second)
	_, err = store.Get(context.Background(), "session-redis")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expired session error = %v, want %v", err, ErrSessionNotFound)
	}
}

func TestRedisSessionStoreAgainstComposeRedis(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set")
	}

	store, err := NewRedisSessionStore(redisURL)
	if err != nil {
		t.Fatalf("new redis session store: %v", err)
	}

	session := StoredSession{
		UserID:         99,
		ZitadelSubject: "compose-smoke",
		Roles:          []string{"app:user"},
		CreatedAt:      time.Now().UTC(),
		ExpiresAt:      time.Now().UTC().Add(time.Minute),
		CSRFSecret:     "compose-smoke-csrf",
	}

	if err := store.Delete(context.Background(), "compose-smoke"); err != nil {
		t.Fatalf("cleanup before save: %v", err)
	}
	if err := store.Save(context.Background(), "compose-smoke", session, time.Minute); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if _, err := store.Get(context.Background(), "compose-smoke"); err != nil {
		t.Fatalf("get session: %v", err)
	}
	if err := store.Delete(context.Background(), "compose-smoke"); err != nil {
		t.Fatalf("delete session: %v", err)
	}
}

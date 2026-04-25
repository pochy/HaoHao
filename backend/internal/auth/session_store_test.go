package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSessionStoreIndexesAndDeletesByUser(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := NewSessionStore(client, time.Hour)

	sessionID, _, err := store.Create(ctx, 42)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := store.Get(ctx, sessionID); err != nil {
		t.Fatalf("Get() after create error = %v", err)
	}

	if err := store.DeleteUserSessions(ctx, 42); err != nil {
		t.Fatalf("DeleteUserSessions() error = %v", err)
	}
	if _, err := store.Get(ctx, sessionID); err != ErrSessionNotFound {
		t.Fatalf("Get() after DeleteUserSessions error = %v, want %v", err, ErrSessionNotFound)
	}
}

func TestSessionStoreRotateUpdatesUserIndex(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := NewSessionStore(client, time.Hour)

	oldSessionID, _, err := store.Create(ctx, 42)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	newSessionID, _, err := store.Rotate(ctx, oldSessionID)
	if err != nil {
		t.Fatalf("Rotate() error = %v", err)
	}

	if _, err := store.Get(ctx, oldSessionID); err != ErrSessionNotFound {
		t.Fatalf("old session Get() error = %v, want %v", err, ErrSessionNotFound)
	}
	if _, err := store.Get(ctx, newSessionID); err != nil {
		t.Fatalf("new session Get() error = %v", err)
	}
	if err := store.DeleteUserSessions(ctx, 42); err != nil {
		t.Fatalf("DeleteUserSessions() error = %v", err)
	}
	if _, err := store.Get(ctx, newSessionID); err != ErrSessionNotFound {
		t.Fatalf("new session after DeleteUserSessions error = %v, want %v", err, ErrSessionNotFound)
	}
}

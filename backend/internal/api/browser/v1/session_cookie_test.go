package v1

import (
	"net/http"
	"testing"
	"time"

	"github.com/pochy/haohao/backend/internal/config"
	"github.com/pochy/haohao/backend/internal/service"
)

func TestSessionCookieManagerBuildSessionCookie(t *testing.T) {
	fixedNow := time.Date(2026, time.April, 18, 15, 0, 0, 0, time.UTC)
	manager := NewSessionCookieManager(config.Config{
		SessionCookieName:     "SESSION_ID",
		SessionCookiePath:     "/",
		SessionCookieSameSite: "Strict",
		SessionCookieSecure:   true,
	})
	manager.now = func() time.Time { return fixedNow }

	cookie := manager.BuildSessionCookie(service.SessionRecord{
		SessionID: "session-123",
		Session: service.StoredSession{
			ExpiresAt: fixedNow.Add(90 * time.Minute),
		},
	})

	if cookie.Name != "SESSION_ID" {
		t.Fatalf("name = %q, want SESSION_ID", cookie.Name)
	}
	if cookie.Value != "session-123" {
		t.Fatalf("value = %q, want session-123", cookie.Value)
	}
	if cookie.Path != "/" {
		t.Fatalf("path = %q, want /", cookie.Path)
	}
	if !cookie.HttpOnly {
		t.Fatal("HttpOnly = false, want true")
	}
	if !cookie.Secure {
		t.Fatal("Secure = false, want true")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("sameSite = %v, want %v", cookie.SameSite, http.SameSiteStrictMode)
	}
	if cookie.MaxAge != 5400 {
		t.Fatalf("maxAge = %d, want 5400", cookie.MaxAge)
	}
	if !cookie.Expires.Equal(fixedNow.Add(90 * time.Minute)) {
		t.Fatalf("expires = %v, want %v", cookie.Expires, fixedNow.Add(90*time.Minute))
	}
}

func TestSessionCookieManagerBuildDeleteSessionCookie(t *testing.T) {
	manager := NewSessionCookieManager(config.Config{
		SessionCookieName:     "SESSION_ID",
		SessionCookiePath:     "/",
		SessionCookieSameSite: "Lax",
		SessionCookieSecure:   false,
	})

	cookie := manager.BuildDeleteSessionCookie()

	if cookie.Name != "SESSION_ID" {
		t.Fatalf("name = %q, want SESSION_ID", cookie.Name)
	}
	if cookie.Value != "" {
		t.Fatalf("value = %q, want empty", cookie.Value)
	}
	if cookie.MaxAge != -1 {
		t.Fatalf("maxAge = %d, want -1", cookie.MaxAge)
	}
	if !cookie.Expires.Equal(time.Unix(0, 0).UTC()) {
		t.Fatalf("expires = %v, want unix epoch", cookie.Expires)
	}
	if !cookie.HttpOnly {
		t.Fatal("HttpOnly = false, want true")
	}
}

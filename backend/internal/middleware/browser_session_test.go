package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pochy/haohao/backend/internal/config"
	"github.com/pochy/haohao/backend/internal/service"
)

func TestLoadBrowserSessionAnonymousWithoutCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	sessions := service.NewSessionService(nil, 8*time.Hour)
	router.Use(LoadBrowserSession(config.Config{SessionCookieName: "SESSION_ID"}, sessions))
	router.GET("/test", func(c *gin.Context) {
		writeBrowserSession(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var body sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Authenticated {
		t.Fatal("authenticated = true, want false")
	}
	if body.SessionID != "" {
		t.Fatalf("sessionID = %q, want empty", body.SessionID)
	}
	if body.LoadErr != "" {
		t.Fatalf("loadErr = %q, want empty", body.LoadErr)
	}
}

func TestLoadBrowserSessionAuthenticatedFromCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := newMiddlewareMemorySessionStore()
	sessions := service.NewSessionService(store, 8*time.Hour)
	if _, err := sessions.Save(context.Background(), "session-1", service.SessionPrincipal{
		UserID:         42,
		ZitadelSubject: "zitadel-42",
		Roles:          []string{"app:user"},
		CSRFSecret:     "csrf",
	}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	router := gin.New()
	router.Use(LoadBrowserSession(config.Config{SessionCookieName: "SESSION_ID"}, sessions))
	router.GET("/test", func(c *gin.Context) {
		writeBrowserSession(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "SESSION_ID", Value: "session-1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var body sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Authenticated {
		t.Fatal("authenticated = false, want true")
	}
	if body.SessionID != "session-1" {
		t.Fatalf("sessionID = %q, want session-1", body.SessionID)
	}
	if body.UserID != 42 {
		t.Fatalf("userID = %d, want 42", body.UserID)
	}
}

func TestLoadBrowserSessionTreatsMissingStoreSessionAsAnonymous(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	sessions := service.NewSessionService(newMiddlewareMemorySessionStore(), 8*time.Hour)
	router.Use(LoadBrowserSession(config.Config{SessionCookieName: "SESSION_ID"}, sessions))
	router.GET("/test", func(c *gin.Context) {
		writeBrowserSession(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "SESSION_ID", Value: "missing"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var body sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Authenticated {
		t.Fatal("authenticated = true, want false")
	}
	if body.LoadErr != "" {
		t.Fatalf("loadErr = %q, want empty", body.LoadErr)
	}
}

func TestLoadBrowserSessionExposesStoreErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	sessions := service.NewSessionService(nil, 8*time.Hour)
	router.Use(LoadBrowserSession(config.Config{SessionCookieName: "SESSION_ID"}, sessions))
	router.GET("/test", func(c *gin.Context) {
		writeBrowserSession(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "SESSION_ID", Value: "session-1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var body sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.LoadErr == "" {
		t.Fatal("loadErr = empty, want session store error")
	}
}

type sessionResponse struct {
	Authenticated bool   `json:"authenticated"`
	SessionID     string `json:"sessionId"`
	UserID        int64  `json:"userId"`
	LoadErr       string `json:"loadErr"`
}

type middlewareMemorySessionStore struct {
	sessions map[string]service.StoredSession
}

func newMiddlewareMemorySessionStore() *middlewareMemorySessionStore {
	return &middlewareMemorySessionStore{
		sessions: make(map[string]service.StoredSession),
	}
}

func (s *middlewareMemorySessionStore) Save(_ context.Context, sessionID string, session service.StoredSession, _ time.Duration) error {
	s.sessions[sessionID] = session
	return nil
}

func (s *middlewareMemorySessionStore) Get(_ context.Context, sessionID string) (service.StoredSession, error) {
	session, ok := s.sessions[sessionID]
	if !ok {
		return service.StoredSession{}, service.ErrSessionNotFound
	}

	return session, nil
}

func (s *middlewareMemorySessionStore) Delete(_ context.Context, sessionID string) error {
	delete(s.sessions, sessionID)
	return nil
}

func writeBrowserSession(c *gin.Context) {
	fromGin := BrowserSessionFromGin(c)
	fromContext := BrowserSessionFromContext(c.Request.Context())
	if fromGin.Authenticated != fromContext.Authenticated ||
		fromGin.SessionID != fromContext.SessionID ||
		fromGin.Session.UserID != fromContext.Session.UserID ||
		errorString(fromGin.LoadErr) != errorString(fromContext.LoadErr) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "context mismatch"})
		return
	}

	c.JSON(http.StatusOK, sessionResponse{
		Authenticated: fromContext.Authenticated,
		SessionID:     fromContext.SessionID,
		UserID:        fromContext.Session.UserID,
		LoadErr:       errorString(fromContext.LoadErr),
	})
}

func errorString(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}

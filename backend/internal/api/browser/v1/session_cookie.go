package v1

import (
	"math"
	"net/http"
	"time"

	"github.com/pochy/haohao/backend/internal/config"
	"github.com/pochy/haohao/backend/internal/service"
)

type SessionCookieManager struct {
	cfg config.Config
	now func() time.Time
}

func NewSessionCookieManager(cfg config.Config) SessionCookieManager {
	return SessionCookieManager{
		cfg: cfg,
		now: time.Now,
	}
}

func (m SessionCookieManager) BuildSessionCookie(record service.SessionRecord) http.Cookie {
	expiresAt := record.Session.ExpiresAt.UTC()
	maxAge := int(math.Ceil(expiresAt.Sub(m.now().UTC()).Seconds()))
	if maxAge < 0 {
		maxAge = 0
	}

	return http.Cookie{
		Name:     m.cfg.SessionCookieName,
		Value:    record.SessionID,
		Path:     m.cfg.SessionCookiePath,
		HttpOnly: true,
		Secure:   m.cfg.SessionCookieSecure,
		SameSite: m.cfg.SessionCookieSameSiteMode(),
		MaxAge:   maxAge,
		Expires:  expiresAt,
	}
}

func (m SessionCookieManager) BuildDeleteSessionCookie() http.Cookie {
	return http.Cookie{
		Name:     m.cfg.SessionCookieName,
		Value:    "",
		Path:     m.cfg.SessionCookiePath,
		HttpOnly: true,
		Secure:   m.cfg.SessionCookieSecure,
		SameSite: m.cfg.SessionCookieSameSiteMode(),
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
	}
}

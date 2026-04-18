package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pochy/haohao/backend/internal/config"
	"github.com/pochy/haohao/backend/internal/service"
)

type BrowserSession struct {
	Authenticated bool
	SessionID     string
	Session       service.StoredSession
	LoadErr       error
}

type browserSessionContextKey struct{}

const browserSessionGinKey = "haohao.browser_session"

func LoadBrowserSession(cfg config.Config, sessions *service.SessionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		browserSession := BrowserSession{}

		cookie, err := c.Request.Cookie(cfg.SessionCookieName)
		switch {
		case errors.Is(err, http.ErrNoCookie):
			setBrowserSession(c, browserSession)
			c.Next()
			return
		case err != nil:
			browserSession.LoadErr = err
			setBrowserSession(c, browserSession)
			c.Next()
			return
		case cookie.Value == "":
			setBrowserSession(c, browserSession)
			c.Next()
			return
		}

		if sessions == nil {
			browserSession.LoadErr = service.ErrSessionStoreUnavailable
			setBrowserSession(c, browserSession)
			c.Next()
			return
		}

		session, err := sessions.Get(c.Request.Context(), cookie.Value)
		switch {
		case err == nil:
			browserSession = BrowserSession{
				Authenticated: true,
				SessionID:     cookie.Value,
				Session:       session,
			}
		case errors.Is(err, service.ErrSessionNotFound):
		default:
			browserSession.LoadErr = err
		}

		setBrowserSession(c, browserSession)
		c.Next()
	}
}

func BrowserSessionFromContext(ctx context.Context) BrowserSession {
	browserSession, ok := ctx.Value(browserSessionContextKey{}).(BrowserSession)
	if !ok {
		return BrowserSession{}
	}

	return browserSession
}

func BrowserSessionFromGin(c *gin.Context) BrowserSession {
	value, ok := c.Get(browserSessionGinKey)
	if !ok {
		return BrowserSession{}
	}

	browserSession, ok := value.(BrowserSession)
	if !ok {
		return BrowserSession{}
	}

	return browserSession
}

func setBrowserSession(c *gin.Context, browserSession BrowserSession) {
	ctx := context.WithValue(c.Request.Context(), browserSessionContextKey{}, browserSession)
	c.Request = c.Request.WithContext(ctx)
	c.Set(browserSessionGinKey, browserSession)
}

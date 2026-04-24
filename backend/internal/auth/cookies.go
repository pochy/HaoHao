package auth

import (
	"net/http"
	"time"
)

const (
	SessionCookieName = "SESSION_ID"
	XSRFCookieName    = "XSRF-TOKEN"
)

func NewSessionCookie(value string, secure bool, ttl time.Duration) http.Cookie {
	return http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	}
}

func NewXSRFCookie(value string, secure bool, ttl time.Duration) http.Cookie {
	return http.Cookie{
		Name:     XSRFCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	}
}

func ExpiredSessionCookie(secure bool) http.Cookie {
	return http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
}

func ExpiredXSRFCookie(secure bool) http.Cookie {
	return http.Cookie{
		Name:     XSRFCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
}


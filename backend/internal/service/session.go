package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type SessionSnapshot struct {
	Authenticated bool
	AuthMode      string
	APISurface    string
}

type SessionService struct{}

func NewSessionService() *SessionService {
	return &SessionService{}
}

func (s *SessionService) Snapshot(_ context.Context) SessionSnapshot {
	return SessionSnapshot{
		Authenticated: false,
		AuthMode:      "stub",
		APISurface:    "browser",
	}
}

func (s *SessionService) NewCSRFCookieValue() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "csrf-placeholder"
	}

	return hex.EncodeToString(buf)
}


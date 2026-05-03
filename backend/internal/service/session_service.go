package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"

	"example.com/haohao/backend/internal/auth"
	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInvalidCSRFToken    = errors.New("invalid csrf token")
	ErrAuthModeUnsupported = errors.New("auth mode unsupported")
)

type User struct {
	ID          int64
	PublicID    string
	Email       string
	DisplayName string
}

type SessionService struct {
	queries  *db.Queries
	store    *auth.SessionStore
	authMode string
}

func NewSessionService(queries *db.Queries, store *auth.SessionStore, authMode string) *SessionService {
	return &SessionService{
		queries:  queries,
		store:    store,
		authMode: strings.ToLower(strings.TrimSpace(authMode)),
	}
}

func (s *SessionService) Login(ctx context.Context, email, password string) (User, string, string, error) {
	if s.authMode == "zitadel" {
		return User{}, "", "", ErrAuthModeUnsupported
	}

	userID, err := s.queries.AuthenticateUser(ctx, db.AuthenticateUserParams{
		Email:    email,
		Password: password,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", "", fmt.Errorf("authenticate user: %w", err)
	}

	user, err := s.loadUserByID(ctx, userID)
	if err != nil {
		return User{}, "", "", err
	}

	sessionID, csrfToken, err := s.IssueSession(ctx, userID)
	if err != nil {
		return User{}, "", "", err
	}

	return user, sessionID, csrfToken, nil
}

func (s *SessionService) CurrentUser(ctx context.Context, sessionID string) (User, error) {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, err
	}

	return s.loadUserByID(ctx, session.UserID)
}

func (s *SessionService) IssueSession(ctx context.Context, userID int64) (string, string, error) {
	return s.IssueSessionWithProviderHint(ctx, userID, "")
}

func (s *SessionService) IssueSessionWithProviderHint(ctx context.Context, userID int64, providerIDTokenHint string) (string, string, error) {
	sessionID, csrfToken, err := s.store.CreateWithProviderHint(ctx, userID, providerIDTokenHint)
	if err != nil {
		return "", "", fmt.Errorf("create session: %w", err)
	}
	return sessionID, csrfToken, nil
}

func (s *SessionService) Logout(ctx context.Context, sessionID, csrfHeader string) (string, error) {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", ErrUnauthorized
	}
	if err != nil {
		return "", err
	}

	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(csrfHeader)) != 1 {
		return "", ErrInvalidCSRFToken
	}

	if err := s.store.Delete(ctx, sessionID); err != nil {
		return "", err
	}

	return session.ProviderIDTokenHint, nil
}

func (s *SessionService) ReissueCSRF(ctx context.Context, sessionID string) (string, error) {
	if _, err := s.CurrentUser(ctx, sessionID); err != nil {
		return "", err
	}

	csrfToken, err := s.store.ReissueCSRF(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", ErrUnauthorized
	}
	if err != nil {
		return "", err
	}

	return csrfToken, nil
}

func (s *SessionService) RefreshSession(ctx context.Context, sessionID, csrfHeader string) (string, string, error) {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", "", ErrUnauthorized
	}
	if err != nil {
		return "", "", err
	}

	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(csrfHeader)) != 1 {
		return "", "", ErrInvalidCSRFToken
	}

	newSessionID, newCSRFToken, err := s.store.Rotate(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", "", ErrUnauthorized
	}
	if err != nil {
		return "", "", err
	}

	return newSessionID, newCSRFToken, nil
}

func (s *SessionService) loadUserByID(ctx context.Context, userID int64) (User, error) {
	record, err := s.queries.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, fmt.Errorf("load user by session: %w", err)
	}

	return User{
		ID:          record.ID,
		PublicID:    record.PublicID.String(),
		Email:       record.Email,
		DisplayName: record.DisplayName,
	}, nil
}

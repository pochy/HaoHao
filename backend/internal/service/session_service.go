package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"example.com/haohao/backend/internal/auth"
	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInvalidCSRFToken    = errors.New("invalid csrf token")
	ErrAuthModeUnsupported = errors.New("auth mode unsupported")
)

type User struct {
	ID              int64
	PublicID        string
	Email           string
	DisplayName     string
	DeactivatedAt   *time.Time
	DefaultTenantID *int64
}

type CurrentSession struct {
	User           User
	ActorUser      *User
	ActiveTenantID *int64
	SupportAccess  *SupportAccess
}

type SessionService struct {
	queries                  *db.Queries
	store                    *auth.SessionStore
	authMode                 string
	enableLocalPasswordLogin bool
	audit                    AuditRecorder
}

func NewSessionService(queries *db.Queries, store *auth.SessionStore, authMode string, enableLocalPasswordLogin bool, audit AuditRecorder) *SessionService {
	return &SessionService{
		queries:                  queries,
		store:                    store,
		authMode:                 strings.ToLower(strings.TrimSpace(authMode)),
		enableLocalPasswordLogin: enableLocalPasswordLogin,
		audit:                    audit,
	}
}

func (s *SessionService) Login(ctx context.Context, email, password string, auditRequest AuditRequest) (User, string, string, error) {
	if !s.enableLocalPasswordLogin || s.authMode == "zitadel" {
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

	if s.audit != nil {
		if err := s.audit.Record(ctx, AuditEventInput{
			AuditContext: UserAuditContext(user.ID, user.DefaultTenantID, auditRequest),
			Action:       "session.login",
			TargetType:   "session",
			TargetID:     "browser",
		}); err != nil {
			_ = s.store.Delete(ctx, sessionID)
			return User{}, "", "", err
		}
	}

	return user, sessionID, csrfToken, nil
}

func (s *SessionService) CurrentUser(ctx context.Context, sessionID string) (User, error) {
	current, err := s.CurrentSession(ctx, sessionID)
	if err != nil {
		return User{}, err
	}
	return current.User, nil
}

func (s *SessionService) CurrentSession(ctx context.Context, sessionID string) (CurrentSession, error) {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return CurrentSession{}, ErrUnauthorized
	}
	if err != nil {
		return CurrentSession{}, err
	}

	return s.currentSessionFromRecord(ctx, sessionID, session)
}

func (s *SessionService) CurrentUserWithCSRF(ctx context.Context, sessionID, csrfHeader string) (User, error) {
	current, err := s.CurrentSessionWithCSRF(ctx, sessionID, csrfHeader)
	if err != nil {
		return User{}, err
	}
	return current.User, nil
}

func (s *SessionService) CurrentSessionWithCSRF(ctx context.Context, sessionID, csrfHeader string) (CurrentSession, error) {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return CurrentSession{}, ErrUnauthorized
	}
	if err != nil {
		return CurrentSession{}, err
	}

	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(csrfHeader)) != 1 {
		return CurrentSession{}, ErrInvalidCSRFToken
	}

	return s.currentSessionFromRecord(ctx, sessionID, session)
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

func (s *SessionService) Logout(ctx context.Context, sessionID, csrfHeader string, auditRequest AuditRequest) (string, error) {
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

	user, userErr := s.loadUserByID(ctx, session.UserID)
	if err := s.store.Delete(ctx, sessionID); err != nil {
		return "", err
	}

	if userErr == nil && s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: UserAuditContext(user.ID, sessionAuditTenantID(user, session.ActiveTenantID), auditRequest),
			Action:       "session.logout",
			TargetType:   "session",
			TargetID:     "browser",
		})
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

func (s *SessionService) SetActiveTenant(ctx context.Context, sessionID, csrfHeader string, tenantID int64, auditRequest AuditRequest) error {
	current, err := s.CurrentSessionWithCSRF(ctx, sessionID, csrfHeader)
	if err != nil {
		return err
	}
	if err := s.store.SetActiveTenant(ctx, sessionID, tenantID); err != nil {
		return err
	}
	if s.audit != nil {
		if err := s.audit.Record(ctx, AuditEventInput{
			AuditContext: UserAuditContext(current.User.ID, &tenantID, auditRequest),
			Action:       "session.tenant_switch",
			TargetType:   "tenant",
			TargetID:     strconv.FormatInt(tenantID, 10),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *SessionService) SetSupportAccessSession(ctx context.Context, sessionID string, supportAccessID, tenantID int64) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("session service is not configured")
	}
	return s.store.SetSupportAccess(ctx, sessionID, supportAccessID, tenantID)
}

func (s *SessionService) ClearSupportAccessSession(ctx context.Context, sessionID string) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("session service is not configured")
	}
	return s.store.ClearSupportAccess(ctx, sessionID)
}

func (s *SessionService) DeleteUserSessions(ctx context.Context, userID int64) error {
	return s.store.DeleteUserSessions(ctx, userID)
}

func (s *SessionService) RefreshSession(ctx context.Context, sessionID, csrfHeader string, auditRequest AuditRequest) (string, string, error) {
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

	user, err := s.loadUserByID(ctx, session.UserID)
	if err != nil {
		return "", "", err
	}

	newSessionID, newCSRFToken, err := s.store.Rotate(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", "", ErrUnauthorized
	}
	if err != nil {
		return "", "", err
	}

	if s.audit != nil {
		if err := s.audit.Record(ctx, AuditEventInput{
			AuditContext: UserAuditContext(user.ID, sessionAuditTenantID(user, session.ActiveTenantID), auditRequest),
			Action:       "session.refresh",
			TargetType:   "session",
			TargetID:     "browser",
		}); err != nil {
			_ = s.store.Delete(ctx, newSessionID)
			return "", "", err
		}
	}

	return newSessionID, newCSRFToken, nil
}

func (s *SessionService) currentSessionFromRecord(ctx context.Context, sessionID string, session auth.SessionRecord) (CurrentSession, error) {
	user, err := s.loadUserByID(ctx, session.UserID)
	if err != nil {
		return CurrentSession{}, err
	}
	current := CurrentSession{
		User:           user,
		ActiveTenantID: optionalInt64(session.ActiveTenantID),
	}
	if session.SupportAccessID <= 0 {
		return current, nil
	}
	row, err := s.queries.GetSupportAccessSessionByID(ctx, session.SupportAccessID)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = s.store.ClearSupportAccess(ctx, sessionID)
		return current, nil
	}
	if err != nil {
		return CurrentSession{}, err
	}
	if row.Status != "active" || timestamptzTime(row.ExpiresAt).Before(time.Now()) || row.SupportUserID != session.UserID {
		if row.Status == "active" {
			_, _ = s.queries.ExpireSupportAccessSession(ctx, row.ID)
		}
		_ = s.store.ClearSupportAccess(ctx, sessionID)
		return current, nil
	}
	impersonated, err := s.loadUserByID(ctx, row.ImpersonatedUserID)
	if err != nil {
		_ = s.store.ClearSupportAccess(ctx, sessionID)
		return current, nil
	}
	supportAccess := supportAccessFromSessionRow(row)
	current.ActorUser = &user
	current.User = impersonated
	current.ActiveTenantID = &row.TenantID
	current.SupportAccess = supportAccess
	return current, nil
}

func (s *SessionService) loadUserByID(ctx context.Context, userID int64) (User, error) {
	record, err := s.queries.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, fmt.Errorf("load user by session: %w", err)
	}
	if record.DeactivatedAt.Valid {
		return User{}, ErrUnauthorized
	}

	return User{
		ID:              record.ID,
		PublicID:        record.PublicID.String(),
		Email:           record.Email,
		DisplayName:     record.DisplayName,
		DefaultTenantID: optionalPgInt8(record.DefaultTenantID),
	}, nil
}

func optionalInt64(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}

func optionalPgInt8(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}

func sessionAuditTenantID(user User, activeTenantID int64) *int64 {
	if activeTenantID != 0 {
		return &activeTenantID
	}
	return user.DefaultTenantID
}

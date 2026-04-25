package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"example.com/haohao/backend/internal/auth"
)

type OIDCLoginResult struct {
	SessionID string
	CSRFToken string
	ReturnTo  string
}

type OIDCLoginService struct {
	providerName   string
	oidcClient     *auth.OIDCClient
	loginState     *auth.LoginStateStore
	identity       *IdentityService
	authzService   *AuthzService
	sessionService *SessionService
}

func NewOIDCLoginService(providerName string, oidcClient *auth.OIDCClient, loginState *auth.LoginStateStore, identity *IdentityService, authzService *AuthzService, sessionService *SessionService) *OIDCLoginService {
	return &OIDCLoginService{
		providerName:   providerName,
		oidcClient:     oidcClient,
		loginState:     loginState,
		identity:       identity,
		authzService:   authzService,
		sessionService: sessionService,
	}
}

func (s *OIDCLoginService) StartLogin(ctx context.Context, returnTo string) (string, error) {
	if s == nil || s.oidcClient == nil || s.loginState == nil {
		return "", ErrAuthModeUnsupported
	}

	state, record, err := s.loginState.Create(ctx, sanitizeReturnTo(returnTo))
	if err != nil {
		return "", fmt.Errorf("create oidc login state: %w", err)
	}

	return s.oidcClient.AuthorizeURL(state, record.Nonce, record.CodeVerifier), nil
}

func (s *OIDCLoginService) FinishLogin(ctx context.Context, code, state string, auditRequest AuditRequest) (OIDCLoginResult, error) {
	if s == nil || s.oidcClient == nil || s.loginState == nil || s.identity == nil || s.sessionService == nil {
		return OIDCLoginResult{}, ErrAuthModeUnsupported
	}

	loginState, err := s.loginState.Consume(ctx, state)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("consume oidc login state: %w", err)
	}

	identity, err := s.oidcClient.ExchangeCode(ctx, code, loginState.CodeVerifier, loginState.Nonce)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("finish oidc code exchange: %w", err)
	}

	user, err := s.identity.ResolveOrCreateUser(ctx, ExternalIdentity{
		Provider:      s.providerName,
		Subject:       identity.Claims.Subject,
		Email:         identity.Claims.Email,
		EmailVerified: identity.Claims.EmailVerified,
		DisplayName:   identity.Claims.Name,
	})
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("resolve local user for oidc identity: %w", err)
	}

	if s.authzService != nil {
		if _, err := s.authzService.SyncGlobalRoles(ctx, user.ID, identity.Claims.Groups); err != nil {
			return OIDCLoginResult{}, fmt.Errorf("sync local roles for oidc login: %w", err)
		}
		if _, err := s.authzService.SyncTenantMemberships(ctx, user.ID, "provider_claim", identity.Claims.Groups); err != nil {
			return OIDCLoginResult{}, fmt.Errorf("sync tenant memberships for oidc login: %w", err)
		}
	}

	sessionID, csrfToken, err := s.sessionService.IssueSessionWithProviderHint(ctx, user.ID, identity.RawIDToken)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("issue local session for oidc login: %w", err)
	}
	if s.sessionService.audit != nil {
		if err := s.sessionService.audit.Record(ctx, AuditEventInput{
			AuditContext: UserAuditContext(user.ID, user.DefaultTenantID, auditRequest),
			Action:       "session.login",
			TargetType:   "session",
			TargetID:     "browser",
		}); err != nil {
			_ = s.sessionService.store.Delete(ctx, sessionID)
			return OIDCLoginResult{}, err
		}
	}

	return OIDCLoginResult{
		SessionID: sessionID,
		CSRFToken: csrfToken,
		ReturnTo:  sanitizeReturnTo(loginState.ReturnTo),
	}, nil
}

func sanitizeReturnTo(returnTo string) string {
	trimmed := strings.TrimSpace(returnTo)
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return "/"
	}
	return trimmed
}

func IsOIDCLoginFailure(err error) bool {
	return err != nil && (errors.Is(err, auth.ErrLoginStateNotFound) || errors.Is(err, ErrInvalidExternalIdentity))
}

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"example.com/haohao/backend/internal/auth"
	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrDelegationNotConfigured       = errors.New("delegated auth is not configured")
	ErrDelegationUnsupportedResource = errors.New("unsupported downstream resource")
	ErrDelegationGrantNotFound       = errors.New("delegated grant not found")
	ErrDelegationInvalidState        = errors.New("invalid delegated auth state")
	ErrDelegationIdentityNotFound    = errors.New("delegated provider identity not found")
	ErrDelegationRefreshTokenMissing = errors.New("delegated refresh token missing")
)

type DelegationStatus struct {
	TenantID        int64
	ResourceServer  string
	Provider        string
	Connected       bool
	Scopes          []string
	GrantedAt       *time.Time
	LastRefreshedAt *time.Time
	RevokedAt       *time.Time
	LastErrorCode   string
}

type DelegatedAccessToken struct {
	AccessToken string
	ExpiresAt   *time.Time
	Scopes      []string
}

type DelegationVerifyResult struct {
	ResourceServer  string
	Connected       bool
	Scopes          []string
	AccessExpiresAt *time.Time
	RefreshedAt     *time.Time
}

type delegationResource struct {
	resourceServer string
	provider       string
	redirectURI    string
	scopes         []string
}

type DelegationService struct {
	queries       *db.Queries
	oauthClient   *auth.DelegatedOAuthClient
	stateStore    *auth.DelegationStateStore
	tokenStore    *auth.RefreshTokenStore
	appBaseURL    string
	defaultScopes []string
	refreshTTL    time.Duration
	accessSkew    time.Duration
	audit         AuditRecorder
}

func NewDelegationService(queries *db.Queries, oauthClient *auth.DelegatedOAuthClient, stateStore *auth.DelegationStateStore, tokenStore *auth.RefreshTokenStore, appBaseURL, defaultScopes string, refreshTTL, accessSkew time.Duration, audit AuditRecorder) *DelegationService {
	return &DelegationService{
		queries:       queries,
		oauthClient:   oauthClient,
		stateStore:    stateStore,
		tokenStore:    tokenStore,
		appBaseURL:    strings.TrimRight(appBaseURL, "/"),
		defaultScopes: normalizeScopeList(strings.Fields(defaultScopes)),
		refreshTTL:    refreshTTL,
		accessSkew:    accessSkew,
		audit:         audit,
	}
}

func (s *DelegationService) ListIntegrations(ctx context.Context, user User) ([]DelegationStatus, error) {
	tenantID, err := delegationTenantID(user)
	if err != nil {
		return nil, err
	}
	return s.ListIntegrationsForTenant(ctx, user, tenantID)
}

func (s *DelegationService) ListIntegrationsForTenant(ctx context.Context, user User, tenantID int64) ([]DelegationStatus, error) {
	if err := s.requireConfigured(); err != nil {
		return nil, err
	}

	rows, err := s.queries.ListOAuthUserGrantsByUserID(ctx, db.ListOAuthUserGrantsByUserIDParams{
		UserID:   user.ID,
		TenantID: tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("list downstream grants: %w", err)
	}

	byResource := make(map[string]db.ListOAuthUserGrantsByUserIDRow, len(rows))
	for _, row := range rows {
		if row.Provider == "zitadel" {
			byResource[row.ResourceServer] = row
		}
	}

	statuses := make([]DelegationStatus, 0, 1)
	for _, resourceServer := range []string{"zitadel"} {
		resource, err := s.resource(resourceServer)
		if err != nil {
			return nil, err
		}

		status := DelegationStatus{
			TenantID:       tenantID,
			ResourceServer: resource.resourceServer,
			Provider:       resource.provider,
			Scopes:         resource.scopes,
		}
		if row, ok := byResource[resource.resourceServer]; ok {
			status.Connected = !row.RevokedAt.Valid
			status.Scopes = normalizeScopeText(row.ScopeText)
			status.GrantedAt = timeFromPg(row.GrantedAt)
			status.LastRefreshedAt = timeFromPg(row.LastRefreshedAt)
			status.RevokedAt = timeFromPg(row.RevokedAt)
			if row.LastErrorCode.Valid {
				status.LastErrorCode = row.LastErrorCode.String
			}
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

func (s *DelegationService) StartConnect(ctx context.Context, user User, sessionID, resourceServer string) (string, error) {
	tenantID, err := delegationTenantID(user)
	if err != nil {
		return "", err
	}
	return s.StartConnectForTenant(ctx, user, tenantID, sessionID, resourceServer, UserAuditContext(user.ID, &tenantID, AuditRequest{}))
}

func (s *DelegationService) StartConnectForTenant(ctx context.Context, user User, tenantID int64, sessionID, resourceServer string, auditCtx AuditContext) (string, error) {
	if err := s.requireConfigured(); err != nil {
		return "", err
	}

	resource, err := s.resource(resourceServer)
	if err != nil {
		return "", err
	}

	state, record, err := s.stateStore.Create(ctx, user.ID, tenantID, resource.resourceServer, hashSessionID(sessionID))
	if err != nil {
		return "", fmt.Errorf("create delegated auth state: %w", err)
	}

	auditCtx.TenantID = &tenantID
	if s.audit != nil {
		if err := s.audit.Record(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "integration.connect_start",
			TargetType:   "integration",
			TargetID:     resource.resourceServer,
			Metadata:     delegationAuditMetadata(resource),
		}); err != nil {
			return "", err
		}
	}

	return s.oauthClient.AuthorizeURL(state, record.CodeVerifier, resource.redirectURI, resource.scopes), nil
}

func (s *DelegationService) SaveGrantFromCallback(ctx context.Context, user User, sessionID, resourceServer, code, state string, auditCtx AuditContext) (DelegationStatus, error) {
	if err := s.requireConfigured(); err != nil {
		return DelegationStatus{}, err
	}

	resource, err := s.resource(resourceServer)
	if err != nil {
		return DelegationStatus{}, err
	}

	record, err := s.stateStore.Consume(ctx, state)
	if err != nil {
		return DelegationStatus{}, fmt.Errorf("%w: %v", ErrDelegationInvalidState, err)
	}
	if record.UserID != user.ID || record.TenantID == 0 || record.ResourceServer != resource.resourceServer || record.SessionHash != hashSessionID(sessionID) {
		return DelegationStatus{}, ErrDelegationInvalidState
	}

	identity, err := s.queries.GetUserIdentityByUserIDProvider(ctx, db.GetUserIdentityByUserIDProviderParams{
		UserID:   user.ID,
		Provider: resource.provider,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DelegationStatus{}, ErrDelegationIdentityNotFound
	}
	if err != nil {
		return DelegationStatus{}, fmt.Errorf("load delegated provider identity: %w", err)
	}

	token, err := s.oauthClient.ExchangeCode(ctx, code, record.CodeVerifier, resource.redirectURI, resource.scopes)
	if err != nil {
		return DelegationStatus{}, err
	}
	if token.RefreshToken == "" {
		return DelegationStatus{}, ErrDelegationRefreshTokenMissing
	}

	ciphertext, keyVersion, err := s.tokenStore.Encrypt(token.RefreshToken)
	if err != nil {
		return DelegationStatus{}, fmt.Errorf("encrypt delegated refresh token: %w", err)
	}

	row, err := s.queries.UpsertOAuthUserGrant(ctx, db.UpsertOAuthUserGrantParams{
		UserID:                 user.ID,
		TenantID:               record.TenantID,
		Provider:               resource.provider,
		ResourceServer:         resource.resourceServer,
		ProviderSubject:        identity.Subject,
		RefreshTokenCiphertext: ciphertext,
		RefreshTokenKeyVersion: keyVersion,
		ScopeText:              scopeText(token.Scopes),
		GrantedBySessionID:     hashSessionID(sessionID),
	})
	if err != nil {
		return DelegationStatus{}, fmt.Errorf("save delegated grant: %w", err)
	}

	status := grantStatusFromUpsertRow(row)
	auditCtx.TenantID = &record.TenantID
	if s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "integration.connect_finish",
			TargetType:   "integration",
			TargetID:     resource.resourceServer,
			Metadata:     delegationAuditMetadata(resource),
		})
	}

	return status, nil
}

func (s *DelegationService) GetAccessToken(ctx context.Context, user User, resourceServer string) (DelegatedAccessToken, error) {
	tenantID, err := delegationTenantID(user)
	if err != nil {
		return DelegatedAccessToken{}, err
	}
	return s.GetAccessTokenForTenant(ctx, user, tenantID, resourceServer)
}

func (s *DelegationService) GetAccessTokenForTenant(ctx context.Context, user User, tenantID int64, resourceServer string) (DelegatedAccessToken, error) {
	if err := s.requireConfigured(); err != nil {
		return DelegatedAccessToken{}, err
	}

	resource, err := s.resource(resourceServer)
	if err != nil {
		return DelegatedAccessToken{}, err
	}

	grant, err := s.queries.GetActiveOAuthUserGrant(ctx, db.GetActiveOAuthUserGrantParams{
		UserID:         user.ID,
		TenantID:       tenantID,
		Provider:       resource.provider,
		ResourceServer: resource.resourceServer,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DelegatedAccessToken{}, ErrDelegationGrantNotFound
	}
	if err != nil {
		return DelegatedAccessToken{}, fmt.Errorf("load delegated grant: %w", err)
	}
	if s.refreshTokenExpired(grant.GrantedAt, grant.LastRefreshedAt) {
		_ = s.queries.MarkOAuthUserGrantRevoked(ctx, db.MarkOAuthUserGrantRevokedParams{
			UserID:         user.ID,
			TenantID:       tenantID,
			Provider:       resource.provider,
			ResourceServer: resource.resourceServer,
			LastErrorCode:  pgtype.Text{String: "refresh_token_expired", Valid: true},
		})
		return DelegatedAccessToken{}, ErrDelegationGrantNotFound
	}

	refreshToken, err := s.tokenStore.Decrypt(grant.RefreshTokenCiphertext, grant.RefreshTokenKeyVersion)
	if err != nil {
		return DelegatedAccessToken{}, fmt.Errorf("decrypt delegated refresh token: %w", err)
	}

	token, err := s.oauthClient.Refresh(ctx, refreshToken, resource.redirectURI, normalizeScopeText(grant.ScopeText))
	if err != nil {
		if auth.IsInvalidGrantError(err) {
			_ = s.queries.MarkOAuthUserGrantRevoked(ctx, db.MarkOAuthUserGrantRevokedParams{
				UserID:         user.ID,
				TenantID:       tenantID,
				Provider:       resource.provider,
				ResourceServer: resource.resourceServer,
				LastErrorCode:  pgtype.Text{String: "invalid_grant", Valid: true},
			})
			return DelegatedAccessToken{}, ErrDelegationGrantNotFound
		}
		return DelegatedAccessToken{}, err
	}

	nextRefreshToken := token.RefreshToken
	if nextRefreshToken == "" {
		nextRefreshToken = refreshToken
	}

	ciphertext, keyVersion, err := s.tokenStore.Encrypt(nextRefreshToken)
	if err != nil {
		return DelegatedAccessToken{}, fmt.Errorf("encrypt rotated refresh token: %w", err)
	}

	scopes := token.Scopes
	if len(scopes) == 0 {
		scopes = normalizeScopeText(grant.ScopeText)
	}

	if _, err := s.queries.UpdateOAuthUserGrantAfterRefresh(ctx, db.UpdateOAuthUserGrantAfterRefreshParams{
		UserID:                 user.ID,
		TenantID:               tenantID,
		Provider:               resource.provider,
		ResourceServer:         resource.resourceServer,
		RefreshTokenCiphertext: ciphertext,
		RefreshTokenKeyVersion: keyVersion,
		ScopeText:              scopeText(scopes),
	}); err != nil {
		return DelegatedAccessToken{}, fmt.Errorf("update delegated grant after refresh: %w", err)
	}

	return DelegatedAccessToken{
		AccessToken: token.AccessToken,
		ExpiresAt:   expiresWithSkew(token.Expiry, s.accessSkew),
		Scopes:      scopes,
	}, nil
}

func (s *DelegationService) VerifyAccessToken(ctx context.Context, user User, resourceServer string) (DelegationVerifyResult, error) {
	token, err := s.GetAccessToken(ctx, user, resourceServer)
	if err != nil {
		return DelegationVerifyResult{}, err
	}
	return delegationVerifyResult(resourceServer, token), nil
}

func (s *DelegationService) VerifyAccessTokenForTenant(ctx context.Context, user User, tenantID int64, resourceServer string, auditCtx AuditContext) (DelegationVerifyResult, error) {
	if err := s.requireConfigured(); err != nil {
		return DelegationVerifyResult{}, err
	}
	resource, err := s.resource(resourceServer)
	if err != nil {
		return DelegationVerifyResult{}, err
	}
	token, err := s.GetAccessTokenForTenant(ctx, user, tenantID, resourceServer)
	if err != nil {
		return DelegationVerifyResult{}, err
	}
	result := delegationVerifyResult(resourceServer, token)
	auditCtx.TenantID = &tenantID
	if s.audit != nil {
		if err := s.audit.Record(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "integration.verify",
			TargetType:   "integration",
			TargetID:     resource.resourceServer,
			Metadata: map[string]any{
				"provider": resource.provider,
			},
		}); err != nil {
			return DelegationVerifyResult{}, err
		}
	}
	return result, nil
}

func delegationVerifyResult(resourceServer string, token DelegatedAccessToken) DelegationVerifyResult {
	now := time.Now().UTC()
	return DelegationVerifyResult{
		ResourceServer:  normalizeResourceServer(resourceServer),
		Connected:       true,
		Scopes:          token.Scopes,
		AccessExpiresAt: token.ExpiresAt,
		RefreshedAt:     &now,
	}
}

func (s *DelegationService) DeleteGrant(ctx context.Context, user User, resourceServer string) error {
	tenantID, err := delegationTenantID(user)
	if err != nil {
		return err
	}
	return s.DeleteGrantForTenant(ctx, user, tenantID, resourceServer, UserAuditContext(user.ID, &tenantID, AuditRequest{}))
}

func (s *DelegationService) DeleteGrantForTenant(ctx context.Context, user User, tenantID int64, resourceServer string, auditCtx AuditContext) error {
	if err := s.requireConfigured(); err != nil {
		return err
	}

	resource, err := s.resource(resourceServer)
	if err != nil {
		return err
	}

	grant, err := s.queries.GetActiveOAuthUserGrant(ctx, db.GetActiveOAuthUserGrantParams{
		UserID:         user.ID,
		TenantID:       tenantID,
		Provider:       resource.provider,
		ResourceServer: resource.resourceServer,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load delegated grant for revoke: %w", err)
	}
	if err == nil {
		refreshToken, err := s.tokenStore.Decrypt(grant.RefreshTokenCiphertext, grant.RefreshTokenKeyVersion)
		if err != nil {
			return fmt.Errorf("decrypt delegated refresh token for revoke: %w", err)
		}
		if err := s.oauthClient.RevokeRefreshToken(ctx, refreshToken); err != nil {
			return err
		}
	}

	if err := s.queries.DeleteOAuthUserGrant(ctx, db.DeleteOAuthUserGrantParams{
		UserID:         user.ID,
		TenantID:       tenantID,
		Provider:       resource.provider,
		ResourceServer: resource.resourceServer,
	}); err != nil {
		return fmt.Errorf("delete delegated grant: %w", err)
	}

	auditCtx.TenantID = &tenantID
	if s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "integration.revoke",
			TargetType:   "integration",
			TargetID:     resource.resourceServer,
			Metadata: map[string]any{
				"provider": resource.provider,
			},
		})
	}

	return nil
}

func (s *DelegationService) DeleteAllGrantsForUser(ctx context.Context, userID int64) error {
	if err := s.requireConfigured(); err != nil {
		return nil
	}

	grants, err := s.queries.ListActiveOAuthUserGrantsByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("list active delegated grants for user: %w", err)
	}
	for _, grant := range grants {
		refreshToken, err := s.tokenStore.Decrypt(grant.RefreshTokenCiphertext, grant.RefreshTokenKeyVersion)
		if err != nil {
			return fmt.Errorf("decrypt delegated refresh token for user revoke: %w", err)
		}
		if err := s.oauthClient.RevokeRefreshToken(ctx, refreshToken); err != nil {
			return err
		}
	}
	if err := s.queries.DeleteOAuthUserGrantsByUserID(ctx, userID); err != nil {
		return fmt.Errorf("delete delegated grants for user: %w", err)
	}
	return nil
}

func (s *DelegationService) requireConfigured() error {
	if s == nil || s.queries == nil || s.oauthClient == nil || s.stateStore == nil || s.tokenStore == nil || s.appBaseURL == "" {
		return ErrDelegationNotConfigured
	}
	return nil
}

func (s *DelegationService) resource(resourceServer string) (delegationResource, error) {
	normalized := normalizeResourceServer(resourceServer)
	if normalized != "zitadel" {
		return delegationResource{}, ErrDelegationUnsupportedResource
	}

	scopes := s.defaultScopes
	if len(scopes) == 0 {
		scopes = []string{"offline_access"}
	}

	return delegationResource{
		resourceServer: "zitadel",
		provider:       "zitadel",
		redirectURI:    s.appBaseURL + "/api/v1/integrations/zitadel/callback",
		scopes:         scopes,
	}, nil
}

func delegationAuditMetadata(resource delegationResource) map[string]any {
	return map[string]any{
		"resourceServer": resource.resourceServer,
		"provider":       resource.provider,
		"scopeCount":     len(resource.scopes),
	}
}

func normalizeResourceServer(resourceServer string) string {
	return strings.ToLower(strings.TrimSpace(resourceServer))
}

func hashSessionID(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return hex.EncodeToString(sum[:])
}

func scopeText(scopes []string) string {
	return strings.Join(normalizeScopeList(scopes), " ")
}

func normalizeScopeText(value string) []string {
	return normalizeScopeList(strings.Fields(value))
}

func normalizeScopeList(scopes []string) []string {
	set := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		trimmed := strings.TrimSpace(scope)
		if trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}

	normalized := make([]string, 0, len(set))
	for scope := range set {
		normalized = append(normalized, scope)
	}
	sort.Strings(normalized)
	return normalized
}

func timeFromPg(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

func expiresWithSkew(expiry time.Time, skew time.Duration) *time.Time {
	if expiry.IsZero() {
		return nil
	}
	expiresAt := expiry.Add(-skew).UTC()
	return &expiresAt
}

func (s *DelegationService) refreshTokenExpired(grantedAt, lastRefreshedAt pgtype.Timestamptz) bool {
	if s.refreshTTL <= 0 || !grantedAt.Valid {
		return false
	}

	base := grantedAt.Time
	if lastRefreshedAt.Valid {
		base = lastRefreshedAt.Time
	}

	return time.Now().After(base.Add(s.refreshTTL))
}

func grantStatusFromUpsertRow(row db.UpsertOAuthUserGrantRow) DelegationStatus {
	return DelegationStatus{
		TenantID:        row.TenantID,
		ResourceServer:  row.ResourceServer,
		Provider:        row.Provider,
		Connected:       !row.RevokedAt.Valid,
		Scopes:          normalizeScopeText(row.ScopeText),
		GrantedAt:       timeFromPg(row.GrantedAt),
		LastRefreshedAt: timeFromPg(row.LastRefreshedAt),
		RevokedAt:       timeFromPg(row.RevokedAt),
		LastErrorCode: func() string {
			if row.LastErrorCode.Valid {
				return row.LastErrorCode.String
			}
			return ""
		}(),
	}
}

func delegationTenantID(user User) (int64, error) {
	if user.DefaultTenantID == nil || *user.DefaultTenantID == 0 {
		return 0, ErrDelegationGrantNotFound
	}
	return *user.DefaultTenantID, nil
}

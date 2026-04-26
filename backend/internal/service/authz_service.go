package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"example.com/haohao/backend/internal/auth"
	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthContext struct {
	AuthenticatedBy string
	Provider        string
	Subject         string
	AuthorizedParty string
	Scopes          []string
	Groups          []string
	Roles           []string
	User            *User
	DefaultTenant   *TenantAccess
	ActiveTenant    *TenantAccess
	Tenants         []TenantAccess
}

type TenantAccess struct {
	ID          int64
	Slug        string
	DisplayName string
	Roles       []string
	Default     bool
	Selected    bool
}

type TenantRoleClaim struct {
	TenantSlug string
	RoleCode   string
}

type authContextKey struct{}

type AuthzService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewAuthzService(pool *pgxpool.Pool, queries *db.Queries) *AuthzService {
	return &AuthzService{
		pool:    pool,
		queries: queries,
	}
}

func ContextWithAuthContext(ctx context.Context, authCtx AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey{}, authCtx)
}

func AuthContextFromContext(ctx context.Context) (AuthContext, bool) {
	authCtx, ok := ctx.Value(authContextKey{}).(AuthContext)
	return authCtx, ok
}

func (a AuthContext) HasRole(role string) bool {
	needle := strings.ToLower(strings.TrimSpace(role))
	if needle == "" {
		return true
	}

	for _, item := range append(append([]string{}, a.Roles...), a.Groups...) {
		if strings.ToLower(strings.TrimSpace(item)) == needle {
			return true
		}
	}

	return false
}

func (a AuthContext) HasProviderRole(role string) bool {
	needle := strings.ToLower(strings.TrimSpace(role))
	if needle == "" {
		return true
	}

	for _, item := range a.Groups {
		if strings.ToLower(strings.TrimSpace(item)) == needle {
			return true
		}
	}

	return false
}

func (s *AuthzService) SyncGlobalRoles(ctx context.Context, userID int64, providerGroups []string) ([]string, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return nil, fmt.Errorf("authz service is not configured")
	}

	roleCodes := normalizeGlobalRoleCodes(providerGroups)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin role sync transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	if len(roleCodes) == 0 {
		if err := qtx.DeleteUserRolesByUserID(ctx, userID); err != nil {
			return nil, fmt.Errorf("delete user roles: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit empty role sync transaction: %w", err)
		}
		return nil, nil
	}

	roles, err := qtx.GetRolesByCode(ctx, roleCodes)
	if err != nil {
		return nil, fmt.Errorf("load roles by code: %w", err)
	}

	roleIDs := make([]int64, 0, len(roles))
	syncedCodes := make([]string, 0, len(roles))
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
		syncedCodes = append(syncedCodes, role.Code)
	}

	if err := qtx.DeleteUserRolesExcluding(ctx, db.DeleteUserRolesExcludingParams{
		UserID:  userID,
		Column2: roleIDs,
	}); err != nil {
		return nil, fmt.Errorf("delete stale user roles: %w", err)
	}

	for _, roleID := range roleIDs {
		if err := qtx.AssignUserRole(ctx, db.AssignUserRoleParams{
			UserID: userID,
			RoleID: roleID,
		}); err != nil {
			return nil, fmt.Errorf("assign user role: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit role sync transaction: %w", err)
	}

	return syncedCodes, nil
}

func (s *AuthzService) AuthContextFromBearer(ctx context.Context, provider string, claims auth.BearerTokenClaims) (AuthContext, error) {
	return s.AuthContextFromBearerWithTenant(ctx, provider, claims, "")
}

func (s *AuthzService) AuthContextFromBearerWithTenant(ctx context.Context, provider string, claims auth.BearerTokenClaims, requestedTenantSlug string) (AuthContext, error) {
	authCtx := AuthContext{
		AuthenticatedBy: "bearer",
		Provider:        strings.ToLower(strings.TrimSpace(provider)),
		Subject:         strings.TrimSpace(claims.Subject),
		AuthorizedParty: strings.TrimSpace(claims.AuthorizedParty),
		Scopes:          claims.ScopeValues(),
		Groups:          mergeClaimValues(claims.GroupValues(), claims.RoleValues()),
	}

	if s == nil || s.queries == nil || authCtx.Provider == "" || authCtx.Subject == "" {
		return authCtx, nil
	}

	user, err := s.queries.GetUserByProviderSubject(ctx, db.GetUserByProviderSubjectParams{
		Provider: authCtx.Provider,
		Subject:  authCtx.Subject,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authCtx, nil
		}
		return AuthContext{}, fmt.Errorf("lookup user by provider subject: %w", err)
	}
	if user.DeactivatedAt.Valid {
		return AuthContext{}, ErrUnauthorized
	}

	localUser := dbUser(user.ID, user.PublicID.String(), user.Email, user.DisplayName, user.DeactivatedAt, user.DefaultTenantID)
	authCtx.User = &localUser

	if len(authCtx.Groups) > 0 {
		roleCodes, err := s.SyncGlobalRoles(ctx, localUser.ID, authCtx.Groups)
		if err != nil {
			return AuthContext{}, fmt.Errorf("sync global roles from bearer claims: %w", err)
		}
		authCtx.Roles = roleCodes
		if _, err := s.SyncTenantMemberships(ctx, localUser.ID, "provider_claim", authCtx.Groups); err != nil {
			return AuthContext{}, fmt.Errorf("sync tenant memberships from bearer claims: %w", err)
		}
	} else {
		roleCodes, err := s.queries.ListRoleCodesByUserID(ctx, localUser.ID)
		if err != nil {
			return AuthContext{}, fmt.Errorf("list local roles by user id: %w", err)
		}
		authCtx.Roles = roleCodes
	}

	tenants, active, def, err := s.resolveTenantAccess(ctx, localUser.ID, localUser.DefaultTenantID, requestedTenantSlug)
	if err != nil {
		return AuthContext{}, err
	}
	authCtx.Tenants = tenants
	authCtx.ActiveTenant = active
	authCtx.DefaultTenant = def

	return authCtx, nil
}

var supportedGlobalRoles = map[string]struct{}{
	"customer_signal_user": {},
	"docs_reader":          {},
	"drive_content_admin":  {},
	"external_api_user":    {},
	"machine_client_admin": {},
	"support_agent":        {},
	"tenant_admin":         {},
	"todo_user":            {},
}

var supportedTenantRoles = map[string]struct{}{
	"customer_signal_user": {},
	"docs_reader":          {},
	"todo_user":            {},
}

func IsSupportedTenantRole(roleCode string) bool {
	_, ok := supportedTenantRoles[strings.ToLower(strings.TrimSpace(roleCode))]
	return ok
}

func (s *AuthzService) BuildBrowserContext(ctx context.Context, user User, activeTenantID *int64) (AuthContext, error) {
	if s == nil || s.queries == nil {
		return AuthContext{}, fmt.Errorf("authz service is not configured")
	}

	roleCodes, err := s.queries.ListRoleCodesByUserID(ctx, user.ID)
	if err != nil {
		return AuthContext{}, fmt.Errorf("list local roles by user id: %w", err)
	}

	requestedSlug := ""
	if activeTenantID != nil {
		tenant, err := s.queries.GetTenantByID(ctx, *activeTenantID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return AuthContext{}, ErrUnauthorized
			}
			return AuthContext{}, fmt.Errorf("load active tenant: %w", err)
		}
		requestedSlug = tenant.Slug
	}

	tenants, active, def, err := s.resolveTenantAccess(ctx, user.ID, user.DefaultTenantID, requestedSlug)
	if err != nil {
		return AuthContext{}, err
	}

	return AuthContext{
		AuthenticatedBy: "session",
		Roles:           roleCodes,
		User:            &user,
		Tenants:         tenants,
		ActiveTenant:    active,
		DefaultTenant:   def,
	}, nil
}

func (s *AuthzService) SyncTenantMemberships(ctx context.Context, userID int64, source string, providerGroups []string) ([]TenantAccess, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return nil, fmt.Errorf("authz service is not configured")
	}

	claims := ParseTenantRoleClaims(providerGroups)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tenant sync transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	if err := qtx.DeleteTenantMembershipsByUserSource(ctx, db.DeleteTenantMembershipsByUserSourceParams{
		UserID: userID,
		Source: source,
	}); err != nil {
		return nil, fmt.Errorf("delete stale tenant memberships: %w", err)
	}

	if len(claims) > 0 {
		roleCodes := tenantRoleCodes(claims)
		roles, err := qtx.GetRolesByCode(ctx, roleCodes)
		if err != nil {
			return nil, fmt.Errorf("load tenant roles by code: %w", err)
		}
		roleIDByCode := make(map[string]int64, len(roles))
		for _, role := range roles {
			roleIDByCode[role.Code] = role.ID
		}

		for _, claim := range claims {
			roleID, ok := roleIDByCode[claim.RoleCode]
			if !ok {
				continue
			}
			tenant, err := qtx.UpsertTenantBySlug(ctx, db.UpsertTenantBySlugParams{
				Slug:        claim.TenantSlug,
				DisplayName: claim.TenantSlug,
			})
			if err != nil {
				return nil, fmt.Errorf("upsert tenant: %w", err)
			}
			if err := qtx.UpsertTenantMembership(ctx, db.UpsertTenantMembershipParams{
				UserID:   userID,
				TenantID: tenant.ID,
				RoleID:   roleID,
				Source:   source,
			}); err != nil {
				return nil, fmt.Errorf("upsert tenant membership: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tenant sync transaction: %w", err)
	}

	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load user after tenant sync: %w", err)
	}
	access, err := s.ListTenantAccess(ctx, userID, optionalPgInt8(user.DefaultTenantID))
	if err != nil {
		return nil, err
	}
	if !user.DefaultTenantID.Valid && len(access) > 0 {
		if _, err := s.queries.SetUserDefaultTenant(ctx, db.SetUserDefaultTenantParams{
			ID:              userID,
			DefaultTenantID: pgtype.Int8{Int64: access[0].ID, Valid: true},
		}); err != nil {
			return nil, fmt.Errorf("set default tenant: %w", err)
		}
		access[0].Default = true
	}
	return access, nil
}

func (s *AuthzService) ListTenantAccess(ctx context.Context, userID int64, defaultTenantID *int64) ([]TenantAccess, error) {
	rows, err := s.queries.ListTenantMembershipRowsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list tenant memberships: %w", err)
	}
	overrides, err := s.queries.ListTenantRoleOverridesByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list tenant role overrides: %w", err)
	}

	tenants := tenantAccessFromRows(rows, overrides)
	for i := range tenants {
		if defaultTenantID != nil && tenants[i].ID == *defaultTenantID {
			tenants[i].Default = true
		}
	}
	return tenants, nil
}

func (s *AuthzService) SelectTenant(ctx context.Context, user User, tenantSlug string) (TenantAccess, error) {
	tenants, active, _, err := s.resolveTenantAccess(ctx, user.ID, user.DefaultTenantID, tenantSlug)
	if err != nil {
		return TenantAccess{}, err
	}
	if active == nil {
		if len(tenants) == 0 {
			return TenantAccess{}, ErrUnauthorized
		}
		return tenants[0], nil
	}
	return *active, nil
}

func (s *AuthzService) resolveTenantAccess(ctx context.Context, userID int64, defaultTenantID *int64, requestedTenantSlug string) ([]TenantAccess, *TenantAccess, *TenantAccess, error) {
	tenants, err := s.ListTenantAccess(ctx, userID, defaultTenantID)
	if err != nil {
		return nil, nil, nil, err
	}

	var defaultTenant *TenantAccess
	var activeTenant *TenantAccess
	for i := range tenants {
		if tenants[i].Default {
			defaultTenant = &tenants[i]
			break
		}
	}
	if defaultTenant == nil && len(tenants) > 0 {
		tenants[0].Default = true
		defaultTenant = &tenants[0]
	}

	requested := strings.ToLower(strings.TrimSpace(requestedTenantSlug))
	if requested != "" {
		for i := range tenants {
			if tenants[i].Slug == requested {
				tenants[i].Selected = true
				activeTenant = &tenants[i]
				return tenants, activeTenant, defaultTenant, nil
			}
		}
		return nil, nil, nil, ErrUnauthorized
	}

	if defaultTenant != nil {
		for i := range tenants {
			if tenants[i].ID == defaultTenant.ID {
				tenants[i].Selected = true
				activeTenant = &tenants[i]
				break
			}
		}
	}
	return tenants, activeTenant, defaultTenant, nil
}

func normalizeGlobalRoleCodes(providerGroups []string) []string {
	set := make(map[string]struct{}, len(providerGroups))
	for _, group := range providerGroups {
		code := strings.ToLower(strings.TrimSpace(group))
		if _, ok := supportedGlobalRoles[code]; ok {
			set[code] = struct{}{}
		}
	}

	roleCodes := make([]string, 0, len(set))
	for code := range set {
		roleCodes = append(roleCodes, code)
	}
	sort.Strings(roleCodes)

	return roleCodes
}

func ParseTenantRoleClaims(providerGroups []string) []TenantRoleClaim {
	set := make(map[TenantRoleClaim]struct{})
	for _, group := range providerGroups {
		parts := strings.Split(strings.ToLower(strings.TrimSpace(group)), ":")
		if len(parts) != 3 || parts[0] != "tenant" {
			continue
		}
		tenantSlug := strings.TrimSpace(parts[1])
		roleCode := strings.TrimSpace(parts[2])
		if tenantSlug == "" || roleCode == "" {
			continue
		}
		if _, ok := supportedTenantRoles[roleCode]; !ok {
			continue
		}
		set[TenantRoleClaim{TenantSlug: tenantSlug, RoleCode: roleCode}] = struct{}{}
	}

	claims := make([]TenantRoleClaim, 0, len(set))
	for claim := range set {
		claims = append(claims, claim)
	}
	sort.Slice(claims, func(i, j int) bool {
		if claims[i].TenantSlug == claims[j].TenantSlug {
			return claims[i].RoleCode < claims[j].RoleCode
		}
		return claims[i].TenantSlug < claims[j].TenantSlug
	})
	return claims
}

func tenantRoleCodes(claims []TenantRoleClaim) []string {
	set := make(map[string]struct{}, len(claims))
	for _, claim := range claims {
		set[claim.RoleCode] = struct{}{}
	}
	codes := make([]string, 0, len(set))
	for code := range set {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func tenantAccessFromRows(rows []db.ListTenantMembershipRowsByUserIDRow, overrides []db.ListTenantRoleOverridesByUserIDRow) []TenantAccess {
	type tenantState struct {
		access TenantAccess
		roles  map[string]struct{}
	}

	byID := make(map[int64]*tenantState)
	for _, row := range rows {
		if !row.TenantActive || !row.MembershipActive {
			continue
		}
		state, ok := byID[row.TenantID]
		if !ok {
			state = &tenantState{
				access: TenantAccess{
					ID:          row.TenantID,
					Slug:        row.TenantSlug,
					DisplayName: row.TenantDisplayName,
				},
				roles: make(map[string]struct{}),
			}
			byID[row.TenantID] = state
		}
		state.roles[row.RoleCode] = struct{}{}
	}

	for _, override := range overrides {
		state, ok := byID[override.TenantID]
		if !ok {
			state = &tenantState{
				access: TenantAccess{
					ID:   override.TenantID,
					Slug: override.TenantSlug,
				},
				roles: make(map[string]struct{}),
			}
			byID[override.TenantID] = state
		}
		switch override.Effect {
		case "deny":
			delete(state.roles, override.RoleCode)
		case "allow":
			state.roles[override.RoleCode] = struct{}{}
		}
	}

	tenants := make([]TenantAccess, 0, len(byID))
	for _, state := range byID {
		if len(state.roles) == 0 {
			continue
		}
		state.access.Roles = make([]string, 0, len(state.roles))
		for role := range state.roles {
			state.access.Roles = append(state.access.Roles, role)
		}
		sort.Strings(state.access.Roles)
		tenants = append(tenants, state.access)
	}
	sort.Slice(tenants, func(i, j int) bool {
		return tenants[i].Slug < tenants[j].Slug
	})
	return tenants
}

func mergeClaimValues(values ...[]string) []string {
	set := make(map[string]struct{})
	for _, group := range values {
		for _, value := range group {
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				set[trimmed] = struct{}{}
			}
		}
	}

	merged := make([]string, 0, len(set))
	for value := range set {
		merged = append(merged, value)
	}
	sort.Strings(merged)

	return merged
}

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

	localUser := dbUser(user.ID, user.PublicID.String(), user.Email, user.DisplayName)
	authCtx.User = &localUser

	if len(authCtx.Groups) > 0 {
		roleCodes, err := s.SyncGlobalRoles(ctx, localUser.ID, authCtx.Groups)
		if err != nil {
			return AuthContext{}, fmt.Errorf("sync global roles from bearer claims: %w", err)
		}
		authCtx.Roles = roleCodes
		return authCtx, nil
	}

	roleCodes, err := s.queries.ListRoleCodesByUserID(ctx, localUser.ID)
	if err != nil {
		return AuthContext{}, fmt.Errorf("list local roles by user id: %w", err)
	}
	authCtx.Roles = roleCodes

	return authCtx, nil
}

var supportedGlobalRoles = map[string]struct{}{
	"docs_reader":       {},
	"external_api_user": {},
	"todo_user":         {},
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

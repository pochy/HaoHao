package api

import (
	"context"
	"errors"
	"strings"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

func requireActiveTenantRole(ctx context.Context, deps Dependencies, sessionID, csrfToken, role, serviceName string) (service.CurrentSession, service.TenantAccess, error) {
	return requireActiveTenantAnyRole(ctx, deps, sessionID, csrfToken, []string{role}, serviceName)
}

func requireActiveTenantAnyRole(ctx context.Context, deps Dependencies, sessionID, csrfToken string, roles []string, serviceName string) (service.CurrentSession, service.TenantAccess, error) {
	if serviceName != "" && deps.SessionService == nil {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error503ServiceUnavailable(serviceName + " is not configured")
	}

	var current service.CurrentSession
	var authCtx service.AuthContext
	var err error
	if csrfToken == "" {
		current, authCtx, err = currentSessionAuthContext(ctx, deps, sessionID)
	} else {
		current, authCtx, err = currentSessionAuthContextWithCSRF(ctx, deps, sessionID, csrfToken)
	}
	if err != nil {
		var statusErr huma.StatusError
		if errors.As(err, &statusErr) {
			return service.CurrentSession{}, service.TenantAccess{}, err
		}
		return service.CurrentSession{}, service.TenantAccess{}, toHTTPErrorWithLog(ctx, deps, "", err)
	}
	if authCtx.ActiveTenant == nil {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error409Conflict("active tenant is required")
	}
	if !activeTenantHasAnyRole(*authCtx.ActiveTenant, roles) {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error403Forbidden(strings.Join(nonEmptyRoles(roles), " or ") + " tenant role is required")
	}
	return current, *authCtx.ActiveTenant, nil
}

func activeTenantHasAnyRole(tenant service.TenantAccess, roles []string) bool {
	for _, role := range roles {
		if strings.TrimSpace(role) == "" || tenantHasRole(tenant, role) {
			return true
		}
	}
	return len(roles) == 0
}

func nonEmptyRoles(roles []string) []string {
	out := make([]string, 0, len(roles))
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role != "" {
			out = append(out, role)
		}
	}
	return out
}

func requireAdminTenantID(ctx context.Context, deps Dependencies, sessionID, csrfToken, tenantSlug string) (service.CurrentSession, service.TenantAdminTenant, error) {
	if deps.TenantAdminService == nil {
		return service.CurrentSession{}, service.TenantAdminTenant{}, huma.Error503ServiceUnavailable("tenant admin service is not configured")
	}

	var current service.CurrentSession
	var authCtx service.AuthContext
	var err error
	if csrfToken == "" {
		current, authCtx, err = currentSessionAuthContext(ctx, deps, sessionID)
	} else {
		current, authCtx, err = currentSessionAuthContextWithCSRF(ctx, deps, sessionID, csrfToken)
	}
	if err != nil {
		var statusErr huma.StatusError
		if errors.As(err, &statusErr) {
			return service.CurrentSession{}, service.TenantAdminTenant{}, err
		}
		return service.CurrentSession{}, service.TenantAdminTenant{}, toHTTPErrorWithLog(ctx, deps, "", err)
	}
	detail, err := deps.TenantAdminService.GetTenant(ctx, tenantSlug)
	if err != nil {
		return service.CurrentSession{}, service.TenantAdminTenant{}, toTenantAdminHTTPError(err)
	}
	if !authCtx.HasRole("tenant_admin") && !authContextHasTenantRole(authCtx, detail.Tenant.Slug, "tenant_admin") {
		return service.CurrentSession{}, service.TenantAdminTenant{}, huma.Error403Forbidden("tenant_admin role is required for this tenant")
	}
	return current, detail.Tenant, nil
}

func authContextHasTenantRole(authCtx service.AuthContext, tenantSlug, role string) bool {
	needleSlug := normalizeTenantSlugForAuth(tenantSlug)
	needleRole := normalizeTenantSlugForAuth(role)
	if needleSlug == "" || needleRole == "" {
		return false
	}
	for _, tenant := range authCtx.Tenants {
		if normalizeTenantSlugForAuth(tenant.Slug) != needleSlug {
			continue
		}
		return tenantHasRole(tenant, needleRole)
	}
	return false
}

func normalizeTenantSlugForAuth(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

package api

import (
	"context"
	"errors"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

func requireActiveTenantRole(ctx context.Context, deps Dependencies, sessionID, csrfToken, role, serviceName string) (service.CurrentSession, service.TenantAccess, error) {
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
		return service.CurrentSession{}, service.TenantAccess{}, toHTTPError(err)
	}
	if authCtx.ActiveTenant == nil {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error409Conflict("active tenant is required")
	}
	if role != "" && !tenantHasRole(*authCtx.ActiveTenant, role) {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error403Forbidden(role + " tenant role is required")
	}
	return current, *authCtx.ActiveTenant, nil
}

func requireAdminTenantID(ctx context.Context, deps Dependencies, sessionID, csrfToken, tenantSlug string) (service.CurrentSession, service.TenantAdminTenant, error) {
	current, err := requireTenantAdmin(ctx, deps, sessionID, csrfToken)
	if err != nil {
		return service.CurrentSession{}, service.TenantAdminTenant{}, err
	}
	detail, err := deps.TenantAdminService.GetTenant(ctx, tenantSlug)
	if err != nil {
		return service.CurrentSession{}, service.TenantAdminTenant{}, toTenantAdminHTTPError(err)
	}
	return current, detail.Tenant, nil
}

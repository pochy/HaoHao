package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type EntitlementBody struct {
	FeatureCode string         `json:"featureCode"`
	DisplayName string         `json:"displayName"`
	Description string         `json:"description"`
	Enabled     bool           `json:"enabled"`
	LimitValue  map[string]any `json:"limitValue"`
	Source      string         `json:"source"`
	UpdatedAt   time.Time      `json:"updatedAt" format:"date-time"`
}

type EntitlementUpdateBody struct {
	FeatureCode string         `json:"featureCode"`
	Enabled     bool           `json:"enabled"`
	LimitValue  map[string]any `json:"limitValue,omitempty"`
}

type ListEntitlementsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
}

type EntitlementListOutput struct {
	Body struct {
		Items []EntitlementBody `json:"items"`
	}
}

type UpdateEntitlementsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          struct {
		Items []EntitlementUpdateBody `json:"items"`
	}
}

func registerEntitlementRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listTenantEntitlements",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/entitlements",
		Summary:     "tenant entitlements を返す",
		Tags:        []string{DocTagPlatformIntegrations},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListEntitlementsInput) (*EntitlementListOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if deps.EntitlementService == nil {
			return nil, huma.Error503ServiceUnavailable("entitlement service is not configured")
		}
		items, err := deps.EntitlementService.List(ctx, tenant.ID)
		if err != nil {
			return nil, toEntitlementHTTPError(err)
		}
		out := &EntitlementListOutput{}
		out.Body.Items = make([]EntitlementBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toEntitlementBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateTenantEntitlements",
		Method:      http.MethodPut,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/entitlements",
		Summary:     "tenant entitlements を更新する",
		Tags:        []string{DocTagPlatformIntegrations},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *UpdateEntitlementsInput) (*EntitlementListOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if current.SupportAccess != nil {
			return nil, huma.Error403Forbidden("entitlement update is disabled during support access")
		}
		if deps.EntitlementService == nil {
			return nil, huma.Error503ServiceUnavailable("entitlement service is not configured")
		}
		updates := make([]service.EntitlementUpdateInput, 0, len(input.Body.Items))
		for _, item := range input.Body.Items {
			updates = append(updates, service.EntitlementUpdateInput{
				FeatureCode: item.FeatureCode,
				Enabled:     item.Enabled,
				LimitValue:  item.LimitValue,
			})
		}
		items, err := deps.EntitlementService.Update(ctx, tenant.ID, updates, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toEntitlementHTTPError(err)
		}
		out := &EntitlementListOutput{}
		out.Body.Items = make([]EntitlementBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toEntitlementBody(item))
		}
		return out, nil
	})
}

func toEntitlementBody(item service.Entitlement) EntitlementBody {
	return EntitlementBody{
		FeatureCode: item.FeatureCode,
		DisplayName: item.DisplayName,
		Description: item.Description,
		Enabled:     item.Enabled,
		LimitValue:  item.LimitValue,
		Source:      item.Source,
		UpdatedAt:   item.UpdatedAt,
	}
}

func toEntitlementHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidEntitlementInput):
		return huma.Error400BadRequest("invalid entitlement input")
	case errors.Is(err, service.ErrEntitlementNotFound):
		return huma.Error404NotFound("entitlement not found")
	default:
		return toHTTPError(err)
	}
}

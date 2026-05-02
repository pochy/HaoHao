package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type TenantSettingsBody struct {
	TenantID                      int64          `json:"tenantId"`
	FileQuotaBytes                int64          `json:"fileQuotaBytes"`
	RateLimitLoginPerMinute       *int32         `json:"rateLimitLoginPerMinute,omitempty"`
	RateLimitBrowserAPIPerMinute  *int32         `json:"rateLimitBrowserApiPerMinute,omitempty"`
	RateLimitExternalAPIPerMinute *int32         `json:"rateLimitExternalApiPerMinute,omitempty"`
	NotificationsEnabled          bool           `json:"notificationsEnabled"`
	Features                      map[string]any `json:"features"`
	CreatedAt                     time.Time      `json:"createdAt" format:"date-time"`
	UpdatedAt                     time.Time      `json:"updatedAt" format:"date-time"`
}

type TenantSettingsRequestBody struct {
	FileQuotaBytes                int64          `json:"fileQuotaBytes"`
	RateLimitLoginPerMinute       *int32         `json:"rateLimitLoginPerMinute,omitempty"`
	RateLimitBrowserAPIPerMinute  *int32         `json:"rateLimitBrowserApiPerMinute,omitempty"`
	RateLimitExternalAPIPerMinute *int32         `json:"rateLimitExternalApiPerMinute,omitempty"`
	NotificationsEnabled          bool           `json:"notificationsEnabled"`
	Features                      map[string]any `json:"features,omitempty"`
}

type GetTenantSettingsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
}

type UpdateTenantSettingsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          TenantSettingsRequestBody
}

type TenantSettingsOutput struct {
	Body TenantSettingsBody
}

func registerTenantSettingsRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "getTenantSettings",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/settings",
		Summary:     "tenant settings を返す",
		Tags:        []string{DocTagTenantWorkspace},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *GetTenantSettingsInput) (*TenantSettingsOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if deps.TenantSettingsService == nil {
			return nil, huma.Error503ServiceUnavailable("tenant settings service is not configured")
		}
		settings, err := deps.TenantSettingsService.Get(ctx, tenant.ID)
		if err != nil {
			return nil, toTenantSettingsHTTPError(err)
		}
		return &TenantSettingsOutput{Body: toTenantSettingsBody(settings)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateTenantSettings",
		Method:      http.MethodPut,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/settings",
		Summary:     "tenant settings を更新する",
		Tags:        []string{DocTagTenantWorkspace},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *UpdateTenantSettingsInput) (*TenantSettingsOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if deps.TenantSettingsService == nil {
			return nil, huma.Error503ServiceUnavailable("tenant settings service is not configured")
		}
		settings, err := deps.TenantSettingsService.Update(ctx, tenant.ID, tenantSettingsInputFromBody(input.Body), sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toTenantSettingsHTTPError(err)
		}
		return &TenantSettingsOutput{Body: toTenantSettingsBody(settings)}, nil
	})
}

func tenantSettingsInputFromBody(body TenantSettingsRequestBody) service.TenantSettingsInput {
	return service.TenantSettingsInput{
		FileQuotaBytes:                body.FileQuotaBytes,
		RateLimitLoginPerMinute:       body.RateLimitLoginPerMinute,
		RateLimitBrowserAPIPerMinute:  body.RateLimitBrowserAPIPerMinute,
		RateLimitExternalAPIPerMinute: body.RateLimitExternalAPIPerMinute,
		NotificationsEnabled:          body.NotificationsEnabled,
		Features:                      body.Features,
	}
}

func toTenantSettingsBody(item service.TenantSettings) TenantSettingsBody {
	return TenantSettingsBody{
		TenantID:                      item.TenantID,
		FileQuotaBytes:                item.FileQuotaBytes,
		RateLimitLoginPerMinute:       item.RateLimitLoginPerMinute,
		RateLimitBrowserAPIPerMinute:  item.RateLimitBrowserAPIPerMinute,
		RateLimitExternalAPIPerMinute: item.RateLimitExternalAPIPerMinute,
		NotificationsEnabled:          item.NotificationsEnabled,
		Features:                      item.Features,
		CreatedAt:                     item.CreatedAt,
		UpdatedAt:                     item.UpdatedAt,
	}
}

func toTenantSettingsHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidTenantSettings):
		return huma.Error400BadRequest(err.Error())
	default:
		return toHTTPError(err)
	}
}

package api

import (
	"context"
	"net/http"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type TenantBody struct {
	ID          int64    `json:"id" example:"1"`
	Slug        string   `json:"slug" example:"acme"`
	DisplayName string   `json:"displayName" example:"acme"`
	Roles       []string `json:"roles,omitempty" example:"todo_user"`
	Default     bool     `json:"default" example:"true"`
	Selected    bool     `json:"selected" example:"true"`
}

type ListTenantsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

type ListTenantsBody struct {
	Items         []TenantBody `json:"items"`
	ActiveTenant  *TenantBody  `json:"activeTenant,omitempty"`
	DefaultTenant *TenantBody  `json:"defaultTenant,omitempty"`
}

type ListTenantsOutput struct {
	Body ListTenantsBody
}

type SelectTenantInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          struct {
		TenantSlug string `json:"tenantSlug" example:"acme"`
	}
}

type SelectTenantOutput struct {
	Body struct {
		ActiveTenant TenantBody `json:"activeTenant"`
	}
}

func registerTenantRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listTenants",
		Method:      http.MethodGet,
		Path:        "/api/v1/tenants",
		Summary:     "現在の user が利用できる tenants を返す",
		Tags:        []string{"tenants"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ListTenantsInput) (*ListTenantsOutput, error) {
		_, authCtx, err := currentSessionAuthContext(ctx, deps, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPError(err)
		}

		out := &ListTenantsOutput{}
		out.Body.Items = toTenantBodies(authCtx.Tenants)
		if authCtx.ActiveTenant != nil {
			body := toTenantBody(*authCtx.ActiveTenant)
			out.Body.ActiveTenant = &body
		}
		if authCtx.DefaultTenant != nil {
			body := toTenantBody(*authCtx.DefaultTenant)
			out.Body.DefaultTenant = &body
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "selectTenant",
		Method:      http.MethodPost,
		Path:        "/api/v1/session/tenant",
		Summary:     "現在の session の active tenant を切り替える",
		Tags:        []string{"tenants"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *SelectTenantInput) (*SelectTenantOutput, error) {
		current, err := deps.SessionService.CurrentSessionWithCSRF(ctx, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toHTTPError(err)
		}
		if deps.AuthzService == nil {
			return nil, huma.Error503ServiceUnavailable("tenant auth is not configured")
		}

		tenant, err := deps.AuthzService.SelectTenant(ctx, current.User, input.Body.TenantSlug)
		if err != nil {
			return nil, toHTTPError(err)
		}
		if err := deps.SessionService.SetActiveTenant(ctx, input.SessionCookie.Value, input.CSRFToken, tenant.ID); err != nil {
			return nil, toHTTPError(err)
		}

		out := &SelectTenantOutput{}
		out.Body.ActiveTenant = toTenantBody(tenant)
		return out, nil
	})
}

func toTenantBodies(items []service.TenantAccess) []TenantBody {
	out := make([]TenantBody, 0, len(items))
	for _, item := range items {
		out = append(out, toTenantBody(item))
	}
	return out
}

func toTenantBody(item service.TenantAccess) TenantBody {
	return TenantBody{
		ID:          item.ID,
		Slug:        item.Slug,
		DisplayName: item.DisplayName,
		Roles:       item.Roles,
		Default:     item.Default,
		Selected:    item.Selected,
	}
}

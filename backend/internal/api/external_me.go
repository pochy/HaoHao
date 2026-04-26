package api

import (
	"context"
	"net/http"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type ExternalMeBody struct {
	Provider        string        `json:"provider" example:"zitadel"`
	Subject         string        `json:"subject" example:"312345678901234567"`
	AuthorizedParty string        `json:"authorizedParty,omitempty" example:"312345678901234568"`
	Scopes          []string      `json:"scopes,omitempty" example:"external:read"`
	Groups          []string      `json:"groups,omitempty" example:"external_api_user"`
	Roles           []string      `json:"roles,omitempty" example:"todo_user"`
	User            *UserResponse `json:"user,omitempty"`
	ActiveTenant    *TenantBody   `json:"activeTenant,omitempty"`
	DefaultTenant   *TenantBody   `json:"defaultTenant,omitempty"`
	Tenants         []TenantBody  `json:"tenants,omitempty"`
}

type GetExternalMeInput struct {
	TenantID string `header:"X-Tenant-ID" doc:"tenant slug for tenant-aware bearer context" example:"acme"`
}

type GetExternalMeOutput struct {
	Body ExternalMeBody
}

func registerExternalRoutes(api huma.API, deps Dependencies) {
	registerExternalDriveRoutes(api, deps)

	huma.Register(api, huma.Operation{
		OperationID: "getExternalMe",
		Method:      http.MethodGet,
		Path:        "/api/external/v1/me",
		Summary:     "現在の external bearer principal を返す",
		Tags:        []string{"external"},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *GetExternalMeInput) (*GetExternalMeOutput, error) {
		authCtx, ok := service.AuthContextFromContext(ctx)
		if !ok {
			return nil, huma.Error500InternalServerError("missing auth context")
		}

		var user *UserResponse
		if authCtx.User != nil {
			res := toUserResponse(*authCtx.User)
			user = &res
		}

		var activeTenant *TenantBody
		if authCtx.ActiveTenant != nil {
			item := toTenantBody(*authCtx.ActiveTenant)
			activeTenant = &item
		}
		var defaultTenant *TenantBody
		if authCtx.DefaultTenant != nil {
			item := toTenantBody(*authCtx.DefaultTenant)
			defaultTenant = &item
		}

		return &GetExternalMeOutput{
			Body: ExternalMeBody{
				Provider:        authCtx.Provider,
				Subject:         authCtx.Subject,
				AuthorizedParty: authCtx.AuthorizedParty,
				Scopes:          authCtx.Scopes,
				Groups:          authCtx.Groups,
				Roles:           authCtx.Roles,
				User:            user,
				ActiveTenant:    activeTenant,
				DefaultTenant:   defaultTenant,
				Tenants:         toTenantBodies(authCtx.Tenants),
			},
		}, nil
	})
}

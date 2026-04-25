package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type MachineClientBody struct {
	ID               int64       `json:"id" example:"1"`
	Provider         string      `json:"provider" example:"zitadel"`
	ProviderClientID string      `json:"providerClientId" example:"312345678901234567"`
	DisplayName      string      `json:"displayName" example:"nightly worker"`
	DefaultTenant    *TenantBody `json:"defaultTenant,omitempty"`
	AllowedScopes    []string    `json:"allowedScopes,omitempty" example:"m2m:read"`
	Active           bool        `json:"active" example:"true"`
	CreatedAt        time.Time   `json:"createdAt" format:"date-time"`
	UpdatedAt        time.Time   `json:"updatedAt" format:"date-time"`
}

type MachineClientRequestBody struct {
	Provider         string   `json:"provider,omitempty" example:"zitadel"`
	ProviderClientID string   `json:"providerClientId" example:"312345678901234567"`
	DisplayName      string   `json:"displayName" example:"nightly worker"`
	DefaultTenantID  *int64   `json:"defaultTenantId,omitempty" example:"1"`
	AllowedScopes    []string `json:"allowedScopes,omitempty" example:"m2m:read"`
	Active           *bool    `json:"active,omitempty" example:"true"`
}

type ListMachineClientsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

type ListMachineClientsOutput struct {
	Body struct {
		Items []MachineClientBody `json:"items"`
	}
}

type CreateMachineClientInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          MachineClientRequestBody
}

type CreateMachineClientOutput struct {
	Body MachineClientBody
}

type MachineClientByIDInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	ID            int64       `path:"id" minimum:"1"`
}

type UpdateMachineClientInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	ID            int64       `path:"id" minimum:"1"`
	Body          MachineClientRequestBody
}

type UpdateMachineClientOutput struct {
	Body MachineClientBody
}

type DeleteMachineClientInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	ID            int64       `path:"id" minimum:"1"`
}

type DeleteMachineClientOutput struct{}

func registerMachineClientRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listMachineClients",
		Method:      http.MethodGet,
		Path:        "/api/v1/machine-clients",
		Summary:     "machine client を list する",
		Tags:        []string{"machine-clients"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ListMachineClientsInput) (*ListMachineClientsOutput, error) {
		if _, err := requireMachineClientAdmin(ctx, deps, input.SessionCookie.Value, ""); err != nil {
			return nil, err
		}
		items, err := deps.MachineClientService.List(ctx)
		if err != nil {
			return nil, toMachineClientHTTPError(err)
		}
		out := &ListMachineClientsOutput{}
		out.Body.Items = make([]MachineClientBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toMachineClientBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createMachineClient",
		Method:      http.MethodPost,
		Path:        "/api/v1/machine-clients",
		Summary:     "machine client を作成する",
		Tags:        []string{"machine-clients"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *CreateMachineClientInput) (*CreateMachineClientOutput, error) {
		current, err := requireMachineClientAdmin(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.MachineClientService.Create(ctx, machineClientInputFromBody(input.Body), sessionAuditContext(ctx, current, nil))
		if err != nil {
			return nil, toMachineClientHTTPError(err)
		}
		return &CreateMachineClientOutput{Body: toMachineClientBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getMachineClient",
		Method:      http.MethodGet,
		Path:        "/api/v1/machine-clients/{id}",
		Summary:     "machine client を取得する",
		Tags:        []string{"machine-clients"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *MachineClientByIDInput) (*CreateMachineClientOutput, error) {
		if _, err := requireMachineClientAdmin(ctx, deps, input.SessionCookie.Value, ""); err != nil {
			return nil, err
		}
		item, err := deps.MachineClientService.Get(ctx, input.ID)
		if err != nil {
			return nil, toMachineClientHTTPError(err)
		}
		return &CreateMachineClientOutput{Body: toMachineClientBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateMachineClient",
		Method:      http.MethodPut,
		Path:        "/api/v1/machine-clients/{id}",
		Summary:     "machine client を更新する",
		Tags:        []string{"machine-clients"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *UpdateMachineClientInput) (*UpdateMachineClientOutput, error) {
		current, err := requireMachineClientAdmin(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.MachineClientService.Update(ctx, input.ID, machineClientInputFromBody(input.Body), sessionAuditContext(ctx, current, nil))
		if err != nil {
			return nil, toMachineClientHTTPError(err)
		}
		return &UpdateMachineClientOutput{Body: toMachineClientBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteMachineClient",
		Method:        http.MethodDelete,
		Path:          "/api/v1/machine-clients/{id}",
		Summary:       "machine client を無効化する",
		Tags:          []string{"machine-clients"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *DeleteMachineClientInput) (*DeleteMachineClientOutput, error) {
		current, err := requireMachineClientAdmin(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if _, err := deps.MachineClientService.Disable(ctx, input.ID, sessionAuditContext(ctx, current, nil)); err != nil {
			return nil, toMachineClientHTTPError(err)
		}
		return &DeleteMachineClientOutput{}, nil
	})
}

func requireMachineClientAdmin(ctx context.Context, deps Dependencies, sessionID, csrfToken string) (service.CurrentSession, error) {
	if deps.MachineClientService == nil {
		return service.CurrentSession{}, huma.Error503ServiceUnavailable("machine client service is not configured")
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
		if errors.Is(err, service.ErrUnauthorized) ||
			errors.Is(err, service.ErrInvalidCSRFToken) ||
			errors.Is(err, service.ErrAuthModeUnsupported) ||
			errors.Is(err, service.ErrInvalidCredentials) {
			return service.CurrentSession{}, toHTTPError(err)
		}
		return service.CurrentSession{}, err
	}
	if !authCtx.HasRole("machine_client_admin") {
		return service.CurrentSession{}, huma.Error403Forbidden("machine_client_admin role is required")
	}
	return current, nil
}

func machineClientInputFromBody(body MachineClientRequestBody) service.MachineClientInput {
	return service.MachineClientInput{
		Provider:         body.Provider,
		ProviderClientID: body.ProviderClientID,
		DisplayName:      body.DisplayName,
		DefaultTenantID:  body.DefaultTenantID,
		AllowedScopes:    body.AllowedScopes,
		Active:           body.Active,
	}
}

func toMachineClientBody(item service.MachineClient) MachineClientBody {
	body := MachineClientBody{
		ID:               item.ID,
		Provider:         item.Provider,
		ProviderClientID: item.ProviderClientID,
		DisplayName:      item.DisplayName,
		AllowedScopes:    append([]string(nil), item.AllowedScopes...),
		Active:           item.Active,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
	if item.DefaultTenant != nil {
		tenant := toTenantBody(*item.DefaultTenant)
		body.DefaultTenant = &tenant
	}
	return body
}

func toMachineClientHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidMachineClient):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, service.ErrMachineClientNotFound):
		return huma.Error404NotFound("machine client not found")
	case errors.Is(err, service.ErrMachineClientInactive), errors.Is(err, service.ErrMachineClientScopeDenied):
		return huma.Error403Forbidden(err.Error())
	default:
		return huma.Error500InternalServerError("internal server error")
	}
}

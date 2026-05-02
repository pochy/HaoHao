package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type IntegrationStatusBody struct {
	ResourceServer  string     `json:"resourceServer" example:"zitadel"`
	Provider        string     `json:"provider" example:"zitadel"`
	Connected       bool       `json:"connected" example:"true"`
	Scopes          []string   `json:"scopes,omitempty" example:"offline_access"`
	GrantedAt       *time.Time `json:"grantedAt,omitempty" format:"date-time"`
	LastRefreshedAt *time.Time `json:"lastRefreshedAt,omitempty" format:"date-time"`
	RevokedAt       *time.Time `json:"revokedAt,omitempty" format:"date-time"`
	LastErrorCode   string     `json:"lastErrorCode,omitempty" example:"invalid_grant"`
}

type ListIntegrationsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

type ListIntegrationsBody struct {
	Items []IntegrationStatusBody `json:"items"`
}

type ListIntegrationsOutput struct {
	Body ListIntegrationsBody
}

type ConnectIntegrationInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	ResourceServer string      `path:"resourceServer" example:"zitadel"`
}

type ConnectIntegrationOutput struct {
	Location string `header:"Location"`
}

type IntegrationCallbackInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	ResourceServer   string      `path:"resourceServer" example:"zitadel"`
	Code             string      `query:"code"`
	State            string      `query:"state"`
	Error            string      `query:"error"`
	ErrorDescription string      `query:"error_description"`
}

type IntegrationCallbackOutput struct {
	Location string `header:"Location"`
}

type VerifyIntegrationInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	ResourceServer string      `path:"resourceServer" example:"zitadel"`
}

type VerifyIntegrationBody struct {
	ResourceServer  string     `json:"resourceServer" example:"zitadel"`
	Connected       bool       `json:"connected" example:"true"`
	Scopes          []string   `json:"scopes,omitempty" example:"offline_access"`
	AccessExpiresAt *time.Time `json:"accessExpiresAt,omitempty" format:"date-time"`
	RefreshedAt     *time.Time `json:"refreshedAt,omitempty" format:"date-time"`
}

type VerifyIntegrationOutput struct {
	Body VerifyIntegrationBody
}

type DeleteIntegrationGrantInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	ResourceServer string      `path:"resourceServer" example:"zitadel"`
}

type DeleteIntegrationGrantOutput struct{}

func registerIntegrationRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listIntegrations",
		Method:      http.MethodGet,
		Path:        "/api/v1/integrations",
		Summary:     "downstream integration の接続状態を返す",
		Tags:        []string{"integrations"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ListIntegrationsInput) (*ListIntegrationsOutput, error) {
		if deps.DelegationService == nil {
			return nil, huma.Error503ServiceUnavailable("delegated auth is not configured")
		}

		current, authCtx, err := currentSessionAuthContext(ctx, deps, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPErrorWithLog(ctx, deps, "", err)
		}
		if authCtx.ActiveTenant == nil {
			return nil, huma.Error409Conflict("active tenant is required before connecting integrations")
		}

		statuses, err := deps.DelegationService.ListIntegrationsForTenant(ctx, current.User, authCtx.ActiveTenant.ID)
		if err != nil {
			return nil, toDelegationHTTPError(err)
		}

		out := &ListIntegrationsOutput{}
		out.Body.Items = make([]IntegrationStatusBody, 0, len(statuses))
		for _, status := range statuses {
			out.Body.Items = append(out.Body.Items, toIntegrationStatusBody(status))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "connectIntegration",
		Method:        http.MethodGet,
		Path:          "/api/v1/integrations/{resourceServer}/connect",
		Summary:       "downstream integration consent を開始する",
		Tags:          []string{"integrations"},
		DefaultStatus: http.StatusFound,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ConnectIntegrationInput) (*ConnectIntegrationOutput, error) {
		if deps.DelegationService == nil {
			return nil, huma.Error503ServiceUnavailable("delegated auth is not configured")
		}

		current, authCtx, err := currentSessionAuthContext(ctx, deps, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPErrorWithLog(ctx, deps, "", err)
		}
		if authCtx.ActiveTenant == nil {
			return nil, huma.Error409Conflict("active tenant is required before connecting integrations")
		}

		location, err := deps.DelegationService.StartConnectForTenant(
			ctx,
			current.User,
			authCtx.ActiveTenant.ID,
			input.SessionCookie.Value,
			input.ResourceServer,
			sessionAuditContext(ctx, current, &authCtx.ActiveTenant.ID),
		)
		if err != nil {
			return nil, toDelegationHTTPError(err)
		}

		return &ConnectIntegrationOutput{Location: location}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "finishIntegrationConnect",
		Method:        http.MethodGet,
		Path:          "/api/v1/integrations/{resourceServer}/callback",
		Summary:       "downstream integration consent callback を完了する",
		Tags:          []string{"integrations"},
		DefaultStatus: http.StatusFound,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *IntegrationCallbackInput) (*IntegrationCallbackOutput, error) {
		if input.Error != "" || deps.DelegationService == nil {
			return &IntegrationCallbackOutput{
				Location: integrationRedirect(deps.FrontendBaseURL, "error", "delegated_callback_failed"),
			}, nil
		}

		user, err := deps.SessionService.CurrentUser(ctx, input.SessionCookie.Value)
		if err != nil {
			return &IntegrationCallbackOutput{
				Location: integrationRedirect(deps.FrontendBaseURL, "error", "missing_session"),
			}, nil
		}

		if _, err := deps.DelegationService.SaveGrantFromCallback(
			ctx,
			user,
			input.SessionCookie.Value,
			input.ResourceServer,
			input.Code,
			input.State,
			service.UserAuditContext(user.ID, nil, auditRequest(ctx)),
		); err != nil {
			return &IntegrationCallbackOutput{
				Location: integrationRedirect(deps.FrontendBaseURL, "error", "delegated_callback_failed"),
			}, nil
		}

		return &IntegrationCallbackOutput{
			Location: integrationRedirect(deps.FrontendBaseURL, "connected", normalizeIntegrationResource(input.ResourceServer)),
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "verifyIntegrationAccess",
		Method:      http.MethodPost,
		Path:        "/api/v1/integrations/{resourceServer}/verify",
		Summary:     "downstream access token を backend 内で取得できるか検証する",
		Tags:        []string{"integrations"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *VerifyIntegrationInput) (*VerifyIntegrationOutput, error) {
		if deps.DelegationService == nil {
			return nil, huma.Error503ServiceUnavailable("delegated auth is not configured")
		}

		current, authCtx, err := currentSessionAuthContextWithCSRF(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toHTTPErrorWithLog(ctx, deps, "", err)
		}
		if authCtx.ActiveTenant == nil {
			return nil, huma.Error409Conflict("active tenant is required before verifying integrations")
		}

		result, err := deps.DelegationService.VerifyAccessTokenForTenant(
			ctx,
			current.User,
			authCtx.ActiveTenant.ID,
			input.ResourceServer,
			sessionAuditContext(ctx, current, &authCtx.ActiveTenant.ID),
		)
		if err != nil {
			return nil, toDelegationHTTPError(err)
		}

		out := &VerifyIntegrationOutput{}
		out.Body.ResourceServer = result.ResourceServer
		out.Body.Connected = result.Connected
		out.Body.Scopes = result.Scopes
		out.Body.AccessExpiresAt = result.AccessExpiresAt
		out.Body.RefreshedAt = result.RefreshedAt
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteIntegrationGrant",
		Method:        http.MethodDelete,
		Path:          "/api/v1/integrations/{resourceServer}/grant",
		Summary:       "downstream integration grant を削除する",
		Tags:          []string{"integrations"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *DeleteIntegrationGrantInput) (*DeleteIntegrationGrantOutput, error) {
		if deps.DelegationService == nil {
			return nil, huma.Error503ServiceUnavailable("delegated auth is not configured")
		}

		current, authCtx, err := currentSessionAuthContextWithCSRF(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toHTTPErrorWithLog(ctx, deps, "", err)
		}
		if authCtx.ActiveTenant == nil {
			return nil, huma.Error409Conflict("active tenant is required before deleting integrations")
		}

		if err := deps.DelegationService.DeleteGrantForTenant(
			ctx,
			current.User,
			authCtx.ActiveTenant.ID,
			input.ResourceServer,
			sessionAuditContext(ctx, current, &authCtx.ActiveTenant.ID),
		); err != nil {
			return nil, toDelegationHTTPError(err)
		}

		return &DeleteIntegrationGrantOutput{}, nil
	})
}

func toIntegrationStatusBody(status service.DelegationStatus) IntegrationStatusBody {
	return IntegrationStatusBody{
		ResourceServer:  status.ResourceServer,
		Provider:        status.Provider,
		Connected:       status.Connected,
		Scopes:          status.Scopes,
		GrantedAt:       status.GrantedAt,
		LastRefreshedAt: status.LastRefreshedAt,
		RevokedAt:       status.RevokedAt,
		LastErrorCode:   status.LastErrorCode,
	}
}

func toDelegationHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrDelegationNotConfigured):
		return huma.Error503ServiceUnavailable("delegated auth is not configured")
	case errors.Is(err, service.ErrDelegationUnsupportedResource):
		return huma.Error404NotFound("unsupported downstream resource")
	case errors.Is(err, service.ErrDelegationGrantNotFound):
		return huma.Error404NotFound("delegated grant not found")
	case errors.Is(err, service.ErrDelegationInvalidState):
		return huma.Error400BadRequest("invalid delegated auth state")
	case errors.Is(err, service.ErrDelegationIdentityNotFound):
		return huma.Error409Conflict("zitadel identity is required before connecting the integration")
	case errors.Is(err, service.ErrDelegationRefreshTokenMissing):
		return huma.Error502BadGateway("provider did not return a refresh token")
	default:
		return huma.Error500InternalServerError("internal server error")
	}
}

func currentSessionAuthContext(ctx context.Context, deps Dependencies, sessionID string) (service.CurrentSession, service.AuthContext, error) {
	current, err := deps.SessionService.CurrentSession(ctx, sessionID)
	if err != nil {
		return service.CurrentSession{}, service.AuthContext{}, err
	}
	if deps.AuthzService == nil {
		return current, service.AuthContext{}, huma.Error503ServiceUnavailable("tenant auth is not configured")
	}
	authCtx, err := deps.AuthzService.BuildBrowserContext(ctx, current.User, current.ActiveTenantID)
	if err != nil {
		return service.CurrentSession{}, service.AuthContext{}, err
	}
	return current, authCtx, nil
}

func currentSessionAuthContextWithCSRF(ctx context.Context, deps Dependencies, sessionID, csrfToken string) (service.CurrentSession, service.AuthContext, error) {
	current, err := deps.SessionService.CurrentSessionWithCSRF(ctx, sessionID, csrfToken)
	if err != nil {
		return service.CurrentSession{}, service.AuthContext{}, err
	}
	if deps.AuthzService == nil {
		return current, service.AuthContext{}, huma.Error503ServiceUnavailable("tenant auth is not configured")
	}
	authCtx, err := deps.AuthzService.BuildBrowserContext(ctx, current.User, current.ActiveTenantID)
	if err != nil {
		return service.CurrentSession{}, service.AuthContext{}, err
	}
	return current, authCtx, nil
}

func integrationRedirect(frontendBaseURL, key, value string) string {
	base := strings.TrimRight(frontendBaseURL, "/")
	query := url.Values{}
	query.Set(key, value)
	return base + "/integrations?" + query.Encode()
}

func normalizeIntegrationResource(resourceServer string) string {
	return strings.ToLower(strings.TrimSpace(resourceServer))
}

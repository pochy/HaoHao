package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"example.com/haohao/backend/internal/auth"

	"github.com/danielgtaylor/huma/v2"
)

type StartOIDCLoginInput struct {
	ReturnTo  string `query:"returnTo"`
	LoginHint string `query:"loginHint"`
}

type StartOIDCLoginOutput struct {
	Location string `header:"Location"`
}

type OIDCCallbackInput struct {
	Code             string `query:"code"`
	State            string `query:"state"`
	Error            string `query:"error"`
	ErrorDescription string `query:"error_description"`
}

type OIDCCallbackOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Location  string        `header:"Location"`
}

func registerOIDCRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID:   "startOIDCLogin",
		Method:        http.MethodGet,
		Path:          "/api/v1/auth/login",
		Summary:       "OIDC login を開始する",
		Tags:          []string{DocTagAuthSession},
		DefaultStatus: http.StatusFound,
	}, func(ctx context.Context, input *StartOIDCLoginInput) (*StartOIDCLoginOutput, error) {
		if deps.OIDCLoginService == nil {
			return nil, huma.Error501NotImplemented("oidc login is not configured")
		}

		location, err := deps.OIDCLoginService.StartLogin(ctx, input.ReturnTo, input.LoginHint)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to start oidc login")
		}

		return &StartOIDCLoginOutput{Location: location}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "finishOIDCLogin",
		Method:        http.MethodGet,
		Path:          "/api/v1/auth/callback",
		Summary:       "OIDC callback を完了する",
		Tags:          []string{DocTagAuthSession},
		DefaultStatus: http.StatusFound,
	}, func(ctx context.Context, input *OIDCCallbackInput) (*OIDCCallbackOutput, error) {
		if input.Error != "" || deps.OIDCLoginService == nil {
			return &OIDCCallbackOutput{
				Location: oidcFailureRedirect(deps.FrontendBaseURL),
			}, nil
		}

		result, err := deps.OIDCLoginService.FinishLogin(ctx, input.Code, input.State, auditRequest(ctx))
		if err != nil {
			return &OIDCCallbackOutput{
				Location: oidcFailureRedirect(deps.FrontendBaseURL),
			}, nil
		}

		return &OIDCCallbackOutput{
			SetCookie: []http.Cookie{
				auth.NewSessionCookie(result.SessionID, deps.CookieSecure, deps.SessionTTL),
				auth.NewXSRFCookie(result.CSRFToken, deps.CookieSecure, deps.SessionTTL),
			},
			Location: oidcSuccessRedirect(deps.FrontendBaseURL, result.ReturnTo),
		}, nil
	})
}

func oidcFailureRedirect(frontendBaseURL string) string {
	return strings.TrimRight(frontendBaseURL, "/") + "/login?error=oidc_callback_failed"
}

func oidcSuccessRedirect(frontendBaseURL, returnTo string) string {
	base := strings.TrimRight(frontendBaseURL, "/")
	if returnTo == "" || !strings.HasPrefix(returnTo, "/") || strings.HasPrefix(returnTo, "//") {
		return base + "/"
	}
	return base + returnTo
}

func buildPostLogoutURL(deps Dependencies, idTokenHint string) string {
	if deps.AuthMode != "zitadel" || deps.ZitadelIssuer == "" || deps.ZitadelClientID == "" || deps.ZitadelPostLogoutRedirectURI == "" {
		return ""
	}

	endSessionURL, err := url.Parse(strings.TrimRight(deps.ZitadelIssuer, "/") + "/oidc/v1/end_session")
	if err != nil {
		return ""
	}

	query := endSessionURL.Query()
	if idTokenHint != "" {
		query.Set("id_token_hint", idTokenHint)
	} else {
		query.Set("client_id", deps.ZitadelClientID)
	}
	query.Set("post_logout_redirect_uri", deps.ZitadelPostLogoutRedirectURI)
	endSessionURL.RawQuery = query.Encode()

	return endSessionURL.String()
}

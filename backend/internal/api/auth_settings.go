package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type AuthSettingsBody struct {
	Mode                      string               `json:"mode" example:"local"`
	LocalPasswordLoginEnabled bool                 `json:"localPasswordLoginEnabled" example:"true"`
	Zitadel                   *ZitadelSettingsBody `json:"zitadel,omitempty"`
}

type ZitadelSettingsBody struct {
	Issuer   string `json:"issuer" format:"uri" example:"http://localhost:8081"`
	ClientID string `json:"clientId" example:"312345678901234567"`
}

type GetAuthSettingsOutput struct {
	Body AuthSettingsBody
}

func registerAuthSettingsRoute(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "getAuthSettings",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/settings",
		Summary:     "現在の認証モード設定を返す",
		Tags:        []string{"auth"},
	}, func(ctx context.Context, input *struct{}) (*GetAuthSettingsOutput, error) {
		body := AuthSettingsBody{
			Mode:                      deps.AuthMode,
			LocalPasswordLoginEnabled: deps.EnableLocalPasswordLogin,
		}

		if deps.AuthMode == "zitadel" {
			body.Zitadel = &ZitadelSettingsBody{
				Issuer:   deps.ZitadelIssuer,
				ClientID: deps.ZitadelClientID,
			}
		}

		return &GetAuthSettingsOutput{Body: body}, nil
	})
}

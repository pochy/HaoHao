package api

import (
	"context"
	"net/http"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type M2MSelfBody struct {
	ID            int64       `json:"id" example:"1"`
	DisplayName   string      `json:"displayName" example:"worker"`
	DefaultTenant *TenantBody `json:"defaultTenant,omitempty"`
	AllowedScopes []string    `json:"allowedScopes,omitempty" example:"m2m:read"`
	Active        bool        `json:"active" example:"true"`
}

type GetM2MSelfOutput struct {
	Body M2MSelfBody
}

func registerM2MRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "getM2MSelf",
		Method:      http.MethodGet,
		Path:        "/api/m2m/v1/self",
		Summary:     "現在の M2M machine client を返す",
		Tags:        []string{DocTagExternalAPIs},
		Security: []map[string][]string{
			{"m2mBearerAuth": {}},
		},
	}, func(ctx context.Context, input *struct{}) (*GetM2MSelfOutput, error) {
		machineCtx, ok := service.MachineClientFromContext(ctx)
		if !ok {
			return nil, huma.Error500InternalServerError("missing machine client context")
		}
		return &GetM2MSelfOutput{
			Body: toM2MSelfBody(machineCtx.Client),
		}, nil
	})
}

func toM2MSelfBody(client service.MachineClient) M2MSelfBody {
	body := M2MSelfBody{
		ID:            client.ID,
		DisplayName:   client.DisplayName,
		AllowedScopes: append([]string(nil), client.AllowedScopes...),
		Active:        client.Active,
	}
	if client.DefaultTenant != nil {
		tenant := toTenantBody(*client.DefaultTenant)
		body.DefaultTenant = &tenant
	}
	return body
}

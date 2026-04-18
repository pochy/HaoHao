package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/pochy/haohao/backend/internal/config"
)

type ExternalHealthBody struct {
	Status  string    `json:"status" example:"ok"`
	Service string    `json:"service" example:"haohao-external-api"`
	Version string    `json:"version" example:"0.1.0"`
	Time    time.Time `json:"time" format:"date-time"`
}

type ExternalHealthOutput struct {
	Body ExternalHealthBody
}

func RegisterHealth(api huma.API, cfg config.Config) {
	huma.Register(api, huma.Operation{
		OperationID: "getExternalHealth",
		Method:      http.MethodGet,
		Path:        "/external/v1/health",
		Summary:     "Get external API health",
		Tags:        []string{"external-system"},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *struct{}) (*ExternalHealthOutput, error) {
		_ = ctx
		_ = input

		return &ExternalHealthOutput{
			Body: ExternalHealthBody{
				Status:  "ok",
				Service: "haohao-external-api",
				Version: cfg.Version,
				Time:    time.Now().UTC(),
			},
		}, nil
	})
}

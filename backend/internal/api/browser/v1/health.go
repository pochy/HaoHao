package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/pochy/haohao/backend/internal/config"
)

type HealthBody struct {
	Status  string    `json:"status" example:"ok"`
	Service string    `json:"service" example:"haohao-browser-api"`
	Version string    `json:"version" example:"0.1.0"`
	Time    time.Time `json:"time" format:"date-time"`
}

type HealthOutput struct {
	Body HealthBody
}

func RegisterHealth(api huma.API, cfg config.Config) {
	huma.Register(api, huma.Operation{
		OperationID: "getHealth",
		Method:      http.MethodGet,
		Path:        "/api/v1/health",
		Summary:     "Get service health",
		Tags:        []string{"system"},
	}, func(ctx context.Context, input *struct{}) (*HealthOutput, error) {
		_ = ctx
		_ = input

		return &HealthOutput{
			Body: HealthBody{
				Status:  "ok",
				Service: "haohao-browser-api",
				Version: cfg.Version,
				Time:    time.Now().UTC(),
			},
		}, nil
	})
}


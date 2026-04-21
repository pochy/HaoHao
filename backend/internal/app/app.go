package app

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
)

type App struct {
	Router *gin.Engine
	API    huma.API
}

type HealthOutput struct {
	Body struct {
		OK bool `json:"ok"`
	}
}

func Build() *App {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	config := huma.DefaultConfig("HaoHao API", "0.1.0")
	api := humagin.New(router, config)

	huma.Get(api, "/api/v1/health", func(ctx context.Context, input *struct{}) (*HealthOutput, error) {
		out := &HealthOutput{}
		out.Body.OK = true
		return out, nil
	})

	return &App{Router: router, API: api}
}
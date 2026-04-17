package main

import (
	"io"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/pochy/haohao/backend/internal/app"
	"github.com/pochy/haohao/backend/internal/config"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard

	cfg := config.Load()

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("build app: %v", err)
	}

	spec, err := application.API.OpenAPI().YAML()
	if err != nil {
		log.Fatalf("render openapi: %v", err)
	}

	if _, err := os.Stdout.Write(spec); err != nil {
		log.Fatalf("write openapi: %v", err)
	}
}

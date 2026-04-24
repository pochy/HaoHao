package main

import (
	"fmt"
	"log"

	"example.com/haohao/backend/internal/app"
	"example.com/haohao/backend/internal/config"
	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	application := app.New(cfg, nil, nil, nil, nil, nil)

	spec, err := application.API.OpenAPI().YAML()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(string(spec))
}

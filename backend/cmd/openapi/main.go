package main

import (
	"flag"
	"fmt"
	"log"

	backendapi "example.com/haohao/backend/internal/api"
	"example.com/haohao/backend/internal/app"
	"example.com/haohao/backend/internal/config"
	"github.com/gin-gonic/gin"
)

func main() {
	surfaceFlag := flag.String("surface", string(backendapi.SurfaceFull), "OpenAPI surface to export: full, browser, or external")
	flag.Parse()

	gin.SetMode(gin.ReleaseMode)

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	openAPI, err := app.NewOpenAPIExport(cfg, backendapi.Surface(*surfaceFlag))
	if err != nil {
		log.Fatal(err)
	}

	spec, err := openAPI.YAML()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(string(spec))
}

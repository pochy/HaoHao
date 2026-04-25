package main

import (
	"fmt"
	"log"

	"example.com/haohao/backend/internal/app"
	"example.com/haohao/backend/internal/config"
	"example.com/haohao/backend/internal/service"
	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	auditService := service.NewAuditService(nil)
	application := app.New(
		cfg,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		auditService,
		service.NewTenantAdminService(nil, nil, auditService),
		service.NewTodoService(nil, nil, auditService),
		service.NewMachineClientService(nil, nil, "", auditService),
		nil,
		nil,
		nil,
	)

	spec, err := application.API.OpenAPI().YAML()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(string(spec))
}

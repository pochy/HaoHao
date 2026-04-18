package main

import (
	"log"
	"net/http"

	"github.com/pochy/haohao/backend/internal/app"
	"github.com/pochy/haohao/backend/internal/config"
)

func main() {
	cfg := config.Load()
	if err := cfg.ValidateAuthRuntime(); err != nil {
		log.Fatalf("invalid auth config: %v", err)
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("build app: %v", err)
	}

	log.Printf("listening on %s", cfg.Address)
	if err := http.ListenAndServe(cfg.Address, application.Router); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

package main

import (
	"log"
	"os"

	"github.com/pochy/haohao/backend/internal/app"
)

func main() {
	built := app.Build()

	if len(os.Args) > 1 && os.Args[1] == "openapi" {
		b, err := built.API.OpenAPI().YAML()
		if err != nil {
			log.Fatal(err)
		}
		if _, err := os.Stdout.Write(b); err != nil {
			log.Fatal(err)
		}
		return
	}

	log.Fatal(built.Router.Run(":8080"))
}
COMPOSE ?= $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; else echo "docker compose"; fi)
SQLC ?= go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0

.PHONY: gen openapi client sqlc backend frontend build-frontend compose-up compose-down

gen: openapi client sqlc

openapi:
	go run ./backend/cmd/openapi > openapi/openapi.yaml

client:
	npm --prefix frontend run gen:api

sqlc:
	$(SQLC) generate -f backend/sqlc.yaml

backend:
	go run ./backend/cmd/server

frontend:
	npm --prefix frontend run dev

build-frontend:
	npm --prefix frontend run build

compose-up:
	$(COMPOSE) up -d postgres redis

compose-down:
	$(COMPOSE) down


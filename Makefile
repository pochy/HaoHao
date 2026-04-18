COMPOSE ?= $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; else echo "docker compose"; fi)
SQLC ?= go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0

GENERATED_ARTIFACTS := openapi/openapi.yaml frontend/src/api/generated backend/internal/db

.PHONY: gen openapi client sqlc check-generated openapi-lint sqlc-vet backend frontend build-frontend compose-up compose-down

gen: openapi client sqlc

openapi:
	go run ./backend/cmd/openapi > openapi/openapi.yaml

client:
	npm --prefix frontend run gen:api

sqlc:
	$(SQLC) generate -f backend/sqlc.yaml

check-generated: gen
	git diff --exit-code -- $(GENERATED_ARTIFACTS)

openapi-lint:
	npm --prefix frontend run lint:openapi

sqlc-vet:
	$(SQLC) vet -f backend/sqlc.ci.yaml

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

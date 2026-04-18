COMPOSE ?= $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; else echo "docker compose"; fi)
PSQL ?= psql
SQLC ?= go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0
SQLC_CONFIG ?= backend/sqlc.yaml
SQLC_CI_CONFIG ?= backend/sqlc.ci.yaml

GENERATED_ARTIFACTS := openapi/openapi.yaml frontend/src/api/generated backend/internal/db

.PHONY: gen openapi client sqlc check-generated openapi-lint sqlc-load-schema sqlc-compile sqlc-vet sqlc-check backend frontend build-frontend compose-up compose-down

gen: openapi client sqlc

openapi:
	go run ./backend/cmd/openapi > openapi/openapi.yaml

client:
	npm --prefix frontend run gen:api

sqlc:
	$(SQLC) generate -f $(SQLC_CONFIG)

check-generated: gen
	git diff --exit-code -- $(GENERATED_ARTIFACTS)

openapi-lint:
	npm --prefix frontend run lint:openapi

sqlc-load-schema:
	@test -n "$(POSTGRESQL_SERVER_URI)" || (echo "POSTGRESQL_SERVER_URI is required. Example: postgresql://haohao:haohao@localhost:5432/haohao?sslmode=disable" >&2; exit 1)
	$(PSQL) "$(POSTGRESQL_SERVER_URI)" -v ON_ERROR_STOP=1 -f db/schema.sql

sqlc-compile:
	$(SQLC) compile -f $(SQLC_CONFIG) --no-remote

sqlc-vet:
	@test -n "$(POSTGRESQL_SERVER_URI)" || (echo "POSTGRESQL_SERVER_URI is required. Example: postgresql://haohao:haohao@localhost:5432/haohao?sslmode=disable" >&2; exit 1)
	$(SQLC) vet -f $(SQLC_CI_CONFIG) --no-remote

sqlc-check:
	$(MAKE) sqlc
	$(MAKE) sqlc-compile
	$(MAKE) sqlc-vet

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

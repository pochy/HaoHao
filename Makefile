SHELL := /bin/bash

export-env = set -a && source .env && set +a
AIR_BIN ?= air
DOCKER_COMPOSE := $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; else echo "docker compose"; fi)
ZITADEL_ENV_FILE := dev/zitadel/.env
ZITADEL_ENV_EXAMPLE := dev/zitadel/.env.example
ZITADEL_COMPOSE_FILE := dev/zitadel/docker-compose.yml
ZITADEL_COMPOSE := $(DOCKER_COMPOSE) --env-file $(ZITADEL_ENV_FILE) -f $(ZITADEL_COMPOSE_FILE)
GO_BINARY_BUILD_FLAGS := -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid="

up:
	$(DOCKER_COMPOSE) up -d

down:
	$(DOCKER_COMPOSE) down

seaweedfs-up:
	$(DOCKER_COMPOSE) --profile seaweedfs up -d seaweedfs

seaweedfs-config:
	$(DOCKER_COMPOSE) --profile seaweedfs config

seaweedfs-down:
	$(DOCKER_COMPOSE) --profile seaweedfs stop seaweedfs

seaweedfs-logs:
	$(DOCKER_COMPOSE) --profile seaweedfs logs -f seaweedfs

zitadel-env:
	@test -f $(ZITADEL_ENV_FILE) || cp $(ZITADEL_ENV_EXAMPLE) $(ZITADEL_ENV_FILE)

zitadel-up: zitadel-env
	$(ZITADEL_COMPOSE) up -d --wait

zitadel-down: zitadel-env
	$(ZITADEL_COMPOSE) down

zitadel-ps: zitadel-env
	$(ZITADEL_COMPOSE) ps

zitadel-logs: zitadel-env
	$(ZITADEL_COMPOSE) logs -f

db-wait:
	@until $(DOCKER_COMPOSE) exec -T postgres pg_isready -U haohao -d haohao >/dev/null 2>&1; do sleep 1; done

db-up: db-wait
	$(export-env) && migrate -path db/migrations -database "$$DATABASE_URL" up

db-down:
	$(export-env) && migrate -path db/migrations -database "$$DATABASE_URL" down 1

# Prefer Homebrew postgresql@18's psql-18 when on PATH; override with make psql PSQL=psql
PSQL ?= $(shell command -v psql-18 2>/dev/null || command -v psql 2>/dev/null || echo psql)

.PHONY: psql sql air-check backend-dev backend-run
psql:
	$(export-env) && $(PSQL) "$$DATABASE_URL" $(ARGS)

sql: psql

db-schema: db-wait
	$(DOCKER_COMPOSE) exec -T postgres pg_dump --schema-only --no-owner --no-privileges -U haohao -d haohao | sed '/^\\restrict /d; /^\\unrestrict /d' | perl -0pe 's/\n+\z/\n/' > db/schema.sql

seed-demo-user: db-wait
	$(DOCKER_COMPOSE) exec -T postgres psql -U haohao -d haohao < scripts/seed-demo-user.sql

sqlc:
	cd backend && sqlc generate

openapi:
	mkdir -p openapi
	go run ./backend/cmd/openapi -surface=full > openapi/openapi.yaml
	go run ./backend/cmd/openapi -surface=browser > openapi/browser.yaml
	go run ./backend/cmd/openapi -surface=external > openapi/external.yaml

gen:
	./scripts/gen.sh

air-check:
	@command -v $(AIR_BIN) >/dev/null 2>&1 || { \
		echo "air is not installed. Install it with:"; \
		echo "  go install github.com/air-verse/air@latest"; \
		echo ""; \
		echo "If air is still not found, add Go's bin directory to PATH:"; \
		echo '  export PATH=$$PATH:$$(go env GOPATH)/bin'; \
		exit 1; \
	}

backend-dev: air-check
	$(export-env) && $(AIR_BIN) -c .air.toml

backend-run:
	$(export-env) && go run ./backend/cmd/main

frontend-dev:
	cd frontend && npm run dev

frontend-build:
	cd frontend && npm run build

binary: frontend-build
	mkdir -p bin
	CGO_ENABLED=0 go build $(GO_BINARY_BUILD_FLAGS) -o ./bin/haohao ./backend/cmd/main

docker-build:
	docker build -t haohao:dev -f docker/Dockerfile .

openfga-bootstrap:
	bash scripts/openfga-bootstrap.sh

test-openfga-model:
	cd openfga && fga model test --tests drive.fga.yaml

smoke-operability:
	bash scripts/smoke-operability.sh

smoke-observability:
	bash scripts/smoke-observability.sh

smoke-tenant-admin:
	bash scripts/smoke-tenant-admin.sh

smoke-customer-signals:
	bash scripts/smoke-customer-signals.sh

smoke-common-services:
	bash scripts/smoke-common-services.sh

smoke-p10:
	bash scripts/smoke-p10.sh

smoke-rate-limit-runtime:
	bash scripts/smoke-rate-limit-runtime.sh

smoke-file-purge: binary
	bash scripts/smoke-file-purge.sh

smoke-openfga:
	bash scripts/smoke-openfga.sh

smoke-backup-restore:
	bash scripts/smoke-backup-restore.sh

e2e: binary
	bash scripts/e2e-single-binary.sh

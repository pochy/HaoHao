SHELL := /bin/bash

export-env = set -a && source .env && set +a
AIR_BIN ?= air
DOCKER_COMPOSE := $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; else echo "docker compose"; fi)
ZITADEL_ENV_FILE := dev/zitadel/.env
ZITADEL_ENV_EXAMPLE := dev/zitadel/.env.example
ZITADEL_COMPOSE_FILE := dev/zitadel/docker-compose.yml
ZITADEL_COMPOSE := $(DOCKER_COMPOSE) --env-file $(ZITADEL_ENV_FILE) -f $(ZITADEL_COMPOSE_FILE)
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BINARY_PATH := ./bin/haohao
BINARY_PACKAGE_BASENAME := haohao_$(GOOS)_$(GOARCH)
BINARY_PACKAGE_GZIP := ./bin/$(BINARY_PACKAGE_BASENAME).tar.gz
BINARY_PACKAGE_ZSTD := ./bin/$(BINARY_PACKAGE_BASENAME).tar.zst
BINARY_MAX_BYTES ?= 57671680
BINARY_GZIP_MAX_BYTES ?= 20971520
GO_BINARY_TAGS := embed_frontend nomsgpack
GO_BINARY_LDFLAGS := -s -w -buildid=
GO_BINARY_BASE_FLAGS := -buildvcs=false -trimpath -tags "$(GO_BINARY_TAGS)" -ldflags "$(GO_BINARY_LDFLAGS)"
GO_BINARY_BUILD_FLAGS := $(GO_BINARY_BASE_FLAGS) -gcflags=all=-l
GO_BINARY_FAST_BUILD_FLAGS := $(GO_BINARY_BASE_FLAGS)

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

openwebui-up:
	$(DOCKER_COMPOSE) --profile openwebui up -d open-webui

openwebui-stack-up:
	$(DOCKER_COMPOSE) --profile openwebui up -d open-webui infinity

openwebui-config:
	$(DOCKER_COMPOSE) --profile openwebui config

openwebui-down:
	$(DOCKER_COMPOSE) --profile openwebui stop open-webui infinity

openwebui-logs:
	$(DOCKER_COMPOSE) --profile openwebui logs -f open-webui infinity

infinity-up:
	$(DOCKER_COMPOSE) --profile infinity up -d infinity

infinity-down:
	$(DOCKER_COMPOSE) --profile infinity stop infinity

infinity-build:
	$(DOCKER_COMPOSE) --profile infinity build infinity

infinity-logs:
	$(DOCKER_COMPOSE) --profile infinity logs -f infinity

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

clickhouse-wait:
	@until $(DOCKER_COMPOSE) exec -T clickhouse sh -lc 'clickhouse-client --user "$$CLICKHOUSE_USER" --password "$$CLICKHOUSE_PASSWORD" --query "SELECT 1" >/dev/null 2>&1'; do sleep 1; done

db-up: db-wait
	$(export-env) && migrate -path db/migrations -database "$$DATABASE_URL" up

db-down:
	$(export-env) && migrate -path db/migrations -database "$$DATABASE_URL" down 1

# Prefer Homebrew postgresql@18's psql-18 when on PATH; override with make psql PSQL=psql
PSQL ?= $(shell command -v psql-18 2>/dev/null || command -v psql 2>/dev/null || echo psql)

.PHONY: psql sql seed-schema-mapping-columns validate-vector-rag-eval eval-vector-rag-retrieval smoke-data-pipeline smoke-data-pipeline-json smoke-data-pipeline-excel smoke-data-pipeline-text smoke-data-pipeline-quarantine smoke-data-pipeline-review smoke-data-pipeline-field-review smoke-data-pipeline-table-review smoke-data-pipeline-schema-mapping-review smoke-data-pipeline-product-review smoke-data-pipeline-suite smoke-lmstudio-vector-api smoke-lmstudio-drive-rag air-check backend-dev backend-run binary binary-fast binary-package binary-package-zstd binary-size-report binary-size-check clickhouse-wait
psql:
	$(export-env) && $(PSQL) "$$DATABASE_URL" $(ARGS)

sql: psql

db-schema: db-wait
	$(DOCKER_COMPOSE) exec -T postgres pg_dump --schema-only --no-owner --no-privileges -U haohao -d haohao | sed '/^\\restrict /d; /^\\unrestrict /d' | perl -0pe 's/\n+\z/\n/' > db/schema.sql

seed-demo-user: db-wait
	$(DOCKER_COMPOSE) exec -T postgres psql -U haohao -d haohao < scripts/seed-demo-user.sql

seed-schema-mapping-columns: db-wait
	$(export-env) && node scripts/seed-schema-mapping-columns.mjs

validate-vector-rag-eval:
	node scripts/validate-vector-rag-evaluation-datasets.mjs

eval-vector-rag-retrieval:
	node scripts/evaluate-vector-rag-retrieval.mjs

smoke-lmstudio-vector-api:
	node scripts/smoke-lmstudio-vector-api.mjs

smoke-lmstudio-drive-rag:
	node scripts/smoke-lmstudio-drive-rag.mjs

smoke-data-pipeline:
	node scripts/smoke-data-pipeline.mjs

smoke-data-pipeline-json:
	node scripts/smoke-data-pipeline.mjs json

smoke-data-pipeline-excel:
	node scripts/smoke-data-pipeline.mjs excel

smoke-data-pipeline-text:
	node scripts/smoke-data-pipeline.mjs text

smoke-data-pipeline-quarantine:
	node scripts/smoke-data-pipeline.mjs quarantine

smoke-data-pipeline-review:
	node scripts/smoke-data-pipeline.mjs review

smoke-data-pipeline-field-review:
	node scripts/smoke-data-pipeline.mjs field_review

smoke-data-pipeline-table-review:
	node scripts/smoke-data-pipeline.mjs table_review

smoke-data-pipeline-schema-mapping-review:
	node scripts/smoke-data-pipeline.mjs schema_mapping_review

smoke-data-pipeline-product-review:
	node scripts/smoke-data-pipeline.mjs product_review

smoke-data-pipeline-suite:
	node scripts/smoke-data-pipeline.mjs suite

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

backend-dev: air-check db-up clickhouse-wait
	$(export-env) && $(AIR_BIN) -c .air.toml

backend-run: db-up clickhouse-wait
	$(export-env) && go run ./backend/cmd/main

frontend-dev:
	cd frontend && npm run dev

frontend-build:
	cd frontend && npm run build

binary: frontend-build
	mkdir -p bin
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build $(GO_BINARY_BUILD_FLAGS) -o $(BINARY_PATH) ./backend/cmd/main

binary-fast: frontend-build
	mkdir -p bin
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build $(GO_BINARY_FAST_BUILD_FLAGS) -o $(BINARY_PATH) ./backend/cmd/main

binary-package: binary
	tar -cf - -C bin haohao | gzip -9 > $(BINARY_PACKAGE_GZIP)

binary-package-zstd: binary
	@command -v zstd >/dev/null 2>&1 || { echo "zstd is required for binary-package-zstd. Install it with: brew install zstd"; exit 1; }
	tar -cf - -C bin haohao | zstd -19 -q -c > $(BINARY_PACKAGE_ZSTD)

binary-size-report: binary
	@raw_bytes=$$(wc -c < $(BINARY_PATH) | tr -d ' '); awk -v b="$$raw_bytes" 'BEGIN { printf "raw binary: %.1f MiB (%d bytes)\n", b / 1024 / 1024, b }'
	@gzip_bytes=$$(gzip -9c $(BINARY_PATH) | wc -c | tr -d ' '); awk -v b="$$gzip_bytes" 'BEGIN { printf "gzip -9 stream: %.1f MiB (%d bytes)\n", b / 1024 / 1024, b }'
	@if command -v zstd >/dev/null 2>&1; then \
		zstd_bytes=$$(zstd -19 -q -c $(BINARY_PATH) | wc -c | tr -d ' '); \
		awk -v b="$$zstd_bytes" 'BEGIN { printf "zstd -19 stream: %.1f MiB (%d bytes)\n", b / 1024 / 1024, b }'; \
	else \
		echo "zstd -19 stream: zstd is not installed"; \
	fi
	@go version -m $(BINARY_PATH) | sed -n '/^\tbuild\t/p'
	@if command -v otool >/dev/null 2>&1; then \
		otool -l $(BINARY_PATH) | awk '/segname/ { seg=$$2 } /filesize/ && seg != "__PAGEZERO" { printf "%s filesize: %.1f MiB (%s bytes)\n", seg, ($$2 + 0) / 1024 / 1024, $$2 }'; \
	elif command -v size >/dev/null 2>&1; then \
		size $(BINARY_PATH); \
	fi

binary-size-check: binary-package
	@raw_bytes=$$(wc -c < $(BINARY_PATH) | tr -d ' '); \
	if [ "$$raw_bytes" -gt "$(BINARY_MAX_BYTES)" ]; then \
		awk -v b="$$raw_bytes" -v max="$(BINARY_MAX_BYTES)" 'BEGIN { printf "raw binary is too large: %.1f MiB > %.1f MiB\n", b / 1024 / 1024, max / 1024 / 1024 }'; \
		exit 1; \
	fi
	@gzip_bytes=$$(wc -c < $(BINARY_PACKAGE_GZIP) | tr -d ' '); \
	if [ "$$gzip_bytes" -gt "$(BINARY_GZIP_MAX_BYTES)" ]; then \
		awk -v b="$$gzip_bytes" -v max="$(BINARY_GZIP_MAX_BYTES)" 'BEGIN { printf "gzip package is too large: %.1f MiB > %.1f MiB\n", b / 1024 / 1024, max / 1024 / 1024 }'; \
		exit 1; \
	fi
	@echo "binary size check passed"

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

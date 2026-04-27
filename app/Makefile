SHELL := /usr/bin/env bash

# -------- Configuration --------
GO            ?= go
GOFLAGS       ?=
SERVICES      := gateway scheduler account billing worker cli
BIN_DIR       := bin

DEV_COMPOSE   := deploy/docker-compose.yml
DEV_DB_DSN    ?= postgres://llmhub:llmhub@localhost:5432/llmhub?sslmode=disable

# -------- Meta --------
.PHONY: help
help:
	@echo "LLMHub development targets:"
	@echo ""
	@echo "  dev-up / dev-down    Start / stop local deps (pg + redis + nats + vault)"
	@echo "  migrate-up / migrate-down  Run goose migrations"
	@echo "  migrate-new NAME=x   Create a new migration skeleton"
	@echo ""
	@echo "  build                Build all service binaries to ./bin"
	@echo "  run-<svc>            Run a single service (e.g. make run-gateway)"
	@echo "  lint                 golangci-lint run"
	@echo "  test                 Run unit tests"
	@echo "  test-golden          Run provider protocol conformance tests"
	@echo "  sqlc-check           Verify sqlc-generated code matches queries"
	@echo "  proto                Generate gRPC + protobuf code via buf"
	@echo "  proto-check          Verify generated proto code is in sync"
	@echo ""
	@echo "  web-dev              Run all Next.js apps in dev mode"
	@echo ""
	@echo "  docker               Build all service docker images"

# -------- Local dev environment --------
.PHONY: dev-up dev-down dev-logs
dev-up:
	docker compose -f $(DEV_COMPOSE) up -d
	@echo "waiting for postgres..."
	@until docker compose -f $(DEV_COMPOSE) exec -T postgres pg_isready -U llmhub >/dev/null 2>&1; do sleep 1; done
	@echo "ready."

dev-down:
	docker compose -f $(DEV_COMPOSE) down

dev-logs:
	docker compose -f $(DEV_COMPOSE) logs -f

# -------- Database --------
.PHONY: migrate-up migrate-down migrate-new
migrate-up:
	goose -dir migrations postgres "$(DEV_DB_DSN)" up

migrate-down:
	goose -dir migrations postgres "$(DEV_DB_DSN)" down

migrate-new:
	@if [ -z "$(NAME)" ]; then echo "usage: make migrate-new NAME=add_xxx"; exit 2; fi
	goose -dir migrations create $(NAME) sql

# -------- Go build / run --------
.PHONY: build
build: $(addprefix build-,$(SERVICES))

build-%:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$* ./cmd/$*

.PHONY: $(addprefix run-,$(SERVICES))
$(addprefix run-,$(SERVICES)):
	$(GO) run ./cmd/$(subst run-,,$@)

# -------- Quality gates --------
.PHONY: lint test test-golden sqlc-check
lint:
	golangci-lint run ./...

test:
	$(GO) test ./... -race -count=1

test-golden:
	@echo "running provider protocol conformance tests..."
	$(GO) test ./test/golden/... -count=1

sqlc-check:
	sqlc diff

# -------- Proto / codegen --------
.PHONY: proto proto-check
proto:
	cd proto && buf generate

proto-check:
	cd proto && buf lint && buf format -d

# -------- Frontend --------
.PHONY: web-install web-dev web-build web-www web-console web-admin web-typecheck
web-install:
	cd web && pnpm install

web-dev:
	cd web && pnpm -r --parallel dev

web-www:
	cd web/www && pnpm dev

web-console:
	cd web/console && pnpm dev

web-admin:
	cd web/admin && pnpm dev

web-build:
	cd web && pnpm -r build

web-typecheck:
	cd web && pnpm -r typecheck

# -------- Docker --------
.PHONY: docker $(addprefix docker-,$(SERVICES))
docker: $(addprefix docker-,$(SERVICES))

docker-%:
	docker build -t llmhub/$*:dev -f deploy/Dockerfile --build-arg SERVICE=$* .

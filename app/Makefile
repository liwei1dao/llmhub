SHELL := /usr/bin/env bash

# -------- Configuration --------
GO            ?= go
GOFLAGS       ?=
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
	@echo "  build                Build the llmhub binary to ./bin"
	@echo "  run                  Run the llmhub server (port 8080)"
	@echo "  lint                 golangci-lint run"
	@echo "  test                 Run unit tests"
	@echo "  test-golden          Run provider protocol conformance tests"
	@echo "  proto / proto-check  Generate / verify gRPC + protobuf code via buf"
	@echo ""
	@echo "  web-install          pnpm install (single web/ project)"
	@echo "  web-dev              next dev (port 3000)"
	@echo "  web-build            next build"
	@echo "  web-typecheck        tsc --noEmit"
	@echo ""
	@echo "  docker-api / docker-web   Build images"

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
.PHONY: build run
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/llmhub ./cmd/llmhub

run:
	$(GO) run ./cmd/llmhub

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
.PHONY: web-install web-dev web-build web-typecheck
web-install:
	cd web && pnpm install

web-dev:
	cd web && pnpm dev

web-build:
	cd web && pnpm build

web-typecheck:
	cd web && pnpm typecheck

# -------- Docker --------
.PHONY: docker-api docker-web
docker-api:
	docker build -t llmhub-api:dev -f deploy/Dockerfile .

docker-web:
	docker build -t llmhub-web:dev -f web/Dockerfile web

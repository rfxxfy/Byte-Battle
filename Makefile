MODULE_NAME := bytebattle

# Load .env file if exists
-include .env
export

OAPI_CODEGEN := $(shell go env GOPATH)/bin/oapi-codegen
MIGRATE_VERSION := v4.19.1

GOBIN := $(shell go env GOPATH)/bin

.PHONY: tidy tools generate generate-api generate-sqlc clean

tidy:
	go mod tidy

# ─────────────────────────────────────────────────────────────────────────────
# Code generation
# ─────────────────────────────────────────────────────────────────────────────

# Install dev tools (oapi-codegen, sqlc)
tools:
	@test -f $(OAPI_CODEGEN) || go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.6.0
	@command -v sqlc >/dev/null 2>&1 || go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0

# Generate API types and server interface from openapi.yaml (no DB required)
generate-api:
	mkdir -p internal/api
	cd api && $(OAPI_CODEGEN) -config oapi-codegen.yaml openapi.yaml

# Generate sqlc query code from SQL files (no DB required)
generate-sqlc:
	sqlc generate

# Generate everything (auto-installs tools if missing)
generate: tools generate-api generate-sqlc

# Remove generated files and binaries
clean:
	rm -rf internal/api/ internal/db/sqlc/ bin/

# ─────────────────────────────────────────────────────────────────────────────
# Migrations (golang-migrate)
# ─────────────────────────────────────────────────────────────────────────────

MIGRATE := $(GOBIN)/migrate
MIGRATIONS_DIR := internal/migrations

# Database DSN — defaults work for local dev with docker-compose
DB_DSN ?= postgres://bytebattle:bytebattle@localhost:5432/bytebattle?sslmode=disable

.PHONY: migrate-tools migrate-up migrate-rollback migrate-down migrate-drop migrate-create migrate-version migrate-force

migrate-tools:
	@test -f $(MIGRATE) || go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)

migrate-up: migrate-tools
	@echo "Applying all migrations..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" up

migrate-rollback: migrate-tools
	@echo "Rolling back last migration..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" down 1

migrate-down: migrate-tools
	@echo "Rolling back all migrations..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" down

migrate-drop: migrate-tools
	@echo "Dropping all tables..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" drop -f

migrate-version: migrate-tools
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" version

migrate-force: migrate-tools
ifndef VERSION
	$(error VERSION is required. Usage: make migrate-force VERSION=N)
endif
	@echo "Force setting version to $(VERSION)..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" force $(VERSION)

migrate-create: migrate-tools
ifndef NAME
	$(error NAME is required. Usage: make migrate-create NAME=migration_name)
endif
	@$(MIGRATE) create -ext sql -dir $(MIGRATIONS_DIR) -seq $(NAME)

# ─────────────────────────────────────────────────────────────────────────────
# Testing
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: test test-unit test-e2e

test-unit:
	@echo "Running unit tests..."
	@go test -v -race -count=1 $(shell go list ./... | grep -v /e2e)

test-e2e:
	@echo "Running e2e tests..."
	@go test -v -count=1 -timeout 120s ./internal/e2e/...

test: test-unit test-e2e

# ─────────────────────────────────────────────────────────────────────────────
# Lint
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: lint fmt

lint:
	@golangci-lint run ./...

fmt:
	@golangci-lint fmt ./...

# ─────────────────────────────────────────────────────────────────────────────
# App
# ─────────────────────────────────────────────────────────────────────────────

APP_NAME := bytebattle
CMD_DIR := ./cmd/$(APP_NAME)

.PHONY: run build

run:
	@echo "Starting $(APP_NAME)..."
	@go run $(CMD_DIR)

build:
	@echo "Building $(APP_NAME)..."
	@go build -o bin/$(APP_NAME) $(CMD_DIR)
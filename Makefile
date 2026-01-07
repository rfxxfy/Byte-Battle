MODULE_NAME := bytebattle

# Load .env file if exists
-include .env
export

OAPI_CODEGEN := $(shell go env GOPATH)/bin/oapi-codegen
MIGRATE_VERSION := v4.19.1
SQLBOILER_VERSION := v4.19.7

GOBIN := $(shell go env GOPATH)/bin
SQLBOILER := $(GOBIN)/sqlboiler
SQLBOILER_PSQL := $(GOBIN)/sqlboiler-psql

.PHONY: tidy tools generate generate-api generate-models clean clean-models

tidy:
	go mod tidy

# ─────────────────────────────────────────────────────────────────────────────
# Code generation
# ─────────────────────────────────────────────────────────────────────────────

# Install all dev tools (codegen, golang-migrate)
tools:
	@test -f $(OAPI_CODEGEN) || go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	@test -f $(SQLBOILER) || go install github.com/aarondl/sqlboiler/v4@$(SQLBOILER_VERSION)
	@test -f $(SQLBOILER_PSQL) || go install github.com/aarondl/sqlboiler/v4/drivers/sqlboiler-psql@$(SQLBOILER_VERSION)
	@test -f $(MIGRATE) || go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)

# Generate API types and server interface from openapi.yaml (no DB required)
generate-api:
	mkdir -p internal/api
	cd api && $(OAPI_CODEGEN) -config oapi-codegen.yaml openapi.yaml

# Generate SQLBoiler models from live DB (requires DB connection via sqlboiler.toml)
generate-models:
	@rm -rf internal/database/models
	@mkdir -p internal/database/models
	@PATH="$(GOBIN):$$PATH" $(SQLBOILER) psql --output internal/database/models --no-tests

# Generate everything (auto-installs tools if missing)
generate: tools generate-api generate-models

# Remove generated files and binaries
clean: clean-models
	rm -rf internal/api/ bin/

clean-models:
	rm -rf internal/database/models

# ─────────────────────────────────────────────────────────────────────────────
# Migrations (golang-migrate)
# ─────────────────────────────────────────────────────────────────────────────

MIGRATE := $(GOBIN)/migrate
MIGRATIONS_DIR := migrations

# Database DSN — defaults work for local dev with docker-compose
DB_DSN ?= postgres://bytebattle:bytebattle@localhost:5432/bytebattle?sslmode=disable

.PHONY: migrate-tools migrate-up migrate-down migrate-down-all migrate-drop migrate-create migrate-version migrate-force

migrate-tools:
	@test -f $(MIGRATE) || go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)

migrate-up: migrate-tools
	@echo "Applying all migrations..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" up

migrate-down: migrate-tools
	@echo "Rolling back last migration..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" down 1

migrate-down-all: migrate-tools
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

.PHONY: test test-prepare

test-prepare:
	@./scripts/prepare-test-env.sh

test: test-prepare
	@echo "Running tests..."
	@go test -v ./...

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
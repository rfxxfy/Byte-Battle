MODULE_NAME := bytebattle

# Load .env file if exists
-include .env
export

.PHONY: tidy

tidy:
	go mod tidy

# ─────────────────────────────────────────────────────────────────────────────
# SQLBoiler
# ─────────────────────────────────────────────────────────────────────────────

# CLI tool versions (single source of truth)
MIGRATE_VERSION := v4.19.1
SQLBOILER_VERSION := v4.19.7

GOBIN := $(shell go env GOPATH)/bin
SQLBOILER := $(GOBIN)/sqlboiler
SQLBOILER_PSQL := $(GOBIN)/sqlboiler-psql

.PHONY: tools generate clean-models

tools:
	@test -f $(SQLBOILER) || go install github.com/aarondl/sqlboiler/v4@$(SQLBOILER_VERSION)
	@test -f $(SQLBOILER_PSQL) || go install github.com/aarondl/sqlboiler/v4/drivers/sqlboiler-psql@$(SQLBOILER_VERSION)

generate: tools
	@echo "Generating sqlboiler models..."
	@rm -rf internal/database/models
	@mkdir -p internal/database/models
	@PATH="$(GOBIN):$$PATH" $(SQLBOILER) psql --output internal/database/models

clean-models:
	rm -rf internal/database/models

# ─────────────────────────────────────────────────────────────────────────────
# Migrations (golang-migrate)
# ─────────────────────────────────────────────────────────────────────────────

MIGRATE := $(GOBIN)/migrate
MIGRATIONS_DIR := migrations

# Database connection (can be overridden: make migrate-up DB_URL=...)
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_USER ?= bytebattle
DB_PASSWORD ?= bytebattle
DB_NAME ?= bytebattle
DB_URL ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

.PHONY: migrate-tools migrate-up migrate-down migrate-drop migrate-create migrate-version migrate-force

migrate-tools:
	@test -f $(MIGRATE) || go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)

migrate-up: migrate-tools
	@echo "Applying all migrations..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" up

migrate-down: migrate-tools
	@echo "Rolling back last migration..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down 1

migrate-down-all: migrate-tools
	@echo "Rolling back all migrations..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down

migrate-drop: migrate-tools
	@echo "Dropping all tables..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" drop -f

migrate-version: migrate-tools
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" version

migrate-force: migrate-tools
ifndef VERSION
	$(error VERSION is required. Usage: make migrate-force VERSION=N)
endif
	@echo "Force setting version to $(VERSION)..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" force $(VERSION)

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
	@go test -v ./cmd/... ./internal/config/... ./internal/server/... ./internal/service/...

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
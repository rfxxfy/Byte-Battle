MODULE_NAME := bytebattle

# Load .env file if exists
-include .env
export

OAPI_CODEGEN := $(shell go env GOPATH)/bin/oapi-codegen
MIGRATE_VERSION := v4.19.1

GOBIN := $(shell go env GOPATH)/bin

.PHONY: tidy tools generate clean

tidy:
	go mod tidy

# ─────────────────────────────────────────────────────────────────────────────
# Code generation
# ─────────────────────────────────────────────────────────────────────────────

# Install codegen tools
tools:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

# Generate API types and server interface from openapi.yaml
generate:
	mkdir -p internal/api
	cd api && $(OAPI_CODEGEN) -config oapi-codegen.yaml openapi.yaml

# Remove generated files and binaries
clean:
	rm -rf internal/api/ bin/

# ─────────────────────────────────────────────────────────────────────────────
# Migrations (golang-migrate)
# ─────────────────────────────────────────────────────────────────────────────

MIGRATE := $(GOBIN)/migrate
MIGRATIONS_DIR := migrations

# Database connection - requires .env file or env vars to be set
DB_URL ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

.PHONY: check-db-env migrate-tools migrate-up migrate-down migrate-drop migrate-create migrate-version migrate-force

check-db-env:
ifndef DB_HOST
	$(error DB_HOST is not set. Create .env from .env.example or export env vars)
endif
ifndef DB_PORT
	$(error DB_PORT is not set)
endif
ifndef DB_USER
	$(error DB_USER is not set)
endif
ifndef DB_PASSWORD
	$(error DB_PASSWORD is not set)
endif
ifndef DB_NAME
	$(error DB_NAME is not set)
endif

migrate-tools:
	@test -f $(MIGRATE) || go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)

migrate-up: check-db-env migrate-tools
	@echo "Applying all migrations..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" up

migrate-down: check-db-env migrate-tools
	@echo "Rolling back last migration..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down 1

migrate-down-all: check-db-env migrate-tools
	@echo "Rolling back all migrations..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down

migrate-drop: check-db-env migrate-tools
	@echo "Dropping all tables..."
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" drop -f

migrate-version: check-db-env migrate-tools
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" version

migrate-force: check-db-env migrate-tools
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

test-prepare: check-db-env
	@./scripts/prepare-test-env.sh

test: test-prepare
	@echo "Running tests..."
	@go test -v ./...

# ─────────────────────────────────────────────────────────────────────────────
# Lint
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: lint

lint:
	@golangci-lint run ./...

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
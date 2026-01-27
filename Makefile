MODULE_NAME := bytebattle

.PHONY: init tidy tools generate clean-models migrate-up migrate-status migrate-file db-reset db-setup run build

init:
	@if [ ! -f go.mod ]; then \
		echo "Init repo: $(MODULE_NAME)"; \
		go mod init $(MODULE_NAME); \
	else \
		echo "go.mod already exists"; \
	fi

tidy:
	go mod tidy

SQLBOILER := sqlboiler
SQLBOILER_PSQL := sqlboiler-psql

tools:
	@command -v $(SQLBOILER) >/dev/null || go install github.com/aarondl/sqlboiler/v4@latest
	@command -v $(SQLBOILER_PSQL) >/dev/null || go install github.com/aarondl/sqlboiler/v4/drivers/sqlboiler-psql@latest

generate: tools
	@echo "Generating sqlboiler models..."
	@rm -rf internal/database/models
	@mkdir -p internal/database/models
	@sqlboiler psql --output internal/database/models

clean-models:
	rm -rf internal/database/models

migrate-up:
	@echo "Applying migrations..."
	@for f in schema/[0-9]*.sql; do \
		echo "Applying $$f..."; \
		docker exec -i bytebattle-postgres psql -U bytebattle -d bytebattle < "$$f" 2>&1 || echo "FAILED: $$f"; \
	done

migrate-status:
	@echo "Existing tables:"
	@docker exec bytebattle-postgres psql -U bytebattle -d bytebattle -c "\dt"

migrate-file:
	@if [ -z "$(FILE)" ]; then echo "Usage: make migrate-file FILE=schema/005_xxx.sql"; exit 1; fi
	@echo "Applying $(FILE)..."
	docker exec -i bytebattle-postgres psql -U bytebattle -d bytebattle < $(FILE)

db-reset:
	docker-compose down -v
	docker-compose up -d
	@$(MAKE) migrate-up

db-setup: migrate-up generate
	@echo "Database ready, models generated"

APP_NAME := bytebattle
CMD_DIR := ./cmd/$(APP_NAME)

run:
	@echo "Starting $(APP_NAME)..."
	@go run $(CMD_DIR)

build:
	@echo "Building $(APP_NAME)..."
	@go build -o bin/$(APP_NAME) $(CMD_DIR)
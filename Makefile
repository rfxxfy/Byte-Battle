MODULE_NAME := bytebattle

.PHONY: init tidy

init:
	@if [ ! -f go.mod ]; then \
		echo "Init repo: $(MODULE_NAME)"; \
		go mod init $(MODULE_NAME); \
	else \
		echo "good. already exists"; \
	fi

tidy:
	go mod tidy

SQLBOILER := sqlboiler
SQLBOILER_PSQL := sqlboiler-psql

.PHONY: tools generate clean-models

tools:
	@command -v $(SQLBOILER) >/dev/null || go install github.com/aarondl/sqlboiler/v4@latest
	@command -v $(SQLBOILER_PSQL) >/dev/null || go install github.com/aarondl/sqlboiler/v4/drivers/sqlboiler-psql@latest

generate: tools
	@echo "Generating sqlboiler models"
	@mkdir -p internal/database
	@rm -rf internal/database/models
	@sqlboiler psql

clean-models:
	rm -rf internal/database/models


SQLBOILER := sqlboiler
MODELS_DIR := internal/database/models

generate: tools
	@echo "Generating sqlboiler models"
	@mkdir -p $(MODELS_DIR)
	@rm -f $(MODELS_DIR)/*.go
	@$(SQLBOILER) psql \
		--config sqlboiler.toml \
		--output $(MODELS_DIR) \
		--pkgname models


APP_NAME := bytebattle
CMD_DIR := ./cmd/$(APP_NAME)

.PHONY: run build

run:
	@echo "→ Starting $(APP_NAME)"
	@go run $(CMD_DIR)

build:
	@echo "→ Building $(APP_NAME)"
	@go build -o bin/$(APP_NAME) $(CMD_DIR)
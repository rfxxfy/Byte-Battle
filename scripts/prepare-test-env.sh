#!/bin/bash
set -e

# ─────────────────────────────────────────────────────────────────────────────
# Recipe: Prepare test environment
# Поднимает БД, накатывает миграции, готовит окружение для тестов
# ─────────────────────────────────────────────────────────────────────────────

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Checking environment variables...${NC}"

required_vars=(DB_HOST DB_PORT DB_USER DB_PASSWORD DB_NAME)

for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
        echo -e "${RED}Error: Environment variable $var is not set.${NC}"
        exit 1
    fi
done

DB_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"

# ─────────────────────────────────────────────────────────────────────────────
# Step 1: Check if migrate is installed
# ─────────────────────────────────────────────────────────────────────────────
GOBIN=$(go env GOPATH)/bin
MIGRATE="${GOBIN}/migrate"

if [ ! -f "$MIGRATE" ]; then
    echo -e "${YELLOW}Installing golang-migrate...${NC}"
    go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
fi

# ─────────────────────────────────────────────────────────────────────────────
# Step 2: Wait for database to be ready
# ─────────────────────────────────────────────────────────────────────────────
echo -e "${YELLOW}Waiting for database...${NC}"

MAX_RETRIES=30
RETRY_COUNT=0

# Try pg_isready first, fallback to psql, then to nc
check_db() {
    if command -v pg_isready > /dev/null 2>&1; then
        pg_isready -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" > /dev/null 2>&1
    elif command -v psql > /dev/null 2>&1; then
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1" > /dev/null 2>&1
    elif command -v nc > /dev/null 2>&1; then
        nc -z "$DB_HOST" "$DB_PORT" > /dev/null 2>&1
    elif command -v docker > /dev/null 2>&1; then
        docker exec bytebattle-postgres pg_isready -U "$DB_USER" -d "$DB_NAME" > /dev/null 2>&1
    else
        # Just try to connect with migrate
        return 0
    fi
}

until check_db; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
        echo -e "${RED}Error: Database not ready after ${MAX_RETRIES} attempts${NC}"
        exit 1
    fi
    echo "  Attempt $RETRY_COUNT/$MAX_RETRIES..."
    sleep 1
done

echo -e "${GREEN}Database is ready!${NC}"

# ─────────────────────────────────────────────────────────────────────────────
# Step 3: Run migrations
# ─────────────────────────────────────────────────────────────────────────────
echo -e "${YELLOW}Running migrations...${NC}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIGRATIONS_DIR="${SCRIPT_DIR}/../migrations"

$MIGRATE -path "$MIGRATIONS_DIR" -database "$DB_URL" up

echo -e "${GREEN}Migrations applied successfully!${NC}"

# ─────────────────────────────────────────────────────────────────────────────
# Done
# ─────────────────────────────────────────────────────────────────────────────
echo -e "${GREEN}=== Test environment ready ===${NC}"

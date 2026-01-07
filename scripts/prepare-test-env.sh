#!/bin/bash
set -e

# Prepare test environment: install migrate if needed, run migrations.
# Uses DB_DSN with a sensible default for local development.

DB_DSN="${DB_DSN:-postgres://bytebattle:bytebattle@localhost:5432/bytebattle?sslmode=disable}"

GOBIN=$(go env GOPATH)/bin
MIGRATE="${GOBIN}/migrate"

if [ ! -f "$MIGRATE" ]; then
    echo "Installing golang-migrate..."
    go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Running migrations..."
$MIGRATE -path "${SCRIPT_DIR}/../migrations" -database "$DB_DSN" up

echo "Test environment ready."

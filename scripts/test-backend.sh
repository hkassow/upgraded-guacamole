#!/usr/bin/env bash
# Runs the Go backend tests against a separate guac_test database in the docker-compose Postgres.
# Your real guac database is never touched: tests refuse to run unless the database name ends in _test.
#
# Usage:
#   scripts/test-backend.sh                          # all tests
#   scripts/test-backend.sh -v -run TestUpdateRecipe # any go test flags
#
# Needs the db container running (docker compose up -d db).
set -euo pipefail
cd "$(dirname "$0")/.."

DB_CONTAINER="${DB_CONTAINER:-guac}"
TEST_DB="${TEST_DB:-guac_test}"

if [[ "$TEST_DB" != *_test ]]; then
	echo "TEST_DB must end in _test (got $TEST_DB)" >&2
	exit 1
fi

if ! docker exec "$DB_CONTAINER" psql -U postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '$TEST_DB'" | grep -q 1; then
	echo "Creating database $TEST_DB"
	docker exec "$DB_CONTAINER" createdb -U postgres "$TEST_DB"
fi

export TEST_DATABASE_URL="${TEST_DATABASE_URL:-postgres://postgres:postgres@localhost:5432/$TEST_DB?sslmode=disable}"

# -p 1: every package shares the test database and wipes it between tests, so run packages one at a time
go test -p 1 "$@" ./...

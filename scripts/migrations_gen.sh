#!/usr/bin/env bash
# Writes the next schema migration for every set (core, modules) whose ent
# tables changed, for MySQL and PostgreSQL:
#   ./scripts/migrations_gen.sh add_feed_language
#   ./scripts/migrations_gen.sh -check     - fail if files are behind ent
# Uses the databases of docker-compose.test.yml.
set -euo pipefail
cd "$(dirname "$0")/.."
compose="docker compose -f docker-compose.test.yml -p kosatka-test"
trap '$compose down -v >/dev/null 2>&1' EXIT
$compose up -d --wait >/dev/null
export KOSATKA_TEST_MYSQL_DSN="root:test@tcp(127.0.0.1:33306)/?parseTime=true"
export KOSATKA_TEST_POSTGRES_DSN="postgres://postgres:test@127.0.0.1:35432/postgres?sslmode=disable"
if [[ "${1:-}" == "-check" ]]; then
  go run ./cmd/migrations-gen -check
  # The example module's own (examples/hello, its own tree).
  (cd examples/hello && go run ./cmd/migrations -check)
else
  go run ./cmd/migrations-gen -name "${1:?usage: $0 <name> | -check}"
fi

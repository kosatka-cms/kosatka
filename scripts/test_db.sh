#!/usr/bin/env bash
# Integration tests on MySQL, PostgreSQL, an S3 storage and Azurite (go test -tags
# dbintegration): starts them from docker-compose.test.yml, runs the tests, stops them.
#   ./scripts/test_db.sh            - every integration test (the example module's too)
#   ./scripts/test_db.sh ./internal/infrastructure/sqldialect/...
set -euo pipefail
cd "$(dirname "$0")/.."

compose="docker compose -f docker-compose.test.yml -p kosatka-test"
trap '$compose down -v >/dev/null 2>&1' EXIT
$compose up -d --wait

export KOSATKA_TEST_MYSQL_DSN="root:test@tcp(127.0.0.1:33306)/?parseTime=true"
export KOSATKA_TEST_POSTGRES_DSN="postgres://postgres:test@127.0.0.1:35432/postgres?sslmode=disable"
export KOSATKA_TEST_S3="http://test:test-secret@127.0.0.1:39000/kosatka-test"
export KOSATKA_TEST_AZURE="http://127.0.0.1:31000/devstoreaccount1"

pkgs=("$@")
all=""
[[ ${#pkgs[@]} -eq 0 ]] && pkgs=(./...) && all=1
go test -tags dbintegration -count=1 ${GOTESTFLAGS:-} "${pkgs[@]}"
# Every test: the example module's too (examples/hello, a Go module of
# its own).
if [[ -n "$all" ]]; then
  (cd examples/hello && go test -tags dbintegration -count=1 ${GOTESTFLAGS:-} ./...)
fi

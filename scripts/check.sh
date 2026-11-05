#!/usr/bin/env bash
# The checks CI runs (.github/workflows/test.yml), locally - so they run
# while CI is off too:
#   ./scripts/check.sh       golangci-lint (gofmt, go vet and the rest;
#                            installed when missing), the unit tests,
#                            govulncheck and gitleaks when installed
#   ./scripts/check.sh -db   and the tests on MySQL and PostgreSQL
#                            (scripts/test_db.sh) and the migration files
#                            against the ent schemas - Docker needed
#   ./scripts/check.sh -race the unit tests with the race detector (as CI)
set -euo pipefail
cd "$(dirname "$0")/.."

db=false
race=()
for arg in "$@"; do
  case "$arg" in
    -db) db=true ;;
    -race) race=(-race) ;;
    *) echo "check.sh: unknown flag $arg (-db, -race)" >&2; exit 2 ;;
  esac
done

# golangci-lint, the version CI runs (.github/workflows/test.yml - keep
# the two the same): on the PATH, in Go's bin directory, else installed
# there. Its config (.golangci.yml) has gofmt, goimports and go vet, the
# integration tests' files too; without it (offline, say) gofmt and go vet
# run by themselves.
GOLANGCI_LINT_VERSION=v2.15.0
lint="$(command -v golangci-lint 2>/dev/null || true)"
gobin="$(go env GOBIN)"
[[ -n "$gobin" ]] || gobin="$(go env GOPATH)/bin"
if [[ -z "$lint" && -x "$gobin/golangci-lint" ]]; then
  lint="$gobin/golangci-lint"
fi
if [[ -z "$lint" ]]; then
  echo "==> Installing golangci-lint ${GOLANGCI_LINT_VERSION}..."
  if go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}"; then
    lint="$gobin/golangci-lint"
  fi
fi
if [[ -n "$lint" ]]; then
  echo "==> golangci-lint (gofmt, go vet and the rest)..."
  if [[ "v$("$lint" version --short 2>/dev/null)" != "$GOLANGCI_LINT_VERSION" ]]; then
    echo "  ⚠ $lint is $("$lint" version --short 2>/dev/null), CI runs ${GOLANGCI_LINT_VERSION}"
  fi
  "$lint" run --timeout 5m ./...
else
  echo "==> golangci-lint unavailable: gofmt and go vet only (CI runs it)"
  # The engine's code, new files too (not git ls-files); ent's generated
  # code is left as ent writes it.
  unformatted="$(gofmt -l cmd internal pkg modulekit kosatka.go | grep -v '/ent/generated/' || true)"
  if [[ -n "$unformatted" ]]; then
    echo "Not gofmt-ed (gofmt -w <file>):"
    echo "$unformatted"
    exit 1
  fi
  # The tag's run has the untagged files too.
  go vet -tags dbintegration ./...
fi

if command -v govulncheck >/dev/null 2>&1; then
  echo "==> govulncheck..."
  govulncheck ./...
else
  echo "==> govulncheck not installed: skipped (CI runs it)"
fi

# Secrets in the committed history (.gitleaks.toml: what isn't one).
if command -v gitleaks >/dev/null 2>&1; then
  echo "==> gitleaks..."
  gitleaks git --redact --no-banner .
else
  echo "==> gitleaks not installed: skipped (CI runs it)"
fi

echo "==> Unit tests..."
go test -count=1 ${race[@]+"${race[@]}"} ./...

# The example module and site: Go modules of their own (examples/), on
# the module SDK - the root's ./... doesn't reach them.
echo "==> The example module and site..."
unformatted="$(gofmt -l examples | grep -v '/ent/generated/' || true)"
if [[ -n "$unformatted" ]]; then
  echo "Not gofmt-ed (gofmt -w <file>):"
  echo "$unformatted"
  exit 1
fi
(cd examples/hello && go vet -tags dbintegration ./... && go test -count=1 ./...)
(cd examples/hello-site && go build -o /dev/null .)

if [[ "$db" == true ]]; then
  echo "==> Database tests (MySQL, PostgreSQL)..."
  ./scripts/test_db.sh
  echo "==> Migration files against the ent schemas..."
  ./scripts/migrations_gen.sh -check
fi

echo "==> All checks passed"

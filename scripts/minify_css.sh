#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

MINIFY="$(command -v minify 2>/dev/null || true)"
if [[ -z "$MINIFY" ]]; then
  GOPATH_BIN="$(go env GOPATH)/bin/minify"
  if [[ -x "$GOPATH_BIN" ]]; then
    MINIFY="$GOPATH_BIN"
  fi
fi
if [[ -z "$MINIFY" ]]; then
  echo "==> Installing minify..."
  go install github.com/tdewolff/minify/v2/cmd/minify@v2.24.22
  MINIFY="$(go env GOPATH)/bin/minify"
fi

echo "==> Minifying CSS..."
count=0
for f in web/static/css/*.css themes/*/static/css/*.css internal/modules/*/static/*.css; do
  [[ -e "$f" ]] || continue
  [[ "$f" == *.min.css ]] && continue
  base="${f%.css}"
  "$MINIFY" -o "${base}.min.css" "$f"
  count=$((count + 1))
done
echo "==> Done: ${count} file(s) -> *.min.css (web/static/css, themes/*/static/css, internal/modules/*/static)"

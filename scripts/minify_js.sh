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

echo "==> Minifying JavaScript..."
count=0
# The engine's scripts: every web/static/js/*.js (the vendored libraries
# are *.min.js already; tinymce/ is a library of its own).
for src in web/static/js/*.js; do
  [[ "$src" == *.min.js ]] && continue
  "$MINIFY" -o "${src%.js}.min.js" "$src"
  count=$((count + 1))
done
# A script in parts: web/static/js/<name>/NN-*.js joined in name order
# into web/static/js/<name>.min.js (app - the engine's script); a
# directory without such parts (tinymce/, a library) is left alone.
for dir in web/static/js/*/; do
  parts=("$dir"[0-9][0-9]-*.js)
  [[ -e "${parts[0]}" ]] || continue
  name="$(basename "$dir")"
  tmp="$(mktemp -d)"
  cat "${parts[@]}" > "$tmp/${name}.js"
  "$MINIFY" -o "web/static/js/${name}.min.js" "$tmp/${name}.js"
  rm -rf "$tmp"
  count=$((count + 1))
done
# The themes' scripts (themeJS): themes/*/static/js/*.js.
for src in themes/*/static/js/*.js; do
  [[ -e "$src" ]] || continue
  [[ "$src" == *.min.js ]] && continue
  "$MINIFY" -o "${src%.js}.min.js" "$src"
  count=$((count + 1))
done
# Modules' own scripts (entity.Module.JS): internal/modules/*/static/*.js.
for src in internal/modules/*/static/*.js; do
  [[ -e "$src" ]] || continue
  [[ "$src" == *.min.js ]] && continue
  "$MINIFY" -o "${src%.js}.min.js" "$src"
  count=$((count + 1))
done
echo "==> Done: ${count} file(s) -> web/static/js/*.min.js, themes/*/static/js/*.min.js, internal/modules/*/static/*.min.js"

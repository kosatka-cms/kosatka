#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
out="${1:-kosatka}"
# The status report's "with uncommitted changes": changes in what the
# binary and the site are built from (other than .DS_Store), not Go's own
# flag, which any untracked file in the working copy sets.
build_paths=(cmd internal pkg modulekit kosatka.go go.mod go.sum web/templates web/static themes)
modified=false
if git status --porcelain --untracked-files=all -- "${build_paths[@]}" | grep -qv '\.DS_Store$'; then
  modified=true
fi
# The engine's release (the status report's "Version"): the vX.Y.Z tag on
# this commit, else the last one with the distance from it
# (v0.3.0-5-g1a2b3c4d); no tag yet - none.
version="$(git describe --tags --match 'v[0-9]*' --abbrev=8 2>/dev/null || true)"
pkg=github.com/kosatka-cms/kosatka/internal/buildinfo
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags "-X ${pkg}.TreeModified=${modified} -X ${pkg}.Version=${version}" \
  -o "$out" ./cmd/app
echo "Version: ${version:-none (no vX.Y.Z tag)}"
if [[ "$out" = /* ]]; then
  echo "Built: $out"
else
  echo "Built: $(pwd)/$out"
fi

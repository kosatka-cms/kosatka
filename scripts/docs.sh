#!/usr/bin/env bash
# The documentation site (website/, the pages are docs/), with Node.js
# when it is installed, else in Docker (node:22):
#   ./scripts/docs.sh        build it into website/dist (a link to a
#                            missing page fails the build)
#   ./scripts/docs.sh dev    serve it at http://localhost:4321/kosatka/,
#                            rebuilt as docs/ changes
set -euo pipefail
cd "$(dirname "$0")/.."

case "${1:-build}" in
  build) run="npm run build" port="" ;;
  dev) run="npm run dev" port="-p 4321:4321" ;;
  *) echo "usage: $0 [build|dev]" >&2; exit 2 ;;
esac
# npm ci every time: node_modules may hold another platform's binaries
# (Docker's Linux ones on a Mac).
cmd="npm ci --no-fund --no-audit && ASTRO_TELEMETRY_DISABLED=1 $run"

if command -v npm >/dev/null 2>&1; then
  (cd website && sh -c "$cmd")
else
  # The whole repository: website/src/content/docs links to ../../../docs.
  tty=""
  if [[ -t 0 ]]; then tty="-it"; fi # Ctrl-C stops dev
  # shellcheck disable=SC2086 # tty, port: no flag or one
  docker run --rm $tty $port -v "$PWD":/repo -w /repo/website node:22 sh -c "$cmd"
fi

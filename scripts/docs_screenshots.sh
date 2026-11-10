#!/usr/bin/env bash
# The documentation's screenshots (docs/assets/screens/*.webp): a fresh
# demo site (app install -demo, English, the Kosatka theme) on a throwaway
# MySQL in Docker, the pages of scripts/screenshots/shots.json taken with
# the Chrome installed on the machine, then the site and the database
# removed. Run it before a release, when the admin's look changed:
#   ./scripts/docs_screenshots.sh
# Needs Docker (the mysql:8.4 image of the integration tests), Google
# Chrome and the minifier of scripts/minify_css.sh.
set -euo pipefail
cd "$(dirname "$0")/.."

port="${SHOTS_PORT:-38600}"
dbport="${SHOTS_DB_PORT:-33307}"
db=kosatka-docs-shots
work="$(mktemp -d)"
app_pid=""

cleanup() {
  if [[ -n "$app_pid" ]]; then kill "$app_pid" 2>/dev/null || true; fi
  docker rm -f "$db" >/dev/null 2>&1 || true
  rm -rf "${work:?}"
}
trap cleanup EXIT

for p in "$port" "$dbport"; do
  if lsof -tiTCP:"$p" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "✗ port $p is in use (SHOTS_PORT, SHOTS_DB_PORT)" >&2
    exit 1
  fi
done

echo "==> A throwaway MySQL..."
docker run -d --rm --name "$db" -e MYSQL_ROOT_PASSWORD=shots -e MYSQL_DATABASE=kosatka \
  -p "127.0.0.1:$dbport:3306" --tmpfs /var/lib/mysql mysql:8.4 \
  --character-set-server=utf8mb4 --collation-server=utf8mb4_unicode_ci >/dev/null
for _ in $(seq 1 60); do
  if docker exec "$db" mysqladmin ping -h 127.0.0.1 -pshots --silent >/dev/null 2>&1 \
    && docker exec "$db" mysql -h 127.0.0.1 -pshots -e "SELECT 1" kosatka >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "==> The engine, its minified CSS and JavaScript..."
./scripts/minify_css.sh >/dev/null
./scripts/minify_js.sh >/dev/null
go build -o "$work/app" ./cmd/app

cat > "$work/config.yaml" <<EOF
env: local
port: $port
public_base_url: http://127.0.0.1:$port
db:
  driver: mysql
  host: 127.0.0.1
  port: $dbport
  scheme: kosatka
  username: root
  password: shots
engine_dir: $PWD
email:
  smtp_host: 127.0.0.1
  smtp_port: 2525
  from_address: noreply@example.com
EOF
export CONFIG_PATH="$work/config.yaml"
export SESSION_SECRET_KEY="docs-screenshots-session-key-0123456789"
export SECRETS_KEY="docs-screenshots-secrets-key-0123456789"

echo "==> The demo site..."
# The site runs in its own directory (its files in web/uploads there),
# the engine's templates, static files and themes from this checkout
# (engine_dir); its mail has a transport, never used.
mkdir -p "$work/site"
engine="$PWD"
cd "$work/site"
"$work/app" install -site-name "Kosatka demo" -admin-name admin -admin-email admin@example.com -demo > "$work/install.log" 2>&1 \
  || { cat "$work/install.log" >&2; exit 1; }
pass="$(sed -n "s/^install: the administrator's password (shown once[^:]*): //p" "$work/install.log")"
if [[ -z "$pass" ]]; then
  echo "✗ no administrator's password in the install's output" >&2
  cat "$work/install.log" >&2
  exit 1
fi
"$work/app" > "$work/site.log" 2>&1 &
app_pid=$!
for _ in $(seq 1 60); do
  curl -sf -o /dev/null "http://127.0.0.1:$port/healthz" && break
  sleep 1
done

echo "==> The screenshots..."
out="$engine/docs/assets/screens"
(cd "$engine/scripts/screenshots" && SHOT_LOGIN=admin@example.com SHOT_PASSWORD="$pass" \
  GOWORK=off go run . -base "http://127.0.0.1:$port" -out "$out" -shots shots.json)
echo "==> Done: $out"

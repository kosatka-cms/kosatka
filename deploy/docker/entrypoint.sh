#!/bin/sh
# The container's start: a new site on an empty database (app install,
# its administrator's password printed once in the log), the pending
# updates on an existing one (app migrate -apply), then the site.
set -e
cd /app
: "${STATIC_ASSET_VERSION:=$(cat /app/STATIC_ASSET_VERSION 2>/dev/null || echo dev)}"
export STATIC_ASSET_VERSION
# DEMO_CONTENT=1 - a new site gets the demo content (install -demo):
# DEMO_SET's (install -demo-set; empty - the blog). SITE_THEME - its theme
# (install -theme; empty - the demo set's, else the starter's).
demo=""
[ "${DEMO_CONTENT:-0}" = "1" ] && demo="-demo"
[ -n "${DEMO_SET:-}" ] && demo="-demo-set ${DEMO_SET}"
theme=""
[ -n "${SITE_THEME:-}" ] && theme="-theme ${SITE_THEME}"
# MODULES - the modules a new site has (install -modules: names separated
# by commas, "all" or "none"); empty - the standard ones.
# Exit 3 - the database has a site already; any other failure (the
# database unreachable, a wrong flag) stops the container.
code=0
./kosatka install $demo $theme -modules "${MODULES:-}" -site-name "${SITE_NAME:-Kosatka}" -admin-name "${ADMIN_NAME:-admin}" \
  -admin-email "${ADMIN_EMAIL:-admin@example.com}" -language "${SITE_LANGUAGE:-en}" || code=$?
case $code in
  0) echo "kosatka: new site installed - the administrator's password is above" ;;
  3)
    echo "kosatka: the site is installed already; applying updates"
    ./kosatka migrate -apply
    ;;
  *)
    echo "kosatka: app install failed (exit $code)" >&2
    exit "$code"
    ;;
esac
exec ./kosatka

# Kosatka in a container: the engine's binary (the modules' templates and
# static files built in) with its templates and static files (web/, the
# themes/). Configured by
# environment variables (deploy/docker/config.yaml lists them); see
# docker-compose.yml for a local site with MySQL.

FROM golang:1.27-alpine AS build
RUN apk add --no-cache bash git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
RUN go install github.com/tdewolff/minify/v2/cmd/minify@v2.24.22
COPY . .
# The pages link the minified CSS/JS (*.min.css, *.min.js); before the
# build, which embeds the modules' (their Assets).
RUN ./scripts/minify_css.sh && ./scripts/minify_js.sh
# The static files' version (?v= on their URLs): the commit, so a new
# image's CSS and JS aren't taken from the browser's cache.
RUN mkdir -p /out && (git rev-parse --short HEAD 2>/dev/null || date +%s) > /out/STATIC_ASSET_VERSION
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-X github.com/kosatka-cms/kosatka/internal/buildinfo.Version=$(git describe --tags --match 'v[0-9]*' --abbrev=8 2>/dev/null || true)" -o /out/kosatka ./cmd/app

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata && adduser -D -H -u 10001 kosatka
WORKDIR /app
COPY --from=build /out/kosatka /out/STATIC_ASSET_VERSION /app/
COPY --from=build /src/web /app/web
COPY --from=build /src/themes /app/themes
COPY deploy/docker/config.yaml /app/config.yaml
COPY deploy/docker/entrypoint.sh /app/entrypoint.sh
RUN mkdir -p /app/web/uploads /app/private && chown -R kosatka /app/web/uploads /app/private
USER kosatka
ENV CONFIG_PATH=/app/config.yaml APP_PORT=3333
EXPOSE 3333
# /healthz answers while the site serves and reaches its database; the
# first start installs the site, hence the grace period.
HEALTHCHECK --interval=30s --timeout=5s --start-period=90s CMD wget -q -O /dev/null "http://127.0.0.1:${APP_PORT}/healthz" || exit 1
ENTRYPOINT ["/app/entrypoint.sh"]

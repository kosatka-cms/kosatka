---
title: Run from source
description: Build Kosatka with Go and run it on your own database.
sidebar:
  order: 3
---

## Requirements

- Go, the version in `go.mod` (1.27);
- MySQL 8 or PostgreSQL 16;
- git, for `app theme` (themes from their own repositories).

## 1. A database

An empty database and a user who owns it. On MySQL, with the `utf8mb4`
character set:

```sql
CREATE DATABASE kosatka CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'kosatka'@'localhost' IDENTIFIED BY 'a-password';
GRANT ALL ON kosatka.* TO 'kosatka'@'localhost';
```

On PostgreSQL:

```sql
CREATE USER kosatka PASSWORD 'a-password';
CREATE DATABASE kosatka OWNER kosatka;
```

## 2. The config

```bash
cp config/local.yaml.example config/local.yaml
export CONFIG_PATH=config/local.yaml
```

In `config/local.yaml` set the database (`db`: `driver`, `host`, `port`,
`scheme` - the database's name, `username`, `password`) and a long random
`session.secret_key` (without one, every restart signs everyone out).
`config/local.yaml` is not in git: keep passwords and keys there only.
Environment variables override the file's values; every setting is in
[the configuration reference](../reference/configuration.md).

## 3. The styles and scripts

The pages link the minified CSS and JavaScript (`*.min.css`,
`*.min.js`), which are not in git. Build them once, and again after
changing a `.css` or `.js` file:

```bash
./scripts/minify_css.sh && ./scripts/minify_js.sh
```

The scripts install the `minify` tool with `go install` when it's
missing.

## 4. Install the site

From the repository's directory:

```bash
go run ./cmd/app install -site-name "My site" -admin-name admin \
  -admin-email admin@example.com
```

`-language ru` makes a Russian site, `-demo` adds the demo content and
`-modules polls,forms` (or `all`) chooses the modules; see
[`app install`](../reference/cli.md#app-install). The command prints the
administrator's password once.

## 5. Serve it

```bash
go run ./cmd/app
```

The site listens on the config's `port` (3333 in the example):
http://localhost:3333. The server reads `web/` and `themes/` from its
working directory, so run it from the repository's directory, or from a
directory that has both.

With `env: local` the templates are read again on every request and the
modules' templates and static files come from their source directories:
edits show up without a restart. Elsewhere they are cached, and a module's
are the ones built into the binary.

## Updating

After pulling a new version, apply the database migrations:

```bash
go run ./cmd/app migrate          # what would run
go run ./cmd/app migrate -apply   # run it
```

then rebuild the styles and scripts and restart the server.

For a production site, build a binary (`go build -o kosatka ./cmd/app`,
after the minify scripts: it carries the modules' minified files) and
run it next to `web/` and `themes/` with a config of its own. Without
`env: local` it is a live site (`env: prod`, the default):
`session.secret_key` is required, and a database that isn't on the same
host gets TLS by default (`db.tls`). The whole setup - a
service, nginx, updates, backups - is in
[Running in production](../guides/production.md).

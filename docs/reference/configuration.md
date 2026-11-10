---
title: Configuration
description: The config file's settings and the environment variables that override them.
sidebar:
  order: 2
---

The server and the commands that use the site read a YAML file: the
path from the `-config <file>` flag (before a command:
`app -config site.yaml migrate`) or the `CONFIG_PATH` environment
variable. `config/local.yaml.example` is a starting point for local
work; `deploy/docker/config.yaml` is the container's, with the variable
of each of its settings.

An environment variable overrides the file's value; a setting with no
variable is set in the file only. A setting not in the file and not in the
environment takes its default.

Much of a site's setup lives in the admin, not here: the site's name and
address, the theme, e-mail, CAPTCHA, social login, uploads. Where both
exist, the config's value is used until one is saved in the admin (the
theme), or wins over the admin's (`public_base_url`; e-mail, the CAPTCHA
and social login keys: the admin can't replace what the server's owner
set).

## The server

| Key | Variable | Default | |
|---|---|---|---|
| `env` | `APP_ENV` | `prod` | `prod` - a live site: templates cached, the modules' templates and static files the binary's, `session.secret_key` required, a remote database encrypted by default, the log from INFO. `local` - development: templates are read again on every request, the modules' templates and static files come from their source directories, a remote database gets no TLS by default, the log has DEBUG lines. Any other value - the site doesn't start: a typo would turn a live site's checks off |
| `port` | `APP_PORT` | - | the HTTP port (the container: 3333) |
| `public_base_url` | `PUBLIC_BASE_URL` | empty | the site's canonical origin (`https://www.example.com`) for absolute links. Overrides the address in Site information, so a copy of the production database keeps the address of the site it runs on. Empty - the admin's. With neither, the sitemap, feeds and `robots.txt` take the request's host, and canonical links are relative |
| `page_cache_max_age` | `PAGE_CACHE_MAX_AGE` | `0` | seconds a shared cache (a CDN in front of the site) may keep a guest's page: `Cache-Control: public, max-age=0, s-maxage=…` on guests' HTML pages, `private, no-store` on the other HTML. `0` - off |
| `trusted_proxies` | `TRUSTED_PROXIES` | loopback | the proxies in front of the site, addresses or networks (`10.0.0.5`, `172.16.0.0/12`; the variable - separated by commas). Only a request from one of them has its `X-Forwarded-For` (read from the right, past the trusted hops) or `X-Real-IP` believed for the visitor's address, and its `X-Forwarded-Proto` for HTTPS; any other request's address and scheme are its connection's. Empty - `127.0.0.0/8` and `::1`, a web server on the same host |
| `theme` | `THEME` | empty | the theme, a directory under `themes/`, while none is chosen on the Appearance page. A site made by `app install` has one chosen (by default `kosatka`) |
| `themes_catalog_url` | `THEMES_CATALOG_URL` | `https://kosatka-cms.github.io/themes/index.json` | the themes' catalog on the Appearance page: the address of an `index.json` (`app theme index` builds one). `off` (or the variable set empty) - no catalog |
| `translations_catalog_url` | `TRANSLATIONS_CATALOG_URL` | `https://kosatka-cms.github.io/translations/index.json` | the translations catalog on the *Languages* tab of *Interface translations* and for `app install -language`: the address of an `index.json` (`https://`, or a file path), the languages' files next to it. `off` (or the variable set empty) - English only (see [Downloading a language](../guides/languages.md#downloading-a-language)) |
| `templates_debug` | `TEMPLATES_DEBUG` | `false` | HTML comments in the pages naming the template files that rendered them and the names tried; for a theme's author, not a live site |
| `secrets_key` | `SECRETS_KEY` | empty | the key the secrets kept in the database are sealed with: the keys and passwords of mail, CAPTCHA and social sign-in saved in the admin, the users' two-factor secrets. A long random string (at least 16 characters; `openssl rand -base64 32`). `new,old` while replacing it: the old key still opens what it sealed, and the site seals it again with the new one at start. Empty - they are kept unsealed. The status report warns about that, and about a guessable key (an example value, too few different characters). See [Secrets in the database](../guides/security.md#secrets-in-the-database) |
| `engine_dir` | `ENGINE_DIR` | empty | the engine's directory when several sites share it ([Several sites on one server](../guides/multisite.md)): the engine's templates and static files are its `web/templates` and `web/static`, the themes it ships its `themes/`; the site's own config, themes and files are in the working directory. Empty - the working directory has them all |

## The database: `db`

| Key | Variable | Default | |
|---|---|---|---|
| `driver` | `DB_DRIVER` | `mysql` | `mysql` or `postgres` |
| `host` | `DB_HOST` | - | |
| `port` | `DB_PORT` | 3306 for MySQL, 5432 for PostgreSQL | |
| `scheme` | `DB_NAME` | - | the database's name |
| `username` | `DB_USER` | - | |
| `password` | `DB_PASSWORD` | - | |
| `tls` | `DB_TLS` | see below | MySQL: the driver's `tls` (`true`, `skip-verify`, `preferred`, `false`); PostgreSQL: the `sslmode` (`disable`, `require`, `verify-full`…) |
| `sql_mode` | `DB_SQL_MODE` | strict | MySQL only: the session's `sql_mode` (`STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION`), so a value too long for its column fails instead of being cut. `server` - the server's own mode |
| `max_open_conns` | `DB_MAX_OPEN_CONNS` | `25` | the connection pool's limit |
| `max_idle_conns` | `DB_MAX_IDLE_CONNS` | `5` | |
| `conn_max_lifetime` | `DB_CONN_MAX_LIFETIME` | `5` | minutes before a connection is recycled |
| `conn_max_idle_time` | `DB_CONN_MAX_IDLE_TIME` | `5` | minutes before an idle connection is closed |

With `tls` unset, `env` not `local` and the database on another host (not
`localhost` or `127.0.0.1`), the connection is encrypted without checking
the certificate: `skip-verify` on MySQL, `require` on PostgreSQL. Many
managed MySQL hosts accept no other connections.

## Sessions: `session`

| Key | Variable | Default | |
|---|---|---|---|
| `secret_key` | `SESSION_SECRET_KEY` | - | the key signing the session cookies: a long random string. Required with `env: prod`; elsewhere a random one is made at each start, and the sessions don't survive a restart |
| `secure` | `SESSION_SECURE` | `false` | the cookies only over HTTPS: `true` on a site served over HTTPS |
| `max_age` | - | `604800` | a session's lifetime, in seconds (7 days) |

## Files: `files`

| Key | Variable | Default | |
|---|---|---|---|
| `public_dir` | `FILES_PUBLIC_DIR` | `./web/uploads` | where uploads go |
| `public_url` | `FILES_PUBLIC_URL` | `/uploads/` | the URL path they are served at. Content stores the files' URLs: change it only on a new site |
| `private_dir` | `FILES_PRIVATE_DIR` | `./private` | files never served as they are, only through the application with a permission check (files sent with forms, the files of fields with *Private files*) |
| `storage` | `FILES_STORAGE` | `local` | where the public files are kept: `local` (`public_dir`) or a storage the binary has - `s3`, `azure` ([Files in object storage](../guides/file-storage.md)) |
| `private_storage` | `FILES_PRIVATE_STORAGE` | `local` | where the private files are kept: `local` (`private_dir`) or a storage the binary has |
| `<storage>.…` | the storage's own | - | a storage's settings, its section of `files` (`files.s3`, `files.private_s3`, `files.azure`…) and its environment variables: [Files in object storage](../guides/file-storage.md) |

The server needs to write to the directories of the files kept on disk.
See [Files in object storage](../guides/file-storage.md).

## Content: `content`

| Key | Variable | Default | What |
|---|---|---|---|
| `trash_days` | `CONTENT_TRASH_DAYS` | `30` | days deleted content stays in the trash before it is deleted for good ([The trash](../guides/content-types.md#the-trash)); `-1` - until the trash is emptied by hand (`CONTENT_TRASH_DAYS=0` does the same, but `0` in the file counts as unset: `30`) |

## The log and metrics: `log`, `metrics`

| Key | Variable | Default | What |
|---|---|---|---|
| `log.format` | `LOG_FORMAT` | `text` | the log on standard output: `text` (`time=… level=INFO msg=…` lines) or `json` (a JSON object a line, for a log collector) |
| `log.requests` | `LOG_REQUESTS` | `false` | a line for every request answered: method, path (without the query string), route, status, time, bytes, user, request id |
| `metrics.listen` | `METRICS_LISTEN` | - | the address of a second HTTP server answering `GET /metrics` in the Prometheus format, e.g. `127.0.0.1:9101`; unset - no metrics |

See [Monitoring](../guides/monitoring.md).

## E-mail: `email`

The site sends through SMTP when `smtp_host` is set, or through a mail
service's API whose module the binary has when that module's variables
are set ([Resend](https://resend.com)'s module: `RESEND_API_KEY`) - the
module's first. Either needs `from_address`. A transport set here wins
over the one saved on the admin's Mail page, which then shows it but
can't change it; its switch and log still apply. A module's variables
without `from_address` fill its settings on the Mail page instead: the
transport is chosen and the sender set there. See
[Email](../guides/mail.md#sending).

| Key | Variable |
|---|---|
| `from_address` | `EMAIL_FROM_ADDRESS` |
| `smtp_host` | `SMTP_HOST` |
| `smtp_port` | `SMTP_PORT` (587 when unset) |
| `smtp_user` | `SMTP_USER` |
| `smtp_password` | `SMTP_PASSWORD` |

## CAPTCHA and OAuth

Keys that win over the ones saved in the admin (the *Spam protection
(reCAPTCHA)* and *Social login* pages, which then don't offer the key
fields; the social login keys provider by provider):

| Key | Variable |
|---|---|
| `recaptcha.site_key`, `recaptcha.secret_key` | `RECAPTCHA_SITE_KEY`, `RECAPTCHA_SECRET_KEY` - Google reCAPTCHA v2 on the account forms, when both are set |
| `oauth.google.client_id`, `oauth.google.client_secret` | `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET` |
| `oauth.vk.client_id`, `oauth.vk.client_secret` | `VK_OAUTH_CLIENT_ID`, `VK_OAUTH_CLIENT_SECRET` |
| `oauth.yandex.client_id`, `oauth.yandex.client_secret` | `YANDEX_OAUTH_CLIENT_ID`, `YANDEX_OAUTH_CLIENT_SECRET` |

## Timeouts: `timeouts`

In seconds.

| Key | Variable | Default | |
|---|---|---|---|
| `server_read` | `TIMEOUT_SERVER_READ` | `15` | reading a request |
| `server_write` | `TIMEOUT_SERVER_WRITE` | `15` | writing the whole response |
| `server_idle` | `TIMEOUT_SERVER_IDLE` | `60` | a keep-alive connection between requests |
| `shutdown` | `TIMEOUT_SHUTDOWN` | `30` | a stopping server waits for the requests in flight |
| `database_query` | `TIMEOUT_DATABASE_QUERY` | `30` | a page's database work |
| `cache_update` | `TIMEOUT_CACHE_UPDATE` | `15` | one refresh of the in-memory caches (the sidebar's blocks) |
| `http_request` | `TIMEOUT_HTTP_REQUEST` | `30` | a call to an outside service (reCAPTCHA, a mail service's API), a session with the SMTP server |

## Other variables

| Variable | |
|---|---|
| `CONFIG_PATH` | the config file |
| `STATIC_ASSET_VERSION` | the `?v=` on the CSS and JavaScript URLs, so browsers fetch new ones after a deploy. Unset - the commit the binary was built from (recorded by Go when it is built in a git checkout), else the git commit of the working directory, else `dev` |
| `GITHUB_TOKEN` | `app theme index`: the GitHub API without the anonymous rate limit |

The container's start script also reads `SITE_NAME`, `SITE_LANGUAGE`,
`ADMIN_NAME`, `ADMIN_EMAIL`, `DEMO_CONTENT`, `DEMO_SET`, `SITE_THEME`
and `MODULES` when it installs a new site; see
[Run in Docker](../getting-started/docker.md).

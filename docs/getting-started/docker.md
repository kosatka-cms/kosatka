---
title: Run in Docker
description: A local Kosatka site with MySQL in one command.
sidebar:
  order: 2
---

All you need is Docker. From the repository's directory:

```bash
docker compose up --build
```

Open http://localhost:8080 - the port is this machine's only: the
compose file is for a local site, its session key is known to everyone.
On its first start the container installs a new site (`app install`):

- the database schema;
- the starter content: the *Article* (with an image) and *Page* content
  types, the front page list, the main menu;
- the demo content: articles with covers, sections, tags, views and
  comments from a few demo users; search, popular content, the latest
  comments and a tag cloud in the sidebar; the sections and an *About*
  page in the menu, a footer menu; every module is installed, and each
  adds its own (a poll in the sidebar, galleries, ratings, a form…).
  All of it is ordinary content and settings: edit or delete it as you
  like;
- the administrator `admin`.

The administrator's password is printed once, in the log:

```bash
docker compose logs app | grep password
```

Sign in at http://localhost:8080/login (see
[After installation](./first-steps.md)).

On the next starts the container applies the database updates
(`app migrate -apply`) and starts the site. If installing fails for
another reason (the database can't be reached, a wrong `MODULES` value),
the container stops with the error in its log
(`docker compose logs app`). The data lives in three volumes: `db`
(MySQL), `files` (uploads) and `private` (the private files, such as
those sent with forms). `docker compose down -v` deletes them, and the
next start installs a new site.

## Settings

`docker-compose.yml` sets the site up through environment variables. A
variable the `app` service's `environment:` doesn't list doesn't reach
the container: add it there or to a `docker-compose.override.yml`. The
container's config file, `deploy/docker/config.yaml`, names the
variable of each setting; all of them are in
[the configuration reference](../reference/configuration.md).

These are read when the container installs a new site. `SITE_LANGUAGE`,
`DEMO_CONTENT`, `DEMO_SET`, `SITE_THEME` and `MODULES` can be set in the
shell, as below; `SITE_NAME`, `ADMIN_NAME` and `ADMIN_EMAIL` have their
values written in `docker-compose.yml`: change them there.

| Variable | Default | What it sets |
|---|---|---|
| `SITE_NAME` | `Kosatka` | the site's name |
| `SITE_LANGUAGE` | `en` | the site's language and that of the starter content: `en`, or a language of the translations catalog (`TRANSLATIONS_CATALOG_URL`), such as `ru` or `de` |
| `ADMIN_NAME`, `ADMIN_EMAIL` | `admin`, `admin@example.com` | the administrator |
| `DEMO_CONTENT` | `1` | `1` - with the demo content (`app install -demo`), `0` - without it |
| `DEMO_SET` | - | another demo set (`app install -demo-set`, see [Demo sets](../contributing/demo-sets.md)) |
| `SITE_THEME` | - | the site's theme (`app install -theme`), a directory of `themes/`; empty - the demo set's, else `kosatka` |
| `MODULES` | `all` | the site's modules (`app install -modules`): names separated by commas, `all` or `none`; set but empty (`MODULES=`) - the standard ones (forms, private messages, subscriptions, video embeds, the action log and the 404 log), the others installed when enabled on the Modules page |

The defaults are the compose file's. Run without it (`docker run`), the
image installs no demo content and the standard modules unless
`DEMO_CONTENT` and `MODULES` are set.

For example:

```bash
SITE_LANGUAGE=ru docker compose up --build
DEMO_CONTENT=0 MODULES=polls,forms docker compose up --build
```

These matter later too:

- `APP_ENV=local` - the templates are read again on every request:
  handy when `web/` or `themes/` is mounted into the container to work on
  a theme.
- `TEMPLATES_DEBUG=1` - HTML comments in the pages name the template
  files that rendered them.
- `THEMES_CATALOG_URL` - the themes' catalog on the Appearance page (the
  address of an `index.json`; Kosatka's by default, set empty for none).
- `TRANSLATIONS_CATALOG_URL` - the translations catalog: the interface's
  languages to download besides English (the address of an `index.json`;
  Kosatka's by default, set empty for none).
- `SESSION_SECRET_KEY`, `SECRETS_KEY` - the compose file holds values for
  a local site only. A real site needs its own long random keys.
- `DB_*` - the database. The compose file runs MySQL (the `db` service)
  and sets only `DB_PASSWORD`. For PostgreSQL, add a database service of
  your own and set `DB_DRIVER=postgres`, `DB_HOST`, `DB_PORT` (`5432`:
  the container's config has MySQL's `3306`), `DB_NAME`, `DB_USER` and
  `DB_PASSWORD`. The container runs with `APP_ENV=prod`, so a database
  on another host gets TLS by default: add `DB_TLS=disable` for a
  PostgreSQL server without it (the official image).

## What's in the image

The `Dockerfile` builds in `golang:1.27-alpine`: it minifies the CSS and
JavaScript and builds the binary, with the release number from the git
tag and the modules' templates and static files built in. The image runs
on `alpine` as an unprivileged user and holds the binary, `web/`,
`themes/`, `deploy/docker/config.yaml` and `deploy/docker/entrypoint.sh`,
the start script. Its health check asks `/healthz` (the site serves and
reaches its database) every 30 seconds, after a grace period for the
first start's install.
`config/local.yaml`, the docs site's `node_modules` and the rest of
`.dockerignore` stay out of the build context.

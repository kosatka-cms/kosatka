---
title: Several sites on one server
description: One engine for any number of sites, each with its own database, files and theme; one command updates them all.
sidebar:
  order: 18
---

Kosatka is multisite: one engine on a server serves any number of sites.
Unlike a multisite that keeps every site in one database, each site here
has its own database, files, theme and service:

- a failure, a load spike or a migration of one site never touches the
  others, and no query can show one site's data on another;
- a site moves to another server with its directory and a dump of its
  database;
- each site turns its own modules on and off, and the sites may even run
  on different databases (MySQL for one, PostgreSQL for another).

What the sites share is the engine: the binary, its templates and static
files, and the themes it ships. Engine releases sit side by side, and
each site runs the one its link points to, so updating is safe:

- `app sites update` moves the sites to a new release one by one - a dump
  of the site's database, its migrations, a restart, a health check -
  and stops at the first problem; the site that failed goes back to the
  release it ran;
- `-only` updates one site first to try a release (a canary), the rest
  after;
- `app sites rollback` brings the sites back to the release they ran
  before.

This guide builds on [Running in production](./production.md): the
server, nginx and HTTPS are set up the same way. The examples use the
directory `/opt/kosatka` and two sites, `blog` and `shop`.

## The layout

```text
/opt/kosatka/
├── releases/                       the engine releases, side by side
│   ├── v0.2.0/
│   │   ├── kosatka                 the binary
│   │   ├── web/templates/, web/static/   its templates and static files
│   │   └── themes/                 the themes it ships, every site's
│   └── v0.3.0/
└── sites/
    ├── blog/                       a site; its directory's name is its name
    │   ├── engine -> ../../releases/v0.3.0   the release it runs
    │   ├── engine.previous -> ../../releases/v0.2.0   the one it ran before
    │   ├── site.yaml               its config
    │   ├── site.env                its secrets, as environment variables
    │   ├── themes/                 its own themes
    │   ├── web/uploads/            its public files
    │   ├── private/                its private files
    │   └── backups/                its database's dumps from before its updates
    └── shop/
        └── …
```

A site's service runs the binary of its `engine` link with the site's
directory as its working directory and `ENGINE_DIR` set to the link (the
config's `engine_dir`): the engine's templates and static files come
from the release, everything else - the config, the themes, the files -
from the site's directory. A theme in the site's `themes/` is used
instead of the release's of the same name. Each site is always on one
release - binary and templates together - whatever the others run.

A site that builds a binary of its own (with modules from their own
repositories, `app module add`) can't share the engine: run it on its
own, as in [Running in production](./production.md).

## Set up the engine

1. Prepare the server as in [Running in production](./production.md#1-prepare-the-server):
   nginx, certbot, a database server and its client (`mysqldump` or
   `pg_dump`, for the dumps before updates).
2. Build the binary and the minified files
   ([section 2](./production.md#2-build-the-binary)), then upload the
   release into a directory of its own, named after its version. Put it
   together in a temporary directory, so the modules' static files don't
   land in your working copy, and upload it in one go:

   ```bash
   stage="$(mktemp -d)"
   mkdir -p "$stage/web"
   cp ./kosatka "$stage/"
   rsync -a ./web/templates ./web/static "$stage/web/"
   rsync -a ./themes "$stage/"
   go run ./cmd/app assets export "$stage/web/static"   # the modules' static files
   ssh root@example.com mkdir -p /opt/kosatka/releases/v0.2.0
   rsync -avz "$stage/" root@example.com:/opt/kosatka/releases/v0.2.0/
   rm -rf "$stage"
   ```

   Or unpack a release's archive (see [Releasing](../contributing/releasing.md))
   there - it has this layout already:

   ```bash
   ssh root@example.com 'mkdir -p /opt/kosatka/releases/v0.2.0 && \
     curl -sSfL https://github.com/kosatka-cms/kosatka/releases/download/v0.2.0/kosatka-v0.2.0-linux-amd64.tar.gz | \
     tar -xz -C /opt/kosatka/releases/v0.2.0 --strip-components=1'
   ```

   A release is never changed once a site runs it: a new version is a new
   directory.

3. Install the service template, one instance per site
   (`kosatka-site@<site>`), from the repository on your machine:

   ```bash
   scp deploy/systemd/kosatka-site@.service root@example.com:/etc/systemd/system/
   ssh root@example.com systemctl daemon-reload
   ```

   ```ini
   [Service]
   WorkingDirectory=/opt/kosatka/sites/%i
   Environment=ENGINE_DIR=/opt/kosatka/sites/%i/engine
   Environment=CONFIG_PATH=/opt/kosatka/sites/%i/site.yaml
   EnvironmentFile=-/opt/kosatka/sites/%i/site.env
   ExecStart=/opt/kosatka/sites/%i/engine/kosatka
   ```

   Another directory: change `/opt/kosatka` in the unit.

## Add a site

1. Create its database and a user for it, MySQL or PostgreSQL (the SQL
   is in [Run from source](../getting-started/from-source.md#1-a-database)).
2. Add the site, on the server:

   ```bash
   read -rsp 'Database password: ' DB_PASSWORD && echo && export DB_PASSWORD
   sudo --preserve-env=DB_PASSWORD /opt/kosatka/releases/v0.2.0/kosatka sites add blog \
     -port 8101 -url https://blog.example.com \
     -db-name blog -db-user blog \
     -site-name "Blog" -admin-name admin -admin-email admin@example.com
   ```

   ```text
   ==> blog: /opt/kosatka/sites/blog
     release: v0.2.0
     core: installed (4 steps)
     …
     install: done - "Blog" (en), administrator "admin"
     install: the administrator's password (shown once, change it after signing in): …
     start: systemctl enable --now kosatka-site@blog
     healthy: http://127.0.0.1:8101/healthz

   The nginx site …; its certificate: certbot certonly --nginx -d blog.example.com
   server {
       …
   ```

   The database password comes from the environment (`DB_PASSWORD`),
   not the command line, which other users of the server can see. The
   command:

   - checks the name (lower-case letters, digits, hyphens), the address,
     and that no other site has the port and nothing listens on it;
   - writes `sites/blog/`: its `engine` link to the release it is run
     from, `site.yaml`, `site.env` with the database user and password
     and generated session and secrets keys (`chmod 600`: systemd reads it before
     dropping to the service's user), `themes/`, the public and private
     file directories;
   - installs the site through the engine (`app install`) and gives its
     file directories to the service's user (`-user`, `www-data`);
   - starts its service (`-start`, `systemctl enable --now
     kosatka-site@{site}`; `''` - don't) and waits for its
     [`/healthz`](#the-health-check);
   - prints its nginx site for HTTPS, with the certbot command for its
     certificate.

   A failed install removes the site's directory again; the database may
   keep a part of the install - empty it before trying again.

   | Flag | |
   |---|---|
   | `-port <n>`, `-url <address>` | where it listens, its public address (required) |
   | `-db-name <name>` | its database (required) |
   | `-db-driver mysql\|postgres`, `-db-host`, `-db-port`, `-db-user` | the database server (default `mysql`, `127.0.0.1`, the driver's port, the site's name) |
   | `-site-name`, `-admin-name`, `-admin-email` | as for [`app install`](../reference/cli.md#app-install) (required) |
   | `-language`, `-demo`, `-modules` | as for `app install` |
   | `-start '<command>'`, `-wait 1m`, `-user www-data`, `-root <dir>` | how to start it, how long it may take to answer, the service's user, the sites' directory |

3. Save the printed nginx site as `/etc/nginx/sites-available/blog`, get
   the certificate, enable the site:

   ```bash
   sudo certbot certonly --nginx -d blog.example.com
   sudo ln -sf /etc/nginx/sites-available/blog /etc/nginx/sites-enabled/
   sudo nginx -t && sudo systemctl reload nginx
   ```

   It serves the engine's static files through the site's `engine` link
   (they follow it to every release), the site's public files from its
   directory, and sends its themes (`/static/themes/`) and everything
   else to the site's port.

### The health check

`/healthz` answers `ok` while the site is up and its database answers,
`503` otherwise. `app sites` waits for it after starting a site; a load
balancer or a container's health check can use it too.

## List the sites

```bash
sudo /opt/kosatka/releases/v0.2.0/kosatka sites list
```

```text
SITE   URL                        RELEASE   MIGRATIONS   HEALTH
blog   https://blog.example.com   v0.3.0    up to date   ok
shop   https://shop.example.com   v0.2.0    up to date   ok
```

Any release's binary lists the sites (`sites/` beside `releases/`;
`-root <dir>` - another directory). Each is read the way its service
runs it: its directory, its config and its `site.env`, and none of
another site's settings. *Release* is the one its `engine` link points
to, *migrations* the ones it would run on its database, *health* its
`/healthz`.

## Update the sites

1. Upload the new release as in [Set up the engine](#set-up-the-engine),
   step 2, into its own directory (`releases/v0.3.0`).
2. Try it on one site first, if you like:

   ```bash
   sudo /opt/kosatka/releases/v0.3.0/kosatka sites update -only blog
   ```

3. Move every site to it:

   ```bash
   sudo /opt/kosatka/releases/v0.3.0/kosatka sites update
   ```

   ```text
   ==> blog
     migrate: nothing to do
     restart: systemctl restart kosatka-site@blog
     healthy: http://127.0.0.1:8101/healthz
   ==> shop
     backup: /opt/kosatka/sites/shop/backups/20261201-120000-v0.2.0.sql.gz
     release: v0.2.0 -> v0.3.0
     core: applied 20261120120000_view_filter_via
     restart: systemctl restart kosatka-site@shop
     healthy: http://127.0.0.1:8102/healthz

   sites: 2 updated
   ```

The release whose binary runs the update is the one the sites move to.
For each site in turn:

1. a dump of its database into its `backups/` (`app backup`; it keeps
   the five newest, `-keep`; `-backup=false` - none);
2. its `engine` link to the new release - the one it ran becomes its
   `engine.previous`;
3. its migrations (`app migrate -apply`, by the new release);
4. a restart of its service and its `/healthz` within a minute.

A site on the new release already (`blog` above, the canary) gets no
dump and no new link: only its migrations, if any are left, and its
restart.

The first failure - a dump, a migration, the restart, a site that doesn't
come up - stops the update. That site goes back to the release it ran
(restarted on it again if it was restarted), the reason and its dump
are named, and the sites after it aren't touched. Fix it and run the
update again: the sites already on the new release have no migrations
left to run.

The migrations change the database in place: a site sent back to its
release runs on a database the new release may have changed. Most
releases only add to the schema, and the older release works on it; if
not, restore the named dump.

The static files' URLs carry the binary's commit (`?v=…`), so a new
release is a new version on every site; `STATIC_ASSET_VERSION` in a
site's `site.env` overrides it.

| Flag | |
|---|---|
| `-only blog,shop` | only these sites, in this order |
| `-restart '<command>'` | how to restart a site, `{site}` - its name; `''` - don't. Default `systemctl restart kosatka-site@{site}` |
| `-wait 2m` | how long a restarted site may take to answer `/healthz` (default 1m) |
| `-backup=false`, `-keep <n>` | no dump before the migrations; the dumps a site keeps (default 5) |
| `-root <dir>` | the sites' directory (default `sites/` beside `releases/`) |

## Roll back

```bash
sudo /opt/kosatka/releases/v0.3.0/kosatka sites rollback
```

Each site (or `-only` ones) goes back to the release it ran before (its
`engine.previous`), is restarted and checked; the release it leaves
becomes its previous one, so a second rollback undoes the first. Its
database stays as the update left it: the summary names its dump from
before the update, to restore if the older release needs the old
schema.

## Remove old releases

A release no site's `engine` or `engine.previous` points to can go:

```bash
ls -l /opt/kosatka/sites/*/engine*       # the releases in use
sudo rm -rf /opt/kosatka/releases/v0.1.0
```

## Logs

Each site logs to its own journal:

```bash
journalctl -u kosatka-site@blog -f
```

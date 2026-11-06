---
title: The app commands
description: Every command of the engine's binary, with its flags.
sidebar:
  order: 1
---

The engine's binary is `app` (`go run ./cmd/app`, or the binary built
from `./cmd/app`; a site with modules of its own builds its own binary,
which has the same commands). Without a command it serves the site; with
one it runs the command and exits. `app help` lists them.

The server and the commands take their config file from `-config <file>`
before the command (`app -config site.yaml migrate`) or from
`CONFIG_PATH` (see [the configuration](./configuration.md)); its
`engine_dir` applies to the commands too. Without one, the server and
the commands that need it say so and exit with code 1.

## `app`

```bash
app [-config <file>]
```

Serves the site on the config's `port`. Run it from a directory with the
engine's `web/` and `themes/`, or - a site sharing the engine - from the
site's directory with `ENGINE_DIR` set (see
[Several sites on one server](../guides/multisite.md)). It stops on
`SIGINT` or `SIGTERM`, waiting for the requests in flight and then for
its background jobs (the cache's refresh, the scheduled ones) before
closing the database, within `timeouts.shutdown`. If it can't serve -
its port taken, say - it exits with code 1, so its service manager sees
it. A request whose handler panics gets a 500, and the log gets the
panic with its stack.

## `app install`

```bash
app install -site-name <name> -admin-name <user> -admin-email <e-mail> \
  [-language en [-translations <url>]] [-demo | -demo-set <set>] [-theme <theme>] [-modules <list>]
```

A new site on an empty database: the schema of the core and of the
site's modules, their starter settings, the site's name and language, the
first administrator and the starter content (the *Article* and *Page*
content types, the front page list, the main menu, a copyright block in
the footer, the `kosatka` theme and its logo). The starter content is
data, `internal/app/starter/starter.yaml`; its labels are translated into
the site's language. It prints the
administrator's generated password once, and, when `SECRETS_KEY` isn't
set, a newly generated one to put in the site's env file. The site's name, its
administrator and the starter content are saved in one transaction: if
it fails, none of them is. On a database that has a site already it
refuses with exit code 3 (other failures exit 1, a wrong flag 2):
updates go through `app migrate`. A failed install leaves its schema:
start again on an empty database.

| Flag | |
|---|---|
| `-site-name` | the site's name (required) |
| `-admin-name` | the administrator's username (required) |
| `-admin-email` | the administrator's e-mail, also the site's (required) |
| `-language` | the site's language and that of the starter content: `en` (the default), a language built in (`ru`), or one of the translations catalog - downloaded first, as `app translations add` does, and refused before anything is installed when the catalog hasn't it |
| `-translations` | the translations catalog a `-language` that isn't built in comes from (an `index.json`'s address or file); by default the config's `translations_catalog_url` |
| `-demo` | add the demo content: articles with covers, sections, tags, comments from demo users, sidebar blocks, an *About* page in the menu, a footer menu, and the installed modules' own demo content |
| `-demo-set` | the demo content of another demo set (implies `-demo`): `blog` (the one above) or a set of `internal/app/demoset/sets`. A set is in the languages it has files for; another `-language` is refused. See [Demo sets](../contributing/demo-sets.md) |
| `-theme` | the site's theme, a directory of `themes/`: the starter's (and the demo's) blocks and menus go in its regions. Empty - the demo set's, else `kosatka` |
| `-modules` | the site's modules: names separated by commas, `all` or `none`. Empty - the standard ones (forms, private messages, subscriptions, video embeds, the action log and the 404 log) and, with `-demo-set`, the modules the set has content for. A module a listed one requires is added. The others are installed later, when enabled on the Modules page |

## `app migrate`

```bash
app migrate                    # list what would run
app migrate -apply             # run it
app migrate -apply -additive   # run what the binary still running is fine with
app migrate -baseline          # record everything as applied, running nothing
```

The schema migrations of the core and of every module that is enabled or
already installed; a module that is switched off and was never installed
is skipped. Migrations are built into the binary and recorded in the
`schema_migrations` table. A module enabled but not installed yet (switched
on in the database, say) is installed by `-apply` as the Modules page
installs it: its tables, its install step, its blocks placed in the site's
theme, its labels in the site's language. A module installed in the
database that the binary doesn't have is listed
(`<name>: installed in the database, not in this binary`) and left as
it is: see
[Using modules](../modules/using-modules.md#a-module-installed-in-the-database-but-not-in-the-binary).

`-additive` (with `-apply`) runs, in each set, only the steps before its
first schema file that drops or changes something (a table, a column or
an index dropped, a column renamed or changed, rows deleted): what the
older binary, still serving the site, is fine with. Run it before you
restart with the new binary, so the new code finds its new columns at
once, and `-apply` after the restart for the rest. A set stopped early
is listed.

`-baseline` is for a database that already has the schema but no record
of it (for example one moved from another installation); run it once.

## `app theme`

Themes kept in their own git repositories. These commands need neither
the config nor the database; run them from the directory with `themes/`.
Installed versions and checksums are kept in `themes/themes.lock`.

| Command | |
|---|---|
| `app theme list [-updates]` | the themes under `themes/`: those installed from a repository with their version and source, the others as shipped with the project; `-updates` also asks each installed theme's repository for a newer release |
| `app theme add <repository>[@v1.2.0] [-name <dir>]` | install a theme from its git repository: its newest release, or the one given. `-name` - the directory under `themes/` (by default the repository's name) |
| `app theme update <name>[@v1.2.0]` | move an installed theme to a release (its newest by default) |
| `app theme remove <name>` | delete an installed theme |
| `app theme verify` | check the installed themes are what `themes/themes.lock` says |
| `app theme search [words] [-catalog <url>]` | search the themes' catalog (by default `$THEMES_CATALOG_URL`) |
| `app theme index -list themes.txt -out index.json` | build a catalog from a list of themes' repositories, one a line. With `$GITHUB_TOKEN` it reads the repositories' stars without the anonymous rate limit |

## `app module`

Modules from outside the engine: Go modules that register themselves. A
site that has one builds its own binary (its `main` calls
`kosatka.Main()`), and `add` and `remove`, run in the directory of that
site's main package, edit its `go.mod` and `kosatka_modules.go`.
Rebuild the binary after them.

| Command | |
|---|---|
| `app module list` | the modules the binary has: the engine's and those from outside it, with their Go modules' versions; then its [file storages](../guides/file-storage.md) |
| `app module add <path>[@version]` | add a module, or a file storage (`go get`; the latest version without one) |
| `app module remove [-force] <path>` | remove it. It refuses while the module is installed in the site's database (the config: `-config` or `CONFIG_PATH`): uninstall it on the Modules page first; a storage - while `files.storage` or `files.private_storage` names it. When the database or the config can't be checked it warns and removes it; `-force` removes it anyway |

## `app assets`

```bash
app assets export <dir>
```

Writes the static files of the modules built into the binary to
`<dir>/modules/<name>/`, for a web server that serves `/static/` itself
(for example nginx with `<dir>` = `web/static`). Without such a web
server the application serves them.

## `app config`

```bash
app config [-json]
```

Prints the site's settings as the engine reads them - the config file
with the environment over it - without the secrets: the environment, the
port, the site address, the theme, the engine directory, the database
(driver, host, port, name), the file directories and where the files
are kept (`local`, or what the storage says: `s3://<bucket>/<prefix>/`).
`-json` prints them as JSON.

## `app backup`

```bash
app backup <file>
```

Dumps the site's database into `<file>` (compressed when it ends in
`.gz`) with the database's tool - `mysqldump` or `pg_dump`, which must be
installed - reading the password from its environment, never the
command line. It never overwrites a file that exists, and a failed dump
leaves no file.

## `app files`

```bash
app files migrate [-private] [-to-local] [-derivatives] [-jobs 8] [-dry-run]
```

Copies the public files from their directory (`files.public_dir`) to the
storage of `files.storage` (a bucket, a container) or back with
`-to-local`: those the destination has already with the same size are
skipped, so it resumes; the image styles' derivatives and dotfiles are
left out (`-derivatives` - the derivatives copied too); nothing is
removed. `-private` copies the private files (`files.private_dir` and
the storage of `files.private_storage`). `-jobs` - how many files are
copied at once (8 by default); `-dry-run` counts what it would copy. See
[Files in object storage](../guides/file-storage.md#existing-files).

## `app search`

```bash
app search reindex
```

Writes the search index again: the words of every content item's title
and text fields. The site writes them itself as content is saved, and
a new site gets them with its starter and demo content; this is for
content changed past the site (rows imported into the database) or an
index to mend. The search keeps working meanwhile. See
[Search](../guides/search.md#the-search-index).

## `app translations`

```bash
app translations list [-catalog <url>]
app translations add <lang> [-catalog <url>]
app translations update [<lang>] [-catalog <url>]
app translations remove <lang>
app translations index [-src .] [-out dist] [-sources <file,…>] [-complete <lang,…>]
```

The interface's languages downloaded from the translations catalog,
as on the *Languages* tab of *Interface translations* - for deploy
scripts and sites run without the admin.

| Command | What it does |
|---|---|
| `list` | the languages: English (the source strings), the built-in ones, the downloaded ones (their version; `0.1.0 -> 0.2.0` - an update in the catalog), the catalog's others; how much of the site's interface each translates, a draft marked |
| `add <lang>` | downloads the language and installs it: its file checked against the catalog's checksum, each string checked, the ones that don't pass left out (the output says how many) |
| `update [<lang>]` | moves the downloaded languages with an update in the catalog (or the one given) to the catalog's version |
| `remove <lang>` | removes a downloaded language; the site's own changes of its strings stay. The site's interface language isn't removed |
| `index` | builds a translations catalog from a translations repository (`-src`): `index.json` and a file per language in `-out`. Each string is checked against the code's strings - the binary's, or the `strings.json` files of `-sources`: one that isn't the code's any more is left out, one that can't stand for its English string fails the build, and so does a `-complete` language missing a string. Needs no site |

For `list`, `add`, `update` and `remove` the catalog is `-catalog` (an
`index.json`'s `https://` address or a file), else the config's
`translations_catalog_url`. A running site
takes a change made here up when it starts again; the admin's page
applies one at once. See
[Downloading a language](../guides/languages.md#downloading-a-language).

## `app sites`

```bash
app sites list   [-root <dir>]
app sites add    <name> -port <n> -url <address> -db-name <name> -site-name … -admin-name … -admin-email … [flags]
app sites update   [-root <dir>] [-only <site,…>] [-restart '<command>'] [-wait <duration>] [-backup=false] [-keep <n>]
app sites rollback [-root <dir>] [-only <site,…>] [-restart '<command>'] [-wait <duration>]
```

The sites sharing the engine on a server (see
[Several sites on one server](../guides/multisite.md)): a site is a
directory of the sites root with its config `site.yaml`, its secrets
`site.env` and its `engine` link to the release it runs
(`<home>/releases/<name>`). The sites root is `<home>/sites` for a
release's binary, `sites/` beside any other binary, or `-root`.

| Command | |
|---|---|
| `list` | each site's address, release, pending migrations and `/healthz` |
| `add` | a new site: checks its name, address and port (no other site's, nothing listening), writes its directory (`site.yaml`, `site.env` with a generated session key, its themes and file directories), installs it (`-site-name`, `-admin-name`, `-admin-email`, `-language`, `-demo` and `-modules`, as for `app install`), starts it (`-start`, default `systemctl enable --now kosatka-site@{site}`) and waits for `/healthz`, then prints its nginx site. The database password comes from `DB_PASSWORD` in the environment. A failed install removes the directory. See [the guide](../guides/multisite.md#add-a-site) |
| `update` | moves each site in turn to the release whose binary runs it: a dump of its database into its `backups/` (`-backup=false` - none; `-keep`, default 5 - the dumps it keeps; none for a site on this release already), its `engine` link to the release (the old one becomes `engine.previous`), `app migrate -apply`, its restart, then `/healthz` within `-wait` (default `1m`). The first failure stops it: that site goes back to its release, its dump is named, the sites after it aren't touched. `-restart` - the restart command, `{site}` - the site's name (default `systemctl restart kosatka-site@{site}`; `''` - none); `-only` - just these sites, in this order (one first - a canary) |
| `rollback` | each site (or `-only` ones) back to its `engine.previous`, restarted and checked; its database isn't changed - its newest dump is named |

Each step runs the engine binary for the site as its service does: in
its directory, with its config, its `site.env` and `ENGINE_DIR`, and
nothing else of the calling environment but `PATH`, `HOME`, `TMPDIR`,
`TZ` and the locale.

## Other commands

The repository has a few more binaries:

- `go run ./cmd/i18n-extract -lang ru` - lists the interface strings of
  the code that the language's catalogs don't translate yet.
- `./scripts/migrations_gen.sh <name>` (`cmd/migrations-gen`) - writes
  the database migrations from the ent schemas; for those who change the
  engine (see [the database](../contributing/database.md)).

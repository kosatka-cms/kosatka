---
title: Distributing a theme
description: A theme in its own git repository - releases, installing and updating it with app theme, themes.lock, the themes catalog and testing.
sidebar:
  order: 6
---

A theme can live in the site's repository, under `themes/`, or in a git
repository of its own that sites install it from.

## The theme's repository

The repository's root is the theme's directory:

```text
kosatka-theme-sand/
├── theme.yaml
├── templates/
├── static/
│   └── screenshot.jpg
└── translations/
```

- **The name.** A site installs the theme into `themes/<name>/`, where
  the name is the repository's, without `.git` and a `kosatka-theme-`
  prefix: `kosatka-theme-sand` becomes `sand`. Name the repository so
  that this is lowercase latin letters, digits, `-` and `_` (a site can
  choose another name with `-name`).
- **Releases are tags** `vX.Y.Z` (`v1.2.0`). Without a version the
  newest release is installed; a pre-release (`v1.3.0-rc1`) only when
  asked for by name.
- Set `version` in `theme.yaml` to the release (`1.2.0` for `v1.2.0`):
  installing warns when they differ. Set `min_version` to the oldest
  engine release the theme works with.
- The minified files (`*.min.css`, `*.min.js`) needn't be in the
  repository: the site builds them, and they are left out of the
  theme's checksum.
- Give it a `screenshot`, a `description`, an `author`, a `license` and
  `tags`: the Appearance page and the catalog show them.

## Installing and updating

The `app theme` commands need neither the config nor the database; run
them from the project's directory (the one with `web/` and `themes/`;
a site sharing the engine - from its own directory, with `ENGINE_DIR`
set). Those that fetch (`add`, `update`, `list -updates`, `index`) need
git.

```bash
app theme add https://github.com/example/kosatka-theme-sand   # the newest release → themes/sand
app theme add git@github.com:example/sand.git@v1.2.0 -name sand
app theme list -updates      # the installed themes, and newer releases
app theme update sand        # to the newest release (or sand@v1.3.0)
app theme remove sand
app theme verify             # the files are what themes.lock says
```

| Command | What |
|---|---|
| `add <repository>[@vX.Y.Z] [-name <dir>]` | installs a release into `themes/<name>/`. The repository is an `https://`, `ssh://`, `git@host:path` or `file://` address |
| `list [-updates]` | every theme of `themes/`: an installed one with its tag and repository, the others as "shipped with the project", and the directories that aren't usable themes with the error; `-updates` asks each repository for a newer release |
| `update <name>[@vX.Y.Z]` | moves an installed theme to a release, its newest by default |
| `remove <name>` | deletes an installed theme and its entry in the lock |
| `verify` | checks that each installed theme is in place and unchanged |
| `check [dir]` | checks a theme before its release (see [below](#checking-a-theme)) |
| `search [words] [-catalog <url>]` | searches the [catalog](#the-themes-catalog) |
| `index -list <file> -out <file>` | builds a catalog |

- The tag is fetched (`git clone --depth 1`) without its `.git`. A tag
  without `theme.yaml` at the root, or with a manifest that fails the
  [checks](./manifest.md#checks), isn't installed; an update that fails
  keeps the previous copy.
- A directory that is already there is never overwritten, a theme kept in
  the project included; `update` and `remove` touch only themes installed
  from a repository.
- After `add`, commit `themes/` and switch the site to the theme on
  **Appearance**. The page shows an installed theme's tag as its
  version and "Installed from <repository>".
- `remove` leaves the theme's block layout and settings in the database
  (harmless). A site that had it active shows the engine's templates
  until another theme is chosen.

The full command reference is in [The app commands](../reference/cli.md#app-theme).

## themes.lock

`themes/themes.lock` records each installed theme:

```json
{
  "themes": {
    "sand": {
      "source": "https://github.com/example/kosatka-theme-sand",
      "version": "v1.2.0",
      "commit": "4f1c2a9e…",
      "hash": "h1:…"
    }
  }
}
```

`hash` is a checksum of the theme's files (as in `go.sum`), the minified
files and `.DS_Store` left out.

Commit the installed theme's files and the lock to the site's
repository, as vendored code: a deploy carries them, and the server
needs no network access to the theme's repository.

`app theme verify` fails when a theme's directory is missing or its
files differ from the release installed. A theme is changed by a new
release, not edited in place: to adjust an installed theme for one site,
make a [subtheme](./manifest.md#subthemes) of it in the site's
repository. Run `verify` before deploying, in CI or in the deploy script.

## The themes catalog

A catalog is an index of themes from their repositories: `index.json`,
built from a list of repositories.

```bash
# themes.txt - a repository a line, # starts a comment
GITHUB_TOKEN=… app theme index -list themes.txt -out index.json
app theme search dark -catalog https://example.com/kosatka-themes/index.json
```

For each repository `index` takes its newest release and records the
manifest at that tag (checked as on install, except that a base theme
isn't looked up), the tag's commit and date, and for a GitHub repository
the screenshot's URL at the tag, the stars and the date of the last push
(`GITHUB_TOKEN` raises GitHub's anonymous limit of 60 requests an
hour). A repository without a release, without a theme or with a broken
manifest is left out with an error, and the others are built; of two
themes with the same name the second is left out.

An entry of `index.json`:

```json
{
  "name": "sand",
  "source": "https://github.com/example/kosatka-theme-sand",
  "version": "v1.2.0",
  "commit": "4f1c2a9e…",
  "released": "2026-05-04T10:00:00Z",
  "title": "Sand",
  "description": "A warm light theme with wide covers.",
  "author": "Example Studio",
  "license": "MIT",
  "tags": ["light", "cards"],
  "min_version": "0.2.0",
  "screenshot": "https://raw.githubusercontent.com/example/kosatka-theme-sand/v1.2.0/static/screenshot.jpg",
  "stars": 12,
  "updated": "2026-05-04T10:00:00Z"
}
```

Where the list and the index are kept and how often the index is rebuilt
is up to whoever runs the catalog: a repository of its own with a
scheduled job is enough.

A site shows the Kosatka themes catalog
(`https://kosatka-cms.github.io/themes/index.json`) unless its config
names another or turns it off: `themes_catalog_url` /
`THEMES_CATALOG_URL` (an `https://` address of an `index.json`, a file
path, or `off`; see [the configuration](../reference/configuration.md)). The page
**Appearance → Themes catalog** (`/admin/config/appearance/catalog`)
reads it at most once an hour (up to 4 MB), searches it by words and
shows each theme's screenshot, release and its date, stars, description,
author, licence, tags, whether it suits the site's engine, and the
command to run: `app theme add …`, or `app theme update …` when a newer
release of an installed theme is out. Themes are installed by the
command, from the project's directory, never from the admin.

An entry that doesn't pass the checks (a name that isn't a directory's,
a repository address that isn't `https://`, `ssh://` or `git@…`, a tag
that isn't `vX.Y.Z`) isn't shown; a homepage or screenshot that isn't
`https://` is dropped, long texts are cut short.

## Checking a theme

`app theme check` checks a theme's directory (by default the current
one) before a release. It needs neither the config nor the project;
give it the theme's repository:

```bash
cd kosatka-theme-sand && app theme check
app theme check ~/src/kosatka-theme-sand
```

```text
problem: templates/header.html:4: loads https://fonts.googleapis.com/css2?family=… from fonts.googleapis.com: the themes catalog takes no third-party resources - ship the file with the theme
problem: templates/footer.html:12: "Made of sand": not in translations/ru.json
warning: theme.yaml:20: version 1.2.0 is the release v1.2.0, with changes after it: raise it for the next release
2 problem(s), 1 warning(s)
```

| It checks | A problem | A warning |
|---|---|---|
| the manifest | it doesn't load (the [checks](./manifest.md#checks) of installing); the screenshot or a stylesheet it names isn't in `static/` | no `description`, `author`, `license`, `tags`, `screenshot`, `version` or `min_version`; a `min_version` newer than this engine; no `README.md` |
| the theme's files | a `/static/themes/<this theme>/…` path, a `themeCSS`/`themeJS` name or a stylesheet's relative `url(…)` with no file behind it | a path into another theme's files (not the base theme's) |
| other servers | a template, stylesheet or script loading a script, a stylesheet, a font, a picture or a frame from another server (a CDN, web fonts) | |
| the catalogs | a string of the theme a language's catalog lacks; a translation that drops a placeholder or adds HTML; a plural string without the language's forms; a catalog that doesn't parse | a translation of a string the theme doesn't use (one of the engine's it rewords is fine) |
| the demo site | `demo/<lang>.yaml` doesn't load as `app install -demo-set` loads it, or puts a block or a menu in a region the theme hasn't | |
| the release | `version` older than the repository's latest release tag | no release yet; `version` newer than the latest tag (not tagged yet); `version` the latest tag's with commits or changes after it |

The strings are those of the templates, scripts and manifest (as
[Translations](./assets-and-translations.md#translations) lists them),
except the engine's and the modules' own: their catalogs translate
them. `translations/en.json`, if any, isn't checked. The release is
compared only when the directory is the root of a git repository: a
theme kept in a site's repository has no tags of its own. The minified
files and the hidden directories are skipped.

It exits with 1 when there is a problem, so a release script or CI can
run it.

## Testing a theme

Before a release:

- **`app theme check`** passes (see [above](#checking-a-theme)).

- **A demo site** (optional): `demo/<lang>.yaml` - the theme's own
  demo set, installed with `app install -demo-set <theme>` (see [Demo
  sets](../contributing/demo-sets.md)): a site that looks like the
  screenshot in one command.
- **The manifest loads.** The theme is on **Appearance** and in
  `app theme list`; a theme with an error in `theme.yaml` isn't, and both
  name its directory with the error.
- **Preview** it on a site with real content, and look at the
  [style guide](./templates.md#the-style-guide) in it.
- **The regions**: open a page with `?blocks_demo=1` as an administrator
  to see every region framed with its name.
- **The templates**: with [template debugging](./templates.md#template-debugging)
  on, check that the theme's files are the ones used.
- **The strings**: every string the templates, scripts and manifest
  translate is in each language's catalog (`app theme check` lists the
  missing ones; on a site, *Interface translations* → *Strings* too).
- **Installing**: a local clone with tags installs as a repository does:
  `app theme add file:///path/to/kosatka-theme-sand@v1.2.0`.

The engine's own tests check the theme mechanisms on a theme of their
own, `testdata/themes/fixture`: one sample of everything a theme may
declare or override (regions, an icon, `theme_head`, a layout, a header
with a region's menu block, a lazy block's placeholder, a teaser, a date
field's template, a View template, a content page). It isn't a theme for
sites. A change to the engine that lets themes override something new
adds a sample there and a test on it; see
[Testing](../contributing/testing.md).

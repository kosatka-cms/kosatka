---
title: Contributing
description: For those who change the engine itself - the main rule, setting up, the checks and how changes are made.
sidebar:
  label: Overview
  order: 0
---

This section is for people who change Kosatka itself: its core, its
bundled modules and themes, its scripts and its documentation. If you
build a site, start with the [guides](../guides/index.md). If you write a
theme or a module for your own site, see [Themes](../themes/index.md) and
[Modules](../modules/index.md): neither needs changes to the engine.

The process in full (issues, pull requests, code style, the licence) is
in the repository's
[`CONTRIBUTING.md`](https://github.com/kosatka-cms/kosatka/blob/main/CONTRIBUTING.md).

## The main rule: a general-purpose engine

Kosatka is a CMS for any site, not the code of one site. Go code,
templates and JavaScript hold no site's content types, fields, URL paths,
sections, block names, texts, IDs or slugs.

- Whatever is specific to a site lives in its data: the database, the
  settings made in the admin, a data migration or `config/*.yaml`. Its
  look lives in its theme.
- When one type or section needs a behaviour, build a general mechanism
  for it: a setting, a flag, a field type, a block plugin. Never write
  `if type == "articles"`.
- If you find site data hardcoded next to your change, don't copy it:
  propose moving it into data.

## Setting up

Run a local site in Docker ([Run in Docker](../getting-started/docker.md))
or build it from source ([Run from source](../getting-started/from-source.md)).
The database tests and the migration generator need Docker either way.

## The checks

```bash
./scripts/check.sh       # golangci-lint (gofmt, go vet and the rest; the CI
                         # version, installed when missing), the unit tests,
                         # govulncheck and gitleaks when installed, the example
                         # module and site in examples/
./scripts/check.sh -race # the unit tests with the race detector (as CI)
./scripts/check.sh -db   # plus the tests on MySQL and PostgreSQL and the
                         # migration files against the ent schemas (Docker)
./scripts/docs.sh        # builds this documentation; a link to a missing
                         # page fails the build
```

`./scripts/docs.sh dev` serves the documentation at
http://localhost:4321/kosatka/ and rebuilds it as `docs/` changes. Both
run in Docker when Node.js isn't installed. [Testing](./testing.md)
describes the tests and how to run part of them.

## How changes are made

- For anything bigger than a small fix, open an issue first: describe the
  problem and the approach.
- One pull request holds one change. Describe it and say how you checked
  it.
- Commit messages are in English: a short summary line, then what changed
  and why.
- A change to how something works updates the matching page of this
  documentation in the same pull request. A new mechanism gets its
  description there too: in [Architecture](./architecture.md) or
  [Database and migrations](./database.md) for the engine's insides, in
  the [guides](../guides/index.md), [themes](../themes/index.md) or
  [modules](../modules/index.md) pages for what site builders and authors
  use.
- Pages start with front matter holding their `title` and have no `#`
  heading of their own. Link pages by their files, relatively
  (`[Views](../guides/views.md)`).

## The pages of this section

- [Architecture](./architecture.md): the layers, the request path, the
  core and the modules.
- [Database and migrations](./database.md): the ent schemas, generated
  migrations, data steps, MySQL and PostgreSQL.
- [Testing](./testing.md): the kinds of tests, their file names, running
  them, the checks of the whole repository.
- [Demo sets](./demo-sets.md): a whole demo site as data, installed with
  `app install -demo-set`.
- [Releasing](./releasing.md): a `vX.Y.Z` tag and what the release
  workflow builds - the archives, the GitHub release, the Docker image.

Theme authors look in [Themes](../themes/index.md), module authors in
[Modules](../modules/index.md) and [the module SDK](../modules/sdk.md).

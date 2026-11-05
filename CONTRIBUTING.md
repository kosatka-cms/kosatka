# Contributing to Kosatka

Thank you for helping. This file covers how changes are made. How the engine
fits together is described in [`docs/contributing/`](docs/contributing/):
[the architecture](docs/contributing/architecture.md),
[the database and migrations](docs/contributing/database.md) and
[testing](docs/contributing/testing.md). Issues and pull requests are
welcome in English or Russian.

## The main rule: a general-purpose engine

Kosatka is a CMS for any site, not the code of one site. Go code, templates
and JavaScript hold no site's content types, fields, URL paths, sections,
block names, texts, IDs or slugs.

- Whatever is specific to a site belongs in its data: the database, the
  admin's settings, a data migration or `config/*.yaml`. Its look belongs
  in its theme.
- If a behaviour is needed for one type or section, build a general
  mechanism for it: a setting, a flag, a field type or a block plugin.
  Never write `if type == "articles"`.
- If you find hardcoded site data next to your change, don't copy it.
  Propose moving it into data.

## Before you start

For anything bigger than a small fix, open an issue first and describe the
problem and the approach. One pull request should hold one change.

## Setting up

Run a local site with `docker compose up --build`, or build it from source
as [Run from source](docs/getting-started/from-source.md) describes. The database tests
need Docker.

## Checks

```bash
./scripts/check.sh       # golangci-lint (gofmt, go vet and the rest; the CI
                         # version, installed when missing), unit tests,
                         # govulncheck and gitleaks when installed, the example
                         # module and site in examples/
./scripts/check.sh -race # the unit tests with the race detector (as CI)
./scripts/check.sh -db   # plus the tests on MySQL and PostgreSQL and the
                         # migration files against the ent schemas
```

CI runs the same checks, golangci-lint, govulncheck and gitleaks
(`.gitleaks.toml` lists what isn't a secret) always. A release is made by
a tag: see [Releasing](docs/contributing/releasing.md).

## Code

- **Layers.** `internal/domain/entity` declares the ports, services
  depend only on them, and `internal/infrastructure/ent` implements them.
  `internal/domain/layers_test.go` checks the direction of the imports.
- **Modules.** A module lives in `internal/modules/<name>/` with its Go
  package, `templates/` and `static/`. The core knows
  modules only through the registry. A module's code and its tests use
  the core only through the SDK (`modulekit`, `modulekit/testkit` for
  tests). See [the modules' docs](docs/modules/index.md)
  and [the SDK](docs/modules/sdk.md).
- **Database.** The schema is described by ent schemas
  (`internal/infrastructure/ent/schema`) for both MySQL and PostgreSQL.
  SQL migrations are not written by hand:
  1. Run `go generate ./internal/infrastructure/ent` (for a module's own
     schema, `go generate ./internal/modules/<name>/ent`).
  2. Run `./scripts/migrations_gen.sh <name>`. It writes the next
     migration for both databases and needs Docker.
  3. Changes to data go into a Go step (`migrations.RegisterUpdate`), with
     values passed as parameters and never concatenated into SQL.
- **Errors.** An error is logged once, where it is handled:
  - services and repositories wrap the error with what they were doing
    (`fmt.Errorf("deleting block (id %v): %w", id, err)`) and return it;
  - the handler that chooses the response logs it;
  - goroutines log their own errors.
- **Text.** Interface strings are written in English; after adding or
  changing them, `go run ./cmd/i18n-extract -sources` updates their list
  (`translations/sources/strings.json`). They are translated in the
  translations repository, [kosatka-cms/translations](https://github.com/kosatka-cms/translations),
  not here: a pull request of the engine carries no translation.
  [Languages](docs/guides/languages.md) explains how a site gets them.
- **Comments.** Match the surrounding code: say what a thing is for and
  why, not what each line does. An exported name has a doc comment that
  starts with the name (golangci-lint checks it everywhere). No history
  of how the code used to be, and a name a comment mentions must exist.

## Tests

- Unit tests for `foo.go` go in `foo_test.go`. Template render tests are
  `<feature>_render_test.go`. Database tests are
  `<source>_integration_test.go` with the `dbintegration` tag, and run on
  both databases.
- Theme mechanisms are tested on the fixture theme in
  `testdata/themes/fixture`, not on a real site's theme. If you add
  something a theme can override, add a sample of it to the fixture.
- Don't end a file name with `_<GOOS>` or `_<GOARCH>` (`_js`, `_linux`…).
  Go builds such a file for that platform only, so its tests would
  silently not run.
- Test data uses made-up names and `@example.com` addresses.

## Documentation

The documentation is Markdown in `docs/`, published as a site
(<https://kosatka-cms.github.io/kosatka/>) built with Starlight from
`website/`. When a change alters how something works, update the matching
page in the same pull request. When it adds a new mechanism, describe it
in the page of the docs its users read (a guide, the themes', the
modules' or the contributing pages).

- Each page starts with front matter holding its `title`; the page has no
  `#` heading of its own.
- Link pages by their files, relatively (`[Views](./views.md)`): the links
  work on GitHub and on the site, and a link to a missing file fails the
  build.
- `./scripts/docs.sh` builds the site (`./scripts/docs.sh dev` serves it
  at http://localhost:4321/kosatka/), in Docker when Node.js isn't
  installed.

## Commits and pull requests

- Write commit messages in English: a short summary line, then what
  changed and why.
- Describe the change in the pull request and say how you checked it.
- By contributing, you agree that your contribution is licensed under the
  [MIT License](LICENSE).

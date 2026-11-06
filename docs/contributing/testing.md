---
title: Testing
description: The kinds of tests in the engine, where they go, how to run them, and the checks that cover the whole repository.
sidebar:
  order: 3
---

## Kinds of tests and their files

Each kind of test has its place, so a test is found next to the code it
checks.

| Kind | File | Notes |
| --- | --- | --- |
| Unit tests | `foo_test.go` for `foo.go`, same package | Past about 600 lines, split by theme: `foo_<theme>_test.go` |
| Template and page render tests | `<feature>_render_test.go` | Render through the template service |
| Database integration tests | `<source>_integration_test.go` | Build tag `dbintegration`; run on MySQL and PostgreSQL |
| End-to-end smoke tests | `internal/app/*_smoke_integration_test.go` | Install a real site on a fresh database and request its pages over HTTP |

Some test files have no source file to pair with:

- `main_test.go` holds only `TestMain`.
- Helpers: `helpers_test.go`, `helpers_integration_test.go`,
  `db_helper_integration_test.go`.
- Checks of the whole repository (see below), such as
  `completeness_test.go`, `portability_test.go`, `index_names_test.go`,
  `internal/domain/layers_test.go` and `internal/modules/css_test.go`.

The smoke tests (`serveSite` in `internal/app/helpers_integration_test.go`)
install every migration set on a fresh database, create a site as
`app install` does, start the application on a test HTTP server and use
it as a visitor or an administrator would: sign in, open admin forms,
create content, follow links.

Test data uses made-up names and `@example.com` addresses.

## Running the tests

```bash
go test ./...                 # unit and render tests
./scripts/check.sh            # golangci-lint (gofmt, go vet and the rest), the
                              # unit tests, govulncheck and gitleaks (when
                              # installed), the example module and site
./scripts/check.sh -race      # the unit tests with the race detector, as CI runs them
./scripts/check.sh -db        # plus the database tests and the migration check
./scripts/test_db.sh          # every integration test, the example module's too
./scripts/test_db.sh ./internal/infrastructure/sqldialect/...   # one package
```

`go test ./...` doesn't build the database tests: they need the
`dbintegration` tag. `./scripts/test_db.sh` starts MySQL and PostgreSQL
in Docker (`docker-compose.test.yml`), runs
`go test -tags dbintegration` on the packages given (all of them by
default, and then `examples/hello`'s), and stops the servers. Extra
flags for `go test` go in `GOTESTFLAGS`, for example
`GOTESTFLAGS="-run TestAdminFormsResave"`.

The integration tests find their servers through two variables:

- `KOSATKA_TEST_MYSQL_DSN`, for example
  `root:test@tcp(127.0.0.1:33306)/?parseTime=true`;
- `KOSATKA_TEST_POSTGRES_DSN`, for example
  `postgres://postgres:test@127.0.0.1:35432/postgres?sslmode=disable`.

The file storages ([`kosatka-storage-s3`](https://github.com/kosatka-cms/kosatka-storage-s3),
[`kosatka-storage-azure`](https://github.com/kosatka-cms/kosatka-storage-azure))
are tested in their own repositories, on an S3-compatible storage and
Azure Blob Storage's emulator; a storage of your own can run the same
checks every store passes (`pkg/filestore/filestoretest`).

The DSN needs the right to create databases. To run the tests against
servers of your own, set the variables and run
`go test -tags dbintegration ./...`. A server without its variable is
skipped.

A database test runs through `testdb.Each` (`internal/testdb`), which
runs it once per configured server, as the subtests `mysql` and
`postgres`, each on a fresh, empty database that is dropped afterwards:

```go
//go:build dbintegration

func TestSomething(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		// db.DB is a *sql.DB, db.Dialect its SQL dialect,
		// db.Conn its address for a test that runs the app on it.
	})
}
```

## The fixture theme

The engine's tests of theme mechanisms render a theme of their own,
`testdata/themes/fixture`, not the theme of a real site. It holds one
sample of everything a theme may declare or override: regions, a
favicon, a stylesheet, `theme_head`, a layout, a header with a region's
menu block, a block placeholder, teasers, a field template, a View
template and a content page. The `service` package's tests switch to it
with `zzUseFixtureTheme`, a module's with `testkit.UseTheme` (as
`internal/modules/error_reports` does). It isn't a theme for sites.

When a change lets themes override something new, add a sample of it to
the fixture and a test that renders it.

## Checks of the whole repository

Some tests check the repository as a whole rather than one function.
When one fails, the change broke a rule of the engine; the test's
message says what to fix.

| Check | Test | What it guards |
| --- | --- | --- |
| Layer direction | `TestLayerImports`, `internal/domain/layers_test.go` | `entity` imports only `pkg/`; `service`, the Views and the themes never import the infrastructure or the config; the packages below `service` (Views, themes, files, assets, image URLs, fallbacks, `audit`) never import `service`; `accounts` and `sociallogin` sit above it and may import it |
| Modules on the SDK | `TestModulesUseSDKOnly`, `internal/modules/sdk_test.go` | Every engine module (each directory with a `wire.go`) imports of the engine only its own packages, `modulekit` and `pkg/*`, in its tests too (its generated ent code aside) |
| The SDK's API | `TestAPI`, `modulekit/api_test.go` | The SDK's exported API is what `modulekit/testdata/api.txt` lists; a change of it is made on purpose: `go test ./modulekit -run TestAPI -update`, and the diff reviewed |
| Module CSS | `TestModuleCSSDoesNotRestyleBootstrap`, `internal/modules/css_test.go` | A module's stylesheet styles Bootstrap classes only under its own markup, never globally |
| Source strings listed | `TestSourcesUpToDate`, `internal/i18nextract/completeness_test.go` | `translations/sources/strings.json`, the list of the interface's strings built into the binary (the admin's *Interface translations*), is what the code has: after adding or changing strings, `go run ./cmd/i18n-extract -sources`. A theme the list has no strings of (a site's own in the tree it builds, one installed from its repository) is left out: its strings come from its catalogs |
| Errors logged once | `TestReturnedErrorsAreNotLogged`, `internal/domain/service/ent_not_found_test.go` | Services, Views, themes, assets, repositories and module code outside the handlers don't log an error they return |
| Portable SQL | `TestNoNewMySQLOnlySQL`, `internal/infrastructure/sqldialect/portability_test.go` | Go code uses no MySQL-only SQL (`ON DUPLICATE KEY`, `RAND()`, `GROUP_CONCAT`...); the `sqldialect` helpers replace it |
| Index names | `TestIndexNamesUniqueAcrossTables`, `internal/infrastructure/ent/schema/index_names_test.go` | An explicit index name is used once: PostgreSQL scopes index names to the schema |
| Field types handled | `TestFieldTypesSnapshot` (`entity`), `TestFieldTypesHaveValueTables` and `TestFieldValuesForNodes` (`ent/database`), `TestFieldTypesDecidedInViews` (`views`), `TestFieldTypesInRevisions` (`service`), `TestFieldTypesChecked` (`form`), `TestFieldTypesInForm`, `TestFieldTypesOnPage` (`interfaces`), `TestFieldTypesHaveSettings` (`admin`) | Every field type of `entity.FieldTypes` is described in each layer's table (`entity` - `fieldTypeDefs`, Views - `viewFieldTypes`, the history - `revisionFieldTypes`, the form - `formFieldTypes` and `form.fieldChecks`, the page - `pageFieldTypes`, the field's settings - `adminFieldTypes`), is kept by a revision, has its storage (`registerFieldStorage`) and is written and read back, is decided for Views (filtered, shown - or a reasoned exception), has its input and value in the content form and shows on the content page. A new field type goes into `entity.FieldTypes`; these tests then name every place still to handle it |

`go run ./cmd/i18n-extract -lang <lang>` lists the strings still
missing a translation into a language.

## File names: the platform suffix

Never end a file name with `_<GOOS>` or `_<GOARCH>`, such as `_js`,
`_linux`, `_windows` or `_arm`, before `.go` or `_test.go`. Go builds
such a file only on that platform, so its tests silently don't run
anywhere else: a file named `extract_js_test.go` would be built only for
`GOOS=js`. Name it differently, for example `extract_scripts_test.go`.

## Module tests

A module's tests use the SDK only, as a module from its own repository
can: `modulekit/testkit` renders the engine's templates with the
module's, fakes the core's services for handler tests, and, with the
`dbintegration` tag, gives a database with every set's schema
(`testkit.SchemaDB`) and the core's repositories and services on it
(`testkit.Deps(db)`). The engine's modules test this way too.

`examples/hello` is a module in its own Go module (its own `go.mod`), on
the module SDK, so the root's `go test ./...` doesn't reach it.
`./scripts/check.sh` vets and tests it and builds the example site
`examples/hello-site`. Its database tests use `modulekit/testkit`
(`testkit.SchemaDB`, the `dbintegration` tag) and run with
`./scripts/test_db.sh` when no package is given. How a module tests
itself is in [Write a module](../modules/write-a-module.md) and
[the module SDK](../modules/sdk.md).

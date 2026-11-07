---
title: Database and migrations
description: The ent schemas, generated migrations for MySQL and PostgreSQL, Go data steps, and testing on both databases.
sidebar:
  order: 2
---

The engine runs on MySQL or PostgreSQL. The schema is described once, by
ent schemas; the SQL migrations for both databases are generated from them
and never written by hand.

## Two databases

`db.driver` / `DB_DRIVER` picks the server: `mysql` (the default) or
`postgres` ([configuration](../reference/configuration.md)). The same
binary, schema and migrations serve both; each migration file exists once
per database. The tests run on MySQL 8.4 and PostgreSQL 16
(`docker-compose.test.yml` and the test workflow).

## The schema

| Where | What |
|---|---|
| `internal/infrastructure/ent/schema` | The core's ent schemas. (A module's tables could live here too, named in its `entity.Module.Tables`; the engine's modules each keep their own schema.) |
| `internal/infrastructure/ent/generated` | The generated client. Never edited, except the hand-written `not_found_entity.go` that makes ent's not-found error match `entity.ErrEntityNotFound`. |
| `internal/modules/<name>/ent` | A module's own schema: `schema/`, `generated/`, `migrations/<dialect>/`. |
| `internal/infrastructure/migrations/files/<dialect>/<set>/` | The core's migration files (`<set>` is `core`), and those of a module whose tables are in the core schema. |

`<dialect>` is `mysql` or `postgres`. Each directory holds
`<version>_<name>.sql` files, where the version is a timestamp, plus
`atlas.sum`, their checksums: a file edited, added or removed by hand
makes `app migrate` fail. The files are embedded in the binary. Each of
the engine's sets starts from one baseline file (`<version>_baseline.sql`)
generated from its ent schema; later changes add files after it.

A module's own schema can't refer to core types, so a core table its
tables reference is described there by a stand-in: a schema with the core
table's name (`schemakit.Table("nodes")`) and only its `id`. The stand-in
gives the foreign key; it is not part of the module's set, so it is never
created or dropped with the module, and the key points at the core's real
table. The module reads core rows through the core's repositories, not
through the stand-in. The module author's side of this is in
[writing a module](../modules/write-a-module.md).

Every foreign key column gets an index (`index.Fields("author_id")` in
the schema's `Indexes`). MySQL makes one for a foreign key itself,
PostgreSQL doesn't: without it, deleting the referenced row and listing
the rows that refer to it scan the whole table. `TestApplyAllSets` fails
on PostgreSQL when a foreign key has none.

## Rows every query leaves out

The `Node` schema's interceptor (`schema/node_blocks.go`) adds its
conditions to every node query - lists, search, counts, feeds and edge
queries alike - so a new query has none to forget. It leaves out:

- A blocks field's items: nodes (`parent_id` set to the node holding
  them), but no content of their own. Code that works on the items
  themselves (reading and saving them, or every node of a type: a field
  set's nodes are items) runs its query with `schema.WithBlocks(ctx)`.
- The content in the trash (`deleted_at` set). The trash's own pages,
  restoring and deleting for good run with `entity.WithTrashed(ctx)`.
- What the request's visitor may not view: the types closed to them and
  the items restricted to other roles (their own aside), from the
  context's `entity.ContentVisitor` (`ContentAccessMiddleware` sets it).
  A context without one, a background job's, is hidden nothing.
  `entity.WithAllNodeTypes(ctx)` sees all the content, the trash
  included, for the site's own lookups (a path already taken, a menu
  link's address).

The `Comment` schema's interceptor leaves out the comments on the
content in the trash or hidden from the visitor. A subquery of another
entity's query (`HasNodeWith`) isn't a node query: it adds
`schema.LiveNodes(ctx)` (those two conditions) itself.

The schema uses the generated `intercept` package, so ent puts the
schema's runtime (defaults, hooks, interceptors) in
`internal/infrastructure/ent/generated/runtime`. The `database` and
`migrations` packages import it blank; a package that builds a core
client without them must import it too, or its queries fail with
"uninitialized interceptor".

## Changing the schema

1. Change the ent schema.
2. Regenerate the client:

   ```bash
   go generate ./internal/infrastructure/ent
   # or, for a module with its own schema
   go generate ./internal/modules/<name>/ent
   ```

3. Write the migration (needs Docker: it starts MySQL and PostgreSQL from
   `docker-compose.test.yml`):

   ```bash
   ./scripts/migrations_gen.sh add_feed_language
   ```

   It writes the next migration of every set whose tables changed, for
   both databases: the core's into
   `internal/infrastructure/migrations/files/`, a module's own schema into
   its `ent/migrations/`. A column, an index or a table taken out of the
   ent schemas is dropped by it too (a unique key that gains a column is
   a new index in place of the old one; see
   [Removing a column or a table](#removing-a-column-or-a-table)).
4. If the rows sites already have need new values, add a
   [data step](#data-steps).
5. Commit the schema, the generated code and the migration files together.

The check that the files keep up with the ent schemas:

```bash
./scripts/migrations_gen.sh -check   # fails if the files are behind ent
./scripts/check.sh -db               # runs it, with the database tests
```

CI runs the same check.

## Migration sets

A set is the core or one module. The tables of a module are its own set -
its `entity.Module.Tables` in the core schema, or the tables of its own
schema (registered by its manifest with `manifestkit.RegisterSchema`). Each
set has its own sequence of files, so a module's tables are created only
when the module is installed. `migrations.Sets()` returns the core first,
then the modules: a module's tables may reference core tables, never the
other way round.

Applied versions are recorded in `schema_migrations`, per set (a set's
install step as the version `install`). Uninstalling a module on the
Modules page (`service.ModuleUninstall`) drops its set with
`Runner.Uninstall`: the tables, each after those of the set that
reference it, and the set's rows of `schema_migrations`, so enabling the
module again installs it from scratch.

## `app migrate`

```bash
app migrate                    # list what would run
app migrate -apply             # run it
app migrate -apply -additive   # per set, only the steps before its first file that drops or changes something
app migrate -baseline          # record everything as applied, running nothing
```

It covers the core and every module that is enabled or already installed;
a module that is switched off and not installed is skipped. Each set gets
its plan (`Runner.Plan`):

- A set with nothing applied yet: every schema file, then the set's
  install step; its data steps are only recorded (a fresh install has no
  old data to fix). A set whose install step failed gets the rest of
  that plan again: neither the step nor its data steps were recorded.
- Otherwise: the pending schema files and data steps, in version order.
- A set whose recorded versions include ones the binary has neither as
  files nor as steps, while its first file isn't recorded, is of another
  history (set up before the files were squashed into a new baseline):
  `migrations.HistoryError`, nothing runs. Unknown versions after a
  recorded first file are a newer binary's, from a site rolled back: no
  error.

`-additive` runs each plan only up to its first schema file that drops,
truncates, deletes, renames or changes something
(`migrations.AdditivePrefix`; a Go step counts as additive): what the
older binary, still serving the site, is fine with. Run it before
restarting on a new binary, and `-apply` after the restart for the rest.

`-baseline` records, without running them, every step of each set that
has nothing recorded yet: once, for a database whose schema already
matches the files (`scripts/schema_drift.sh` compares a site's MySQL
database with the ent schemas). The status report lists the steps a site
has pending. See [the CLI reference](../reference/cli.md#app-migrate).

## Go steps

Some changes can't be a schema file. They are Go functions registered by
set (`internal/infrastructure/migrations`), and each gets an
`Env`: `DB` (`*sql.DB`), `Dialect`, `Client` (the generated ent client).

| Step | Registered with | Runs |
|---|---|---|
| Data step | `migrations.RegisterUpdate(set, migrations.Update{Version, Description, Run})` | Once, at its `Version` among the set's files; a fresh install only records it. |
| Install step | `migrations.RegisterInstall(set, run)` | Once, after the set's tables are first created. |

Modules register theirs through `modulekit/manifestkit`
(`RegisterInstall`, `RegisterUpdate`, with `InstallEnv`), from their
manifest's `RegisterSteps`. A module's uninstall step isn't a migration
step: it is the `Parts.Uninstall` its `Wire` returns, run when the module
is uninstalled, before its tables are dropped.

### Data steps

A data step moves or fixes data. Its `Version` is a timestamp, later than
the schema files it relies on, and shares the set's sequence with them.
A core data step is a file of its own in
`internal/infrastructure/migrations`, `update_<name>.go`, tested by
`update_<name>_integration_test.go`.

It uses SQL on `env.DB`, not the generated client: the client describes the
schema as it is now, while the step runs on a database whose schema is as
it was when the step was released. A column renamed or dropped later would
break a step written with the client. Values go only as parameters - `?`
placeholders through `env.Dialect.Rebind` (or ent's `dialect/sql` builder),
never concatenated into the query:

```go
// update_robots_extra.go
func init() {
	RegisterUpdate(CoreSet, Update{
		Version:     "20261110120000",
		Description: "robots_extra_default",
		Run: func(ctx context.Context, env Env) error {
			_, err := env.DB.ExecContext(ctx,
				env.Dialect.Rebind(`UPDATE site_information SET robots_extra = ? WHERE robots_extra IS NULL`),
				"")
			if err != nil {
				return fmt.Errorf("setting robots_extra: %w", err)
			}
			return nil
		},
	})
}
```

`app migrate -apply -additive` runs data steps too, while the older
binary still serves the site: a step leaves the data in a shape that
binary still reads.

### Install steps

An install step writes the rows a set can't work without; the core's
(`install_core.go`) writes the single row of each settings table, the
base and administrator roles, the text formats, the avatar image styles
and the date formats. It runs right after the set's tables are created,
when the schema is the current one, so it writes through `env.Client` (a
module with its own schema opens its own client on
`manifestkit.Driver(env)`). It runs from `app install`, from
`app migrate` and when a module is first enabled on the Modules page.

A set's install step is there from its first version: one added to a
set the sites have already would run on them. Rows an existing set needs
later are a data step.

### Removing a column or a table

A column removed from an ent schema is dropped by the generated migration
(`DROP COLUMN`). If its data must be kept, move the data with a data step in
an earlier release, and remove the column in a later one.

A whole ent schema removed works the same way: the generator drops its table
(`DROP TABLE`) in the migration of the set whose files created it. ent only
compares the tables it still has, so the generator looks for the rest itself:
the tables the set's files create that no set's ent schema has any more. The
data goes with the table, so move what is still needed in an earlier release.

## Portable SQL

Code that writes SQL by hand stays portable through
`internal/infrastructure/sqldialect`:

| Helper | Instead of |
|---|---|
| `Dialect.Rebind` | `?` placeholders on PostgreSQL (`$1`, `$2`…) |
| `Dialect.Upsert` | `ON DUPLICATE KEY UPDATE`, `INSERT IGNORE`, `REPLACE INTO` |
| `Dialect.InsertID` | `LastInsertId` (PostgreSQL uses `RETURNING`) |
| `Dialect.Random` | `RAND()` |
| `Dialect.CurrentDate` | `CURDATE()` |
| `Dialect.UnixSeconds` | `UNIX_TIMESTAMP(col)`: a column the database fills itself (`DEFAULT CURRENT_TIMESTAMP`) as Unix seconds, whatever the session's time zone |

`TestNoNewMySQLOnlySQL` (`internal/infrastructure/sqldialect/portability_test.go`)
scans `internal`, `pkg` and `cmd` and fails on MySQL-only constructs
(those above, and `GROUP_CONCAT`) outside the generated code and the
tests.

On MySQL the connection uses `clientFoundRows=true`, so an `UPDATE` reports
the rows it matched, as on PostgreSQL.

## Strict mode on MySQL

The application, and the migrations that run through it, set the MySQL
session's `sql_mode` (`db.sql_mode` / `DB_SQL_MODE`; by default
`STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION`).
A value too long for its column or a missing `NOT NULL` value is an error,
not silently cut or filled. `server` keeps the server's own mode; the
status report warns while the mode isn't strict. The setting isn't used
on PostgreSQL.

## Testing on both databases

Database tests carry the `dbintegration` build tag. `testdb.Each`
(`internal/testdb`) runs a test once per server it has a DSN for
(`KOSATKA_TEST_MYSQL_DSN`, `KOSATKA_TEST_POSTGRES_DSN`), each time on a
fresh, empty database dropped afterwards. `./scripts/test_db.sh` starts
the servers in Docker and runs the tests:

```bash
./scripts/test_db.sh                                    # every integration test
./scripts/test_db.sh ./internal/infrastructure/sqldialect/...
```

How to run part of them or use servers of your own, and a test's
skeleton: [Testing](./testing.md#running-the-tests).

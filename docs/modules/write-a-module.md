---
title: Write a module
description: A step-by-step tutorial building a small module in its own repository with the module SDK.
sidebar:
  order: 2
---

This tutorial builds `hello`, a module with one page (`/hello`) that
lists greetings from its own database table. It is the module in the
repository's
[`examples/hello`](https://github.com/kosatka-cms/kosatka/tree/main/examples/hello);
the code below is taken from there, except the install step and the
demo content (steps 11 and 12), which the example doesn't have. At the
end, a site builds its own binary with the module.

A module from its own repository may import only the engine's public
packages: the module SDK (`github.com/kosatka-cms/kosatka/modulekit` and
its subpackages) and the utilities under `pkg/`. Go doesn't let it import
the engine's `internal/` packages. [The module SDK](./sdk.md) describes
what the SDK has.

You need Go (the version in the engine's `go.mod`), Docker or a local
MySQL and PostgreSQL to generate migrations and run the database tests,
and the engine's `web/` and `themes/` (a checkout of its repository) to
run a site.

## What you will build

```text
kosatka-hello/
├── go.mod
├── hello.go                 init(): registers the module
├── manifest.go              the module's description and its steps
├── wire.go                  its repository, controller and route
├── controller.go            the /hello page
├── repository.go            the greetings, through its own ent client
├── install.go               its install step (step 11)
├── demo.go, demo/en.json    its demo content (step 12)
├── repository_integration_test.go   its database test (step 13)
├── templates/hello/page.html
├── templates/blocks/hello_latest.html   its block (step 15)
├── static/hello.css
├── translations/ru.json
├── ent/
│   ├── ent.go               the schema's registration
│   ├── schema/greeting.go   the table
│   ├── generated/           the generated ent client, and one file of yours
│   └── migrations/          mysql/ and postgres/, generated
└── cmd/migrations/main.go   writes the next migration
```

## 1. Create the Go module

```bash
mkdir kosatka-hello && cd kosatka-hello
go mod init example.com/kosatka-hello
go get github.com/kosatka-cms/kosatka@latest
go get entgo.io/ent@v0.14.6
```

Use the version of `entgo.io/ent` the engine's `go.mod` requires. While
you work on the module next to a checkout of the engine, you can point
the module at it instead of a release, as the example does, with a line
in `go.mod`:

```text
replace github.com/kosatka-cms/kosatka => ../kosatka
```

## 2. Define the table

A module keeps its tables in its own [ent](https://entgo.io) schema.
`ent/schema/greeting.go`:

```go
// Package schema - the hello module's own ent schema.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// Greeting - a greeting on the hello page.
type Greeting struct {
	ent.Schema
}

// Annotations of the Greeting.
func (Greeting) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("hello_greetings")}
}

// Fields of the Greeting.
func (Greeting) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int")),
		field.String("text").MaxLen(255),
		field.Time("created_at").SchemaType(schemakit.MySQLType("datetime")).Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}
```

`schemakit` holds the conventions the engine's own schema follows:

- `schemakit.Table(name)` names the table and, on MySQL, gives it the
  site databases' `utf8mb4_unicode_ci` collation (case-insensitive
  comparison). Start table names with the module's name, so they don't
  clash with the core's or another module's.
- `schemakit.MySQLType(t)` sets a column type on MySQL only; PostgreSQL
  takes ent's default for the field.

The schema works on both databases the engine supports, MySQL and
PostgreSQL.

## 3. Generate the ent client

`ent/ent.go` generates the client into `ent/generated`, embeds the
migration files and describes the schema for the engine:

```go
// Package ent - the hello module's own ent schema (schema/), its generated
// client (generated/, go generate) and migration files
// (migrations/<dialect>/, written by cmd/migrations).
package ent

import (
	"embed"
	"io/fs"

	"example.com/kosatka-hello/ent/generated/migrate"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
)

//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate ./schema --target generated

//go:embed migrations
var files embed.FS

// Schema - the module's tables and migration files (Dir: where the files
// are, from the module's root - cmd/migrations runs there).
var Schema = manifestkit.Schema{Tables: migrate.Tables, Files: migrationFiles(), Dir: "ent/migrations"}

// migrationFiles - the embedded migrations/ directory as the root.
func migrationFiles() fs.FS {
	sub, err := fs.Sub(files, "migrations")
	if err != nil {
		panic(err) // a constant, valid path
	}
	return sub
}
```

`//go:embed` fails on an empty directory, and the migrations only appear
in step 10, so give `ent/migrations` a file now, then generate the
client:

```bash
mkdir -p ent/migrations
echo "Generated by go run ./cmd/migrations; never edited by hand." > ent/migrations/README.md
go generate ./ent
```

Then add one hand-written file next to the generated code,
`ent/generated/not_found_entity.go`. It makes ent's "not found" error
match the engine's, so a page that loads a missing record answers 404
instead of 500:

```go
package generated

// Hand-written, not generated: ent's codegen leaves files without its
// "Code generated" header alone.

import "github.com/kosatka-cms/kosatka/modulekit/schemakit"

// Is makes errors.Is(err, schemakit.ErrEntityNotFound) true for ent's
// not-found errors, so the engine recognizes the module's missing record
// as it does the core's (its handlers answer 404).
func (e *NotFoundError) Is(target error) bool {
	return target == schemakit.ErrEntityNotFound
}
```

Run `go generate ./ent` again after every change to `ent/schema`.

## 4. Describe the module

The manifest says what the module owns. The engine reads it to build the
Modules page, to create and drop the module's tables, and to hide its
parts while it is disabled. `manifest.go`:

```go
package hello

import (
	"example.com/kosatka-hello/ent"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Name - the module's machine name.
const Name = "hello"

// Module - the module's registry entry.
func Module() manifestkit.Module {
	return manifestkit.Module{
		Name:        Name,
		Label:       i18n.N("Hello"),
		Description: i18n.N("An example module from outside the engine: the greetings page (/hello)."),
		Category:    manifestkit.CategoryContent,
		Tables:      []string{"hello_greetings"},
		Data:        []manifestkit.ModuleData{{Label: i18n.N("Greetings"), Table: "hello_greetings"}},
		CSS:         []string{"hello"},
	}
}

// RegisterSteps registers the module's own ent schema: its tables and
// migration files.
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
}
```

- `Name` is the machine name. It must be unique among the site's
  modules (a clash stops the binary at startup). The module's static
  files are served under `/static/modules/<name>/`.
- `Label` and `Description` show on the Modules page. `i18n.N` marks a
  string for translation without translating it (step 7).
- `Tables` lists every table the module owns: they are created when the
  module is installed and dropped when it is uninstalled.
- `Data` - the tables whose rows the Modules page counts (*What the
  module includes*); each must be in `Tables`.
- `CSS` - the stylesheets in `static/`, without the extension (`JS` is
  the same for scripts).

A manifest can declare much more: content types, blocks, permissions,
admin pages, required modules. One flag matters for a module that sites
add later: without `OffByDefault`, a site that already exists has the
module on as soon as its binary has it. See
[The manifest](./sdk.md#the-manifest).

`RegisterSteps` registers the module's steps: here only its schema, as
the set named after the module. Steps 11 and 12 add an install step and a
demo step to it.

## 5. Read and write the data

`repository.go` opens the module's own ent client for each call on
`modulekit.Driver(ctx, db)`: the site's database in its dialect, or the
core's transaction when `ctx` carries one - a write of the module's then
goes with the core's change, a hook's too:

```go
package hello

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"example.com/kosatka-hello/ent/generated"
	"example.com/kosatka-hello/ent/generated/greeting"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// Greeting - one greeting.
type Greeting struct {
	ID        int
	Text      string
	CreatedAt time.Time
}

// Repository - the greetings, on the module's own ent schema (ent/).
type Repository struct {
	db *sql.DB
}

// NewRepository - the greetings on db, in the site's dialect.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// client - the module's ent client: the context's transaction (InTx - a
// hook running in the core's change, say), else the database.
func (r *Repository) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, r.db)))
}

// Add stores a greeting.
func (r *Repository) Add(ctx context.Context, text string) (Greeting, error) {
	row, err := r.client(ctx).Greeting.Create().SetText(text).SetCreatedAt(time.Now()).Save(ctx)
	if err != nil {
		return Greeting{}, fmt.Errorf("adding a greeting: %w", err)
	}
	return Greeting{ID: row.ID, Text: row.Text, CreatedAt: row.CreatedAt}, nil
}

// List - the greetings, newest first.
func (r *Repository) List(ctx context.Context) ([]Greeting, error) {
	rows, err := r.client(ctx).Greeting.Query().Order(generated.Desc(greeting.FieldCreatedAt), generated.Desc(greeting.FieldID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing greetings: %w", err)
	}
	out := make([]Greeting, 0, len(rows))
	for _, row := range rows {
		out = append(out, Greeting{ID: row.ID, Text: row.Text, CreatedAt: row.CreatedAt})
	}
	return out, nil
}
```

Repositories don't log their errors: they wrap them with what they were
doing and return them. The handler that answers the request logs them
once.

To write its rows and the core's in one transaction, the module runs
both inside `modulekit.InTx` (see [The module SDK](./sdk.md#transactions-with-the-core)).

## 6. The page

`controller.go`. A module's controller embeds the core's
`modulekit.BaseController`, which renders templates and answers errors
the way the core's pages do:

```go
package hello

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Controller - the greetings page.
type Controller struct {
	*modulekit.BaseController
	greetings *Repository
}

// Page handles GET /hello: the greetings, newest first.
func (c *Controller) Page(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := c.DBContext(r)
	defer cancel()
	list, err := c.greetings.List(ctx)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error listing greetings")
		return
	}
	title := i18n.T(r.Context(), "Hello")
	data := c.GetCommonData(r)
	data["title"] = title
	data["greetings"] = list
	data["pageSEO"] = modulekit.PageSEOForList(title, r.URL.Path)
	c.RenderOrFail(w, r, "hello/page.html", data)
}
```

- `DBContext` - the request's context, bounded by the database query
  timeout.
- `GetCommonData` - the data the site's layout needs on every page (the
  signed-in user, the sidebar, the admin toolbar).
- `RenderLoadFailure` - a missing record is a 404 (logged at debug
  level), any other error a 500 (logged as an error).
- `PageSEOForList` - the page's title and meta tags, with its canonical
  address.
- `RenderOrFail` - renders the template; a failure is logged and
  answers 500.

The template, `templates/hello/page.html` (`layout.html` puts it in the
site's layout):

```html
{{template "layout.html" .}}

{{define "title"}}<h1>{{.title}}</h1>{{end}}

{{define "content"}}{{attach "hello"}}
<section class="hello-greetings">
    <p class="hello-lead">{{t "Greetings from a module of its own repository."}}</p>
    {{if .greetings}}
    <ul class="hello-list">
        {{range .greetings}}<li>{{.Text}}</li>{{end}}
    </ul>
    {{else}}
    <p class="hello-empty">{{t "No greetings yet."}}</p>
    {{end}}
</section>
{{end}}
```

Keep the module's templates in a directory named after the module
(`templates/hello/`): template paths are shared by the engine and all
modules, and a theme can replace a module's template by putting its own
at the same path. `{{attach "hello"}}` tells the page to load the
module's CSS and JavaScript; a page that doesn't show anything of the
module doesn't load them.

## 7. Styles and translations

`static/hello.css`:

```css
/* The hello module's page. */
:where(.hello-greetings) .hello-list { padding-left: 1.25rem; }
:where(.hello-greetings) .hello-empty { opacity: .7; }
```

The page links `/static/modules/hello/hello.min.css`; a module may ship
only `hello.css`, which is served in its place. Keep the module's rules
low in specificity (`:where()`), so a theme can restyle them.

The interface strings are written in English in the code and the
templates. Other languages come from `translations/<lang>.json`, for
example `translations/ru.json`:

```json
{
  "contexts": {},
  "strings": {
    "An example module from outside the engine: the greetings page (/hello).": "Пример модуля не из движка: страница приветствий (/hello).",
    "Greetings": "Приветствия",
    "Greetings from a module of its own repository.": "Приветствия от модуля из своего репозитория.",
    "Hello": "Привет",
    "No greetings yet.": "Приветствий пока нет."
  }
}
```

In Go, `i18n.T(ctx, "…")` translates for the request's language; in
templates, `{{t "…"}}`.

## 8. Wire it

`Wire` is called once at startup. It gets the core's services
(`modulekit.Deps`), builds the module's own, and returns what the module
adds (`modulekit.Parts`). `wire.go`:

```go
package hello

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// Wire - the module's repository, controller and route (modulekit).
func Wire(d modulekit.Deps) modulekit.Parts {
	c := &Controller{BaseController: modulekit.NewBaseController(d.Base), greetings: NewRepository(d.DB)}
	return modulekit.Parts{Routes: func(r modulekit.Routes) {
		r.Mux.Handle("GET /hello", http.HandlerFunc(c.Page))
	}}
}
```

The routes use Go's `net/http` patterns. Their mux is gated by the
module's state: while the module is disabled they answer 404. Besides
`Routes`, `Parts` can carry the pages of the module's content types, a
handler for paths nothing else matched, an uninstall step, status
report checks and a user's content to delete with the account; `Wire`
is also where the module registers its hooks (blocks, parts of content
pages, the *Create* menu…). See
[The module SDK](./sdk.md#what-wire-returns).

## 9. Register the module

`hello.go` registers the module when its package is imported. A site
imports it, and that is all it takes:

```go
// Package hello - an example of a module from outside the engine.
package hello

import (
	"embed"
	"io/fs"

	"github.com/kosatka-cms/kosatka/modulekit"
)

//go:embed templates static
var assets embed.FS

//go:embed translations/*.json
var translations embed.FS

func init() {
	catalog, err := fs.Sub(translations, "translations")
	if err != nil {
		panic(err) // a constant, valid path
	}
	modulekit.Register(modulekit.Registration{
		Module:  Module(),
		Steps:   RegisterSteps,
		Wire:    Wire,
		Catalog: catalog,
		Assets:  assets,
	})
}
```

- `Module` - the description from step 4.
- `Steps` - `RegisterSteps`; the engine calls it after its own modules'.
- `Wire` - step 8.
- `Catalog` - a file system with `<lang>.json` at its root.
- `Assets` - a file system with `templates/` and `static/` at its root.

The templates and static files are embedded: the module works from the
site's binary alone, with no files of its own on the server.

The module builds now:

```bash
go mod tidy
go build ./...
```

## 10. Generate the migrations

SQL migrations are never written by hand: the engine's generator writes
them from the ent schema, for MySQL and PostgreSQL. The module calls it
from a small command of its own, `cmd/migrations/main.go`:

```go
// Command migrations writes the hello module's next migration
// (kosatka.GenerateMigrations), run from the module's root with the two
// test databases running:
//
//	KOSATKA_TEST_MYSQL_DSN=… KOSATKA_TEST_POSTGRES_DSN=… go run ./cmd/migrations -name <name>
//	… go run ./cmd/migrations -check
package main

import (
	"github.com/kosatka-cms/kosatka"

	_ "example.com/kosatka-hello"
)

func main() { kosatka.GenerateMigrations() }
```

The generator needs a MySQL and a PostgreSQL server to compute the
changes on. For example, in Docker:

```bash
docker run -d --name hello-mysql -e MYSQL_ROOT_PASSWORD=test -p 33306:3306 \
  mysql:8.4 --character-set-server=utf8mb4 --collation-server=utf8mb4_unicode_ci
docker run -d --name hello-postgres -e POSTGRES_PASSWORD=test -p 35432:5432 postgres:16

export KOSATKA_TEST_MYSQL_DSN="root:test@tcp(127.0.0.1:33306)/?parseTime=true"
export KOSATKA_TEST_POSTGRES_DSN="postgres://postgres:test@127.0.0.1:35432/postgres?sslmode=disable"
```

Then, from the module's root (MySQL needs a few seconds to start; until
then the generator can't connect):

```bash
go run ./cmd/migrations -name greetings
```

It writes the next migration of each set whose tables changed, here
`ent/migrations/mysql/<timestamp>_greetings.sql` and
`ent/migrations/postgres/<timestamp>_greetings.sql` (with their
`atlas.sum`), and prints `mysql/hello: new migration "greetings"` and
the same for `postgres`. The engine's own migrations are already in its
binary and are not written into the module's tree.
`go run ./cmd/migrations -check` writes nothing and fails when the files
are behind the schema: run it in the module's CI.

The migration files are embedded (`ent/ent.go`), so a new version of the
module brings its database changes with it, and the site's `app migrate`
applies them.

## 11. Add an install step

The example has no install step; this section adds one. An install step
runs once, when the module is installed (on `app install`, the first
time it is enabled on the Modules page, or by `app migrate -apply` for a
module that is on but not installed yet), after its tables are created.
Use it for starter rows. Create `install.go`:

```go
package hello

import (
	"context"
	"fmt"
	"time"

	"example.com/kosatka-hello/ent/generated"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// install - the first greeting, in English (LabelColumns: it is put in the
// site's language).
func install(ctx context.Context, env manifestkit.InstallEnv) error {
	c := generated.NewClient(generated.Driver(manifestkit.Driver(env)))
	if err := c.Greeting.Create().SetText(i18n.N("Welcome!")).SetCreatedAt(time.Now()).Exec(ctx); err != nil {
		return fmt.Errorf("the first greeting: %w", err)
	}
	return nil
}
```

Register it in `RegisterSteps`, and name the column of labels it writes
in the manifest, so they are translated into the site's language (add
`"Welcome!"` to `translations/ru.json` too):

```go
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
	manifestkit.RegisterInstall(Name, install)
}
```

```go
// in Module():
LabelColumns: []manifestkit.ModuleLabelColumn{{Table: "hello_greetings", Column: "text"}},
```

`manifestkit.Driver(env)` opens the module's client on the step's
database. An install step that fails isn't recorded: the next
`app migrate -apply` runs it again. Give a module its install step
before its first release. Rows a later version adds go both in the
install step, for new sites, and in a data step, for the sites that
have the module already (see
[Changing the module later](#changing-the-module-later)).

## 12. Add demo content

`app install -demo` fills a new site with demo content, and then runs the
demo step of each installed module. This section adds one to `hello`.
The texts come from `demo/<lang>.json` (`demo/en.json` for a language
without its own file), for example `demo/en.json`:

```json
{
  "menu": "Greetings",
  "greetings": ["Hello from the demo", "Good morning"]
}
```

and `demo.go`:

```go
package hello

import (
	"context"
	"embed"
	"fmt"

	"example.com/kosatka-hello/ent/generated"
	"github.com/kosatka-cms/kosatka/modulekit/demokit"
)

//go:embed demo/*.json
var demoFS embed.FS

// demoContent - demo/<lang>.json.
type demoContent struct {
	Menu      string   `json:"menu"`
	Greetings []string `json:"greetings"`
}

// createDemo adds the demo greetings and a link to /hello in the footer
// menu.
func createDemo(ctx context.Context, env demokit.Env) error {
	var content demoContent
	if err := demokit.Content(env, demoFS, &content); err != nil {
		return err
	}
	c := generated.NewClient(generated.Driver(demokit.Driver(env)))
	for _, text := range content.Greetings {
		if err := c.Greeting.Create().SetText(text).SetCreatedAt(env.Now).Exec(ctx); err != nil {
			return fmt.Errorf("greeting %q: %w", text, err)
		}
	}
	return demokit.MenuLink(ctx, env, env.FooterMenu, content.Menu, "/hello", 0, 30)
}
```

`demokit.Content` reads `demo/<lang>.json`, or, on a site installed with
a demo set that has content for the module (`app install -demo-set`), the
set's, in the same format ([Demo sets](../contributing/demo-sets.md)).

Register it in `RegisterSteps` (`manifest.go` imports
`github.com/kosatka-cms/kosatka/modulekit/demokit` for it):

```go
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
	manifestkit.RegisterInstall(Name, install)
	demokit.Register(Name, createDemo)
}
```

`demokit.Driver(env)` gives the module's client the install transaction:
its rows go in together with the core's demo content, and nothing stays
if the install fails. `demokit.Env` also has the site's language, the
administrator, the demo users and articles, the theme and the menus;
`demokit` can also create content items, comments, pictures,
vocabularies and terms, and place blocks and block Views (see
[Demo content](./sdk.md#demo-content)).

## 13. Test it

Tests that need a database use `modulekit/testkit`. Like the engine's
own database tests, they build with the `dbintegration` tag. The
module's schema is installed with its install step, so the table starts
with the greeting of step 11: the test checks the two newest.
`repository_integration_test.go`:

```go
//go:build dbintegration

package hello

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestRepository - greetings are stored in the module's own table and
// listed newest first.
func TestRepository(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		greetings := NewRepository(db.DB)
		for _, text := range []string{"Hi", "Hello there"} {
			if _, err := greetings.Add(ctx, text); err != nil {
				t.Fatal(err)
			}
		}
		list, err := greetings.List(ctx)
		if err != nil || len(list) < 2 || list[0].Text != "Hello there" || list[1].Text != "Hi" {
			t.Errorf("greetings: %+v %v", list, err)
		}
	})
}
```

`testkit.SchemaDB` runs the test on a fresh database of each server
named by `KOSATKA_TEST_MYSQL_DSN` and `KOSATKA_TEST_POSTGRES_DSN` (step
10), with the schema installed: the core's, the engine's modules' and
the module's own. With neither variable set, the test is skipped. A
repository that needs the core's (content, users, files) takes them
from `testkit.Deps(db)`, as `Wire` takes them from `Deps`.

`testkit` also renders the module's templates with the engine's
(`testkit.UseEngineRoot`, which also serves the registered module's
`assets`, then `testkit.RenderPageContent`) and gives handler
tests fakes of the core's services and a session with a CSRF token - see
[Tests in the SDK reference](./sdk.md#tests).

```bash
go test -tags dbintegration ./...
```

## 14. Build a site with it

Publish the module (tag a release, for example `v0.1.0`). A site that
uses it builds its own binary, from a main package of its own that
calls `kosatka.Main()` (step 1 of
[Add a module from its own repository](./using-modules.md#add-a-module-from-its-own-repository)
creates one). In that package:

```bash
go run . module add example.com/kosatka-hello@v0.1.0
go build -o kosatka .
```

Run the binary from a directory with the engine's `web/` and `themes/`.
On a site that already exists, `hello` is on as soon as the binary has
it (its manifest doesn't set `OffByDefault`), and `app migrate -apply`
installs it; a new site gets it with the standard modules too when
`-modules` lists them all:
`app install … -modules action_log,forms,not_found_log,private_messages,subscriptions,video_embed,hello`.
Then open `/hello`. The full steps, the deployment too, are in
[Using modules](./using-modules.md#add-a-module-from-its-own-repository).

To try the example without publishing anything, the repository's
`examples/hello-site` uses the module from its directory (its `go.mod`
replaces the engine and the module with the repository's own):

```bash
cd examples/hello-site
go run . module list    # the engine's modules, then hello
go build -o site .
```

Run `site` from the repository's root, which has `web/` and `themes/`.

## 15. Add a block and a status line

The example also has a block an administrator places on the *Blocks*
page - the latest greetings, as many as its setting says - and a line of
its own on the status report.

Declare the block in the manifest:

```go
// PluginBlockLatest - the latest greetings, a block placed on the Blocks
// page (templates/blocks/hello_latest.html).
const PluginBlockLatest manifestkit.PluginBlockID = "hello_latest"

// in Module():
PluginBlocks: []manifestkit.ModulePluginBlock{{ID: PluginBlockLatest, Label: i18n.N("Latest greetings")}},
```

The repository gets the two queries they need, the newest greetings and
their count; `List` becomes `Latest(ctx, 0)`:

```go
// List - the greetings, newest first.
func (r *Repository) List(ctx context.Context) ([]Greeting, error) {
	return r.Latest(ctx, 0)
}

// Latest - the newest limit greetings (0 - all).
func (r *Repository) Latest(ctx context.Context, limit int) ([]Greeting, error) {
	q := r.client(ctx).Greeting.Query().Order(generated.Desc(greeting.FieldCreatedAt), generated.Desc(greeting.FieldID))
	if limit > 0 {
		q = q.Limit(limit)
	}
	rows, err := q.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing greetings: %w", err)
	}
	out := make([]Greeting, 0, len(rows))
	for _, row := range rows {
		out = append(out, Greeting{ID: row.ID, Text: row.Text, CreatedAt: row.CreatedAt})
	}
	return out, nil
}

// Count - how many greetings there are.
func (r *Repository) Count(ctx context.Context) (int, error) {
	n, err := r.client(ctx).Greeting.Query().Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting greetings: %w", err)
	}
	return n, nil
}
```

Give the block a setting and its data in `Wire`, and the status line in
its `Parts`. `wire.go` becomes:

```go
package hello

import (
	"context"
	"net/http"
	"strconv"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// latestCount - the latest greetings block's setting: how many it shows.
var latestCount = modulekit.PluginBlockSetting{
	Key: "count", Label: i18n.N("How many"), Type: modulekit.PluginBlockSettingNumber, Default: "3", Min: 1, Max: 20,
}

// Wire - the module's repository, controller and route, its block (its
// setting, its data) and its line of the status report (modulekit).
func Wire(d modulekit.Deps) modulekit.Parts {
	greetings := NewRepository(d.DB)
	c := &Controller{BaseController: modulekit.NewBaseController(d.Base), greetings: greetings}
	modulekit.SetPluginBlockSettings(PluginBlockLatest, func(context.Context) ([]modulekit.PluginBlockSetting, error) {
		return []modulekit.PluginBlockSetting{latestCount}, nil
	})
	modulekit.SetPluginBlockData(PluginBlockLatest, func(ctx context.Context, p modulekit.BlockPlacement) (any, error) {
		n, _ := strconv.Atoi(p.PluginSettingOr(latestCount))
		return greetings.Latest(ctx, n)
	})
	return modulekit.Parts{
		Routes: func(r modulekit.Routes) {
			r.Mux.Handle("GET /hello", http.HandlerFunc(c.Page))
		},
		Requirements: func(ctx context.Context) []modulekit.Requirement {
			req := modulekit.Requirement{Title: i18n.T(ctx, "Greetings")}
			n, err := greetings.Count(ctx)
			if err != nil {
				req.Severity, req.Value = modulekit.RequirementError, i18n.T(ctx, "Unavailable")
				return []modulekit.Requirement{req}
			}
			req.Value = strconv.Itoa(n)
			if n == 0 {
				req.Severity = modulekit.RequirementWarning
				req.Description = i18n.T(ctx, "No greetings yet.")
			}
			return []modulekit.Requirement{req}
		},
	}
}
```

Add the new strings (`"How many"`, `"Latest greetings"`,
`"Unavailable"`) to `translations/ru.json`.

The data loader runs with every refresh of the blocks' cache (templates
don't query the database); the block's template, in
`templates/blocks/hello_latest.html`, gets its result as `.Data`:

```html
{{define "block_hello_latest"}}{{if .Data}}
<div class="block hello-latest">
    <h3 class="block-title">{{t "Latest greetings"}}</h3>
    <ul class="hello-list">{{range .Data}}<li>{{.Text}}</li>{{end}}</ul>
</div>
{{end}}{{end}}
```

Its setting is the `count` field on the block's placement page; the
template could read it too, as `.Placement.PluginSetting "count"`.

## Changing the module later

- **Schema changes.** Edit `ent/schema`, run `go generate ./ent`, then
  `go run ./cmd/migrations -name <what_changed>`. A column or an index
  removed from the schema is dropped (`DROP COLUMN`, `DROP INDEX`), and
  so is a table removed from it (`DROP TABLE`): move their data in an
  earlier release.
- **Data changes.** Code that moves or fixes rows is a data step,
  registered in `RegisterSteps`:

  ```go
  manifestkit.RegisterUpdate(Name, manifestkit.Update{
  	Version: "20261101120000", Description: "greetings_drop_empty", Run: dropEmptyGreetings,
  })
  ```

  `Version` is a timestamp placed among the module's migration files,
  after the ones the step relies on. Write the step in SQL on `env.DB`,
  with values passed as parameters (`?` and `env.Dialect.Rebind`), not
  with the generated client: the client describes the newest schema,
  not the one the step runs on.

  ```go
  // dropEmptyGreetings deletes the greetings without a text.
  func dropEmptyGreetings(ctx context.Context, env manifestkit.InstallEnv) error {
  	q := env.Dialect.Rebind("DELETE FROM hello_greetings WHERE text = ?")
  	if _, err := env.DB.ExecContext(ctx, q, ""); err != nil {
  		return fmt.Errorf("deleting empty greetings: %w", err)
  	}
  	return nil
  }
  ```

  On a new install, data steps are only recorded, not run.
- **Uninstall.** The engine removes what the manifest describes: the
  module's content, its blocks' placements, its permissions, its tables
  (`Tables`) and the files only they used (the full list:
  [Uninstall a module](./using-modules.md#uninstall-a-module)). If the
  module keeps something else, such as rows of the core's tables (name
  them in the manifest's `CoreData`) or its own files in
  `modulekit.PublicFiles()`, return an uninstall step from `Wire`
  (`Parts.Uninstall`) that removes it.

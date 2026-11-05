# Examples

## A module from outside the engine

`hello/` is a module in a Go module of its own (`example.com/kosatka-hello`),
as it would be in its own repository. It is built on the module SDK only
(`github.com/kosatka-cms/kosatka/modulekit` and `pkg/*`), with a page
(`/hello`) listing the greetings of its own table, a stylesheet, templates
and translations:

| Path | What's there |
|---|---|
| `hello.go` | `init()` registers the module (`modulekit.Register`): importing the package is enough |
| `manifest.go` | Its description (`manifestkit.Module`) and its schema's registration |
| `wire.go`, `controller.go`, `repository.go` | Its route, page and data |
| `ent/` | Its own ent schema, generated client and migrations |
| `cmd/migrations/` | Writes its next migration (`kosatka.GenerateMigrations`) |
| `templates/`, `static/`, `translations/` | Built into the binary (`embed`) |

## A site with it

`hello-site/` is a site's own binary: `main.go` calls `kosatka.Main()`, and
`kosatka_modules.go` imports the module. `app module add` writes that file:

```bash
cd examples/hello-site
go run . module add example.com/kosatka-hello@v0.0.0   # go get + kosatka_modules.go
go run . module list                                  # the engine's modules and hello
go build -o site .                                    # the site's binary
```

Both `go.mod` files replace the engine (and the module) with this
repository's directories; a real site requires their releases instead. Run
the binary from a directory with the engine's `web/` and `themes/` (here,
the repository's root), install a site with the module (`site install …
-modules forms,error_reports,hello` - the list replaces the standard
modules) or enable it on the Modules page, and open `/hello`.

The details: [Write a module](../docs/modules/write-a-module.md) (a
tutorial built on `hello`) and [Using modules](../docs/modules/using-modules.md)
(adding one to a site).

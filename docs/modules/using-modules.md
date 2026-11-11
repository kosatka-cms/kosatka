---
title: Using modules
description: Turn modules on and off, choose them at install, uninstall them, and add a module from its own repository to a site.
sidebar:
  order: 1
---

This page is for a site's owner: how to manage the modules a site has,
and how to add one that lives outside the engine. For what the modules
are, see [the overview](./index.md).

## The Modules page

Open **Modules** in the admin toolbar (`/admin/config/modules`). It lists
every module the site's binary has, by category - *Content*, *Media*,
*Community*, *Administration*, *Integrations*, then *Other* for the
modules without one - each with a switch to enable it and a short
description. The categories are folded, each line naming its modules
that are on; click one to open it, or *Expand all*. The links at the top
open a category and say how many of its modules are on. A module that
isn't installed (never was, or was uninstalled) is marked *Not
installed*, one from its own repository *from its own repository*; a
disabled, installed one has an **Uninstall** link.

Under each module, *What the module includes* lists what it owns on this
site: the modules it requires and those that require it, its content
types and how many items each has, where its blocks are placed, its
permissions and how many roles have them, its admin pages, how many
rows its data tables have, and its CSS and JavaScript files.

## Enable a module

1. Tick the module and click **Save**.
2. The first time, the module installs itself: its tables are created,
   its install step writes its starter data (for example, the image
   styles of *Galleries*), and its blocks that name a region are placed
   in the site's theme. Labels it writes are put in the site's language.
   If the install fails, nothing is saved.
3. Give roles the module's permissions, if it has any (**Users →
   Permissions**), and place its blocks (**Structure → Blocks**, see
   [Blocks](../guides/blocks.md)).

A module can require others. It can only be enabled together with them,
and they can't be disabled while it is on: the page refuses and saves
nothing.

## Disable a module

Untick it and save. While it is off:

- its pages answer 404 (pages with the site's 404 page, `/api/*`
  addresses with a JSON error);
- its admin pages, its links on pages and its items in the *Create
  content* menu are hidden, and its CSS and JavaScript are not loaded;
- its blocks are not offered for placement, and placed ones are not
  shown;
- its permissions are hidden from the permissions page (the roles keep
  them);
- content of its types doesn't open and can't be created, edited,
  deleted or commented on; it isn't in the sitemap or search;
- its sort criteria, filters and fields in Views are skipped;
- its processing in the text formats (*Processing when shown*) and its
  choices in a field's *How to show* are off: texts and fields show as
  they would without it.

The data stays: enabling the module again brings everything back.
Disabling doesn't edit your Views: one that lists content of the
module's type still lists it (the items don't open), so edit the View
yourself.

## Choose the modules of a new site

`app install` installs the standard modules (Forms, Private messages,
Subscriptions, Video embeds, Action log and 404 log;
with `-demo-set`, also those the set has content for) unless `-modules`
says otherwise:

```bash
app install -site-name "Example" -admin-name admin -admin-email admin@example.com \
  -modules polls,forms,error_reports,private_messages,subscriptions
```

`-modules` takes machine names separated by commas, `all` or `none`.
A module the listed ones require is added. The others are recorded as
disabled and not installed: their tables don't exist and `app migrate`
skips them until you enable them on the Modules page. With `-demo`, each
installed module adds its own demo content. See
[`app install`](../reference/cli.md#app-install).

## Uninstall a module

Uninstalling removes a module's data for good. Back up the database and
the files first (see [Backups](../guides/production.md#10-backups) in
Running in production).

1. Disable the module and save.
2. Click **Uninstall** next to it. When a module can't be uninstalled,
   the reason is shown instead: another installed module requires it
   (uninstall that one first), or it keeps data in the core's tables and
   has no uninstall step for them.
3. The page lists everything that will be deleted. Read it.
4. If the module has content, delete it first: type the module's machine
   name and click **Delete the content**. Each item is deleted with its
   comments, field values and the files nothing else uses. A large site
   takes several runs; the page says when some are left.
5. Type the machine name (again, if you deleted content) and click
   **Uninstall**.

This removes the module's block placements, the menu links to its
pages, its permissions and its content types' permissions in roles, the
search, SEO, breadcrumb and block visibility settings of its content
types, its sort criteria, filters and fields in Views, the texts of its
emails changed on the *Mail* page, its processing
in the text formats, its choices in the fields' *How to show* (those
fields show as the core shows them), whatever its own uninstall step
removes, its tables, its records of applied migrations and the files
only its tables used. The module stays on the Modules page, disabled;
enabling it again installs it from scratch. If something fails halfway,
run the uninstall again: each step can be repeated.

## Updates

A module's database changes come with the binary, like the core's. After
deploying a new binary, run `app migrate` (the plan) and
`app migrate -apply`. It covers the core and every module that is
enabled or installed; an enabled module that isn't installed yet is
installed. See
[`app migrate`](../reference/cli.md#app-migrate).

## Add a module from its own repository

A module from outside the engine is a Go module. To use it, the site
builds its own binary: the engine plus the modules it imports. The
repository's
[`examples/hello-site`](https://github.com/kosatka-cms/kosatka/tree/main/examples/hello-site)
is such a site, with the example module
[`examples/hello`](https://github.com/kosatka-cms/kosatka/tree/main/examples/hello).

You need Go (the version in the engine's `go.mod`) on the machine that
builds the binary.

1. Create the site's main package, once. In an empty directory:

   ```bash
   go mod init example.com/my-site
   ```

   and a `main.go`:

   ```go
   package main

   import "github.com/kosatka-cms/kosatka"

   func main() { kosatka.Main() }
   ```

   Then fetch the engine:

   ```bash
   go mod tidy
   ```

   This binary is the whole engine: it has every command of `app`
   (`install`, `migrate`, `module`, `assets`…).

2. Add the module by its Go package path, with a version or without one
   (the latest):

   ```bash
   go run . module add example.com/kosatka-hello@v1.2.0
   ```

   It runs `go get` and writes `kosatka_modules.go`, a file that imports
   the module:

   ```go
   // Code generated by "app module"; DO NOT EDIT.

   // The modules from outside the engine this site has: each registers
   // itself when imported (modulekit.Register).

   package main

   import (
   	_ "example.com/kosatka-hello"
   )
   ```

   Don't edit it by hand; `app module add` and `app module remove` keep
   it. If `go get` fails, the file is left as it was. Both commands work
   only in a site's own main package: in the engine's repository they
   refuse, since its modules are built in.

3. Check the binary has it:

   ```bash
   go run . module list
   ```

   It prints the engine's modules and then each module from outside it
   with its Go package and version.

4. Build the binary, for example for a Linux server:

   ```bash
   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o kosatka .
   ```

   Like the engine's own binary, it runs from a directory with the
   engine's `web/` and `themes/`: take them from the engine's repository
   at the release your `go.mod` requires.

5. Deploy it as in [Running in production](../guides/production.md),
   with your binary in place of the engine's. A module's templates and
   static files are inside the binary; if nginx serves `/static/`, export
   them with your binary when you assemble the static files:

   ```bash
   ./kosatka assets export "$stage"
   ```

   (see [`app assets`](../reference/cli.md#app-assets)).

6. On a site that already exists, a module new to the binary is on at
   once, unless its manifest sets `OffByDefault`: `app migrate -apply`
   (step 5) installs it. A module with `OffByDefault` stays off until
   you enable it on the Modules page. A new site installs it when its
   manifest sets `InstallByDefault`; otherwise list it with the standard
   ones:
   `-modules action_log,forms,not_found_log,private_messages,subscriptions,video_embed,hello`.

To update a module, run `go run . module add <path>@<new version>`,
rebuild, deploy, and run `app migrate -apply`.

## Remove a module from the binary

1. Uninstall it on the Modules page first (see
   [Uninstall a module](#uninstall-a-module)).
2. Remove it, with the site's config so the command can check the
   site's database:

   ```bash
   CONFIG_PATH=/path/to/config.yaml go run . module remove example.com/kosatka-hello
   ```

   This drops the import from `kosatka_modules.go` and the requirement
   from `go.mod` (`go get …@none`). While the module is still installed
   in the site's database, the command refuses and changes nothing. When
   it can't reach the database (no config on the build machine, say), it
   warns and removes the module: make sure you uninstalled it. `-force`
   removes an installed module anyway.
3. Rebuild and deploy the binary.

With several sites on one engine (see
[Several sites on one server](../guides/multisite.md)), uninstall it on
each site first: `app module remove` checks the one database its config
names.

### A module installed in the database but not in the binary

A binary built without a module the database has installed (it was
removed without being uninstalled, or the site runs an older or other
build) still serves the site, but:

- content of the module's types answers 404, and it leaves the sitemap,
  the search and the latest comments;
- the module's pages, blocks and admin pages are gone; menu links to its
  pages lead to 404;
- its sort criteria, filters and fields in Views are skipped, so a View
  built on them lists other content;
- it isn't on the Modules page, so it can't be uninstalled from there.

Its tables and data stay. The status report shows the warning *Modules
missing from the binary*, the site logs it when it starts
(`level=WARN`, with `module=<name>`), and `app migrate` prints
`<name>: installed in the database, not in this binary`. To clean up,
deploy a binary that has the module again, at the version the database
has: the module comes back as it was. Disable it and uninstall it on the
Modules page, then remove it from the binary.

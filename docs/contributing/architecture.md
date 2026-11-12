---
title: Architecture
description: What the engine is made of, how a request goes through it, and the rules each part follows.
sidebar:
  order: 1
---

Kosatka is a Go program (module `github.com/kosatka-cms/kosatka`) that
renders pages on the server with `html/template`, keeps its data in MySQL
or PostgreSQL through ent, and is built from a small core plus modules.
This page is the map: where things are, how they depend on each other,
and the few rules every part follows.

Everything specific to a site lives in its data (the database, the admin's
settings, `config/*.yaml`) and in its theme, never in the engine's code.

## Package map

| Path | What it holds |
|---|---|
| `kosatka.go` | The engine as a library: `kosatka.Main()` (the server and the commands) and `kosatka.GenerateMigrations()`. A site with modules from other repositories builds its own binary on it. |
| `cmd/app` | The engine's own binary (calls `kosatka.Main`). |
| `cmd/migrations-gen`, `cmd/i18n-extract` | The migrations generator (`internal/migrationsgen`), the check for untranslated strings. |
| `internal/app` | The program: configuration, wiring of every service and controller (`wiring.go`), the routes (`routes.go`), the `app` commands (`commands.go`: `migrate`, `install`, `theme`, `module`, `assets`, `config`, `sites`, `backup`, `files`), what a new site starts with (`starter/starter.yaml`) and the demo content (`demo/`, `demoset/`). |
| `internal/domain/entity` | Domain types and the repository interfaces (the ports). No database, no HTTP. |
| `internal/domain/service` | The logic, written against the ports. |
| `internal/domain/views` | Views: their definitions, running them, list fields and rows. |
| `internal/domain/sociallogin` | Signing in and signing up through OAuth providers (Google, VK, Yandex): the callback, linking an account, the providers' keys (the admin's or the config's). |
| `internal/domain/themes` | Themes: their manifests (`theme.yaml`, subthemes), the list on the Appearance page, theme settings, the blocks a theme comes with, the catalog and `themes/themes.lock`. |
| `internal/domain/accounts`, `internal/domain/files` | Registration, invitations, password reset, the account settings and the profile fields; uploads and the users' own files. |
| `internal/domain/twofactor`, `internal/domain/audit` | Two-factor sign-in (an authenticator app's codes, recovery codes); the actions an action log keeps (`audit.Record`, stored by the action log module). |
| `internal/domain/assets`, `internal/domain/imageurl`, `internal/domain/fallback` | The versioned URLs of static files (`?v=`); the web paths of public files and their image-style derivatives; logging an error the code falls back from. |
| `internal/infrastructure/ent` | ent: `schema/` (the schema), `generated/` (generated code, never edited), `database/` (the repositories that implement the ports). |
| `internal/infrastructure/migrations` | Migration sets, the migration runner and the Go steps. |
| `internal/infrastructure/sqldialect` | The few SQL constructs that differ between MySQL and PostgreSQL. |
| `internal/infrastructure/oauth`, `internal/infrastructure/demo` | The site's OAuth providers, built from the config and the admin's settings; the modules' demo steps and what every demo step shares (texts by language, generated pictures, nodes, comments, menu links). |
| `internal/interfaces` | HTTP: the public controllers, their common base (`BaseController`, `BaseDeps`), the extension points for modules (`modules_api.go`), `middleware/`, `form/`. |
| `internal/interfaces/admin` | The admin's controllers, built on what `interfaces` exports. `interfaces` never imports it; `internal/app` and the router do. |
| `internal/http` | The router: the engine's routes and the gate in front of module routes. |
| `internal/modules/<name>` | One module each; `manifests/` lists their descriptions, `modules.go` their wiring. |
| `internal/paths`, `internal/config`, `internal/themeinstall`, `internal/buildinfo` | Theme and file paths with template lookup, configuration, installing themes from their repositories, the running binary's release and build. |
| `internal/testdb`, `internal/i18nextract` | Test databases, collecting the interface strings. |
| `modulekit` (and `manifestkit`, `schemakit`, `demokit`, `testkit`) | The public module SDK. |
| `pkg/*` | Helpers that know nothing of the domain, part of the module SDK: `csrf`, `filestore`, `httputil`, `i18n`, `imagestyle`, `htmlsanitize`, `mailtemplate`, `pagination`, `panicsafe`, `ratelimit`… |
| `internal/lib/*` | Helpers that know nothing of the domain, for the engine only: `mailsender`, `oauth`, `recaptcha`, `secretbox`, `searchtext`, `dateformat`… A module gets what it needs of them through `modulekit`. |
| `web/templates`, `web/static`, `translations/` | The engine's templates (the admin among them), its stylesheets and scripts, and its translation catalog. |
| `themes/<name>/` | A theme: `theme.yaml`, `templates/`, `static/`, `translations/`. |

## Layers and the direction of imports

The domain is built as ports and adapters (wherever `pkg/*` is allowed
below, `internal/lib/*` is too: the same kind of helpers, kept out of
the SDK):

- `internal/domain/entity` declares the types and the interfaces of the
  repositories. It imports only `pkg/*`.
- `internal/domain/service` depends on those interfaces, `pkg/*`, the
  domain packages below it (`views`, `themes`, `files`, `assets`,
  `imageurl`, `fallback`, `audit`), `internal/paths` and
  `internal/buildinfo`. It never imports `internal/infrastructure` or
  `internal/config`: the adapters and `internal/app` translate what they
  have into `entity` types.
- `internal/domain/views` imports `entity`, `internal/domain/imageurl`,
  `internal/domain/fallback`, `internal/paths` and `pkg/*`. The services
  call it, never the other way round.
- `internal/domain/themes` imports `entity`, `internal/domain/assets`,
  `internal/domain/fallback`, `internal/paths`, `internal/buildinfo` and
  `pkg/*`. The services and the handlers call it, never the other way
  round.
- `internal/domain/files` - uploads (validation, resizing, storage), the
  users' own files and where they are used - sits below the services: it
  imports `entity`, `internal/domain/imageurl`, `internal/paths` and
  `pkg/*`. It keeps the files in a `pkg/filestore` store
  (`paths.PublicStore()`), as do the image styles, the file server and
  the modules (`modulekit.PublicFiles()`): nothing writes the public
  files to a directory itself, so the store decides where they live.
- `internal/domain/sociallogin` comes above the services: it imports
  `entity`, `service` (the users' and the auth services), `accounts` and
  `pkg/*`, and nothing in `service` imports it.
- `internal/domain/accounts` - registration, invitations, password
  reset, the account settings and the profile fields - comes above the
  services too: it imports `entity`, `service` (the users' service) and
  `pkg/*`; nothing in `service` imports it, `sociallogin` does.
- `internal/domain/assets` (`internal/paths`, `internal/buildinfo` and
  `pkg/*`), `internal/domain/imageurl` (the web paths of public files and
  their image-style derivatives: `entity`, `internal/paths`, `pkg/*`),
  `internal/domain/fallback` (only `entity` and `pkg/*`) and
  `internal/domain/audit` (the actions an action log keeps: only
  `pkg/*`) sit below all of them.
- `internal/infrastructure/ent` implements the ports.
- `internal/interfaces` is the way in from HTTP; it calls the services.
- `internal/app` puts it all together.

`internal/domain/layers_test.go` (`TestLayerImports`) parses the imports of
these packages and fails on one that goes the wrong way.

Two conventions keep ent out of the domain:

- **A missing record** is `entity.ErrEntityNotFound`. ent's not-found
  error matches it through `errors.Is` (a hand-written
  `not_found_entity.go` next to the generated code), so services and
  handlers test for it without knowing ent.
- **Transactions** go through `entity.UnitOfWork.Do`. Repositories called
  with its context write in it; a nested `Do` joins the outer one instead
  of opening a second transaction.

## A request's path

1. **Middleware** (`internal/interfaces/middleware`), wired in
   `internal/app/wiring.go` (`withMiddleware`). In the order a request
   meets it:
   - the security headers on every response (`SecurityHeadersMiddleware`)
     and `/healthz`, answered ahead of the layers below
     (`HealthMiddleware`);
   - a handler's panic becomes a 500 with its stack in the log
     (`RecoverMiddleware`);
   - a 503 at once while the database doesn't answer
     (`DatabaseDownMiddleware`; `internal/lib/dbwatch` pings it every
     5 seconds and pauses the scheduler and the cache refresh too);
   - a user read by ID once per request (`UserMemoMiddleware`, see
     [Caches](#caches));
   - page cache for guests (`PageCacheMiddleware`, on while
     `page_cache_max_age` is above 0): a guest's HTML page (GET or HEAD
     without the session cookie, 200, setting no cookie) gets
     `Cache-Control: public, max-age=0, s-maxage=<page_cache_max_age>`,
     any other HTML `private, no-store`; it sees every cookie the layers
     inside it set;
   - the language of the address's prefix (`LanguageMiddleware`:
     `/en/news/x` is served as `/news/x` in English);
   - the session and the user: a blocked user's session is ended
     (`InactiveUserSessionMiddleware`), the signed-in user
     (`OptionalAuthMiddleware`), who acts, for the action log
     (`ActorMiddleware`), what of the content the visitor may not view
     (`ContentAccessMiddleware`, see
     [Rows every query leaves out](./database.md#rows-every-query-leaves-out)),
     a user whose role needs two-factor sign-in sent to turn it on
     (`TwoFactorSetupMiddleware`), the admin's theme preview, the user's
     time zone, the time of the last visit;
   - the request body limit (`BodyLimitMiddleware`), before anything
     parses a form;
   - CSRF (`CSRFMiddleware`) on every POST, PUT, PATCH and DELETE but the
     view count's `/api/node/view`: the `csrf_token` field or the
     `X-CSRF-Token` header;
   - maintenance mode;
   - blocked IP addresses and bots;
   - redirects;
   - the modules' hooks on a 404 (`NotFoundHooksMiddleware`).
2. **The router** (`internal/http/router.go`, `internal/app/routes.go`).
   Module routes go through a mux that answers 404 while the module is
   off (`internal/http/modules.go`).
3. **Content by its path.** A path no route claims goes through node access
   (`NodeAccessBySlugMiddleware`) and the node dispatcher
   (`interfaces.NodeDispatcher`): the content item with that path, sent to
   the controller of its type - `DynamicContentController` for types
   created in the admin, a module's controller for a module's type
   (`RegisterType`). With no such item, the enabled modules' fallbacks
   run (a feed, a forum section), then a View's page, then a term's page,
   then `/node/<id>` (a redirect to the item's address); otherwise the 404
   page.
4. **The controller** embeds `BaseController`. Its common dependencies are
   `interfaces.BaseDeps`, its own an `XxxDeps` struct; `wiring.go` builds
   it, in stages run in order (`wireData`, `wirePresentation`,
   `wireAccess`, `wireModules`, `wireServices`, `wireModuleLifecycle`,
   `wireControllers`): what a later stage needs from an earlier one is a
   field of `wiring`. An admin route goes in `internal/http/router_admin.go`
   through `adminRoutes` (`handle` - signed in with access to the admin;
   `with` - a further permission). Database work runs in `DBContext(r)`.
5. **Services** do the work through the ports; **repositories**
   (`internal/infrastructure/ent/database`) run the queries. A row read
   or deleted by its key goes through `one` / `all` / `notFoundAs`
   (`entity_rows.go`): ent's not-found becomes `entity.ErrEntityNotFound`
   naming what was looked for, and rows become entities.
6. **Rendering** - `service.TemplateService` (see
   [Themes and templates](#themes-and-templates)).

### The visitor's IP

Everything that needs the visitor's address (rate limits, IP blocking,
CAPTCHA, form submissions, error reports) calls `httputil.ClientIP`
(`pkg/httputil/clientip.go`). It returns the connection's address, unless
the connection comes from a trusted proxy: then it reads `X-Forwarded-For`
from the right, skipping trusted hops, or `X-Real-IP`. The trusted proxies
are `trusted_proxies` / `TRUSTED_PROXIES`
([configuration](../reference/configuration.md)), loopback by default,
set at startup with `httputil.SetTrustedProxies`. A client talking to the
application's port directly can't fake its address.

## Content model

Every content item is a row of `nodes`; its type is a row of
`content_types` (one created in the admin) or a module's manifest entry.
Fields are created in the admin, with no DDL at run time: a field's
storage (`field_storages`) can be shared by several types, each with an
instance of it (`field_instances`), and the values live in typed tables
(`field_value_*`), each read and written by its own store. Saving an
item is one transaction - its values, `taxonomy_index`, the search
index, the modules' parts, its revision; deleting one goes through
`NodeDeleteService`, via the trash for an admin type's item. The tables
and the steps: [Content model](../reference/content-model.md).

How these look to a site builder: [content types](../guides/content-types.md),
[taxonomy](../guides/taxonomy.md), [Views](../guides/views.md).

## Modules

Optional features are modules, each in `internal/modules/<name>/`: its Go
package, `manifest/` (its description), `ent/` (its schema),
`templates/`, `static/`. Templates and static files are embedded in the
binary.

- **Registry.** A module describes itself in its `manifest` subpackage
  (`Module()`: content types, permissions, plugin blocks, admin menu items,
  tables, assets); its schema and its install, data and demo steps are
  registered by the manifest's `RegisterSteps`. `manifests.All()` lists
  the engine's modules, `manifests.Register()` puts them in the registry
  and calls their `RegisterSteps`, and the core knows modules only
  through it (`entity.Modules`, `entity.ModuleByName`…).
- **Wiring.** A module's `wire.go` has `Wire(modulekit.Deps)
  modulekit.Parts`: from the core services in `Deps` it builds its
  repositories, services and controllers, registers its hooks, and returns
  `Parts` - `Routes`, `NodeTypes` (pages of its content types),
  `Fallback` (an answer for an unmatched path), `Uninstall` (its own
  uninstall step), `Requirements` (lines of the status report),
  `UserContent` (a user's content, counted and deleted when an account
  is deleted with all its content).
  `internal/app` calls `modules.Wire` and never names a module. A new
  engine module is its directory plus its lines in `manifests.All()` and
  `manifests.Register()` (its `RegisterSteps`) and one in
  `internal/modules/modules.go`.
- **The SDK boundary.** A module uses the core only through `modulekit`
  and `pkg/*`, so it would build outside this repository.
  `TestModulesUseSDKOnly` (`internal/modules/sdk_test.go`) checks the
  imports of every engine module (each directory there with a `wire.go`),
  its tests included: they use `modulekit/testkit`. A module from its own
  repository registers itself with `modulekit.Register` in `init()`.
- **Tables.** Each module's tables form their own migration set, created
  when the module is first installed. Each engine module keeps its own ent
  schema (`internal/modules/<name>/ent`); there a core table it references is
  described by a stand-in - a schema with the core table's name
  (`schemakit.Table("nodes")`) and only its `id` - which gives the foreign
  key but is never created or dropped with the module. See
  [Database and migrations](./database.md).

The module author's view: [writing a module](../modules/write-a-module.md),
[the SDK](../modules/sdk.md).

## Themes and templates

A template is looked up by its path relative to the templates root
(`paths.ResolveTemplate`): the active theme, then its base themes, then the
engine (`web/templates`), then the modules. An administrator previewing a
theme gets that theme for the request. The engine renders the HTML
document (`partials/html.html`); the theme decides the layout and the
regions (`theme.yaml`, `layout.html`).

Parsed templates are cached in the template service. With `env: local`
(`APP_ENV`) the cache is off and module files are read from disk, so an
edit shows on reload. Details: [templates](../themes/templates.md).

The engine's script, `app.js`, is written in parts:
`web/static/js/app/NN-*.js` (the core and behaviors' registry, dialogs,
the page, comments, the content form's fields, the editor, the lightbox,
Views…). `scripts/minify_js.sh` joins them in name order into
`app.min.js`; they share one script's top level, so a part may use what
an earlier one declares. A new behavior goes in the part of its kind.

Pages link the minified files (`*.min.css`, `*.min.js`), built from the
stylesheets and scripts and not kept in git: after changing one, run
`./scripts/minify_css.sh` and `./scripts/minify_js.sh`. Run them before
`go build` too: the binary embeds the modules' minified files
([Run from source](../getting-started/from-source.md)).

## Caches

`service.CacheService` keeps the menus, block placements, pre-rendered View
blocks, plugin blocks' data and the latest comments in memory. It is filled
when the server starts and refreshed every 15 minutes; a change in the
admin forces a refresh (`ForceUpdate`, asynchronous, concurrent calls
merged). Content changes do too, by themselves: `database.OnContentChange`
hooks the site's ent client - every node and comment written, once its
transaction commits - to a debounced refresh (2 s after the last change,
at most 10 s after the first); the deletions' raw paths signal it
themselves. A part that fails to load keeps what it had (the menus, a
menu's tree, a region's blocks). A View block whose output depends on
the current page isn't pre-rendered.

What it keeps is rendered as for a guest. While a content type is closed
to guests, a refresh also renders each View block, the latest comments
and each plugin block's data as for a visitor who may view every type;
those that differ (`SidebarData.AccessDependent`) are loaded again for a
visitor with other closed types than a guest's, on their first request,
and kept per set of closed types until the next refresh
(`SidebarDataFor`). What is built on demand is kept for the refresh it
was built from only (`visitorCache`): a request that outlives a refresh
doesn't leave its older copy behind.

Rows read on most pages are kept too. The site information and the
comment settings are read once and kept by their repository until it
writes them. A user read by ID is kept for the rest of the request
(`entity.WithUserMemo`, set by `UserMemoMiddleware`): the middlewares,
the handler and the page data read the signed-in user once. A write of
any user clears that. Reads inside a transaction always go to the
database.

The content processor keeps the texts it rewrote (their images as
previews in PhotoSwipe links) by the text's hash, up to 32 MiB, least
recently used first out: rewriting a text reads each of its images from
the files' store. A text with an image that failed isn't kept. Clearing
all the derivatives (Performance) forgets them.

The caches live in the process's memory: one process runs a site. The
rest of the site's state is the process's too - its languages and theme,
the paths, the modules' registries, the secrets key: set once at startup
(`internal/app`) and read by every package. Several sites on one server
are several processes ([Several sites on one server](../guides/multisite.md)).

## Translations

Source strings are English. Templates translate with `t`, `tc`, `plural`
and `tjs`, Go code with `i18n.T` and `i18n.Plural` (`pkg/i18n`); `i18n.N`
marks a string translated later (a label stored in data); scripts use
`Kosatka.t`. A Go call keeps its strings on one line:
`cmd/i18n-extract` reads Go code line by line. The engine has no
catalogs: English is built in, every other language is downloaded from
a translations catalog (the Kosatka one is built from the
`kosatka-cms/translations` repository, which checks the translations
against the engine's strings). Catalogs are looked up theme (then its
base themes), then the modules from outside the engine (their
`translations/`), then the downloaded languages.
`go run ./cmd/i18n-extract -sources` writes
`translations/sources/strings.json`, the list of every string of the
code with whose it is, built into the binary: the admin's *Interface
translations* lists the site's strings from it, a downloaded language is
checked against it, and a site's own changes of the strings (the
`interface_strings` table) are looked up before the catalogs. See
[languages](../guides/languages.md).

## Errors and logging

An error is logged once, where it is handled:

- Services, repositories and module code don't log an error they return.
  They wrap it with what they were doing:
  `fmt.Errorf("deleting block (id %v): %w", id, err)`. Compare errors only
  with `errors.Is` / `errors.As`. A service returning what its repository
  gives does it in one line: `entity.Found` (a missing record stays "not
  found", anything else is wrapped) or `entity.Wrapped`; `entity.IsNotFound`
  tells a missing record.
- The handler that decides the response logs it:
  `RenderLoadFailure` (a missing record - 404 and a debug line; anything
  else - 500 and an error line) or `LogFailure` (the handler picks the
  response itself: a redirect, JSON, a fallback).
- A service that goes on with a fallback (a default value, an empty list)
  logs with `service.LogFallback` (`fallback.Log` in the packages below
  `service`, `modulekit.LogFallback` in a module): debug for a missing
  record, warn otherwise.
- Goroutines and background jobs log their own errors (no one to return
  them to) and recover from panics with `panicsafe.Recover`.

`TestReturnedErrorsAreNotLogged` fails on code that logs an error and
returns it too.

What the visitor reads is a message made for them: a check that failed
returns `i18n.Errorf("The file is too large.")`, and `i18n.ErrorText`
translates it through any wrapping. Any other error shows as "The
operation failed. Try again." - its text (paths, queries, bucket names)
belongs in the log; `i18n.IsUserError` tells the two apart.

## Where to go next

- [Database and migrations](./database.md)
- [Testing](./testing.md)

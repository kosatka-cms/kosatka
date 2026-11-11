---
title: The module SDK
description: The packages a module is built on, what it can declare and contribute, and the key functions of each.
sidebar:
  order: 3
---

A module in its own repository can't import the engine's `internal/`
packages, so everything it uses of the core is in public packages: the
module SDK. Its data types are the core's own (type aliases), so a module
and the core speak the same types; most services and repositories a
module gets are interfaces of the SDK's own, with only what a module
needs - none of the core's setup, nor a way past its paths (content is
deleted through `NodeDeleter`, which removes its files, comments and the
modules' data too).

The SDK is not stable yet: until the engine's 1.0 release, a minor
release may change it, and its release notes say how. The engine's
test `TestAPI` keeps a listing of the SDK's API
(`modulekit/testdata/api.txt`; all of it but testkit's database part,
built with the `dbintegration` tag), so every change of it is a
deliberate one, seen in review.

This page groups the SDK by what a module does with it and names the key
identifiers. The full API, with every function and type, is on
[pkg.go.dev](https://pkg.go.dev/github.com/kosatka-cms/kosatka/modulekit).
For a worked example, see [Write a module](./write-a-module.md).

## The packages

| Package | For |
|---|---|
| `github.com/kosatka-cms/kosatka/modulekit` | The module's code: registration, `Wire` (`Deps`, `Parts`), controllers, hooks, the core's types and helpers |
| `modulekit/manifestkit` | The manifest: the module's description (`Module` and its parts), its schema, install and data steps |
| `modulekit/schemakit` | The module's own ent schema: table and column conventions, "not found" |
| `modulekit/demokit` | The module's demo step for `app install -demo` |
| `modulekit/testkit` | The module's tests: rendering, handler fakes, the database (build tag `dbintegration`) |
| `github.com/kosatka-cms/kosatka` | `Main()` - the engine, for a site's own binary; `GenerateMigrations()` - the migrations generator, for a module's tree |
| `github.com/kosatka-cms/kosatka/pkg/...` | Utilities with no domain: `i18n`, `csrf`, `httputil`, `pagination`, `ratelimit`, `slug` and others |

`manifestkit`, `schemakit` and `demokit` are light: they don't pull in
the core's services. A manifest built on them, without `modulekit`, can
live in a package of its own, as the engine's modules' manifests do.

## Registration

A module registers itself from its package's `init()`:

```go
modulekit.Register(modulekit.Registration{
	Module:  Module(),       // manifestkit.Module
	Steps:   RegisterSteps,  // its schema, install, data and demo steps
	Wire:    Wire,           // func(modulekit.Deps) modulekit.Parts
	Catalog: catalog,        // fs.FS with <lang>.json at its root (its translations/)
	Assets:  assets,         // fs.FS with templates/ and static/ at its root
})
```

The engine lists such modules after its own. A name the binary already
has stops it at startup. Every field but `Module` may be nil.

## The manifest

`manifestkit.Module` describes everything the module owns. The engine
uses it to build the Modules page, to install and uninstall the module,
and to hide its parts while it is disabled.

| Field | |
|---|---|
| `Name`, `Label`, `Description` | The machine name, and the label and description on the Modules page (mark them with `i18n.N`) |
| `Category` | Its group on the Modules page: `CategoryContent`, `CategoryMedia`, `CategoryCommunity`, `CategoryAdministration`, `CategoryIntegrations` or `CategoryOther`. The set is the engine's; none, or a value the engine doesn't know, puts the module under *Other* |
| `Requires` | Modules it needs: it is enabled only together with them, and they can't be disabled while it is on |
| `NodeTypes` | Its content types (`ModuleNodeType`: `Name`, `Label`, `CreatePath`, `PathPrefix`, `OwnDiscussion`, `EditPath`, `DeletePath`) |
| `PluginBlocks` | Its blocks for **Structure → Blocks** (`ModulePluginBlock`: `ID`, `Label`, `Region`). A block with a `Region` is placed in that region of the site's theme when the module is installed (if the theme has it); the site moves or removes it like any block |
| `Permissions` | Its permissions on the permissions page (`ModulePermission`: `Key`, `Label`) |
| `AdminMenu` | Its admin pages (`ModuleMenuItem`: `Section` - one of `AdminSectionContent`, `AdminSectionStructure`, `AdminSectionConfiguration`, `AdminSectionReports`; `Item` - an `AdminMenuItem` with `Label`, `Path`, optionally `Group` - a group of Configuration: `AdminGroupSite` (*Site*), `AdminGroupTexts` (*Texts and files*) or `AdminGroupModules` (*Modules*), the item after the group's last one - and `Permission`) |
| `CaptchaForms` | Its forms that can show the reCAPTCHA (`ModuleCaptchaForm`: `Key`, `Label`, `Default`) |
| `FileSources` | Its texts that can link to users' files, for the files manager's "used in" (`ModuleFileSource`): `Source` (the key the uses are recorded under), `Table`, its HTML `Columns`, the `IDColumn` they are recorded against, the `Label` of a place of use; the title and link are the content item's (`NodeBacked`), else the start of `TitleColumn` and `EditPath` + the id - without `TitleColumn`, neither (a private text) |
| `NodeFiles` | Its tables of files attached to content items: deleting an item deletes the files nothing else uses (`ModuleNodeFiles`: the file in `Table`'s `FileColumn`, the item in its `NodeColumn` - or, with `Via`, in the `NodeColumn` of the `Via` table's row that `ViaColumn` points to) |
| `MaintenancePaths` | Path prefixes that stay open in maintenance mode |
| `Paths` | The addresses of its own pages (`"/quotes"`: that path and the ones under it): uninstalling removes the menu links to them |
| `RobotsDisallow` | Paths added to `robots.txt` while it is enabled |
| `LabelColumns` | Columns of labels its install step writes in English, put in the site's language (`ModuleLabelColumn`: `Table`, `Column`) |
| `Tables` | Every table it owns: created on install, dropped on uninstall |
| `Data` | Those of `Tables` the Modules page counts (`ModuleData`: `Label`, `Table`) |
| `OffByDefault` | Off until enabled on the Modules page, on sites that already exist. Without it, a module new to an existing site's binary is on at once, and `app migrate -apply` installs it |
| `InstallByDefault` | Installed on a new site unless `app install -modules` says otherwise |
| `CoreData` | What it keeps in the core's tables beyond the above, as the uninstall page names it; such a module is uninstalled only with its own uninstall step |
| `CSS`, `JS` | Its files in `static/`, without the extension; loaded only on pages that show something of the module |

### Steps

Registered from the module's `RegisterSteps` (the `Steps` of its
registration), each under the module's name:

- `manifestkit.RegisterSchema(name, manifestkit.Schema{Tables, Files, Dir})` -
  its own ent schema: its tables (`migrate.Tables` of the generated
  client), its embedded migration files and their directory in its tree.
- `manifestkit.RegisterInstall(name, fn)` - runs once when the module is
  installed, after its tables are created. `fn` gets
  `manifestkit.InstallEnv` (`DB`, `Dialect`); `manifestkit.Driver(env)`
  opens the module's own client on it. One that fails isn't recorded:
  the next `app migrate -apply` runs it again. Give the module its
  install step in its first release (one added later may run on the
  sites that have the module). Rows a later version adds go both in the
  install step, for new sites, and in a data step, for the sites that
  have the module already.
- `manifestkit.RegisterUpdate(name, manifestkit.Update{Version, Description, Run})` -
  a data step, run by `app migrate` in order with the migration files
  (`Version` is a timestamp among them); a new install only records it.
  Write it in SQL on `env.DB`, with `env.Dialect.Rebind` for the `?`
  placeholders (`env.Dialect` is `manifestkit.DialectPostgres` on
  PostgreSQL).
- `demokit.Register(name, fn)` - the demo step (below).

The core's rows an install or data step changes go through
`manifestkit`'s helpers:

- `manifestkit.EnableTextFilter(ctx, env, name)` - turns the module's
  text filter (`AddTextFilter`) on for the text formats with the visual
  editor.
- `manifestkit.EnsureImageStyle(ctx, env, manifestkit.ImageStyle{…})` - an
  image style of the core, left alone if the site has one of that name.
- `manifestkit.GrantPermission(ctx, env, role, perm)` - gives the role
  named `role` (`manifestkit.RoleUser` - every signed-in user's) the
  permission `perm`: who may use the module by default. A site without
  the role is left as it is.
- `manifestkit.FullAccessRoles(ctx, env)` - the names of the roles with
  full access (the administrator's): an install step names them where its
  settings give something to roles, so what only they may do shows in the
  settings as theirs.
- `manifestkit.PlaceModuleBlocks(ctx, env, name)` - places the module's
  blocks that name a `Region`, as installing it does: for a data step
  when an installed module gets such a block.

## What Wire gets

`Wire(d modulekit.Deps) modulekit.Parts` is called once at startup.
`Deps` holds the core's database and services:

- **Database:** `DB` (`*sql.DB`) - for the module's own tables, `Logger`.
  The core's tables aren't part of the SDK: their columns change with the
  engine's migrations, and a write past the core skips what it does
  besides (the search index, the caches). Read and change content,
  users and terms through the services below (`Nodes`, `NodeRepo`,
  `Users`, `Taxonomy`…); a module that needs more asks for it in the SDK.
- **Controllers:** `Base` - what `modulekit.NewBaseController` needs
  (pass it on as it is); `Auth` - who the user is and what they may,
  `Templates` - the site's templates.
- **The site:** `SiteInformation` - its name, e-mail and languages;
  `RefreshBlocks()` - rebuilds the blocks' cache in the background, after
  a change the cached blocks show (content and comments rebuild it
  themselves).
- **Content:** `Nodes`, `NodeRepo`, `NodeDeleter`, `ContentTypes`,
  `Teasers` (content cards), `Comments`, `Taxonomy`, `Vocabularies`.
  `NodeDeleter.Remove(ctx, id, user)` takes an item out of sight as the
  user's action - into the trash, or, of a module's content type (which
  has no trash), deleted for good; `Delete(ctx, id)` deletes one for
  good; `AfterDelete(fn)` adds the module's own cleanup after every
  delete (files it keeps outside the database). `Comments` moderates
  too: `Approve`, `Hold(ctx, id, reason)` (back to waiting for approval;
  `CommentReason(CommentReasonReported, "3")` - the reason the comments'
  queue shows as *3 complaints*), `MarkSpam`, `Delete` (with the
  replies); a comment's `Status` is `CommentPublished`, `CommentPending`
  or `CommentSpam`.
- **Files:** `Files`, `FileUpload`, `UploadSettings`, `ImageStyles`.
- **Users and mail:** `Users` (by id, or the active users of some roles -
  `UsersWithRoles(ctx, roleIDs)`: whom to tell what a role should do),
  `Roles` - the site's roles (`Roles(ctx)`: a module's choice of who may
  do something), `Mail`.
  `UserRoleNames(user)` - a user's roles (`RoleUser` - every signed-in
  user's, `RoleAnonymous` - a guest's), `PermFullAccess` - the
  administrator role's permission; `role.Grants(perm)` - whether a role
  grants a permission itself (`role.Grants(modulekit.PermFullAccess)` -
  the roles with full access).
- **Addresses:** `Redirects` - the site's redirects (the *Redirects*
  page's): `GetRedirect(path)` - where one sends a path, if one does;
  `Create(ctx, from, to)` adds one.

## What Wire returns

Every field of `modulekit.Parts` is optional:

| Field | |
|---|---|
| `Routes func(modulekit.Routes)` | Registers the module's routes on `Routes.Mux`, a `modulekit.Mux` (`Handle`, `HandleFunc`, Go's `net/http` patterns): a module's own registration functions take that type. The mux answers 404 while the module is disabled. `Routes.Store` is the session store, `Routes.AdminOnly` wraps a handler of the admin: signed in, with `PermAccessAdmin` |
| `NodeTypes` | The pages of its content types (`map[modulekit.ContentType]modulekit.NodeTypeController`: `Detail`, `CreateForm`, `Create`, `Edit`, `Save`, `Delete`) |
| `Fallback` | Answers a path no route matched (a feed's address, a forum section), while the module is on; returns true when it answered |
| `Uninstall` | Its own uninstall step, for what the engine doesn't remove itself (its `CoreData`, its folder in `PublicFiles()`): run after the module's content is deleted, before its tables are dropped |
| `Requirements` | Its lines on the status report, checked while it is on |
| `UserContent` | Its content of a user that *Delete the account and all its content* (People) also counts and deletes, before the account: `modulekit.UserContent{Label, Count, Delete}`. `Label` names the count on the confirmation, a translation source with `@count` (`"private messages: @count"`). Nothing is asked of a module never installed |

A content type's controller that also implements
`modulekit.NodeFormPreviewer` (`FormPreview(w, r)`) previews its form:
the form's *Preview* button posts the form to `POST
/create/{type}/preview` or `/node/{id}/edit/preview`
(`modulekit.FormPreviewPath(type, id)`; `r.PathValue("id")` is empty on
create), in a new tab, and `FormPreview` renders the item as it would be
saved, saving nothing. The module's edit page shows the button when its
data has `"formPreviewURL"` (that path); the preview page shows the
strip over it when its data has `"unsavedPreview"` (a
`*modulekit.UnsavedPreview`: `Errors` - what saving would refuse,
`FilesLeft` - files were chosen and not shown). A type without
`FormPreview` has no preview: those addresses answer 404.

`Requirements` is a `modulekit.StatusCheck`: `func(ctx) []modulekit.Requirement`.
A `Requirement` has a `Title` (what was checked), a `Value` (what was
found), a `Description` (HTML: what to do, when not OK) - all translated
in `ctx`'s language - and a `Severity`: `RequirementOK`,
`RequirementWarning` or `RequirementError`. A warning or an error also
shows on the admin's warning bar. `modulekit.CheckWritableStore(title,
store)` is a ready check that a store (`modulekit.PublicFiles`,
`modulekit.PrivateFiles`) takes a file, `modulekit.CheckWritableDir(title,
dir)` the same for a directory.

## Hooks

A module adds to the core's pages by registering functions from `Wire`,
each under the module's name - its manifest's `Name`, the first argument
or the `Module` field. They are called only while the module is
enabled: a function doesn't check `ModuleEnabled` itself. The names say
what a function does: `Add…` adds one of many contributions, `Set…`
sets the one of its kind (a site has one workflow, one translator), and
`On…` listens to events.

| Function | Adds |
|---|---|
| `AddPageData(module, fn)` | Data to every page the site renders, the admin's pages too |
| `AddUserMenuCount(data, n)` | Called from the `AddPageData` function: adds `n` to the number on the user menu's toggle (`.userMenuCount`, shown when above 0), for example the user's unread items |
| `AddProfileTab(t)` | A tab of the user profile, `ProfileTab{Module, Path, Label, Allowed, Badge, Load}` (below) |
| `AddNodePage(module, fn)` | Parts of a content item's page: `NodePageParts{Data, Links, Comments}`. `Links` (`NodeLink{URL, Label}`) are shown under the item; `Data` is `.moduleNode.<module>` for the module's templates `node_links_<module>` (under the item) and `node_actions_<module>` (next to *Edit*), and for its plugin blocks on the page (`.Ctx.moduleNode.<module>`). `Comments: true` - the module shows the item's comments in place of the core's (a comments service): its template `comments_<module>` takes the place of the comments section, with the page's data; the item's comment mode still applies (*hidden* - nothing), the first enabled module saying so wins. A module rendering its own content type's page puts `ModuleNodeData(ctx, node)` in its data as `moduleNode` |
| `AddTermPage(module, fn)` | Data of a term's page (a tag, a section): `fn(ctx, term)`'s result is `.moduleTerm.<module>`, read by the module's template `term_actions_<module>` |
| `AddCreateLinks(module, fn)` | Items of the user's *Create* menu (`CreatableMaterialLink`) |
| `AddContentTypeSettings(s)` | A section of the content type form: `ContentTypeSettings{Module, Load, Save}`, rendered by its template `content_type_settings_<module>` - rows of the form's *Modules* group, like the form's own settings: `<div class="admin-setting"><div class="admin-setting-text"><label class="admin-setting-label">…</label><span class="admin-setting-help">Module · …</span></div><div class="admin-setting-control">…</div></div>` (a switch: `admin-setting-control admin-setting-control--switch form-check form-switch` around an `<input type="checkbox" role="switch" class="form-check-input">`) |
| `AddNodeSettings(s)` | A section of the content item form: `NodeSettings{Module, Load, Save, Default}`, rendered by `node_settings_<module>` - a line of the form's side column: `<details class="node-settings-section"><summary><span class="node-settings-title">…</span><span class="node-settings-value">the current value</span></summary><div class="node-settings-body">…the fields…</div></details>`. The section is shown to *Administer content* only: `Save` runs when such a user saves; a new item saved by anyone else gets `Default` (optional - the module's defaults), and an existing one keeps its settings |
| `AddUserSettingsSection(s)` | A section of a user's profile form (`/me/edit`, and an administrator's edit of another user) - the user's own settings in the module, such as its e-mails: `UserSettingsSection{Module, Load, Save}`, `Load` and `Save` get the user edited; rendered by `user_settings_<module>` - a card of the form's side column: `<section class="node-edit-card"><h2 class="node-edit-card-title">…</h2>…</section>`. A form without the section (a theme's own) still posts to `Save`: mark the section with a hidden field and save only when it is there |
| `AddMail(d)` | One of the site's e-mails, the module's: `MailDef{Key, Module, Label, Description, Recipient, Optional, Placeholders, Sample, Default, Load, Save, Weight}`. It is listed on *Configuration → Mail → Emails* while the module is on, its text edited there; `Deps.Mail.Send(ctx, key, to, data)` sends it. `Key` - `"<module>.<name>"`; `Label`, `Description` (when it is sent), `Recipient` - translation sources (`i18n.N`); `Optional` - it has a switch there (else always sent); `Placeholders` - all it may use (`MailSitePlaceholders`, `MailUserPlaceholders`, its own); `Sample` - sample values of its own, for the page's preview and test e-mail; `Default` - its built-in subject and text (`i18n.N`), kept changed by the page; or `Load` and `Save` when the module keeps the text itself. `MailTemplatePath(key)` - the e-mail's page there, for a link from the module's settings |
| `AddAccountSettingsSection(s)` | A section of the *Account settings* page - the module's settings of accounts: `AccountSettingsSection{Module, Load, Save}`, rendered by `account_settings_<module>` - a group, like the page's own: `<section class="admin-settings-group">` with `<h2 class="admin-settings-title">…</h2>` and a `<div class="admin-settings-card">` of rows (`admin-setting`: `admin-setting-text` with the label, `admin-setting-control` with the field); the module stores them in its own tables |
| `AddSitemapPaths(module, fn)` | Its pages in the sitemap |
| `AddTextFilter(f)` | Processing of HTML text when it's shown, on for the text formats that tick it (`TextFilter{Name, Module, Label, Description, Apply}`): `Apply(ctx, html)` returns the text to show; `ctx` carries the page's language. It runs on every show, so it should be cheap: no network, cached data. `EditorButton` and `EditorPlugin` (the module's script registering the button as a TinyMCE plugin, `static/<name>.js`) add a button to those formats' editors |
| `AddFieldFormatter(f)` | A way of showing a field (`FieldFormatter{Name, Module, Label, FieldType, Render}`), chosen per field on the content type's *Display: detail page* tab and per View field. `Render(ctx, value, layout)` returns the HTML: `value` is the field's as stored (a link field's - `Links(value)`), `layout` how several values show (`FieldLayoutAll`, `FieldLayoutFeatured` - the first large, the others small; for `FieldLayoutFirst` the value holds the first only). A part it can't show (a link to a site it doesn't know) it shows as the core does. It runs on every show: cheap, no network. `Hosts(ctx)` - its sites, offered as a link field's allowed ones; `EditorScript` - its script (`static/<name>.js`) the content form loads for the fields it shows (a preview of a pasted link, say). Off with its module, a field shows as links; uninstalling the module resets the fields' choice |
| `RecordAction(ctx, a)` | Records what the module did for the action log: `Action{Type, Object, Changes}` - the type (one the module registered), what it was done to (`ActionObject{Kind, ID, Label, Subtype, URL}`: the name at the time, so the log keeps it after a delete; `URL` - none for a deleted one) and what changed (`ActionChange{Field, From, To}`, shown as people read them). Who did it - the request's user and address - comes from `ctx`. Without the *Action log* module it costs nothing |
| `RecordNodeCreated(ctx, n)`, `RecordNodeSaved(ctx, before, after)` | For a content type the module saves with its own code (its own form): the log's *Content created*, and what a save changed of the item - published or unpublished, a new author, a new address - as the core's content form records them. Call them after the save, with the item as stored (reloaded), so the author's name and the address are the real ones. Deleting through `NodeDeleter` is recorded already |
| `NodeActionObject(n)` | The content item `n` as an action's `Object`: an action of the module on it (a form's submissions deleted) shows in the item's log too |
| `AddActionGroup(g)`, `AddActionType(t)` | The module's kind of actions (`ActionGroup{Name, Module, Label, Description}` - its line on the log's filter and its switch on the log's settings) and its actions (`ActionType{Name, Group, Label, Tone}`; `Tone` - `ActionToneCreate`, `…Change`, `…Delete`, `…Access`, `…Login`, `…Fail`, the action's colour in the log) |
| `OnAction(module, fn)` | A listener of every recorded action (`ActionEvent`: the action, `Actor{UserID, IP}`, `At`) - an action log of its own; it runs in the acting request, so it should be quick. `ActionActorFrom(ctx)`, `ActionGroups()`, `ActionTypeOf(name)` - who acts, the groups, a type by name |
| `AddAdminMenuCount(module, path, count, label…)` | The number by the admin menu item at `path` - the module's own (its manifest's `AdminMenu`) or the core's: what waits there, for example new reports. `count(ctx)` runs for each page that shows the admin menu, so it should be a cheap, indexed query; 0 or an error (logged) shows none. A core item is shown with the module off too, without the number. The admin's Overview lists every counted item under *Needs attention*; the optional `label` (`i18n.N`) says what is counted there when the item's own label doesn't (*Not found (404)* by *Redirects*) |
| `AddDashboardBlock(b)` | A block on the admin's Overview (`/admin`): `DashboardBlock{Name, Module, Title, Link, LinkLabel, Side, Weight, Permission, Data}`. It is shown in a card titled `Title` (`i18n.N`), with an optional "all" link, in the wide column or the narrow one (`Side`), after the core's blocks, lightest `Weight` first; while the module is enabled and to who has `Permission` ("" - anyone in the admin). Its template is the module's `{{define "dashboard_<Name>"}}` in `templates/partials/<module>_hooks.html`, with `.Data` - what `Data(ctx)` returned for the request (an error leaves the block out, logged). Rows styled like the core's: `.admin-dash-row`, `.admin-dash-sub`, `.admin-dash-empty` |
| `OnNotFound(module, fn)` | A call for every page request (`GET`, `HEAD`) answered 404: `fn(ctx, path, referrer)` - the path without the query and where the visitor came from. A path a redirect answers and a request the site blocks never get there. It runs in the request after the answer, so it should be quick; its errors are its own to log |
| `AddJob(module, j)` | A background job: `Job{Name, Every, Run}` - the site's scheduler runs `Run(ctx)` every `Every` (a minute at least, up to 50 seconds a run) while the module is enabled; an error is logged |
| `AddPathOwner(module, fn)` | Its addresses: `fn(ctx, path)` tells whether the module answers `path` (no leading `/`), for example a page it serves from `Fallback`. A View page and other modules' pages don't take such an address; a disabled module's addresses are free |
| `SetPluginBlockTargets(id, fn)` | What its block can show (the choice on the block's placement page) |
| `SetPluginBlockSettings(id, fn)` | Its block's settings on the placement page (`PluginBlockSetting`: `Key`, `Label`, `Type` - `PluginBlockSettingNumber`, `…Select`, `…Text` - `Default`, `Min`, `Max`, `Options`); the template reads one with `.Placement.PluginSetting "<key>"` |
| `AddEmbedKind(k)` | A kind of thing a text embeds with a tag `[[<kind>:<name>]]` (and a template with `{{embed}}`, [Embeds](../guides/embeds.md)): `EmbedKind{Name, Module, Render}`; `Render(ctx, name)` returns its HTML for the request's visitor - `ctx` carries their access, language and zone, so content queries see what they may - or `ErrEntityNotFound` for a missing one (nothing is shown). With `Label`, `Search(ctx, query, limit)` and `Describe(ctx, name)` (returning `EmbedOption{Name, Title, Meta, Thumbs, Lines, More, Note}`) the kind is offered in the editor's *Insert…* window: Search lists the things found by their title, Describe gives one for the window's preview and the card over its tag. `EmbedTemplate(ctx, templates, define, data)` renders one of the module's `templates/blocks/*.html` defines for it |
| `SetPluginBlockData(id, fn)` | Its block's data: `fn(ctx, modulekit.BlockPlacement)` runs with every refresh of the blocks' cache, for each placement; the template gets the result as `.Data` (templates don't query the database) |
| `AddViewSort(s)`, `AddViewFilter(f)`, `AddViewField(f)` | A sort criterion, a filter or a field of content Views (below) |
| `Deps.Comments.OnNodeCommentsChanged(module, fn)` | A call after a comment of a content item is created, edited or deleted |
| `Deps.Comments.OnCommentPosted(module, fn)` | A call with each new published comment (`modulekit.Comment`: its content item `Nid`, author `CreatedBy`, `ParentID` of a reply), after it is saved; an error is logged |
| `AddFormSection(s)` | A section of a core admin form: `FormSection{Module, Form, Load, Save}` - `Form` names it (`FormMenuLink` - a menu link's, `FormBlock` - a custom block's, `FormBlockPlacement` - a block placement's, on a custom block's page too, `FormTerm` - a taxonomy term's, `FormSiteInformation` - Site information's, `FormView` - a View's settings modal (Bootstrap's form layout), `FormField` - a content type's field's), `Load(ctx, ref)` gives its data for the thing (`ref` - its id; nil - nothing to show), `Save(ctx, ref, r)` stores the submitted form once the form saved the thing (its error fails the save). Its template is `form_<Form>_<module>`, reading `.moduleForms.<Form>.<module>`, rows like the form's own (`admin-setting`) |
| `SetNodeWorkflow(w)` | The site's editorial workflow (one module): `NodeWorkflow{Module, Handles, Load, Validate, Apply}`. The items of the types it `Handles` are published by it, not by their form: the form shows its `node_workflow_<module>` template (in place of the publishing state and the save button) from `.nodeWorkflow`, what `Load(ctx, type, node, user)` returns; `Validate` - why the transition submitted can't be made (shown with the form's errors); `Apply(ctx, node, user, r, isNew)` - after the form saved the item, in its transaction: the transition and the item's publishing. The items' own publish/unpublish buttons are left out, and the core's own publishing - bulk actions, the publish routes, the schedule - leaves them alone |
| `OnNodeFormSaved(module, fn)` | A call after a content item's own form saved it, created or edited (`fn(ctx, node, isNew)`, `node` with its `ID` and `Type`), in the save's transaction: its error fails the save. Changes made elsewhere - the bulk actions, publishing, the schedule, a copy - don't call it |
| `OnNodePublished(module, fn)` | A call when a content item becomes public (published and not on moderation): created so, or by its form, its publishing, the bulk actions or the schedule. Not again while it stays public; an error is logged |

The templates of hooks are `{{define}}`s in the module's
`templates/partials/<module>_hooks.html` (a path no other module uses),
for example `{{define "page_end_hello"}}` for the end of every page. A
block's template is `{{define "block_<id>"}}` in `templates/blocks/`.
A settings section's template (`AddContentTypeSettings`,
`AddNodeSettings`, `AddUserSettingsSection`,
`AddAccountSettingsSection`) reads what its `Load` returned as
`.moduleSettings.<module>`.
The core's templates call these hooks (`{{moduleHook "<hook>" .}}`
renders `<hook>_<module>` of every enabled module, with the page's data):

| Hook | Where |
|---|---|
| `head` | `<head>` of every page of the site (not the admin's), after the canonical address - e.g. `<link rel="alternate">` tags |
| `page_end` | The end of every page of the site (not the admin's), before the footer |
| `node_workflow` | A content item's form, the side column's first card - the workflow's state and transitions (`SetNodeWorkflow`) |
| `node_form_top` | A content item's form, over its fields - a note about the item (the page's data) |
| `node_notice` | A content item's page, over the item, where its *Pending moderation* notice is - a note about the item (`.moduleNode.<module>` of `AddNodePage`), e.g. the editorial workflow's state for its author and editors |
| `node_links`, `node_actions` | A content item's page: under the item, next to *Edit* (`AddNodePage`). Something bigger on the page (a gallery, a list) is a plugin block placed in a theme region |
| `comments_actions` | A content item's comments, under their heading - e.g. a subscription button (`.moduleNode.<module>` of `AddNodePage`) |
| `comments` | A content item's comments, in place of the core's - only the module whose `AddNodePage` part says `Comments: true` for the item; `.commentsModule` is its name |
| `comment_links` | A comment's links, after *Reply*, *Edit*, *Delete* - an `<li>` each, e.g. *Report*. The data: `.Comment`, `.currentUser`, `.Held` (its author's own, waiting for approval) |
| `term_actions` | A term's page, under its title (`.moduleTerm.<module>` of `AddTermPage`) |
| `content_type_settings`, `node_settings`, `user_settings`, `account_settings` | The content type form, the content item form, the profile form, the *Account settings* page (the settings sections above) |
| `form_<Form>` | A core admin form with module sections (`AddFormSection`): `form_menu_link`, `form_block`, `form_block_placement`, `form_term`, `form_site_information`, `form_view`, `form_field` |
| `user_menu` | Items of the user menu in the site's header (the *User menu* block), after *My profile*. An item with the `data-mobile-user-menu` attribute is copied to the mobile menu too |
| `admin_user_menu` | Items of the user menu in the admin's header |
| `redirects_tabs` | Tabs of the *Redirects* page next to its own (an `<li class="nav-item">` each); the page shows tabs only while a module adds one |
| `admin_people_actions` | Buttons of a user in the admin's users list, after *Profile*; the data is `dict "User" "Ctx"` (the user, the page's data) |
| `user_actions` | Actions about a user next to their name, the data being the user (`modulekit.User`) - e.g. "Send private message" by a forum topic's author. A module showing users calls `{{moduleHook "user_actions" <user>}}`; a module offering an action defines `user_actions_<module>` |

### A profile tab

`AddProfileTab(modulekit.ProfileTab{…})` gives the user profile a tab at
`/me/<Path>` (the visitor's own profile) and `/<profile>/<Path>`, after
the profile's own tabs and before the tabs of Views:

| Field | |
|---|---|
| `Module` | The module's name: the tab is there only while the module is enabled |
| `Path` | The URL segment. A View's profile tab can't take it; one saved with it earlier isn't listed while the module is on |
| `Label` | The tab's name in English (mark it with `i18n.N`), translated when shown |
| `Allowed(ctx, viewer, profile, isOwner, canAdminister)` | Whether the visitor (`viewer`, nil for a guest) may open the tab on `profile`'s page; `isOwner` - their own profile, `canAdminister` - they may administer users. False: the tab isn't listed and its URL answers 403, a guest is offered to log in. Nil: everyone may |
| `Badge(ctx, viewer)` | A number next to the label on the viewer's own profile (0 - none), such as their unread items |
| `Load(ctx, w, r, profile, data)` | Fills the page's data for the tab's content. The data has `profileUser` and a `pageSEO` (the tab's title, which `Load` may change, for example to `NoIndex`) |

The tab's content is the module's `{{define "profile_tab_<module>"}}`
(in `templates/partials/`), rendered with the page's data. The module's
own pages under the tab (`/me/<Path>/…`) are its routes.

### Sort criteria, filters and fields in Views

A module adds a value of its own to content Views - a rating, a number
of votes - from `Wire`. A View keeps the criterion by its `Key`,
`"<module>.<name>"` of at most 32 characters; the module computes the
values for the nodes of a list at once.

| Type | Fields |
|---|---|
| `modulekit.ViewSort` | `Module`, `Key`, `Label` (English, translated when shown), `Params`, `Values(ctx, ids, params)` - each node's number to sort by (a node left out counts as 0) |
| `modulekit.ViewFilter` | `Module`, `Key`, `Label`, `Values(ctx, ids)` - the nodes' numbers; a node left out has none. The View compares them with the filter's value: `eq`, `gte`, `lte`, `empty`, `not_empty`. A choice filter has `Options(ctx)` (its values, `modulekit.ViewOption{Value, Label}`) and `Choices(ctx, ids)` (each node's value) instead: `eq`, `in` (one of several), `empty`, `not_empty`; exposed, it is a select of its options |
| `modulekit.ViewField` | `Module`, `Key`, `Label`, `Params`, `Render(ctx, ids, params)` - each node's markup (`template.HTML`, shown as it is; a node left out shows nothing) |

`Params` are the criterion's or the field's settings in each View
(`modulekit.ViewParam` - the kinds of a plugin block's setting: a
number from `Min` to `Max`, a choice of `Options`, a line of text). The
View page shows them under the picker; `params` passed to `Values` and
`Render` hold every one - its value in the View, or its `Default`.

```go
modulekit.AddViewSort(modulekit.ViewSort{
	Module: "ratings",
	Key:    "ratings.weighted",
	Label:  i18n.N("Rating (weighted)"),
	Params: []modulekit.ViewParam{{
		Key: "min_votes", Label: i18n.N("Votes needed"),
		Type: modulekit.PluginBlockSettingNumber, Default: "5", Min: 1, Max: 1000,
	}},
	Values: func(ctx context.Context, ids []int, params map[string]string) (map[int]float64, error) {
		return repo.WeightedRatings(ctx, ids, params["min_votes"])
	},
})
```

A criterion sorts, and a filter narrows, every node the View's other
filters match (in memory, like a sort by a field), so keep `Values` one
query for all the `ids`. While the module is disabled its criteria are
neither offered nor applied; uninstalling it deletes them from the Views.

## Controllers and requests

- `modulekit.BaseController` (embed it), built with
  `modulekit.NewBaseController(d.Base)`: `GetCommonData` (a page's
  data; `AdminPageData` - an admin page's), `DBContext`,
  `RenderOrFail`, `RenderNotFound`, `RenderForbidden`,
  `RenderBadRequest`, `RenderLoadFailure`, `LogFailure`,
  `GetCurrentUser`, `AuthService()` (`HasPermission`, `CanEditNode`…),
  breadcrumb helpers (`SetPathTrail`, `SetNodeSubpageTrail`…).
- CSRF: the site checks the token of every state-changing request to a
  module's routes too (the `csrf_token` field or the `X-CSRF-Token`
  header): a handler doesn't check it again. The page's script adds it
  to the forms and to `fetch` requests; a form that must work without
  JavaScript puts it in a hidden `csrf_token` field
  (`csrf.EnsureToken(store, w, r)` in `pkg/csrf`).
- Sign-in on a module's routes, with `Routes.Store`:
  `AuthMiddleware(store)` (a guest is sent to `/login`),
  `AuthJSONMiddleware(store)` (a guest gets a JSON error),
  `OptionalAuthMiddleware(store)` (a guest goes on); then
  `ContextUserID(r.Context())` - the user's ID (in a page hook without
  the request, `AddNodePage`, `AddTermPage`, the hook's ctx), and the
  controller's `GetCurrentUser(r)` - the user. On the pages of a
  module's content type the core's access check has put the user and
  the item in the request: `RequestUser(r)`, `RequestNode(r)`.
- SEO: `PageSEOForList`, `PageSEOForPaginatedList`, `NodeMetaFromPageSEO`,
  `TruncateMeta`.
- Addresses: `PublicBaseURL(r)`, `MailBaseURL()`, `SiteDomain()`,
  `PublicFilesURL()`, `OriginalImage`, `StyledImage`, `NodeRelPath`;
  `StyledPicture`, `OriginalPicture` - an image's `Picture` for the
  template function `img` (with the style's 2× copy and its size);
  `SafeRedirectPath(p)` - `p` when it is a path of this site (a form's
  "back" field), else `""`.
- Languages: `SiteLanguages()` - the default language and the others in
  use (`SiteLanguage{Code, Prefix}`; none while the [Languages
  module](./builtin/languages.md) is off); `LocalPath(code, path)` - a
  path in a language's addresses (`/de/news` for `/news`). A request's
  language is `i18n.FromContext(r.Context())`, and its path comes
  without the prefix. `SetLanguagesSwitch(module)`, `ApplySiteLanguages`
  and `MoveDefaultLanguage` are the Languages module's own.
- Content: `ChangeNode(ctx, nodeID, user, change)` - a change of an item
  (its publishing, say) in one transaction, recorded in its history as
  the bulk actions' are (a revision, the action log, `OnNodePublished`).
  `CopyNode(ctx, nodeID, user, title)` - a copy of an item of
  the site's own content type, as its *Make a copy* makes it (field
  values, flags, view roles), titled `title` (`""` - "Copy of …"),
  unpublished, in the language of `ctx` (`i18n.WithLang`): its id;
  `ErrNodeNotCopyable` for a field set's item or a module's type's.
- The site's texts by language (the Languages module's own):
  `SetTextTranslator(module, fn)` - a text of a thing (`TextMenuLinkTitle`,
  `TextBlockTitle`, `TextPlacementTitle`, `TextTermName`,
  `TextTermDescription`, `TextViewTitle`, `TextViewDescription`,
  `TextFieldLabel`; ref - its id; `TextSiteName`, `TextSiteSlogan` -
  ref `SiteTextRef`) in a language;
  `SetLanguageVisibility(module, fn)` - whether a thing (`VisibilityMenuLink`,
  `VisibilityBlockPlacement`) shows on a page in a language;
  `SetNodeURLsInLanguage(module, fn)` - content items' addresses in a
  language. While the module is off, every text is as entered and every
  thing shows in each language.
- Lists: `SetContentListFilter(module, fn)` - what narrows every
  content View's items for a request (pages, blocks, the glossary) and
  the search's: `fn(ctx)` returns a condition on the nodes' query
  (`func(*entsql.Selector)`, `s.C("langcode")` a nodes' column) or nil.
  One module sets it (the Languages module, by the page's language).
- Public files: `PublicFiles()` - the store the public files are kept
  in (`pkg/filestore`), for a module keeping files of its own there,
  under a folder named after it. A file's key is its path under
  `PublicFilesURL()`: `Put(ctx, "mymodule/a.jpg", r, opts)` makes it
  public at `PublicFilesURL() + "mymodule/a.jpg"`. Write through the
  store, never to a directory: it may be an object storage.
- Visitors' files: `DocumentAllowed(name, head)` - the core's check of a
  file a visitor sends (its name, its first bytes): not an extension
  never accepted (pages, scripts, programs), not content a browser could
  run (HTML, SVG, scripts, executables) whatever the name.
- Private files: `PrivateFiles()` - the store of the files never served
  as they are (on disk or in a private bucket): a module keeps them
  under a folder named after it and hands one out itself, after its own
  access check (`Open` gives a file `http.ServeContent` serves).
- Field values: `FileIDs(v)` - the files of an image or file field's
  value (one, or several in order for a field with several images),
  `FirstFileID(v)` - the first of them.
- State: `ModuleEnabled(name)`, `NodeTypeEnabled(type)` (false for a
  disabled module's content type), `RequestTheme(ctx)`, `Captcha()`.
- Bots: `CheckBotTrap(r)` - an error when a form looks sent by a bot
  (the site's bot trap, on *Configuration → Comments*); the form's
  template puts the trap's fields in with `{{botTrap}}` (nothing while
  the trap is off).
- Addresses taken: `PathOwner(ctx, path, module)` - what else on the
  site answers `path`: `"routes"` (the engine's), `"content"`,
  `"views"`, `"terms"`, another module's name, `""` - nothing. A module
  letting the administrator choose an address checks it before saving.
- Feeds: `RSSDoc`, `RSSChannel`, `RSSItem`, `RSSPostURL`.
- E-mails: `UserMailTokens(baseURL, site, user)` - the `[site:…]` and
  `[user:…]` placeholders of an e-mail to the user (a
  `pkg/mailtemplate.TokenData`; the module's own go in its `Extra`, such
  as `"[pm:sender]"` → a name), then `Deps.Mail.Send(ctx, key, to,
  data)` sends the module's e-mail (`AddMail`): its text with them, unless
  it is switched off; `Deps.Mail.SendPlain(to, subject, body)` - a
  message as given (a form's notice); `UserStatusActive`.
- Content types: `ContentTypes.NodeTypeLabels(ctx)` - every type's label
  by machine name, the modules' content types too;
  `NodeTypesByLabel(labels)` - those types sorted by label, for a "Type"
  filter.
- The core's data types: `User`, `Node`, `ContentTypeDef`,
  `FieldInstanceDef`, `TaxonomyTerm`, `Comment`, `File`, `PageSEO` and
  the repositories and services behind `Deps`.

In templates, a module uses the engine's functions: `t` (translate),
`attach "<module>"` (load its CSS and JS on this page),
`moduleEnabled "<module>"`.

A module's admin page (a template under `templates/admin/`, its data
from the controller's `AdminPageData(r)`) gets the admin's layout and
its parts. A list row's actions are icons, as on every admin list: a
link or a button with the class `admin-icon-btn`
(`admin-icon-btn admin-icon-btn--danger` for deleting), the action's name
in `title` and `aria-label`, and the icon inside -
`{{template "admin_icon_edit"}}`, `{{template "admin_icon_delete"}}` or
`{{template "admin_icon_copy"}}`:

```html
<a href="/admin/config/example/{{.ID}}/edit" class="admin-icon-btn" title="{{t "Edit"}}" aria-label="{{t "Edit"}}">{{template "admin_icon_edit"}}</a>
```

A test rendering such a page with `testkit.RenderNamed` passes
`"admin/icons.html"` along.

## The database

A module's tables live in its own ent schema, with the conventions of
`schemakit`:

- `schemakit.Table(name)` - the table's name and, on MySQL, the site
  databases' collation (`utf8mb4_unicode_ci`).
- `schemakit.MySQLType(t)` - a column type on MySQL only;
  `schemakit.FixedChar(n)` - `CHAR(n)` on both databases.
- `schemakit.NotIncremental()` - an id set by the code (a settings
  table's single row).

Its client opens for each call on `modulekit.Driver(ctx, d.DB)`: the
site's database in its dialect, or the core's transaction `ctx` carries,
so a write of the module's in a hook running inside the core's change
(`OnNodeFormSaved`, the workflow's `Apply`) goes with it. Hand-written
SQL on `Deps.DB` takes `?` placeholders through `modulekit.Rebind`.
Other helpers: `modulekit.OrderRandom` (a random order on both
databases), `modulekit.IsDuplicateKey(err)`.

### Transactions with the core

`modulekit.InTx(ctx, db, fn)` runs `fn` in the core's unit of work (and
joins one already open in `ctx`). The core's repositories called with
`fn`'s `ctx` write in that transaction, and so does the module's own
client when it is opened on `modulekit.Driver(ctx, db)`:

```go
err := modulekit.InTx(ctx, d.DB, func(ctx context.Context) error {
	id, err := d.NodeRepo.CreateRow(ctx, node)
	if err != nil {
		return err
	}
	c := generated.NewClient(generated.Driver(modulekit.Driver(ctx, d.DB)))
	return c.Thing.Create().SetNodeID(id).Exec(ctx)
})
```

Outside a transaction, `modulekit.Driver` is the database itself. A
content item of a module's type and the module's own rows are created
this way as one whole.

### Stand-ins for the core's tables

A module's schema can't import the core's ent types. To have a foreign
key to a core table, describe that table in the module's schema as a
stand-in: a schema with the same table name and only its `id`, of the
core's column type. For example, the core's content items:

```go
// Node - the core's nodes table as the module's stand-in: its id only.
type Node struct {
	ent.Schema
}

func (Node) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("nodes")}
}

func (Node) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
	}
}
```

An edge to it gives the module's table the foreign key. A table of the
module's schema named as a core table is not the module's: its migrations
leave it out (it is never created or dropped with the module) and point
the foreign keys at the real table. Read the core's rows through the
core's repositories (`Deps.NodeRepo`, `Deps.Users`), not through the
stand-in. The core's content tables and what deleting an item does to a
module's rows: [Content model](../reference/content-model.md).

## Errors

- **"Not found."** The core's repositories and services return
  `modulekit.ErrEntityNotFound` for a missing record, wrapped or not:
  check it with `modulekit.IsNotFound(err)`. A module's generated ent
  package makes its own not-found error match it with a hand-written
  `Is` method against `schemakit.ErrEntityNotFound` (see
  [Write a module](./write-a-module.md#3-generate-the-ent-client)), and
  the module's own "not found" wraps `modulekit.ErrEntityNotFound`; then
  `RenderLoadFailure` answers 404 for it instead of 500.
- **Logging.** An error is logged once, where it is handled. A module's
  services and repositories wrap errors
  (`fmt.Errorf("adding a greeting: %w", err)`) and return them; the
  handler logs with `RenderLoadFailure` or `LogFailure`. A service that
  goes on with a default value logs with `modulekit.LogFallback` (debug
  for a missing record, else a warning).

## Demo content

`demokit.Register(name, fn)` sets the module's demo step, run by
`app install -demo` after the core's demo content. `fn` gets
`demokit.Env`: the site's language (`Lang`), the administrator
(`CreatedBy`), the demo users by name (`Users`), the demo articles
(`Articles`, the newest first), the site's theme (`Theme`), the menus
(`MainMenu`, `FooterMenu`; 0 - none) and `Now`;
`env.User(name)` - a demo user's ID; on a site installed with a demo
set, its content for the module (`demokit.Content`) and its nodes by key
(`env.Node(key)`). The core's rows a demo step adds through `demokit`'s
helpers, in the install's transaction.

- `demokit.Content(env, fsys, &v)` - the step's content: a demo set's
  ([Demo sets](../contributing/demo-sets.md)), else `demo/<lang>.json`.
- `demokit.Load(fsys, lang, &v)` - the texts from `demo/<lang>.json`
  (`demo/en.json` for a language without its own).
- `demokit.Driver(env)` - the install transaction for the module's own
  client.
- `demokit.CreateNode`, `demokit.CreateComments`, `demokit.ImageFile` -
  a demo content item of the module's type, its comments, a drawn
  picture.
- `demokit.SetPicture(ctx, env, rel, file, alt, at)` - a picture of the
  demo set's own, named in the module's content as `{file: …}` (a path in
  the set's directory, a theme's `demo/`): stored at `rel` with its file
  row. The set's check finds every `file` in the modules' content, so a
  module names its pictures that way.
- `demokit.Vocabulary`, `demokit.Term`, `demokit.IndexTerm` - a
  vocabulary, a term, an item filed under a term.
- `demokit.PlacePlugin(ctx, env, id, config, demokit.Placement{…})`,
  `demokit.MenuLink(ctx, env, menuID, title, url, nodeID, weight)` - a
  block in a region of the site's theme (`Placement{Region, Title,
  ShowTitle, Weight, Visibility}`), a menu link.
- `demokit.PlaceView(ctx, env, demokit.ViewBlock{…}, demokit.Placement{…})` -
  a block View of content (`ViewBlock{Name, Label, PageSize, Fields,
  Filters, Sorts}`) placed in a region. Its fields
  (`ViewBlockField{Builtin, Label, ShowLabel, LinkTo, Params}`), filters
  (`ViewBlockFilter{Builtin, Operator, Value}`) and sorts
  (`ViewBlockSort{Builtin, Direction, Params}`) name a built-in
  criterion in `Builtin` (`"title"`, `"status"`) or a module's
  (`"<module>.<key>"`, with its `Params`) - say, a top list with the
  module's own field and sort.

## File storages

A file storage keeps the site's files elsewhere than its disk - a
bucket, a container: the site chooses it with `files.storage` (the
public files) and `files.private_storage` (the private ones), not on
the Modules page, so it is a package of its own rather than a module. It
registers itself from its `init()`:

```go
func init() {
	modulekit.AddFileStorage(modulekit.FileStorage{
		Name:    "blob",      // the value of files.storage choosing it
		Public:  openPublic,  // func(modulekit.FileStorageConfig) (filestore.Store, publicURL string, error)
		Private: openPrivate, // func(modulekit.FileStorageConfig) (filestore.Store, error); nil - no private files
	})
}
```

A store is a `filestore.Store` (`pkg/filestore`); `filestore.CheckKey`
and `CheckDir` check the keys it is given, and a store that is a
`fmt.Stringer` names where it keeps the files in `app config`
(`blob://bucket/prefix/`). `Public` may return the address where the
storage serves the files itself (a bucket's public address, a CDN): the
site redirects its files' addresses there and checks it is an http(s)
one.

`FileStorageConfig.Decode(section, &settings)` reads the storage's
settings: the config file's `files.<section>` by the struct's `yaml`
tags, then the environment by its `env` tags, as the engine's own
settings. Keep keys out of the file (`yaml:"-"` with an `env` tag).
`PublicStorage` names where the public files are, so a private store can
refuse to share their place; `CacheControl` is the header of the public
files a store writes.

A storage's tests get a config with `testkit.FileStorageConfig(publicStorage,
sections)`, and a store passes the checks every store does with
`filestoretest.Run` (`pkg/filestore/filestoretest`). The S3 and Azure
storages are examples:
[`kosatka-storage-s3`](https://github.com/kosatka-cms/kosatka-storage-s3),
[`kosatka-storage-azure`](https://github.com/kosatka-cms/kosatka-storage-azure).

## Ways of sending email

The engine sends email through SMTP. A module adds another way - a mail
service's HTTP API - from its `init()`, next to its `Register`:

```go
modulekit.AddMailTransport(modulekit.MailTransport{
	Name:   "mailapi", // the saved choice: lower case, digits, _; at most 16
	Module: "mailapi", // the module it belongs to: it sends while the module is enabled
	Title:  "Mail API",
	Fields: []modulekit.MailTransportField{{
		Name: "api_key", Label: i18n.N("API key"), Secret: true, Required: true,
		Env: "MAILAPI_KEY", // set from the server's environment, the admin can't change it
	}},
	New: func(values map[string]string, from string, timeout time.Duration) (modulekit.MailSender, error) {
		return newSender(values["api_key"], from, timeout), nil
	},
})
```

The Mail page offers it while the module is enabled, with its fields: a
secret is never shown back, and an empty one keeps the saved value. The
values are kept with the site's mail settings, sealed with its key
(`SECRETS_KEY`). When every required field has its `Env` variable set
and the server's configuration has the sender address
(`EMAIL_FROM_ADDRESS`), the transport is the server's and wins over the
one saved in the admin. Without that address a field set in the
environment shows on the Mail page as set there, and the environment's
value is used once the admin chooses the transport.
`New` builds the sender (`SendPlain(to, subject, body)`); the sends go
through the site's mail service, so its switch, its log and its emails'
texts apply. Uninstalling the module forgets its settings, and a site
that sent through it goes back to the server's settings. The Resend
module is an example:
[`kosatka-module-resend`](https://github.com/kosatka-cms/kosatka-module-resend).

## Tests

`modulekit/testkit` has what a module's tests need, so they import
nothing of the engine but the SDK - the engine's own modules are tested
this way too.

**Rendering.** The engine's templates are read from its source tree:
`testkit.UseEngineRoot(t)` makes it the test's working directory (found
from testkit's own path - the engine's module in the module cache, for a
module from its own repository). Such a module's templates aren't in
that tree: `UseEngineRoot` serves them from the `Assets` of its
registration, as a site does (the test's package or one it imports
registers the module - a test of a subpackage imports the module's root,
`_ "<module path>"`). `testkit.UseModuleAssets(t, map[string]fs.FS{"<module>": assets})`
serves the given modules' embedded files only, even an engine module's.
Then:

| | |
|---|---|
| `RenderPageContent(t, page, data)`, `RenderPageBlock(t, page, block, data)` | A page template's block as the site renders it (the layout, the partials, the modules' hooks); `PageData()` - a page's data with no blocks placed, for rendering `layout.html` |
| `RenderNamed(t, name, data, files...)` | A template defined in files |
| `RenderPlacement(t, placement, pageData)` | A region with one block placed (a plugin block through the module's `block_<id>`) |
| `WithModules(t, map[string]bool{…})` | Modules switched on and off for the test (restored after it); it registers the modules first (`RegisterModules()`) |
| `UseTheme(t, themesDir, theme)`, `UseModuleAssets(t, assets)`, `ResolveTemplate(path)` | A theme, the embedded assets as on a site, where a template resolves |
| `UseImageStyleFormats(t, formats)`, `UseImageStyleVersions(t, versions)` | The image styles' output formats, their versions in the images' addresses (`?v=`) |

**Handlers.** `testkit.Templates()` is the site's template service for
`modulekit.BaseDeps.Templates`; `testkit.Auth{UserID, Allow,
Permissions}`, `testkit.Users{Known}` and `testkit.Nodes{Known}` stand in
for the core's services (any other method panics); `testkit.Session(t)`
gives a session with a CSRF token and `testkit.PostForm(path, form,
cookies)` a form posted with it; `testkit.ModuleMux(mux, module,
notFound)` is the module's mux as the site gives it (404 while the module
is off); `testkit.NodeDeleter(nodes, delete)` deletes content;
`testkit.CaptureActions(t)` - the actions recorded for the action log
during the test; `testkit.Logger()` writes nowhere.

**The database** (build tag `dbintegration`). `testkit.SchemaDB(t, fn)`
runs `fn` on a fresh database of each server named by
`KOSATKA_TEST_MYSQL_DSN` and `KOSATKA_TEST_POSTGRES_DSN` (a subtest
each; with neither set, the test is skipped), with the schema of the
core, the engine's modules and the modules the test binary imports
installed, their install steps too. `testkit.DB` has `DB` (`*sql.DB`)
and `Dialect`. `testkit.Deps(db)` is what `Wire` gets on a site, the
core's repositories and services on that database (`NodeRepo`, `Users`,
`Files`, `ContentTypes`, `ImageStyles`, `Taxonomy`, `Comments`, `DB`,
`Logger`; the other fields are nil): build the module from it as its
`Wire` does. `testkit.Comments(d)` - its comments as a test writes them
(`Create`, `Delete`: each change tells the modules' hooks).

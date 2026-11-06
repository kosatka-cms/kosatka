---
title: Modules
description: What a Kosatka module is, the modules the engine ships with, and how a site gets more.
sidebar:
  label: Overview
  order: 0
---

A module is a feature a site can have or go without. It brings its own
pages and addresses, database tables, admin pages, blocks, permissions
and sometimes content types. A site switches its modules on and off on
the **Modules** page of the admin (`/admin/config/modules`), and can
remove one completely there.

The core is always on: content and content types, users and roles,
taxonomy, Views, blocks, menus, text formats, image styles, SEO,
breadcrumbs, the sitemap, the site's RSS feed, maintenance mode and the
appearance settings are not modules.

## The engine's modules

These modules are built into the engine:

| Module | Name | What it does |
|---|---|---|
| [Polls](./builtin/polls.md) | `polls` | The *Poll* content type with voting, and a *Poll* block that shows a chosen poll |
| [Feeds](./builtin/feeds.md) | `feeds` | Configurable XML feeds of content at their own addresses (the site's RSS at `/rss.xml` is not affected) |
| [Galleries](./builtin/galleries.md) | `galleries` | Image galleries attached to content: a *Galleries* button and a block (*The content's galleries*) on content pages, gallery pages, an overview of the content's galleries (**Content → Galleries**) |
| [Error reports](./builtin/error-reports.md) | `error_reports` | Visitors select a mistake in the text and press Ctrl+Shift+E; the reports go to **Content → Error reports**, the new ones counted in the menu; an *Error report hint* block |
| [Forms](./builtin/forms.md) | `forms` | Forms with their own questions (feedback, applications, questionnaires): the *Form* content type, its submissions with CSV export, a *Form* block |
| [Private messages](./builtin/private-messages.md) | `private_messages` | Conversations between users: the *Messages* tab of the profile (`/me/messages`), the e-mail of a new message, the unread count in the user menu |
| [Subscriptions](./builtin/subscriptions.md) | `subscriptions` | Following the comments of content and terms: notifications of new comments, replies and new content on the *Notifications* tab of the profile (`/me/notifications`) and by e-mail, at once or as a daily digest |
| [Video embeds](./builtin/video-embeds.md) | `video_embed` | A link to a YouTube, VK Video, Rutube or Vimeo video on its own line in a text shows as the video: a preview, the player on a click; a *Video* button in the editor |
| [Action log](./builtin/action-log.md) | `action_log` | Who did what: content deleted or unpublished, users blocked, roles' permissions and settings changed, sign-ins (**Reports → Action log**); other modules add their own actions |
| [404 log](./builtin/not-found-log.md) | `not_found_log` | The addresses visitors asked for and the site didn't find, with how many times and where from; a redirect from any of them in a click (**Configuration → Redirects**, the *Not found (404)* tab) |
| [Languages](./builtin/languages.md) | `languages` | The site in several languages: each one besides the default at its own address prefix (`/de/…`) with the interface in it, a *Language switcher* block, `hreflang` links (**Configuration → Languages**) |
| [Ratings](./builtin/ratings.md) | `ratings` | Star ratings of content: votes on content pages (*The content's rating* block), the rating's sorts, filters and field in Views for top lists |
| [Complaints](./builtin/complaints.md) | `complaints` | Signed-in visitors report content and comments (a reason, details); the queue under **Content → Complaints**, the new ones counted in the menu; enough complaints hide the item until a moderator decides |

The name is the module's machine name: you type it to confirm an
uninstall and pass it to `app install -modules`. Each module's page
describes what it adds and how to use it.

## The modules a new site gets

A new site (`app install`) installs the standard modules: **Forms**,
**Private messages**, **Subscriptions**, **Video embeds**, **Action
log** and **404 log**. The others are there but not
installed; enabling one on the Modules page installs it. To choose a
different set at install time, pass `-modules` (see
[Choose the modules of a new site](./using-modules.md#choose-the-modules-of-a-new-site)).

## Enable, disable, uninstall

- **Enable.** Tick the module on the Modules page and save. The first
  time, this installs it: its tables are created and its starter data is
  written.
- **Disable.** Untick it. Its pages answer 404, its links, blocks and
  permissions are hidden, and content of its types doesn't open. Its
  data stays in the database: enabling it again brings everything back.
- **Uninstall.** A disabled module can be removed with all its data: its
  content (with comments and files nothing else uses), its tables, its
  blocks' placements, its permissions in roles and the settings of its
  content types. This can't be undone.

The details and the warnings are in [Using modules](./using-modules.md).

Some smaller features are switched on and off in their own settings
rather than on the Modules page: the back-to-top button
(**Appearance**) and search (**Configuration → Search and SEO →
Search**).

## Modules from their own repositories

A module doesn't have to be part of the engine. A module in its own Go
module registers itself when it is imported, and a site that wants it
builds its own binary: a small `main` package that calls
`kosatka.Main()` and imports the module. `app module add` writes that
import. Once the site runs the rebuilt binary, the module shows on the
Modules page like any other, marked *from its own repository*.

The repository has a working example: the module
[`examples/hello`](https://github.com/kosatka-cms/kosatka/tree/main/examples/hello)
and the site
[`examples/hello-site`](https://github.com/kosatka-cms/kosatka/tree/main/examples/hello-site)
that uses it.

Modules of the Kosatka project in their own repositories, each with its
documentation in its README:

| Module | Name | What it does |
|---|---|---|
| [Forum](https://github.com/kosatka-cms/kosatka-module-forum) | `forum` | Forum sections and topics under `/forum`, its settings page and the *Forum* topic content type |
| [Quotes](https://github.com/kosatka-cms/kosatka-module-quotes) | `quotes` | Quotes and their authors: the `/quotes` pages and a *Random quote* block |
| [Editorial workflow](https://github.com/kosatka-cms/kosatka-module-workflow) | `workflow` | States of content (draft, in review, approved, published, archived or your own) and the transitions between them by role, with comments; the editorial queue (**Content → Editorial**) |
| [Resend](https://github.com/kosatka-cms/kosatka-module-resend) | `resend` | The site's email through the [Resend](https://resend.com) API: a way of sending on the Mail page besides SMTP ([Email](../guides/mail.md#sending)) |

## File storages

The file storages - [`kosatka-storage-s3`](https://github.com/kosatka-cms/kosatka-storage-s3)
and [`kosatka-storage-azure`](https://github.com/kosatka-cms/kosatka-storage-azure) -
also come from their own repositories and get into the site's binary
with `app module add`, but they aren't modules: the config chooses one
(`files.storage`) before the site reaches its database, so a storage
isn't on the Modules page and can't be switched off there - hence the
other name. Which there are and how to set one up: [Files in object
storage](../guides/file-storage.md).

## Where to go next

- [Using modules](./using-modules.md) - turning modules on and off,
  uninstalling, adding a module from its own repository to a site.
- [Write a module](./write-a-module.md) - a step-by-step tutorial with
  the module SDK.
- [The module SDK](./sdk.md) - the packages a module is built on and
  what it can add to a site.
- [The app commands](../reference/cli.md) - `app install`,
  `app migrate`, `app module`, `app assets`.

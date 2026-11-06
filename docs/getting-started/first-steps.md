---
title: After installation
description: Signing in, the admin and the first settings of a new site.
sidebar:
  order: 4
---

## Sign in

Open `/login` and sign in as the administrator with the password
`app install` printed. Then change it: your profile's *Edit* page
(`/me/edit`) asks for the current password and takes a new one.

## The admin

`/admin` opens the admin on its **Overview**:

- the figures - the published content (and how much of it in the last 7
  days), the comments and the new users of the last 7 days, the views
  today (while views are counted); the users' figure only for who
  manages users. Each opens its list;
- *Needs attention* - every admin menu item with something waiting:
  content on moderation, new error reports, new 404 addresses and the
  like - or *Nothing is waiting*;
- the recent changes of the content (for who may see its history, with
  a link to **Content → Changes**), today's most viewed content, the
  latest comments;
- the modules' blocks, for example the action log's recent actions.

Each part is shown only to who may open what it lists. The main places:

| Where | What's there |
|---|---|
| `/admin/content` | the content: create, edit, publish, delete |
| `/admin/config/system/site-information` | the site's name and slogan, address, e-mail, language, time zone, logo and icon, the front page's list, the 403 and 404 pages, who runs the site (for search engines) |
| `/admin/config/content-types` | the content types and their fields |
| `/admin/config/taxonomy` | the vocabularies and their terms |
| `/admin/config/views` | the Views: lists shown as pages, blocks, under content, on term pages, on profiles |
| `/admin/config/blocks` | the blocks in the theme's regions |
| `/admin/config/menus` | the menus |
| `/admin/config/appearance` | the themes and their settings |
| `/admin/config/modules` | the modules: enable, disable, uninstall |
| `/admin/people` | the users, their roles and permissions |
| `/admin/config/mail` | how the site sends e-mail |
| `/admin/reports/status` | the status report: the version, the database, what needs attention |

## First settings

1. **Site information.** The site's name, its address (used in absolute
   links, feeds and the sitemap; `public_base_url` in the config overrides
   it), e-mail, language, time zone and logo. Without the address no
   e-mail with a link is sent (registration, password resets,
   notifications).
2. **E-mail.** Password resets and e-mail verification need it: set up
   SMTP on the Mail page (or a mail service's API with its module, such
   as Resend's), or in the config (`email`); see [Email](../guides/mail.md).
3. **Modules.** A new site has the standard ones (forms, private
   messages, subscriptions, video embeds, the action log and the 404
   log). Enabling another module on the Modules page installs it:
   its tables and its starter settings. Each
   [module's page](../modules/index.md) says how to use it.
4. **Appearance.** The engine ships the `kosatka` theme (the default)
   and its own templates (*Base*). Others, such as Respawn and C64, are installed from
   their own repositories with [`app theme add`](../reference/cli.md#app-theme).
5. **Content types.** A new site has *Article* and *Page*. Add your own
   types and fields, then the Views that list them.

The status report (and a warning bar in the admin) shows what still
needs attention: database migrations not applied yet, modules installed
in the database but missing from the binary, a public files directory
(or bucket) the site can't write to, the site's address or e-mail not
set up, a MySQL session that isn't strict, the background cache refresh
or the background jobs (the scheduled publishing) falling behind, the
enabled modules' own checks.

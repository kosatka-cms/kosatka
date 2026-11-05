---
title: 404 log
description: The addresses visitors asked for and the site didn't find - how many times, when last, where from - with a redirect from any of them in a click.
sidebar:
  order: 13
---

The *404 log* module (`not_found_log`) collects the addresses visitors
asked for and the site didn't find (answered with "404 Not found"): old
links from other sites and search engines after content moved, mistyped
links in your own texts, pages removed. Each address shows how many
times it was asked for, when first and last, and where the last visitor
came from. From any of them you add a redirect in a click.

A new site gets the module installed and enabled (it is one of the
standard modules of `app install`). On a site created before the engine
had it, it starts disabled: enable it on the **Modules** page (see
[Using modules](../using-modules.md)). The log starts with the requests
made after that.

## Where it is

The log is a tab of the redirects page: **Configuration → Redirects**,
*Not found (404)* (`/admin/config/redirects/not-found`). The page's
other tab, *Redirects*, is the redirects page itself: adding a redirect,
the bulk import and the list. The number on the *Not found (404)* tab is
how many new addresses wait. The same number is shown by the *Redirects*
menu item and under *Needs attention* on the admin's Overview, unless
the settings turn it off.

Like the redirects page, the log needs the *Access administration
(/admin)* permission.

## What gets in

An address gets in when a page request (`GET` or `HEAD`) is answered
404. Not in it:

- addresses a redirect answers - they never reach the "not found" page;
- requests the site blocks (blocked addresses and bots);
- addresses matching the settings' patterns: what scanners look for on
  any site (`*.php`, `/wp-*`, `/.env*`, `/.git*` and the like) is there
  from the start;
- addresses longer than 1024 characters.

The query string isn't part of the address: `/news?page=2` and `/news`
are the same address, like for redirects.

## Working with it

The list shows the new addresses, the most asked for first, 50 to a
page. The filter finds them by a part of the address and shows the new,
the ignored or all of them.

- **Came from** - the page the last visitor came from: another site's
  (a link that opens it) or one of this site's own pages, marked *on this
  site* - a broken link in your own text, worth fixing there; *direct* -
  the browser didn't say.
- **Redirect** opens the form under the row: *From* is the address, *To* -
  where to send it, a path on the site (`/articles/new-slug`) or a full
  address. Below it the content whose title matches the address's last
  word is offered: a click puts its address in *To*. *Add the redirect*
  adds an ordinary redirect - it is on the *Redirects* tab like any other
  and can be deleted there - and the address leaves the log.
- **Ignore** moves the address out of the new ones (it is still counted,
  under *Ignored*); **Restore** brings it back.

An address that got a redirect any other way (the *Redirects* tab, the
import, a changed content address) leaves the list at once and the log
within an hour.

## Settings

**Log settings** on the log's page (`/admin/config/redirects/not-found/settings`):

| Setting | |
|---|---|
| *Forget an address not asked for in* | 30, 90 (the default), 180 or 365 days. A background job forgets older addresses once an hour, and those a redirect answers now |
| *Count in the menu* | The number of new addresses by the *Redirects* menu item (and on the Overview). On by default |
| *Address patterns* | Addresses never logged, one pattern per line; `*` is any characters, letter case aside, and the pattern covers the whole address: `*.php` - any address ending in `.php`, `/wp-*` - any starting with `/wp-` |

*Clear the log* forgets every address, the ignored ones too; the
redirects and the settings stay.

The log keeps up to 10,000 addresses: anyone can ask for made-up ones.
A full log still counts the addresses it has and adds new ones again
once old ones are forgotten or the log is cleared.

## For module authors

The log listens to the core's `OnNotFound` hook, adds its tab through
the `redirects_tabs` template hook and its redirects through
`Deps.Redirects` - all three are open to any module, see
[The module SDK](../sdk.md).

## Uninstalling

Uninstalling the module deletes its tables with the log and its
settings. The redirects added from it stay: they are the site's.

---
title: Feeds
description: XML feeds of your content at their own addresses, for news aggregators and RSS readers, each with its own content and field mapping.
sidebar:
  order: 4
---

The **Feeds** module (`feeds`) publishes XML feeds of the site's content
at addresses you choose, for example `/feeds/news.xml`. Each feed has its
own set of content types, an optional filter by terms, a format and, for
each content type, the fields that give an item's text, cover and
category.

The site's own RSS feed at `/rss.xml` is part of the core and isn't
affected by the module: it always lists the View chosen as the *Front
page feed* in **Site information** (see
[Addresses and SEO](../../guides/urls-and-seo.md#the-rss-feed)). Use
this module when you need more feeds: one per section, a full-text feed
for an aggregator, a feed with a different cover image.

A new site doesn't install the module. Enable it on the **Modules** page
(see [Using modules](../using-modules.md)).

## What it adds

- **Configuration → Modules → Feeds** in the admin
  (`/admin/config/feeds`): the list of feeds and their settings. Like
  most admin pages, it needs the *Access administration (/admin)*
  permission; the module adds no permissions of its own.
- The feeds themselves, each at its own address. Anyone can open an
  enabled feed, signed in or not.

There are no blocks and no content types.

## The list of feeds

The *Feeds* page shows each feed with its *Name* (and under it the
feed's address, a link that opens the feed), *Format*, *Types* (the
machine names of its content types) and *Status* (*enabled* or
*disabled*). The name or the pencil (*Edit*) opens the feed's form; the
bin (*Delete*) removes it after a confirmation. **Add feed** opens an
empty form.

## Create a feed

1. Open **Feeds** and click **Add feed**.
2. Fill in the top of the form:

   | Field | What it does |
   |---|---|
   | Name | The feed's name in the admin. Required. |
   | Path | The feed's address without the leading `/`, for example `feeds/news.xml`. Required. |
   | Format | *Dzen (full text)* (the default) or *RSS 2.0*; see [Formats](#formats). |
   | Feed enabled | Ticked by default. An unticked feed keeps its settings, but its address answers 404. |

   The path may contain Latin letters, digits, `-`, `_`, `.` and `/`; it
   is saved in lower case, without slashes at the ends. The form refuses
   an address something else on the site answers: the engine (such as
   `/rss.xml`), a content item, a View page, a term page or another
   feed. A View page can't take an enabled feed's address either. A form
   that can't be saved comes back with what you typed.

3. Under *Content items*, choose what goes in:

   | Field | What it does |
   |---|---|
   | Number of items | At most this many items, 1 to 500. Empty or `0`: 50. |
   | Not older than, days | Only items created within this many days. `0`: the format's default (*Dzen (full text)*: 7 days, *RSS 2.0*: no limit). |
   | Only terms (IDs) | Term IDs separated by commas: only content tagged with any of them. Empty: all content of the selected types. |

   A term's ID is the number in the address of its edit page,
   `/admin/config/taxonomy/{vid}/terms/{id}/edit` (see
   [Taxonomy](../../guides/taxonomy.md)).

4. In the table below, tick the content types the feed includes (at
   least one) and, for each ticked type, choose the fields. The table
   lists the site's content types (**Structure → Content types**); a
   module's own type, such as polls or forum topics, can't be in a feed.

   | Column | Choices | What it gives |
   |---|---|---|
   | Text | *- main text -* or any `text` field of the type | The item's text. *- main text -* is the item's body, the first text field shown "in article body" (see [The content page](../../guides/content-types.md#the-content-page)). |
   | Cover | *- None -* or an `image` field | The item's cover image. |
   | Category | *- None -* or a `taxonomy` field | The item's categories: the names of its terms in that field. Only *Dzen (full text)* uses it. |

   The choices of unticked types are ignored.

5. Under *Images*, choose the *Image style* for the cover and the
   images inside the text. *- originals -* uses the uploaded files as
   they are. The list shows each style's format, JPEG or WebP; Dzen
   takes only JPEG, GIF and PNG, so choose a JPEG style for it (see
   [Image styles](../../guides/files-and-images.md#image-styles)).
6. Under *Channel*, optionally set the feed's own texts:

   | Field | Empty means |
   |---|---|
   | Channel title | The site's name. |
   | Description | The site's slogan (or its name, if there is no slogan). |
   | Language | The site's language. Enter a language code, such as `en`. |

7. Click **Save**. The form opens again with *Saved.*; *Open feed*
   beside **Save** opens the feed.

If something is wrong, the form says what, for example *Enter the feed
name.*, *This address is already used on the site: by the engine,
content, a View page, a term page or another feed.* or *Select at least
one content type.*

## What goes in a feed

A feed lists published content of its types that isn't waiting for
moderation, newest first by creation date, within its day window and
term filter, up to its number of items. It is the same for everyone who
opens it, so it lists only what a guest may see: content closed to
guests (a [closed section](../../guides/security.md#closed-sections) or
a closed item) is left out. Each item links to the content's
page, and its date is the creation date. Links and images use the site's
address (see
[Addresses and SEO](../../guides/urls-and-seo.md#the-sites-address)),
or the address of the request when it isn't set.

## Formats

**Dzen (full text)** follows the export format of the Dzen aggregator
and suits any service that wants whole articles. Each item has:

- the title, the link (also its unique ID) and the date;
- the full text from the *Text* field, with relative links and image
  addresses made absolute, local images switched to the feed's image
  style, and the "read more" marker removed;
- the cover from the *Cover* field, if the item has one;
- a category for each term in the *Category* field.

The channel's description is reduced to plain text on one line.

**RSS 2.0** is a plain feed for RSS readers. Each item has the title,
the link, the date, the beginning of the text as plain text (up to 600
characters) and the cover, if any. The *Category* column isn't used.

Both are served as `application/rss+xml`. A browser, a CDN or a proxy
may keep a copy of a feed for up to 10 minutes, so a change can take
that long to show.

## Disabling and uninstalling

While the module is disabled, its feeds' addresses answer 404 and the
*Feeds* page is hidden; the feeds are kept and come back when you enable
it again.

The module keeps its feeds in its own tables. Uninstalling it deletes
those tables, that is, every feed and its settings; your content isn't
touched. See [Using modules](../using-modules.md#uninstall-a-module).

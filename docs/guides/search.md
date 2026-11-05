---
title: Search
description: Setting up what the site search finds and how its results look, the search block, the results page and the live search API.
sidebar:
  order: 8
---

The site search finds published content by its title and by the text
fields you choose. It has two faces: the **live search**, a drop-down of
results under the search box while the visitor types, and the **results
page** (`/search?q=…`) the box opens when submitted. Both search the same
content types, set up on one admin page.

The search looks for the words of the query in the site's own index of
words (see [The search index](#the-search-index)), in any order, by
their stems: "cats" finds "cat", "running" finds "run". Case doesn't
matter, and neither do "ё" and "е". A query is cut to 100 characters. A visitor's address may search 20 times in a row, then once
a second (the live search and the results page together).

## Set up the search

Open **Configuration → Search and SEO → Search** (`/admin/config/search`).

Every content type and module's type (a forum topic, a poll) is
searched from the start, and so is a type added later: by its title and
all its text fields, with the default settings. The table lists them; a type not set up yet comes
after the others, by name. A content type that redirects to the address
from a field ([Content types](./content-types.md)) isn't searched until
you add it.

- The pencil (*Change*) opens a type's form: what it searches and how
  its results look (see below). *Save* keeps them.
- *Remove type from search*, at the top of the form beside *Save*,
  leaves the type out. Its settings are kept: choose it under *Types in
  search* and click *Add to search* to search it again.
- Drag the rows into the order the result groups should come in and
  click *Save order*.

### A type's settings

| Setting | What it does |
|---|---|
| Limit | how many items of the type the field search may bring (5 for a new type). The title matches are added on top of it. `0` turns the field search off for the type: it is then found by title only |
| Label in results | the type's name shown on its results (the type's name by default) |
| Also search in fields | the type's text fields searched besides the title: *All text fields* (the default for a type added to the search; the fields the type gets later too), or *Only the ticked ones* - none ticked, the title only. The title is always searched |
| Result title | the text field shown as the result's title; by default the content's title |
| Second line | a text field shown under the title |
| Image | the image field shown with the result (an item without a value shows the field's default image). By default none: the results show the *Image for results without their own* |
| Image style | the [image style](./files-and-images.md) of that picture; by default the original |
| Badge: vocabulary terms | a vocabulary whose terms on the item are shown on the result, for example its section |

A module's own type has no fields to choose: it is searched by title.

### General settings

| Setting | What it does |
|---|---|
| Live search on the site | the search on or off. Off, the search block is hidden and `/search` and `/api/search` answer 404. The content search in the admin's forms (choosing content for a reference field) keeps working |
| Image for results without their own | the address of an image shown on results that have no image |
| Links before a query is entered: menu | a [menu](./menus.md) whose top-level links the search block shows under its field while the query is shorter than 3 characters: shortcuts to the site's sections ([the live search API](#the-live-search-api) returns them too) |

## How results are found

A query needs at least 3 characters. It is split into words (letters
and digits: punctuation and markup aren't words); each of them must be
found, in any order, by its stem in one of the site's languages. The
last word may also be the beginning of a word: the visitor may still be
typing it, so "summ" finds "summer". Words of one letter are skipped,
and only the first 8 words count. Then:

1. For each type with fields to search and a limit above 0: the items
   with every word in the title or in one of the chosen fields, up to
   the type's limit.
2. Then one search by title across all the searched types: the items
   with every word in the title, added to those found.
3. The results are grouped by type, in the list's order. Inside a type,
   the items with the whole query in the title come first, then those
   with all the words in the title, then the rest.

Only published content that isn't in moderation is found, and only what
the visitor may view: a [closed section](./security.md#closed-sections)
is left out for those it is closed to. Of many equally good matches,
the newest are taken. Content of a disabled module's types is skipped.

The live search shows at most 50 results. The results page raises each
type's limit (up to 200 results in all) and shows them 20 per page.

## The search index

The site keeps the words of each content item's title and of its text
fields' values (the text, not the HTML): they are written as the item
is saved, from its form or by a module, so a new item is found at once.
The words are kept by their stems in the item's language (the site's
language for an item without one). Stems are known for Arabic, Danish,
Dutch, English, Finnish, French, German, Hungarian, Irish, Italian,
Norwegian, Portuguese, Romanian, Russian, Spanish, Swedish, Tamil and
Turkish; in another language the words are kept and found whole.

All the text fields are in the index, so choosing a type's *Also search
in fields* takes effect at once. When content was changed past the site
(rows imported into the database, say), write the index again with
[`app search reindex`](../reference/cli.md#app-search).

## The search block

The search box is a system block. To put it on the site:

1. Open **Structure → Blocks** (`/admin/config/blocks`).
2. Under *Add system block*, choose *Search the site* and the region, for
   example the header or the sidebar.

See [Blocks and regions](./blocks.md) for regions, order and visibility.

The block shows the live results as the visitor types (after a short
pause) and opens the results page on *Search* or Enter. A theme may style
it or ship its own version of the block.

## The results page

`/search?q=<query>` shows the results with their title, second line,
image, badge and type label, and pages through them (`&page=2`). A
result found by a type's *Also search in fields* shows a snippet: about
200 characters of the first of those fields that has the query's words,
around them, the words highlighted. It has
*Home › Search* breadcrumbs, or the trail of a menu link to `/search` if
one exists.

Search results aren't meant for search engines: the page is marked
`noindex, follow` and `robots.txt` disallows it (see
[Addresses and SEO](./urls-and-seo.md)).

## The live search API

The block's drop-down uses `GET /api/search?q=<query>`, which a theme can
use too. It answers JSON:

- a query shorter than 3 characters - the menu links from the general
  settings:

  ```json
  {"categories": [{"label": "Events", "url": "/events"}]}
  ```

- otherwise - up to 50 results:

  ```json
  {"results": [{"id": 42, "type": "event", "url": "/event/summer-fair",
    "title": "Summer Fair", "subtitle": "City park", "image_url": "…",
    "badge": "Outdoor", "type_label": "Events"}]}
  ```

  `badge` is the names of the item's terms from the badge vocabulary,
  separated by ` / ` (empty without one).

## Permissions

The search needs no permission: anyone, guests included, can use it, and
it only ever finds published content the visitor may view. Setting it up
takes access to the admin.

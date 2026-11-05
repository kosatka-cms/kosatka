---
title: Taxonomy
description: Vocabularies and terms, attaching them to content, term pages and the tag cloud.
sidebar:
  order: 2
---

Taxonomy classifies content. A **vocabulary** is a set of terms - for
example "Sections" (News, Reviews, Guides) or "Tags" (free keywords). A
**term** is one entry of a vocabulary. Terms can form a tree, can have
their own public page listing the content tagged with them, and are
attached to content through a taxonomy field of the content type.

Vocabularies and terms are managed in the admin under **Structure →
Taxonomy** (`/admin/config/taxonomy`). A new site has no vocabularies;
with `app install -demo` it gets two example ones.

## Create a vocabulary

1. Open `/admin/config/taxonomy`.
2. Under *Add vocabulary*, fill in:
   - **Name** - for example "Tags". Required.
   - **Slug (optional)** - the vocabulary's identifier; empty - generated
     from the name. It must be unique.
   - **Term path prefix** - the first part of new terms' addresses, for
     example `tags`. Empty - taken from the vocabulary name.
   - Under *More*: **Description** and **Weight** (optional), and
     **Term pages** - check it if terms should have public pages (off
     by default).
3. Click *Add*.

The vocabulary list can be reordered by dragging rows by ☰; click *Save
order*. A vocabulary's name and its *Terms* button open its terms.

## Vocabulary settings

Open a vocabulary's settings with the pencil icon in the list, or with
*Vocabulary settings* on its terms page
(`/admin/config/taxonomy/{id}/edit`).

| Setting | What it does |
|---|---|
| Name | The vocabulary's label. |
| Address | Its slug: the unique identifier set as *Slug* when it was added. |
| Term path prefix | New terms get the address `/<prefix>/<name>` unless one is typed by hand. Empty - a prefix from the vocabulary name. Changing it doesn't change existing terms' addresses. |
| Public term pages | Each term has a page at its address listing its content. Off - term addresses return 404: the vocabulary is for classification only. |
| Description | A note for administrators. |
| Weight | The position in the vocabulary list. |

A vocabulary can be deleted only when it has no terms.

## Terms

The *Terms* button of a vocabulary (`/admin/config/taxonomy/{vid}/terms`)
lists its terms as a tree.

To add a term, fill in the form above the list and click *Add*; the
last three fields are under *More*:

| Field | What it does |
|---|---|
| Name | The term's name. Required. |
| Parent | The parent term; *- no parent -* makes it a top-level term. |
| Path | The term's address, for example `tags/go`. Empty - the vocabulary prefix plus the name, transliterated (`tags/go`, then `tags/go-1` if it's taken). |
| Description | Shown on the term's page above its content list. |
| Weight | The order among terms with the same parent. |

The term's own page (click its name) has the same fields, the path
labelled *Address*, plus **Page title**: the heading of the term page and
the `[term:title]` token in the
[SEO templates](./urls-and-seo.md#term-pages). Empty - the name.
Clearing the *Address* there gives the term a new address the same way
as on the add form (its current address doesn't count as taken). The
*Parent* can't be the term itself or one of its own children,
grandchildren and so on.

### The tree

Drag a term by ☰ to change its order or its parent, then click *Save
order*:

- drop it on the top third of a row - before that term;
- on the bottom third - after it, on the same level;
- in the middle - as its child.

A term moves with its children. You can also pick the parent in the
*Parent* column; the row moves by itself.

A term's page shows only the content tagged with that term, not with its
children.

### Delete a term

A term can be deleted only when it has no child terms and no content is
tagged with it. The confirmation page says what is in the way: the child
terms, or the number of content items using it (published or not). Move
or delete the children, and remove the term from that content first.

## Attach a vocabulary to a content type

Terms are set on content only through a field of type `taxonomy`.

1. Open the content type's *Fields* tab
   (`/admin/config/content-types/{id}/fields`).
2. Add a field of type `taxonomy`. Check **Multiple values** if an item
   can have several terms (for tags), leave it off for one (for a
   section).
3. On the field's page, choose the **Vocabulary** and the **Widget**, and
   click *Save*.

| Widget | Values | Looks like | New terms |
|---|---|---|---|
| `select` | one or several | a drop-down (a multi-select for several) | no |
| `radio` | one only | radio buttons | no |
| `checkboxes` | several only | checkboxes | no |
| `autocomplete` | several only | a tag input with suggestions | yes |

`select` is the default for one value, `autocomplete` for several. All
widgets but autocomplete show the terms indented by the tree.

With **autocomplete**, editors type terms and pick suggestions; a name
that isn't in the vocabulary yet becomes a new term when the content is
saved. If a term with that name already exists (ignoring case), it's
reused. A form sent back with errors creates no terms: the names typed
stay in the field. New terms are created at the top level; give them a
parent later on the *Terms* page.

The vocabulary and the number of values are shared by every content type
the field is attached to; the widget is set per type. See
[Content types and fields](./content-types.md) for the rest of the field
settings and for changing one value to several.

On the content page, a taxonomy field shows its terms as links to their
term pages; the terms of a vocabulary without public term pages are
shown as plain text. A taxonomy field can also be part of the type's
[URL pattern](./content-types.md#url-pattern):
`[node:field:field_section]/[node:title]` gives addresses like
`/reviews/new-phone`.

## Term pages

When a vocabulary has *Public term pages* on, each of its terms is served
at its own address: a term with the path `tags/go` is at `/tags/go`. If
content or a View page already uses the same address, that one wins.

The page shows the term's page title (or its name), its description and
the list of content tagged with it, with paging. The list comes from a
[View](./views.md) with the display type **Term page**:

- `app install` creates one, *Term pages*, for all vocabularies, with
  weight 0: the published content with the term, newest first, 10 per
  page.
- A term page uses the first published Term page display, by *Weight*
  (lower first; at equal weights the one made first), whose
  **Vocabularies** include the term's vocabulary; none checked - all
  vocabularies. To give one vocabulary's terms a different list, filter
  or sort, make a Term page display for that vocabulary with a weight
  below *Term pages*' (say `-1`) and publish it.
- The View needs the `has_current_term` contextual filter: it takes the
  term from the page address.

Without a Term page View the list is empty. The items are shown as the
theme's teaser cards: the View sets which content, in what order and how
many per page, not the fields or the look of the list.

The `<title>` and meta description of term pages come from the SEO
templates' *Term pages* section - one template for all vocabularies and
one per vocabulary; see [Term pages](./urls-and-seo.md#term-pages) in
*Addresses and SEO*.

## Tag cloud

The **Tag cloud** block shows the terms used by the most published
content, in alphabetical order, each in a size by how much content it
has. Place it in a region on the [Blocks](./blocks.md) page and set:

| Setting | Default | What it does |
|---|---|---|
| Vocabulary | All vocabularies | The terms of one vocabulary, or of all. |
| Number of terms | 10 | How many terms to show (up to 100). |
| Size levels | 6 | How many sizes the terms are spread over (up to 10). |

Content waiting for moderation isn't counted. A term of a vocabulary
without public term pages is shown without a link. The cloud is built
with the site's block cache, so new content shows up in it at the
cache's next refresh, a few seconds later ([Caching](./views.md#caching)).

## Other places that use a vocabulary

The engine doesn't know your vocabularies: wherever terms matter, you
choose the vocabulary in that setting.

- Views: filters on a taxonomy field, the vocabularies of a *Term page*
  display, and the related-content filters `similar_to_current_node`
  and `shares_terms_with_current` ([Views](./views.md)).
- Breadcrumb rules: content gets its place in the menu by its terms
  ([Breadcrumb rules](./urls-and-seo.md#breadcrumb-rules)).
- Live search: the badge shown next to results of a type
  ([Search](./search.md)).
- Modules, for example the forum's sections.

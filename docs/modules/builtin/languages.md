---
title: Languages
description: The site in several languages - each one besides the default at its own address prefix, with the interface in it, translations of content, a language switcher and hreflang for search engines.
sidebar:
  order: 14
---

The *Languages* module (`languages`) makes a site of several languages.
The default language keeps its addresses as they are; every other
language gets an address prefix: with English as the default and German
added, `/news/x` is in English, `/de/news/x` in German. Under a prefix
the whole interface is in that language, `<html lang>` included.

The module is off on every site until you enable it on the **Modules**
page (see [Using modules](../using-modules.md)). Enabling it the first
time installs it and puts the *Language switcher* block in the theme's
header; the block shows nothing until the site has a second language.

## The Languages page

**Configuration → Languages** (`/admin/config/languages`) lists the
default language first, then the others in the order they were added:
the code, the address prefix, how much content is in it, and whether the
interface has a translation in it. A language without one shows the
interface in English; its content is shown as written.

- **Add a language.** Pick the language and its address prefix (empty -
  the language's code): lower-case Latin letters, digits and hyphens, up
  to 12 characters (`de`, `pt-br`). A prefix is refused when another
  language has it, when the site has an address like it (a page, a View,
  a feed, an engine route such as `admin`), or when content has
  addresses starting with it - the prefix would hide them.
- **Remove a language.** Its content stays, at addresses without the
  prefix. Content addresses are unique across the site, so nothing
  clashes.
- **No translation.** What the lists in a language show of the content
  without a version in it (below): *its original, marked with its
  language* (the default), *nothing* - only the language's content, or
  *everything* - the lists aren't divided by language.
- **Translatable types.** The content types whose items get the
  *Translations* tab (below). None at first.
- **Make default.** For a language the interface is translated into.
  Its addresses lose the prefix; the old default's get its code as the
  prefix (`/en/…`). Changing *Interface language* in *Site information*
  does the same.

The page needs the *Access administration (/admin)* permission.

## A content item's language

A content item is in the language of the address it was created at:
added at `/de/create/article`, it is German, at `/de/<its path>`. Who
may administer content changes it in the form's *Language* section; a
language another version of the item has isn't offered there. Its
address without the prefix redirects there with a 301, and so does its
address under another language's prefix: each item has one address.
Content created before the module was enabled is in the default
language.

## Translations

A translation is a content item of its own, in another language, linked
to the original: it has its own address, title, fields, status and
history. The originals and translations of one item make a set.

An item of a translatable type has the *Translations* tab
(`/node/<id>/translations`) for who may edit it. It lists the site's
languages, the default one first, with the item's version in each:

- the original and the translations, each with its status and *Edit*;
- *Outdated* by a translation whose original was saved through its form
  after the translation was; saving the translation clears the mark;
- *Translate* by a language without a version, for who may create the
  type's content. It copies the item whose tab it is into that language -
  the same title and field values, unpublished - and opens the copy's
  form. Translate the text there and publish it.

The form of a translation names its language and its original (with the
mark); the form of an original lists its translations. Changes made
outside the form - publishing, the bulk actions, the schedule - don't
mark translations outdated.

Deleting the original leaves its translations: they remain a set without
an original. Terms, menus and blocks are shared by all languages for now.

Only the site's own content types (not a module's, such as a forum
topic or a poll) can be translated.

## Lists

Every content View - a page, a block, the front page's list - shows
the content of the page's language: on `/de/` the German items, on the
default language's addresses the default language's (and the items
without a language of their own, such as some modules' content). What
it shows of an item without a version in the language follows *No
translation*:

| Setting | On `/de/`, an item in English only | Its German translation exists |
|---|---|---|
| Its original, marked with its language | the English item, marked *English* | the translation only |
| Nothing | not shown | the translation only |
| Everything | every item in every language | both |

The mark is a badge after the title - in the engine's teaser and in a
View's *Title* field; a theme shows it with the `otherLanguage` template
function. A View block is rendered in each language once and kept until
the blocks' cache refreshes.

## Menus

A menu is one for all languages; each link adapts to the page's:

- **Its address.** A link to content leads to the item's published
  version in the page's language, when it has one. A link to another of
  the site's addresses (a term, a View's page such as `/calendar`) goes
  under the language's prefix: `/de/calendar` on a German page. Another
  site's address stays as it is.
- **Its name.** The link's form has *Name in other languages*: a field
  per language besides the default one. Empty - the name as entered.
- **Its languages.** *Languages* in the same form: *All languages*, or
  only the chosen ones - a link to an English-only shop shows on the
  English pages alone. A hidden link hides its children too.

The engine's own links follow the page's language the same way: the
logo, the pager, the breadcrumbs, tags and the glossary. A theme does so
with the `localURL` template function.

## Blocks

A block placement's form (**Structure → Blocks**, a block's edit page)
has two more parts:

- **Title in other languages** - a field per language besides the
  default one; empty - the title as entered (a View block without a
  title of its own: the View's title). A custom block's title is
  translated on its own page.
- **Languages** - next to the roles and pages of its visibility: *All
  languages*, or only the chosen ones.

A custom block's text is one: for a text in each language, make a block
per language and show each on its language's pages only.

## Taxonomy terms

A term is one for all languages: its address and its content are
shared. Its form (**Structure → Taxonomy**, a term's edit page) has *In
other languages*: its name and description in each language besides
the default one; empty - as entered.

A term's page shows them in the page's language (its page title too:
with a translated name, the page is titled by it), and so do the tags
in content, the tag cloud, Views' term fields and the search's badges.
The term's page lists the content of the page's language, by *No
translation*.

## The site's name, Views, fields

Three more forms have a section for the other languages:

- **Site information** (*In other languages*) - the site's name and
  slogan: the header, the window's title, `[site:name]` in the SEO
  templates.
- **A View's settings** (*In other languages*) - its title and meta
  description: its page's heading and SEO (with a title translated, the
  page's meta title is it), its block's title when the placement has
  none of its own.
- **A content type's field** (*Label in other languages*) - its label on
  the content's page (a View has its fields' labels of its own).

## Search

The search - its page and the live search under the field - finds the
content of the page's language, as the lists show it (*No translation*):
on `/de/search` the German items.

## The language switcher

The *Language switcher* block (`language_switcher`) lists the site's
languages by their codes (`EN`, `DE`), the page's own marked. A link
opens the same page in that language. On a content page, a language's
link opens the item's published translation into it, and the language's
front page when there is none. Move the block on the
[Blocks](../../guides/blocks.md) page like any other.

The markup is a Bootstrap `nav nav-pills` in a `nav.language-switcher`:
a theme restyles it by that class, or replaces the template by defining
`block_language_switcher` itself. In the header region it sits at the
region's end, on the menu's line; the Kosatka theme draws it as its
header buttons are, the page's language marked like the menu's current
item.

## Search engines

The front page, a View's page and a term's page list their address in
every language in `<head>`, with `x-default` for the default language's;
so does a content item with published translations, `x-default` being
its version in the default language:

```html
<link rel="alternate" hreflang="en" href="https://example.com/">
<link rel="alternate" hreflang="de" href="https://example.com/de/">
<link rel="alternate" hreflang="x-default" href="https://example.com/">
```

A content item's canonical address and its entry in the sitemap carry
its language's prefix; so does the canonical address of the other pages
in a language - the front page, a View, a list: `/de/` and `/de/videos`
are their own.

## Switching it off and uninstalling

Disabled, the module's prefixes stop working: every address is in the
default language, `/de/…` answers 404, and content in other languages is
at its address without a prefix. The list of languages is kept, so
enabling the module again brings them back.

Uninstalling a disabled module forgets the list of languages, which
items translate which, the texts entered in the other languages (menu
link names, titles, terms, labels) and the languages chosen for menu
links and blocks. The content keeps its language.

## Not there yet

- E-mails in the recipient's language, a View's field labels and
  exposed filters' labels per language.

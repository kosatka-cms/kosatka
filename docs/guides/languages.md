---
title: Languages
description: The site's language, several languages, what gets translated, date formats and time zones, and adding a language.
sidebar:
  order: 14
---

A Kosatka site has one language unless the [Languages
module](../modules/builtin/languages.md) adds others. The interface
(buttons, labels, messages, e-mails) is written in English in the code
and translated through catalogs; your content is whatever language you
write it in.

## The site's language

The language is chosen when the site is installed, with `app install
-language <code>` (`en` by default; see
[the CLI](../reference/cli.md#app-install)). A language that isn't built
in is [downloaded](#downloading-a-language) from the translations
catalog first. The starter data is written
in that language too: the content types and their fields, the front
page's View, the main menu, the names of roles, text and date formats
and image styles, and, with `-demo`, the demo content.

Later you change it in *Site information*
(`/admin/config/system/site-information`), *Interface language*. The
list has English and every language the site has a translation catalog
for: built into the engine, or [downloaded](#downloading-a-language). A new language takes effect on the next page load, `<html lang>`
included.

Changing the language later translates the interface, not the starter
data: those names were written once, at install, and are now ordinary
data you edit like the rest.

## What is translated

| Part | Translated by |
|---|---|
| The engine's interface: the admin, account pages, messages, e-mails' defaults | the engine's catalog |
| A module's interface | the module's catalog |
| The theme's templates | the theme's catalog |
| Month names in dates | the catalogs (see below) |
| Content, comments, menu links, block and View titles, field labels, settings you type | nothing: they are shown as entered (on a site of [several languages](#several-languages), some take a version per language) |

A string with no translation yet is shown in English. The theme's
catalog is looked up first, so a theme can change any string of the
engine or of a module for its site; the site's own changes (below) come
before all of them.

## Changing the interface's strings

*Interface translations* (`/admin/config/translations`, under
Configuration, *Site*) lets an administrator change any string of the
interface for this site, without a new catalog or a new build: a
translation they would word differently, a string still untranslated,
or the English text itself.

The *Languages* tab lists the interface's languages, how many of the
site's strings each translates (the untranslated ones a link to them)
and how many the site has changed. The *Strings* tab lists the strings
of the engine, of the enabled modules and of the active theme in a
language, filtered by a text (in English or in the translation),
untranslated or changed only, and where they come from. *Edit* (or
*Translate*) opens a string's translation in place; a plural string has
a field for each plural form of the language, labelled with numbers it
is for (Russian: *1, 21, 31...*, *2, 3, 4...*, *0, 5, 6...*).

A changed string is shown instead of the catalog's at once, in the
interface's every part, and stays through updates of the engine, the
modules and the theme; *Back to the default* removes the change. An
English string changed is also what other languages show while they
don't translate it.

A translation is output as HTML, so it is checked when saved: it keeps
every placeholder of the English string (`@name`, `%name`, `:url`;
`@count` of a plural form may be spelled out) and has no HTML but the
English string's own tags, exactly as they are. A `<` or `>` a browser
shows as text (`a < b`, `<name>` in another alphabet) is text.

When an update changes a string's English text, the site's change of
the old text no longer applies: the *Strings* tab lists such changes
under the table, to remove them once the new text is changed if needed.

## Downloading a language

English, the language of the code's strings, is built into the engine.
Every other language is downloaded on the *Languages* tab of *Interface
translations* from a translations catalog, so the engine doesn't carry
every language a site might want. By default it is the Kosatka
catalog (`https://kosatka-cms.github.io/translations/index.json`, built
from the `kosatka-cms/translations` repository); the site's
configuration can name another, or none:
`translations_catalog_url` / `TRANSLATIONS_CATALOG_URL` (see
[Configuration](../reference/configuration.md)). Without a catalog, the
interface is in English.

The tab lists the catalog's languages with how much of the site's
interface each translates (from the command line - [`app
translations`](../reference/cli.md#app-translations)); a draft, machine translated and not yet
reviewed, is marked so. *Download* installs a language: it is then a
choice of *Interface language* in *Site information*, and on a site of
[several languages](#several-languages) that language's pages have
their interface in it. The
catalog is read again an hour later or with *Check for updates*; a
language whose catalog version changed shows *Update*. *Remove* takes a
downloaded language away, except the site's interface language.

A downloaded file is checked before anything is installed: its
checksum must be the catalog's, and each string is checked like a
change made on the *Strings* tab (the English string's placeholders
kept, no HTML but its tags) and must be a string of the site; the ones
that aren't are left out, and the tab says how many. A language's
translation is stored in the site's database, so it survives updates
of the engine; the site's own changes of its strings stay over it, an
update and a removal included.

### A translations catalog

A catalog is a directory served over HTTPS (or a local directory, for a
site's own): an `index.json` and a catalog file per language next to
it, in the [format above](#adding-a-language-for-translators):

```json
{
  "generated": "2026-11-04T12:00:00Z",
  "languages": [
    {"code": "de", "version": "0.2.0", "file": "de.json",
     "sha256": "<the SHA-256 of de.json, hex>", "strings": 2814, "draft": true}
  ]
}
```

`code` is the language's code, `version` the file's own version (a new
one is an update), `file` a file name next to the index, `sha256` its
checksum, `strings` how many strings it translates (shown before it is
downloaded), `draft` marks a machine translation. An entry that isn't
valid, and one for a built-in language, is left out.

[`app translations index`](../reference/cli.md#app-translations) builds
a catalog from a translations repository kept by language and
component:

```
de/language.yaml      version: 0.2.0, draft: true
de/engine.json        the engine's strings (the catalog format)
de/polls.json         a module's strings
```

It checks each string against the code's (`translations/sources/strings.json`
of the engine and of the modules), leaves out the ones the code no
longer has, fails on one that can't stand for its English string, and
writes `index.json` and one `<code>.json` per language, the checksums
counted from the files.

## Dates

Dates are shown through named date formats: `/admin/config/date-formats`.
A new site has *Short* (`Y-m-d`, the default for a date with no format
chosen), *Medium* (`j F Y`) and *Date and time* (`Y-m-d H:i`). A
content type's date field and a View field pick a format by its machine
name.

The page lists the pattern letters, with a live example as you type. The
month and weekday letters follow the site's language:

| Letter | | Example in English |
|---|---|---|
| `F` | the month name inside a date | 31 December 2027 |
| `f` | the month name on its own | December 2027 |
| `M` | the abbreviated month name | Dec |
| `l` | the day of the week | Friday |
| `D` | the abbreviated day of the week | Fri |

The difference between `F` and `f` matters in languages where a month
changes its form inside a date. The time has a 24-hour clock (`H`, `G`)
and a 12-hour one (`h`, `g`, with `A` or `a` for AM/PM: `g:i a` -
7:30 pm); `N` is the day's number in the week, 1 (Monday) to 7. Any
other character is output as is; a backslash outputs a pattern letter as
itself (`\a\t` - at). The default format and a format a field still uses
can't be deleted.

## Numbers

A number field that is whole or decimal shows its value with the
language's separators - translations of `.` (context *Decimal
separator*) and `,` (context *Thousands separator*): Russian has a comma
and a no-break space. Thousands are separated from five digits of the
whole part, so a year stays as it is.

## Time zone

Dates are stored in UTC and shown in the site's *Time zone* (*Site
information*): publishing dates, comments, "today" in View filters, the
dates in upload directories.

With *Users may set their own time zone* on ([Account
settings](./users-and-permissions.md#account-settings)), each user can
pick a time zone in their profile; a new account, registered or
signed in with a social account, gets the browser's. Guests and users
without their own see the site's.

## Several languages

The [Languages module](../modules/builtin/languages.md) adds languages
besides the default one, each at its own address prefix (`/de/…`) with
the interface in it, a language switcher block and `hreflang` links for
search engines, and translations of content: a version of an item in
each language, linked to the original.

Menus, blocks and taxonomy terms are shared by all languages: their forms
take a menu link's name, a block's title and a term's name and
description in each language, and a menu link or a block can be shown
on the pages of chosen languages only. The site's name and slogan, a
View's title and meta description and a field's label are translated
the same way. Not there yet: e-mails in the recipient's language, a
View's field labels and the exposed filters' labels per language.

## Adding a language (for translators)

Only English is in the engine's code. The other languages of the
engine, its standard modules and the Kosatka theme are translated in the
translations repository,
[`kosatka-cms/translations`](https://github.com/kosatka-cms/translations),
a catalog per component; a module or a theme from its own repository
keeps its catalogs itself:

| Whose strings | Where |
|---|---|
| The engine's | `<code>/engine.json` in the translations repository |
| A standard module's | `<code>/<module>.json` there (`de/polls.json`) |
| The Kosatka theme's | `<code>/theme-kosatka.json` there |
| A module's or a theme's from its own repository | `translations/<code>.json` in its repository |

A catalog is a JSON file:

```json
{
  "strings": {
    "Save": "Speichern",
    "@count view": ["@count Aufruf", "@count Aufrufe"]
  },
  "contexts": {
    "Month name in a date": {"May": "Mai"}
  }
}
```

The key is the English string as the code has it; placeholders such as
`@name` stay as they are. A plural is an array with one form per plural
category the language uses, in the order zero, one, two, few, many,
other (English and German have two, Russian three, Japanese one).

A new language in the translations repository starts with
`<code>/language.yaml` (its version, and `draft: true` until someone
reviews it). To see what is still untranslated, build the catalog there
with the engine's binary:

```bash
app translations index -src . -out dist -complete de
```

It checks each string against the engine's (placeholders, tags, a
string the code no longer has) and lists the strings `de` still misses.
Merged, the language is published in the Kosatka
[translations catalog](#a-translations-catalog), and sites download it.
A theme's and a module's catalogs from their own repositories are read
from their directories when the site starts or switches to the theme;
*Interface translations* → *Strings* (*Show*: *Untranslated*) lists the
ones a language's catalog lacks.

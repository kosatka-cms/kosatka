---
title: Addresses and SEO
description: URL aliases, redirects, breadcrumbs, page titles and descriptions, schema.org markup, the sitemap, robots.txt, the RSS feed, caching for a CDN, counters and the cookie banner.
sidebar:
  order: 7
---

This page covers how a site's pages are addressed and how search engines
see them: content addresses and their patterns, redirects, breadcrumbs,
the title and description of each page, the schema.org markup, the
sitemap, `robots.txt`, the site's RSS feed, canonical links, caching
behind a CDN, and the counters and the cookie banner.

Most of it is under **Configuration → Search and SEO** in the admin:
*SEO* (`/admin/config/seo`), *URL aliases* (`/admin/config/url-aliases`),
*Redirects* (`/admin/config/redirects`) and *Counters and codes*
(`/admin/config/integrations`). Breadcrumbs are under **Structure →
Breadcrumbs** (`/admin/config/breadcrumbs`).

## The site's address

Absolute links - canonical links, `og:url`, the sitemap, the RSS feed,
`robots.txt`, the structured data, the links in e-mails - use the site's
address. Set it once:

- in the admin: **Site information** (`/admin/config/system/site-information`),
  field *Site address*, for example `https://www.example.com`;
- or in the config: `public_base_url` / `PUBLIC_BASE_URL`
  ([the configuration](../reference/configuration.md)). It overrides the
  admin's field (which then shows the config's value, greyed out), so a copy
  of the production database on a staging site keeps the staging address.

The value is an origin only: `http(s)://` and a host, no path. With
neither set, the sitemap, the feed and `robots.txt` take the address from
the request, canonical links and `og:url` are bare paths, the pages carry
no [structured data](#structured-data-schemaorg), and the account e-mails
aren't sent (see [Email](./mail.md)).

## Content addresses

Every content item has an address (its *slug*), for example
`article/my-first-post`; the page is at
`https://www.example.com/article/my-first-post`. Addresses are built
automatically from the content type's **URL pattern**.

### URL patterns

Set the pattern on the content type's settings (field *URL pattern*) or
for all types at once on **URL aliases → Patterns**
(`/admin/config/url-aliases/patterns`). An empty pattern means
`[node:type]/[node:title]`. Under each type the page lists the tokens its
pattern can take.

| Token | Value |
|---|---|
| `[node:type]` | the type's machine name |
| `[node:title]` | the title, transliterated to Latin letters |
| `[node:created:Y]`, `[node:created:m]`, `[node:created:d]` | the creation date's year, month, day (the site's time zone) |
| `[node:field:<machine_name>]` | the term in a taxonomy field of the type: the term's own address without its vocabulary's prefix (so with its parent terms) |

Only taxonomy fields can be used in a pattern; text, number and other
fields can't. The pattern's own text may contain lowercase Latin letters,
digits and `-`, `_`, `.`, `/`. The SEO templates' syntax works here too:
`[a|b]` takes the first token with a value, `{…}` is a part dropped when
a token inside it is empty (see [Syntax](#syntax) below).

Examples:

| Pattern | Address |
|---|---|
| empty | `article/my-first-post` |
| `news/[node:created:Y]/[node:title]` | `news/2026/my-first-post` |
| `[node:field:section]/[node:title]` | `guides/basics/my-first-post` (the term "Basics" under "Guides") |

A token with no value leaves no empty segment. An address that is taken
gets `-1`, `-2`… at the end. Cyrillic letters are written in Latin ones
(`щ` - `sch`, `ё` - `yo`), as in uploaded files' names, and accents are
dropped (`é` - `e`); a title that leaves nothing (no Latin or Cyrillic
letters, no digits) gives the type's machine name and a timestamp.

The patterns page saves all types at once: if one pattern is wrong,
nothing is saved and the error shows under that type.

Content of a module's own type (a forum topic, a poll) gets its address
from the module; such types have no pattern.

### A content item's address

In the content form, the **Address** section (shown to users with
the *administer content* permission) has:

- **Generate the URL alias automatically** - on: the address is rebuilt
  from the pattern each time the item is saved. Off: the address is the
  one typed in the field below.
- **Slug (URL)** - the address without the leading `/`. On a new item it
  may be left empty to generate it. A typed address another page of the
  site already answers - a View's page, a term's page, one of the
  engine's own (`login`) or a module's - isn't taken: the content would
  hide that page, or never be reached.

Users without *administer content* don't have the section, and an
address, meta tags or comment settings they send anyway are ignored.

New content follows the pattern unless an address is typed. Whenever a
saved item's address changes, by hand or by the pattern, the old address
redirects (301) to the new one, so links to it keep working.

Every item is also reached at `/node/<ID>`, whatever its address: that
link redirects (302, as the address may change) to the item's current
address, for whoever may see the item. The action log links to content
this way. An item whose own address is `node/<number>` keeps it.

A changed pattern applies to new content and to automatic addresses the
next time their content is saved. Existing addresses stay as they are
until you rebuild them.

### The list of addresses

**URL aliases → Addresses** (`/admin/config/url-aliases`) lists every
content address with its title and type, and whether it is set *by the
pattern* or *by hand*. Filter by a part of the address (*Address
contains*) or by type; the pencil (*Change*) opens the content's edit form.

Terms have their own addresses, set by their vocabulary: see
[Taxonomy](./taxonomy.md).

### Rebuild existing addresses

After changing a pattern, apply it to the content that already exists:

1. Open **URL aliases → Rebuild** (`/admin/config/url-aliases/rebuild`).
2. Choose the content type. To make hand-set addresses follow the pattern
   too, tick *Also the addresses set by hand (they will follow the
   pattern)*.
3. Click *Show what changes*. Nothing is written yet: the page shows how
   many addresses would change and the first changes, *Now* and *Will be*.
4. Click *Rebuild the addresses*. On a large site the work stops when the
   request's time runs out; click *Continue* to carry on from there.

Every changed address redirects (301) from the old one. The content's
"updated" time doesn't change.

## Redirects

**Redirects** (`/admin/config/redirects`) holds the site's permanent (301)
redirects: a request to *From* is sent to *To*. The engine adds one
whenever a content address changes; add your own for moved or removed
pages.

- **Add redirect**: *From* - a path on the site without the leading `/`
  (`articles/old-slug`); *To* - where to send it: a path on the site with
  the leading `/` (`/articles/new-slug`), or a full address. Both are
  saved as typed.
- **Bulk import**: one redirect per line, either `from -> to` or an Apache
  line `RewriteRule from$ to [R=301,L]`. Empty lines and lines starting
  with `#` are skipped. Here a leading `/` is removed from *From* and
  added to *To* if missing.

The list below shows the redirects by *From*, 50 to a page; the search
finds a part of either address.

A redirect matches the path only: the query string isn't compared, nor
passed on.

Which old addresses need a redirect? The
[404 log](../modules/builtin/not-found-log.md) module adds a *Not found
(404)* tab to this page: the addresses visitors asked for and the site
didn't find, the most asked for first, each with a button that adds its
redirect here.

## Breadcrumbs

Breadcrumbs are built from the site's [menus](./menus.md): there's no
separate tree of sections, the menus are the tree. The trail always
starts with *Home*.

- **A page whose address is a menu link** - a View's page, a term page,
  the search page - gets the trail of that link and its parents. It needs
  no rule.
- **A content item** gets its place from the first matching
  **breadcrumb rule**: the rule's menu link and its parents, then the
  item's title. This holds even when the item's own address is in a
  menu; without a matching rule it gets *Home › title*.
- **Anything else** gets *Home › title*.

When an address is in several menus, the deepest trail wins; at equal
depth, the menu with more links. Neighbouring links with the same
address collapse into one, and disabled menu links (with everything
under them) are skipped.

For example, with *Guides › Basics* in the main menu, `/guides/basics`
linked from *Basics* and a rule "Article with the term Basics in
*Section* → Basics":

| Page | Breadcrumbs |
|---|---|
| `/guides/basics` (a View page in the menu) | Home › Guides › Basics |
| `/tags/go` (a term page not in a menu) | Home › Go |
| an article in the section *Basics* | Home › Guides › Basics › the article's title |

### Breadcrumb rules

Open **Structure → Breadcrumbs** (`/admin/config/breadcrumbs`). A rule
has:

| Setting | |
|---|---|
| Content type | required |
| Term field | optional: a taxonomy field of the type. Empty (*any content of the type*) - the rule matches all content of the type |
| Terms | with a field: the rule matches content that has one of these terms in that field, or a child term of one |
| Menu link | any enabled link of any menu; the content's trail is this link and its parents, then the title |

To add a rule, choose the type, optionally the field, and the menu link,
then click *Add*. If you chose a field, the rule's page opens to pick the
terms (Ctrl/⌘ to pick several); a rule with a field and no terms matches
nothing. After changing the field, save the rule to refresh the list of
terms.

The first matching rule from the top applies: drag the rows into order
and click *Save order*. Put narrow rules (with terms) above the general
rule for the same type.

A rule whose menu link was disabled is marked *link not found or
disabled* and doesn't apply. Deleting a menu link or a field deletes the
rules that use it.

Pages with breadcrumbs also get them as schema.org `BreadcrumbList`
structured data in the page's `<head>`, when the site's address is set.

## SEO: titles and descriptions

The `<title>` and the meta description of content pages and term pages,
and the `og:image` of content pages, are built from **templates with
tokens**. Everything is on **SEO** (`/admin/config/seo`).

Note that the title is used as is: the engine adds no site name to it.
To have one, put `[site:name]` in the template, for example
`[node:title] | [site:name]`.

### Content pages

For a content page, the title and the description are chosen separately.
The first one with a value wins:

1. **The content item's own value** - *Page title* and *Description (meta
   description)* in the **Meta tags** section of its edit form. Leave them
   empty unless the item needs a hand-written title: a copy of the
   generated one would stop following the templates.
2. **The first matching rule by terms** (see below).
3. **The content type's template** - *Content types* table, the pencil
   (*Change*).
4. **The default template** - *Default for all content*.
5. **The built-in value**: the title - the content's title, or "Title
   (Alternative title)" when the type has an *Alternative title* field
   set; the description - the beginning of the text.

If the chosen template renders empty (it has nothing but tokens, all of
them empty), the built-in value is used. The description is stripped of
HTML and cut to 160 characters.

**Image (og:image)** is a template of its own, on the default and on each
type (not on rules), usually an image field:
`[node:field:cover_image]`. It gives the original image's address, made
absolute. Empty - the page has no `og:image`.

### Rules by terms

A rule gives its own title and description to content of a type with
certain terms, for example "Article in the section *Reviews*". It
matches the same way as a breadcrumb rule: a content type, optionally a
term field and its terms (child terms match too).

1. Under *Rules by terms*, choose the content type and, optionally, the
   term field; fill in *Title* and/or *Description*; click *Add*.
2. If you chose a field, the rule's page opens: pick the terms and save.
3. Drag the rules into order and click *Save order*: the first matching
   rule from the top wins.

An empty template in a rule falls back to the type's template.

### Content tokens

| Token | Value |
|---|---|
| `[node:title]` | the content's title |
| `[node:field:<machine_name>]` | a field's value: text as is; terms - their names separated by commas; a date - day.month.year (`15.05.2027`); a number as is; an image - the original file's path |
| `[node:field:<machine_name>:year]` | the year of a date field |
| `[node:field:<machine_name>:iso]` | a date field in ISO 8601 (`2027-05-15T10:00:00+03:00`), for the [schema.org markup](#structured-data-schemaorg) |
| `[node:field:<machine_name>:<format>]` | a date field in a format from **Date formats** (`/admin/config/date-formats`) by its machine name, for example `[node:field:event_date:medium]`, in the page's language |
| `[node:summary]` | the beginning of the content's text |
| `[site:name]` | the site's name |

An unknown token renders empty. The template pages list the type's fields
with their machine names.

### Syntax

| Syntax | Meaning | Example |
|---|---|---|
| `[a\|b]` | the first token with a value | `[node:field:local_date:year\|node:field:date:year]` |
| `{ … }` | an optional part, dropped entirely if any token inside it is empty | `[node:title]{ ([node:field:event_date:year])}` gives "Summer Fair (2027)", or just "Summer Fair" without a date |
| `{A\|B}` | the first variant whose tokens all have values | `{[node:field:title_original] ([node:title])\|[node:title]}` |

Inside `[…]`, `|` still means "or the next token".

### Term pages

Under *Term pages*: a title and a description for the pages of all
vocabularies (*Title (all vocabularies)*, *Description (all
vocabularies)*), and per vocabulary (the pencil, *Change*, in the table). A
vocabulary's template wins over the shared one; with neither, the title
is the term's *Page title*, or its name.

| Token | Value |
|---|---|
| `[term:title]` | the term's *Page title* (a field in the term's form), otherwise its name |
| `[term:name]` | the term's name |
| `[term:description]` | the term's description |
| `[site:name]` | the site's name |

The syntax above works here too. The term's *Page title* also becomes
the term page's heading, so a term can keep a short name for labels and
have a longer heading on its page.

### Examples

| Where | Template | Result |
|---|---|---|
| default | `[node:title] \| [site:name]` | My first post \| Example Site |
| type *Event* | `[node:title]{ - [node:field:event_date:medium]}` | Summer Fair - 15 May 2027 |
| rule: *Article*, *Section* = Reviews | `[node:title] - review` | New Phone - review |
| vocabulary *Tags* | `[term:title] \| [site:name]` | Go \| Example Site |

### What templates don't set

Canonical links, `og:url` and `noindex` aren't templated; the engine
sets them:

- A content page's canonical link and `og:url` are its address on the
  site's address.
- A list on page 2 and on is its own canonical page (`?page=2`), its title
  gets "- page 2". Other non-empty query parameters stay in the
  canonical link.
- `noindex, follow` goes on pages with no content of their own: the
  sign-in page and the two-factor pages, a user's files, the search
  results, a View page with an active exposed filter; modules mark their
  own such pages. The other account pages are kept from crawlers by
  [robots.txt](#robotstxt).

The front page's title is the site's name and slogan, its description
the slogan (both from **Site information**).

## Structured data (schema.org)

Search engines read a page better, and may show it as a rich result,
when it says what it is in schema.org's vocabulary (JSON-LD in the
page's head). With [the site's address](#the-sites-address) set,
breadcrumbs carry it, and so do content pages whose type is chosen on
**SEO** - the *Schema.org type* of *Default for all content* or of a
content type (*As for all content*, *- none -*, or its own):

| Type | For | Its own properties |
|---|---|---|
| Article, News article, Blog post | articles, news, posts | Keywords, Section |
| Web page | other pages | Keywords |
| Event | events | Start, End (a date field with `:iso`), Place, Place's address |
| Product or service | goods, services, offers | Brand, SKU, Price, Currency, Availability |

The rest goes in by itself:

- every type: the content's title as its name (an article's headline),
  the description and image the SEO templates above give the page, its
  address;
- the articles and the web page: when the item was published and last
  changed;
- the articles: the author (their profile) and the site as the
  publisher; an event: the site as its organizer.

A property's value is a template with the [content tokens](#content-tokens),
for example `[node:field:tags]` for the keywords or
`[node:field:start:iso]` for an event's start; an empty one is left out.
A content type's SEO page shows the markup of its newest item, with a
link to Google's Rich Results Test.

### Who runs the site

**Site information** has *Who runs the site - for search engines*: an
organization, a local business or a person, its name (the site's by
default), its profiles elsewhere (an address per line), a phone and an
email, a local business's address; the site's logo is its logo. It is
the publisher of the articles' markup, and the front page carries it
with the site itself. *The site's search in the results* adds the
site's search (`/search?q=…`) for search engines to offer.

## The sitemap

`/sitemap.xml` is a sitemap index, built on request:

- `/sitemap/static.xml` - the pages of enabled Views (except those with a
  `%` segment in the address), pages that enabled
  modules add, and the links of the menus marked *Include in the sitemap*
  (a checkbox in the menu's settings, see [Menus](./menus.md));
- `/sitemap/nodes/1`, `/sitemap/nodes/2`… - the content, up to 45,000
  addresses per file, with their last-updated date.

Content is listed when a guest may see it: published, not in moderation,
not in a [closed section](./security.md#closed-sections). Left out too:
content of disabled modules' types, of types with a *Redirect to the
address from field* (see [Content types](./content-types.md)), and the
items used as the site's error pages.

There's nothing to set up besides the site's address. Submit
`https://www.example.com/sitemap.xml` to search engines; `robots.txt`
names it too.

## robots.txt

`/robots.txt` is built by the engine: for every crawler it disallows the
admin, the sign-in and account pages, the content forms and actions, the
search results, and the pages the enabled modules list; then it names
the sitemap.

To add your own lines, open **Configuration → Security → Access
blocking** (`/admin/config/robots`), section *robots.txt*. The text is
added after the engine's rules as written, for example:

```
User-agent: ExampleBot
Disallow: /
```

The same page blocks crawlers by User-Agent and by IP address. That
blocks requests outright, unlike `robots.txt`, which crawlers may ignore.

## The RSS feed

`/rss.xml` is the site's RSS 2.0 feed: the first 20 items of the View
chosen as the *Front page feed* in **Site information**, the same list
the front page shows a guest. Each item has the title, the link, the
beginning of the text, the date and, if the View shows an image field,
the image. The channel's title is the site's name, its description the
slogan.

With no front page feed chosen, the feed has no items.

For more feeds - per section, in other formats, with their own field
mapping - enable the **Feeds** module; it's set up at
`/admin/config/feeds`.

## Caching pages in a CDN

When a CDN or a caching proxy is in front of the site, it can keep the
pages guests see. Set `page_cache_max_age` / `PAGE_CACHE_MAX_AGE` in
[the configuration](../reference/configuration.md) to the number of
seconds a page may be kept, for example `300`.

With it set:

- an HTML page answered with 200 to a guest (no session cookie, a `GET`
  or `HEAD` request) that sets no cookie gets
  `Cache-Control: public, max-age=0, s-maxage=300`: the CDN keeps it for
  300 seconds, browsers ask again each time;
- every other HTML answer - to a signed-in user, an error page, a page
  that sets a cookie - gets `private, no-store` and is never shared;
- answers that set their own `Cache-Control` (the sitemap, the feed, the
  static files) keep it.

`0`, the default, turns it off. Guests see a change to the content only
after the CDN's copy expires, so keep the time short or purge the CDN's
cache after an important change.

## Counters and codes

*Configuration → Counters and codes* (`/admin/config/integrations`) puts
external services on every page:

- **Site ownership verification** - `name=value` lines, each a
  `<meta name="…" content="…">` (Yandex.Webmaster, Google Search Console,
  Bing).
- **Yandex.Metrica** - the counter number and its options (click map,
  external links, accurate bounce rate, session replay).
- **Google Analytics** - the stream ID, `G-…`.
- **Custom code** - HTML/JS at the end of `<head>` and before
  `</body>` (an ad loader, a chat), only for the role with full access;
  the latter can stay off service pages (the admin, 404).

None of the counters and codes is on a page whose address carries a
secret - a password reset link, a preview, an unsubscribe link (a
`token` parameter or a `{token}` in the route): a counter would send the
address on to its service. The verification metas stay.

### The cookie banner

The site's own cookies - the session of a signed-in user, the protection
of forms - need no consent. Analytics and ads do in the EU, and the
banner asks for it. *Visitors' consent (cookie)* on the same page:

| Setting | |
|---|---|
| Cookie banner | on or off (off by default) |
| Mode | *Consent first* (the default) - the codes of a group run only once the visitor allows the group; *Notice only* - they run at once, the banner tells about them and offers *Got it* |
| Banner text | empty - the default text in the site's language |
| "More" page | the privacy policy: a path on the site (`/privacy`) or an address; empty - no link |
| Ask everyone again | forgets every visitor's choice - after the policy changed |

Each code has its *Consent* group: *Analytics* (the default for
Yandex.Metrica and Google Analytics), *Advertising*, or *No consent
needed* (the default for the custom code). With *Consent first* the
banner - a bar at the bottom of the page - offers the groups the site's
codes are in: *Accept all*, *Necessary only*, or *Customize* - a tick per
group and *Save the choice*. Until then a group's codes don't run at
all: they wait in the page and the browser runs them after the visitor's
yes. If no code waits for consent, there is no banner.

The choice is kept in the visitor's browser for a year (the
`kosatka_consent` cookie), so pages cached for guests stay the same for
everyone. A link to `#cookie-settings` - say, a footer menu link with
that address - opens the banner again to change the choice; taking a
group back reloads the page.


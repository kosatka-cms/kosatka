---
title: Embeds
description: Showing a block or a View's list inside a text or a template with a tag such as [[block:contacts]].
sidebar:
  order: 5
---

A tag in a text shows something in its place when the text is shown:

```
<p>Our opening hours:</p>
<p>[[block:opening_hours]]</p>
<p>The latest news:</p>
<p>[[view:latest_news]]</p>
```

The text keeps the tag as typed; what it shows is the thing as it is now -
change the block once and every text embedding it shows the change.

## The tags

| Tag | Shows |
|---|---|
| `[[block:<name>]]` | a custom block ([Blocks](./blocks.md)) by its *Machine name*: its title when *Show title* is on, and its text |
| `[[view:<name>]]` | a View's *Block* display by its machine name: the list as the block shows it, its first page, without a pager |
| `[[poll:<id>]]` | a poll, as the *Poll* block shows it ([Polls](../modules/builtin/polls.md)) |
| `[[form:<id>]]` | a form, as the *Form* block shows it ([Forms](../modules/builtin/forms.md)) |
| `[[gallery:<id>]]` | a gallery: its title and previews, opened in the lightbox ([Galleries](../modules/builtin/galleries.md)) |

The last three come with their modules, while the module is enabled;
their `<id>` is the poll's or the form's content ID (the number in its
address `/node/<id>/edit`) or the gallery's (in `/galleries/<id>`).
Unlike a machine name, which you choose, an ID is given by the site:
the same tag on another site shows another item, or nothing.

A block's machine name is made from its name when it is created
(`Opening hours` - `opening_hours`) and can be changed on its page.
Changing it breaks the tags with the old one. A View display's machine
name is in its *Page settings* ([Views](./views.md#create-a-view)),
changed there too.

Nothing is shown for a tag whose block or View doesn't exist, an
unpublished block, a display that isn't a *Block* display or is
unpublished (*Status* off), or a display with a contextual filter (it
needs the page's own content). A tag of an unknown kind, or of a kind
whose module is disabled, is left as typed.

A tag alone in its paragraph takes the paragraph's place; put a block
or a list on a line of its own, not in the middle of a sentence.

## Turning it on

Tags work in the texts of a text format with *Embeds* switched on (*Text
formats*, `/admin/config/text-formats`: the format's page, under
*Processing when shown*). It is off in every format until you turn it
on. Turn it on only for the formats your editors use: anyone writing in
such a format can show any block or View there.

## In the editor

A format with *Embeds* on gives its visual editor an *Insert…* button.
It opens a window: the kinds on the left (blocks, lists, polls, forms,
galleries - those of the enabled modules), the things of the chosen one
in the middle, found by their title, and on the right what the chosen
one shows: a gallery's pictures, a list's first items, a poll's options,
a form's questions, a block's text. *Insert* puts its tag in a paragraph
of its own at the cursor. The arrows choose in the list, Enter inserts,
Esc closes.

In the editor each tag alone in its paragraph has a card above it: what
it shows, and *Change* - the same window, its choice replacing the tag.
A tag of nothing (deleted, renamed, or not yours to see) gets a yellow
card with *Choose another*: nothing shows there on the site. The cards
are the editor's only; the text keeps just the tags, which can be typed
or edited by hand too.

The window lists what the editor may see: of content, only what is
published; of Views, the published *Block* displays without a
contextual filter. An unpublished block is listed with a note that
nothing shows in its place. The window is open to those who may write in
a format with *Embeds* on.

## As each visitor sees it

What a tag shows is put in for each visitor: an embedded View lists what
its filters pick (a `status` filter leaves unpublished content out -
[Views](./views.md#choosing-the-content-filters)), and of that only the
content the visitor may see: the content closed to them
([Security](./security.md#closed-sections)) is left out. So embedding a
list in a public page never reveals more than the list itself would as a
block. A block embedded in a text shows whatever its visibility on the
block layout: it is the block's text that is embedded, not its
placement. A poll and a form are loaded for each visitor, as their
blocks are, and only for those who may see them. A gallery shows to
those its content is open to; for the others, and for a hidden or empty
gallery, nothing is in its place.

The lists are kept in memory per kind of visitor, like the View blocks,
and rebuilt on the same schedule ([Views](./views.md#caching)).

## Where tags are left out

A tag stands for something else, so it is left out wherever the text is
shown as plain text: teasers and excerpts, the RSS feed, meta
descriptions, the e-mails of the *Subscriptions* module.

An embedded thing can embed others - a block whose text has a tag -
three levels deep at most; a thing embedding itself shows nothing the
second time.

## In a template

A theme's template embeds the same way with the `embed` function:

```
{{embed "block:opening_hours"}}
```

It works whatever the text formats. The names are the site's own: a
theme using them works on that site only - keep `embed` out of a theme
meant for other sites
([Templates](../themes/templates.md#template-functions)).

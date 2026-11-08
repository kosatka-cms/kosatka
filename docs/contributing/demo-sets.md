---
title: Demo sets
description: A whole demo site as data - its theme, fields, content types, Views, blocks, menus and content - installed with app install -demo-set.
sidebar:
  order: 4
---

`app install -demo` fills a new site with a blog about the engine. A
**demo set** is another demo site described as data - a game portal, a
recipe site, a band's site. `app install -demo-set <set>` installs it on top of
what every new site starts with (`internal/app/starter/starter.yaml`);
`-demo-set blog` is the same as `-demo`. Nothing of a set is in code:
adding one is adding a file.

A set is a directory of `internal/app/demoset/sets`, with a file per
language, `<lang>.yaml`, holding everything in that language: labels,
titles and texts. A site is installed in a language the set has.

A theme may bring a demo site of its own: `demo/<lang>.yaml` in the
theme's directory, in the same format (for theme authors: [A theme's
demo site](../themes/demo-site.md)). `-demo-set <theme>` (the theme's
directory name) installs it and puts the site in that theme, unless the
set's `site.theme` names another; an engine's set of the same name comes
first. Its blocks go in the theme's own regions too, and its texts may
show the theme's pictures (`/static/themes/<theme>/images/…`).
`app theme check` loads it as the install does and checks that its
blocks and menus are in the theme's regions ([Checking a
theme](../themes/distribute.md#checking-a-theme)).

```bash
app install -site-name "Recipes" -admin-name admin -admin-email admin@example.com \
  -language en -demo-set <set>
```

The engine's tests install one of their own,
`internal/app/demoset/testdata/sets/fixture/en.yaml` - a little of every
section below.

`-theme` puts the site in another theme: the set's blocks and menus go in
its regions (the engine's themes share them: `header`, `sidebar`,
`content_top`, `content_bottom`, `footer`…).

## What a set holds

Everything a set creates is an ordinary record the site edits or deletes
in the admin. A reference is by name: a field's storage, a vocabulary, a
node's key, a View. A set may use the starter's fields (`field_body`,
`field_image`), content types (`article`, `page`), image styles and Views.

| Section | What it is |
|---|---|
| `site` | `theme` (empty - the starter's), `slogan`, `front_page_view` (a View of the set or the starter); `main_menu` and `footer_menu` - the menus the modules' demo content adds its links to |
| `users` | the demo users' names: members with a generated avatar who write and comment |
| `image_styles` | as in `starter.yaml`: `name`, `label`, `width`, `height`, `mode`, `anchor`, `hidpi` |
| `vocabularies` | `name` (a machine name), `label`, `path` (where its term pages go), `terms` - names, or `{name, description}` |
| `field_storages` | `name`, `type` (`text`, `number`, `bool`, `date`, `taxonomy`, `list`, `reference`, `image`, `file`, `link`…), `multiple`; a taxonomy's `vocabulary`, a reference's `target` (content types), a list's `options` (`{value, label}`), any other setting of the type in `config` |
| `content_types` | as in `starter.yaml`, plus `path` (its items' addresses: `<path>/<title>`) and `show_view_count`; `preview: false` - its form without *Preview*; a field may be `required`, have a `date_format` |
| `nodes` | the content, see below |
| `views` | Views of content: `name`, `label`, `display_type` (`page` with a `path`, `block`, `attachment` with `attach_types` and an `attach_title`, `term` with `term_vocabularies`, `user_tab` - a profile tab, with `tab_path` and `tab_access`: `all` or `owner`), `style` (`list`, `grid` with `grid_columns`, `table`, `carousel`), `theme_template` (a template of the site's theme rendering the list, `views/<name>.html`), `page_size`, `skip` (the first N items left out), `pager`, `header` (HTML above the list), `group_by` (a field) with `group_date_format`; `fields`, `filters`, `sorts` on a field storage (`storage`) or a built-in field (`builtin`: the names the Views editor uses, e.g. `title`, `created_at`, `view_count` for a field, `views` for a sort; an unknown one stops the load; a module's criterion by its key, `ratings.weighted`, with its settings in `params`) |
| `blocks` | placed in a theme's `region` with a `title` (`show_title`) and a `weight` (0 - its place in the list, times ten): a View of the set (`view`), a plugin block (`plugin`: `search_form`, `last_comments`, `tag_cloud` with its `vocabulary`…; `config`) or a text block (`body`); on every page, the front page only (`visibility: homepage`) or the pages of content `types` |
| `menus` | `name`, `label`, `region`, `links` - to an address (`url`), a node (`node`: its key), a View page (`view`) or a term (`term`: `vocabulary/term`). Links to a menu the site has (the starter's `main`) go after its own |
| `permissions` | what roles may beyond the starter's, by role name: `user: [create blog content, edit own blog content, use_text_format_basic_html]` |
| `modules` | the modules' demo content, by module name - see below |

A filter is fixed (`value`; `relative: true` - a relative date), exposed
to visitors (`exposed`: the query parameter, `exposed_label`) or
`contextual`. A fixed taxonomy filter names its terms, a reference filter
the nodes' keys.

## Content

```yaml
nodes:
  - key: hades
    type: game
    title: Hades
    ago: 400d          # created this long before the install: 3d, 5h, 2d6h, 30m
    views: 5400
    fields:
      field_cover: 1   # a generated picture, by its number
      field_release: 2020-09-17
      field_platforms: [PC, Nintendo Switch]
      field_body: |
        <p>…</p>
  - key: review_hades
    type: review
    title: "Hades: why death is only the beginning"
    author: retro_anna
    fields:
      field_game: hades      # a node's key
      field_verdict: great   # a list option's value
    comments:
      - author: night_owl
        body: Thirty runs in, still new lines.
        replies:
          - {author: retro_anna, body: …}
```

A node has a `key` other nodes, menus and filters name it by, its `type`,
`title`, `author` (a user of the set; none - the administrator), `ago`,
`promote` (none - its type's), `sticky`, `comment_mode` (`open`,
`closed` or `hidden`; none - its type's), `path` (none - its type's,
`<path>/<title>`), `views` (its view count) and `comments` (threaded, by
the set's users). Its `fields`, by the storage's type:

| Type | Value |
|---|---|
| text | a string (HTML in the field's first text format) |
| number, bool | a number, `true`/`false` |
| date | `2006-01-02`, `2006-01-02 15:04`, or relative to the install in days, years or hours: `+10d`, `-2y`, `+5h` - a release calendar stays in the future |
| taxonomy | a term's name (several - a list) |
| list | an option's value (several - a list) |
| reference | a node's key (several - a list) |
| image, file | a generated picture's number (several - a list), or `{picture, alt}` |
| link | an address, or `{url, title}` (several - a list) |

## Modules' content

A module with a demo step (polls, galleries, forms, ratings) adds its content to the blog from its manifest's
`demo/<lang>.json` (`internal/modules/<name>/manifest/demo`). A set gives
it its own instead, under `modules`, in the same format:

```yaml
modules:
  polls:
    title: What are you playing this weekend?
    options: [Something new, An old favourite]
    votes: {pixel_kid: 0, retro_anna: 1}
    menu: Poll
    block: Poll of the week
  galleries:
    node: review_outer_wilds   # the gallery goes on this node
    title: The solar system
    pictures: [The campfire, The observatory]
  forms:
    title: Request
    elements:
      - {name: topic, type: select, label: Topic, options: [Design, Support], prefill: topic}
      - {name: consent, type: consent, label: I agree to the processing of my data}
    block: {region: sidebar, title: Write to us, weight: 10}  # the Form block
  ratings:
    types: [game]              # the content types rated
    votes:
      - {node: hades, scores: {pixel_kid: 5, retro_anna: 4}}
```

A form's question with `prefill` is filled from that address parameter
(`?topic=Design`); `block` places the *Form* block showing the form in a
region (`visibility: homepage` - on the front page only); `menu` - the
title of its link in the footer menu (none - no link).

Without `-modules`, `app install` installs the standard modules and the
ones the set has content for; with `-modules`, only those listed (and
the modules they require), and the set's content for a module the site
doesn't have is skipped. A module the site isn't built with at all is
an error in the engine's set (a misspelt name, say) and skipped in a
theme's (a module of its own repository, which only some sites have),
with a line in the install's output. A module's links go in the menus the `site`
section names (none - no link).

`TestSetsLoad` loads every set in every language: an unknown key, a name
used twice and a reference to something the set hasn't fail it (one to
the starter's fields, content types and Views is checked on install);
`TestDemoSet*` (`internal/app`) install a set on MySQL and PostgreSQL and
open its pages.

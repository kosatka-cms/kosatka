---
title: The manifest
description: Every key of a theme's theme.yaml - its description, regions, starter blocks, settings, icon and base theme - and how the engine checks them.
sidebar:
  order: 2
---

`theme.yaml` at the root of the theme's directory describes the theme to
the engine and to the admin. A complete example:

```yaml
name: Sand
description: A warm light theme with wide covers.
version: 1.2.0
author: Example Studio
homepage: https://github.com/example/kosatka-theme-sand
license: MIT
tags: [light, cards, two-columns]
screenshot: screenshot.jpg
min_version: 0.2.0
favicon: /static/themes/sand/favicon.png
default_region: sidebar
regions:
  - id: header
    label: Header
  - id: sidebar
    label: Sidebar
  - id: footer
    label: Footer
blocks:
  - plugin: copyright
    region: footer
settings:
  - id: accent
    label: Accent colour
    type: color
    default: "#b45309"
    css_var: --sand-accent
```

## Keys

| Key | Type | Meaning |
|---|---|---|
| `name` | text | the theme's name in the admin, in English (translated by the theme's catalog). Without it the directory's name is shown |
| `description` | text | one or two sentences for the Appearance page, in English (translated) |
| `version` | `X.Y.Z` | the theme's release. Empty - the theme comes with the engine or the project. For a theme installed with `app theme add`, the Appearance page shows the installed tag instead |
| `author` | text | who made it |
| `homepage` | URL | its page or repository, `http://` or `https://` |
| `license` | SPDX expression | `MIT`, `GPL-2.0-or-later`, `MIT OR Apache-2.0` |
| `tags` | list | up to 20 lowercase words of latin letters, digits and `-` (`dark`, `two-columns`); translated by the theme's catalog |
| `screenshot` | path | a picture in the theme's `static/`: `.png`, `.jpg` or `.webp`, 3:2 (for example 1500×1000) |
| `min_version` | `X.Y.Z` | the oldest engine release the theme works with (see [below](#engine-version)) |
| `favicon` | path | the theme's icon, a path on the site (see [below](#icon)) |
| `default_region` | region ID | where a newly placed block goes. Empty or not one of `regions` - the first region |
| `regions` | list | the theme's [regions](#regions), required |
| `blocks` | list | the [starter blocks](#starter-blocks) |
| `settings` | list | the theme's [settings](#settings) |
| `base` | directory | the base theme of a [subtheme](#subthemes) |
| `stylesheets` | list | stylesheets the engine links after the theme's own hooks (see [Stylesheets](./assets-and-translations.md#stylesheets)) |

Everything but `regions` is optional. Write `name`, `description`, the
labels of regions, settings and options and the tags in English: the
string extractor finds them in `theme.yaml` (the tags when written as a
list in brackets, `tags: [dark, retro]`), and the theme's
`translations/<lang>.json` translates them.

Stars and the date of the last update are not in the manifest: the
[themes catalog](./distribute.md#the-themes-catalog) takes them from the
theme's repository.

## Regions

A region is a named area where the site's administrator places blocks.
The manifest lists every region the theme's pages render, in the order
the Blocks page shows them:

```yaml
regions:
  - id: header
    label: Header
  - id: sidebar
    label: Sidebar
```

- `id` - the machine name templates use:
  `{{template "region_blocks" (dict "Region" (region "header") "Ctx" .)}}`.
- `label` - its name on the Blocks page, in English (translated).

Declare every region the pages render, including those rendered by the
engine's templates the theme keeps. The engine's templates render these:

| ID | Rendered by | Where |
|---|---|---|
| `header` | `header.html` | the header row |
| `sidebar` | `partials/main_column.html` (`sidebar.html`) | the side column, shown only when it has blocks for the page |
| `content_top` | `partials/main_column.html` | between the page's title and its content |
| `content_bottom` | `partials/main_column.html` | after the content |
| `content_inline` | `dynamic/page.html` | inside a content page, after its fields |
| `footer` | `footer.html` | the footer |
| `footer_bottom` | `footer.html` | the footer's last line |

A theme may name its regions as it likes (`header_nav`, `main_bottom`…)
and render them from its own templates. Keeping the engine's IDs has a
benefit: when the site switches themes, the new theme's block layout is
copied from the current one region by region, matching IDs.

Every theme also has the region **Hidden** (`disabled`), which is not
declared: a block placed there is kept but shown nowhere.

## Starter blocks

`blocks` lists blocks the theme comes with. They are placed when the
theme gets its block layout for the first time: on `app install` for a
new site's theme, or when the theme is first opened on the Blocks page,
previewed or made active (after the blocks copied from the current
theme).

```yaml
blocks:
  - plugin: copyright
    region: footer_bottom
    settings:
      since: "2015"
  - title: Buttons
    region: footer
    weight: -10
    body: |
      <div class="sand-buttons">…</div>
```

| Key | Meaning |
|---|---|
| `plugin` | a system block's ID: `logo`, `search_form`, `user_menu`, `last_comments`, `tag_cloud`, `copyright`, or a module's |
| `settings` | a system block's settings (key → value), as on its placement page; for `copyright`: `holder`, `since` |
| `title` | a custom block's title (required for one), or a system block's placement title |
| `show_title` | `true` - show the title |
| `body` | a custom block's HTML (required for one) |
| `region` | one of the theme's regions (required; not `disabled`) |
| `weight` | its order in the region, smaller first |

- A block is either a system block (`plugin`) or a custom one (`title`
  and `body`); `settings` are a system block's only.
- A block already in the layout is not placed again: a system block in
  the same region, a custom block with the same title. A system block the
  engine doesn't have (a module that isn't there) is skipped.
- A custom block's HTML is cleaned as if saved in the admin.
- After that they are ordinary blocks: the site edits or removes them on
  the Blocks page, and the theme doesn't bring them back.

Put site content that the theme provides (a copyright line, badges) in
starter blocks rather than in templates: the site can then change it.

## Settings

`settings` are options the site's administrator sets on the theme's
settings page (**Appearance** → *Settings*,
`/admin/config/appearance/theme/<theme>`). Templates read them with
`themeSetting "<id>"`.

```yaml
settings:
  - id: accent
    label: Accent colour
    type: color
    default: "#b45309"
    css_var: --sand-accent
  - id: accent_dark
    label: Accent colour of the dark scheme
    type: color
    default: "#f59e0b"
    css_var: --sand-accent
    selector: ':root[data-bs-theme="dark"]'
  - id: scheme
    label: Colour scheme by default
    type: select
    default: light
    options:
      - value: light
        label: Light
      - value: dark
        label: Dark
  - id: marquee
    label: Marquee under the header
    type: bool
    default: true
  - id: tagline
    label: Line under the site's name (empty - the site's slogan)
    type: text
    max_length: 120
```

| Key | Meaning |
|---|---|
| `id` | its name in templates: lowercase latin letters, digits and `_`, starting with a letter; unique |
| `label` | its label on the settings page, in English (translated) |
| `type` | `color`, `select`, `bool` or `text` |
| `default` | its value until the administrator sets one; required for `color` and `select` |
| `options` | a `select`'s choices: `value` and `label` (translated) |
| `css_var` | a `color` only: the CSS variable (`--name`) the engine sets to it |
| `selector` | where `css_var` is set; `:root` by default |
| `max_length` | a `text` only: the longest value in characters, 200 by default |

The values by type:

| Type | Value | `themeSetting` gives |
|---|---|---|
| `color` | `#rrggbb` | the string |
| `select` | one of the `options`' values | the string |
| `bool` | on or off; `default: true` or `false` | `true` or `false` |
| `text` | one line up to `max_length` characters, may be empty | the string |

- A colour with `css_var` needs no template: the engine puts
  `<style id="theme-settings">:root { --sand-accent: #b45309; }</style>`
  after the theme's styles on every page, with the saved value or the
  default. Derive the theme's other shades from it (`color-mix()`), and
  one setting recolours the whole theme.
- For a dark scheme's colour, set the same variable in a stronger
  selector (`:root[data-bs-theme="dark"]`), both in `selector` and in the
  theme's CSS, so the `:root` value doesn't win.
- A text setting never becomes a CSS variable. An empty text is allowed:
  give the template a fallback,
  `{{or (themeSetting "tagline") .siteSlogan}}`.
- A saved value the theme no longer accepts (an option removed) reads as
  the default.
- Each theme keeps its own values. **Save and preview** on an inactive
  theme's settings page saves them and opens the [preview](./index.md#choosing-a-theme).

## Icon

`favicon` is the theme's icon: a path on the site starting with `/`,
usually a file of the theme's `static/`
(`/static/themes/sand/favicon.png`) or of the engine's
(`/static/images/kosatka-icon.svg`, which most bundled themes use).
PNG, SVG, ICO, GIF, WebP and JPEG are recognised; a PNG is also the home
screen icon on phones (180×180 px or larger).

The site's own icon, uploaded on **Site information**, replaces it in
every theme. The engine outputs the icon itself; templates don't.

The logo is not part of the manifest: it is uploaded on **Site
information** and shown by the *Logo* system block or by the theme's
header (see [Images, logo and icon](./assets-and-translations.md#images-logo-and-icon)).

## Subthemes

A theme can build on another: `base` names the base theme's directory.
The subtheme takes from its base everything it doesn't have itself, so it
can change a few files of a theme without copying it.

```yaml
name: Sand
base: kosatka
stylesheets: [sand]
settings:
  - id: accent
    label: Accent colour
    type: color
    default: "#b45309"
    css_var: --kosatka-accent
```

| What | How a subtheme gets it |
|---|---|
| templates | looked up in the subtheme, then its base (then the base's base), then the engine. Put in the subtheme only the files it changes |
| `regions`, `default_region` | its base's, if it declares no regions of its own |
| `settings` | its base's plus its own; one with the same `id` replaces the base's (a different default colour, say) |
| `blocks` | its base's, if it lists none |
| `favicon` | its base's, if it has none |
| stylesheets | `themeCSS "<name>"` finds the file in the first theme of the chain that has it, so the base's `theme_head` keeps working; the subtheme's own CSS goes in `stylesheets`, linked after the base's |
| translations | the subtheme's catalog, then its base's, then the engine's and the modules' |

`name`, `description` and the other descriptive keys are not inherited.
The block layout and the setting values are the subtheme's own, as for
any theme. The Appearance page shows "Subtheme of <base>" under its name.

A base must be installed, the chain must not loop, and it may be at most
8 themes deep, the subtheme itself included.

## Engine version

`min_version` is compared with the engine's release (from the build's
git tag: `v1.2.0`, or `v1.2.0-3-g1a2b3c4` three commits after it, both
1.2.0). A theme that needs a newer engine is listed with the reason, but
can't be previewed or made active. A build that isn't from a release (a
developer's) accepts every theme.

## Checks

The engine checks the manifest when it reads it. A theme whose manifest
doesn't parse or fails a check can't be chosen: the Appearance page and
`app theme list` leave it out of the themes and name its directory apart,
with the error; `app theme add` refuses it:

- there is at least one region (or a base theme that has them);
- `version` and `min_version` are `X.Y.Z`; `homepage` is an `http(s)`
  address; `license` is an SPDX expression; `tags` are at most 20
  lowercase words;
- `screenshot` is a `.png`, `.jpg` or `.webp` path inside `static/`;
- `favicon` is a path on the site (`/…`, not another host's `//…`);
- `stylesheets` are names of lowercase latin letters, digits, `-` and
  `_`;
- `base` exists and the chain doesn't loop;
- settings: valid IDs without duplicates, a known type, a select with
  options, `css_var` on a colour only, no `< > { } ;` in `selector`, a
  default valid for the type;
- blocks: a system block or a custom block with a title and a body, a
  title of at most 255 characters, a region of the theme's.

When the engine reads it:

- regions, labels and descriptions - every time the admin's block and
  appearance pages need them, so a deployed change applies without a
  restart;
- `settings`, `base`, `stylesheets` and `favicon` - once per run: restart
  the server after changing them.

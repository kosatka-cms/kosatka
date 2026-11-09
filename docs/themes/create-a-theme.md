---
title: Create a theme
description: A first theme step by step - a manifest, a replaced template, a stylesheet and a setting, then switching the site to it.
sidebar:
  order: 1
---

This tutorial makes a theme called *Sand*: the engine's layout with its
own footer, its own colours and an accent colour the site's
administrator can change. It takes a site running from source
([Run from source](../getting-started/from-source.md)) with
`env: local`, so template edits show up without a restart.

## 1. Make the directory

In the project's directory:

```bash
mkdir -p themes/sand/templates/partials themes/sand/static/css themes/sand/translations
```

The directory's name (`sand`) is the theme's key: it is in its asset
URLs (`/static/themes/sand/…`), in the admin's addresses and in the
database's block layout. Use lowercase latin letters, digits, `-` and
`_`, and don't rename it later.

## 2. Write the manifest

`themes/sand/theme.yaml`:

```yaml
name: Sand
description: A warm light theme with a credit line in the footer.
version: 0.1.0
author: Example Studio
license: MIT
tags: [light, two-columns]
default_region: sidebar
regions:
  - id: header
    label: Header
  - id: sidebar
    label: Sidebar
  - id: content_top
    label: Content top
  - id: content_bottom
    label: Content bottom
  - id: content_inline
    label: Inside content (content page)
  - id: footer
    label: Footer
  - id: footer_bottom
    label: Footer bottom line
blocks:
  - plugin: copyright
    region: footer_bottom
settings:
  - id: accent
    label: Accent colour
    type: color
    default: "#b45309"
    css_var: --sand-accent
  - id: credit
    label: Credit line in the footer
    type: bool
    default: true
```

- `regions` are the areas the theme's templates (and the engine's
  templates it keeps) render blocks into. These are the engine's own
  regions: keeping their IDs means the blocks placed for the current
  theme land in the same places when the site switches to *Sand*.
- `blocks` places the copyright line in the footer the first time the
  theme gets a block layout.
- `settings` adds two options to the theme's settings page: a colour that
  becomes the CSS variable `--sand-accent`, and an on/off switch that the
  footer template reads.

Every key is described in [the manifest reference](./manifest.md). Name,
labels and description are written in English; other languages come from
the theme's [translations](./assets-and-translations.md#translations).

## 3. Replace a template

The engine's footer is `web/templates/footer.html`. A file with the same
relative path in the theme replaces it. Copy it and add a line:

`themes/sand/templates/footer.html`:

```html
{{define "footer"}}
<footer class="site-footer">
    <div class="container">
        {{if regionHasBlocks "footer" .}}
        <div class="site-footer-region">
            {{template "region_blocks" (dict "Region" (region "footer") "Ctx" .)}}
        </div>
        {{end}}
        {{template "region_blocks" (dict "Region" (region "footer_bottom") "Ctx" .)}}
        {{if themeSetting "credit"}}
        <p class="sand-credit">{{t "Made of sand and patience"}}</p>
        {{end}}
    </div>
</footer>
{{template "html_scripts" .}}
{{template "html_end" .}}
{{end}}
```

- `region_blocks` renders the blocks placed in a region;
  `regionHasBlocks` leaves the wrapper out when there are none.
- `themeSetting "credit"` is the switch from the manifest (`true` or
  `false`), `t` translates a string to the page's language.
- The footer must end with `html_scripts` and `html_end`: they load the
  engine's scripts and close the document. See
  [Templates](./templates.md#the-page-and-the-html-document).

## 4. Add a stylesheet

`themes/sand/static/css/sand.css`:

```css
:root {
    --bs-body-bg: #fdf8f0;
    --bs-body-color: #2b2118;
    --bs-border-color: #eadfcf;
}

.site-header {
    border-bottom: 3px solid var(--sand-accent, #b45309);
}

.site-name,
.sand-credit {
    color: var(--sand-accent, #b45309);
}
```

The engine's markup uses Bootstrap 5 classes, so a theme mostly changes
Bootstrap's variables and styles the engine's classes (`site-header`,
`site-name`…). `--sand-accent` is set by the engine from the theme's
setting.

Pages link to the minified file, `sand.min.css`. Build it, and again
after each change to the CSS:

```bash
./scripts/minify_css.sh
```

## 5. Link the stylesheet

The engine links the theme's styles through the hooks in
`partials/theme_head.html`. A theme's copy of that file replaces the
engine's, and defines all three hooks.

`themes/sand/templates/partials/theme_head.html`:

```html
{{define "theme_framework"}}
    <link rel="stylesheet" href="{{ staticCSS "bootstrap" }}">
{{end}}
{{define "theme_head"}}
    <link rel="stylesheet" href="{{ staticCSS "base" }}">
    <link rel="stylesheet" href="{{ themeCSS "sand" }}">
{{end}}
{{define "theme_editor_css"}}{{ staticCSS "bootstrap" }},{{ staticCSS "base" }},{{ themeCSS "sand" }}{{end}}
```

- `theme_framework` - the Bootstrap build: the engine's Bootstrap 5.3
  here.
- `theme_head` - the engine's neutral `base.css`, then the theme's CSS.
  `themeCSS "sand"` is `/static/themes/sand/css/sand.min.css?v=…`.
- `theme_editor_css` - the same styles for the text editors, so text
  being edited looks as it does on the page (a comma-separated list of
  URLs).

A shorter way, when the engine's hooks suit you (Bootstrap, light or dark
as the visitor's device is, and `base.css`): leave
`partials/theme_head.html` out and add `stylesheets: [sand]` to the
manifest. The engine then links `css/sand.min.css` after its own styles
(the text editors don't get it then: the engine's `theme_editor_css` is
empty). See [Stylesheets](./assets-and-translations.md#stylesheets).

## 6. Make it active

Open **Appearance** (`/admin/config/appearance`). *Sand* is in
the list. If it isn't, its manifest has an error: a theme whose
`theme.yaml` doesn't parse or fails a check is left out of the list, and
the page names its directory above the list with what is wrong. Compare
it with [the manifest reference](./manifest.md#checks).

1. **Preview** shows your site in *Sand* to you alone; the visitors keep
   the active theme.
2. **Settings** shows the accent colour and the credit switch. Change
   the colour and press **Save and preview**.
3. **Set as default** switches the site to *Sand*, at once.

The Blocks page (`/admin/config/blocks`) now has a *Sand* tab with the
blocks copied from the previous theme and the copyright line.

## 7. See what rendered what

Stop the server and start it again with template debugging:

```bash
TEMPLATES_DEBUG=1 go run ./cmd/app
```

The page source then has comments naming the template files used, the
theme first:

```html
<!-- PAGE: home/index.html - theme: sand → (engine) -->
…
<!-- BEGIN footer - themes/sand/templates/footer.html -->
```

To change another part of the page, find the file in these comments and
copy it into the theme under the same path. See
[Template debugging](./templates.md#template-debugging).

## What changes need a restart

With `env: local` templates show up on reload, and CSS too once
minified. The browser keeps static files for a year under the same
`?v=`, though: after changing a stylesheet, reload bypassing the cache (a
hard reload, or with the cache off in the browser's developer tools).

The server reads some things once per run and needs a restart after you
change them:

- the theme's settings, `base`, `stylesheets`, `favicon` and `logo` in
  `theme.yaml` (regions, labels and description are re-read by the admin
  pages);
- the theme's `translations/*.json`.

## Next

- Replace more templates: [Templates](./templates.md).
- Translate the strings: [Translations](./assets-and-translations.md#translations).
- Build on an existing theme instead of starting from the engine:
  [Subthemes](./manifest.md#subthemes).
- Publish it: [Distributing a theme](./distribute.md).
- Give it a demo site, installed with one command: [A theme's demo site](./demo-site.md).

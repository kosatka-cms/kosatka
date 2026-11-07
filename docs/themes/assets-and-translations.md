---
title: Styles, images and translations
description: A theme's static files - stylesheets and their minified copies, scripts, images, the logo and icon - and the translations of its strings.
sidebar:
  order: 4
---

## Static files

Everything in the theme's `static/` is served at
`/static/themes/<theme>/`:

| File | URL |
|---|---|
| `themes/sand/static/css/sand.min.css` | `/static/themes/sand/css/sand.min.css` |
| `themes/sand/static/images/wave.svg` | `/static/themes/sand/images/wave.svg` |
| `themes/sand/static/screenshot.jpg` | `/static/themes/sand/screenshot.jpg` |

The application serves these files itself, with long-lived caching. When
a web server in front of it serves `/static/` from `web/static` (as in
[Running in production](../guides/production.md)), the theme's `static/`
is copied to `web/static/themes/<theme>/` on deploy, so the URLs stay the
same.

Within the theme's CSS, refer to its own files relatively
(`url(../images/wave.svg)` from `css/`), so the stylesheet doesn't depend
on the theme's directory name.

## Stylesheets

The engine's markup is Bootstrap 5 markup. A theme links its CSS from
three hooks in its own `partials/theme_head.html`, which replaces the
engine's file and must define all three:

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

| Hook | What |
|---|---|
| `theme_framework` | the Bootstrap 5 build and its fonts. The engine ships Bootstrap 5.3 (`staticCSS "bootstrap"`) and the Bootswatch *Cosmo* skin (`staticCSS "bootswatch-cosmo"`) at the version of its JavaScript, without the skin's font (a theme on it links Source Sans Pro, or another font, itself: see [Fonts](#fonts)); a theme can link a build of its own from its `static/` instead. A script that sets the colour scheme before the page paints goes here too |
| `theme_head` | the theme's stylesheets. `base.css` is the engine's neutral styling of its markup (header, footer, cards, lists); keep it and add to it, or leave it out and style everything yourself |
| `theme_editor_css` | the stylesheet URLs, comma-separated, that the text editors use, so text being edited looks as on the page |

Without its own `partials/theme_head.html` a theme gets the engine's
hooks (a subtheme, its base's): Bootstrap, light or dark as the visitor's
device is, and `base.css`; the engine's `theme_editor_css` is empty, so
the editors keep their default look.

In the page's `<head>` the styles come in this order: `theme_framework`,
`theme_head`, the manifest's `stylesheets`, the colours of the theme's
settings (`<style id="theme-settings">`), then the engine's `menu.css`
and `components.css`, and, loaded without blocking the page, the
modules' CSS, `photoswipe.css` (the lightbox) and `overlays.css` (the
scripts' windows and toasts). The engine's components and most of the
modules' blocks write their rules inside `:where()` and in Bootstrap's
variables, so a theme's rule wins over them and they follow the theme's
colours and its light or dark scheme. Other modules' styles (the pages
of the forum and of private messages, the ratings, embedded videos…)
are at ordinary specificity after the theme's CSS: a theme restyles them
with a more specific selector.

`themeCSS "<name>"` is the URL of `css/<name>.min.css` in the theme,
versioned. In a subtheme it finds the file in the first theme of the
chain that has it, so a base theme's `theme_head` works unchanged.
`themeCSSAsync "<name>"` gives the tags that load it without blocking
the page.

### The manifest's stylesheets

`stylesheets` in `theme.yaml` lists more files of `css/` the engine links
by itself, after `theme_head`:

```yaml
stylesheets: [sand]
```

This links `/static/themes/sand/css/sand.min.css`. It is the way for a
[subtheme](./manifest.md#subthemes) to add its CSS on top of its base's,
and works for any theme, with its own hooks or the engine's. The bundled
themes add them to `theme_editor_css` too
(`{{range themeStylesheets}},{{.}}{{end}}`); a theme with its own
`theme_editor_css` should do the same.

### Styling the engine's markup

- **Change Bootstrap through its variables**, not by rewriting its
  classes: colours, background and fonts are `:root` variables
  (`--bs-body-bg`, `--bs-body-color`, `--bs-border-color`,
  `--bs-primary`, `--bs-body-font-family`…); a component has its own
  (`--bs-btn-bg`, `--bs-btn-hover-bg`…). Write a rule for a class itself
  only where the variables don't reach (a shape, a border), and then for
  all its variants at once. The [style guide](./templates.md#the-style-guide)
  shows whether they still match.
- **A dark scheme**: Bootstrap's `data-bs-theme="dark"` on `<html>`. Set
  the dark values in `:root[data-bs-theme="dark"]`.
- **The engine's accents**: `base.css` sets and uses `--kosatka-accent`,
  `--kosatka-accent-hover`, `--kosatka-radius`, `--kosatka-reading` and
  `--kosatka-shadow`; the profile pages (`profile.css`) read `--kosatka-link`,
  `--kosatka-tint`, `--kosatka-line`, `--kosatka-highlight`… when the
  theme sets them, and Bootstrap's colours without them.
- **The content form** (`/node/{id}/edit`, `/create/<type>`) is a service
  page (no sidebar) of two columns: `node-edit-main` with the fields on
  `node-edit-card` cards and `node-edit-aside` with the save card
  (`node-edit-save`, kept in view) and the settings
  (`node-settings-section`). Its cards take `--kosatka-surface` (the
  theme's card colour; the page's background without it), the save card
  sticks below `--node-edit-sticky-top` (the theme's sticky header
  height; `base.css` sets `5rem`).
- **The main menu** is the theme's to style; the engine's `menu.css` only
  styles the user menu.
- **Carousels and the lightbox** need no script of the theme's: a
  container with `data-view-carousel` becomes a Swiper carousel, and
  links `a.content-image-link` inside `.node-content`, `.item-content`
  or a container with `data-lightbox` open in the image viewer. Their
  attributes are in Page scripts: [carousels](./scripts.md#carousels),
  [the lightbox](./scripts.md#the-lightbox).
- **The editors**: a content field's editor has the class `node-content`
  on its body, so the theme's rules for text apply there; rules only for
  the editor go on `.mce-content-body`. Keep the editor's background a
  field's (white by default), not the page's.
- Don't write `<style>` in templates; colours the administrator can
  change are [settings](./manifest.md#settings) with `css_var`.

### Fonts

Serve a theme's fonts from its `static/`, not from a font service: the
pages then make no requests to other servers (the
[themes catalog](https://github.com/kosatka-cms/themes) doesn't take a
theme that does). Put the font files (`woff2`) in `static/fonts/` with
their license, their `@font-face` rules in a stylesheet of their own,
`css/fonts.css`, with URLs relative to it
(`url(../fonts/inter/inter-latin.woff2)`), and link it in
`theme_framework` and in `theme_editor_css`:

```html
<link rel="stylesheet" href="{{ themeCSS "fonts" }}">
```

A file per script, each with its `unicode-range` (Latin, Cyrillic…),
lets a page load only the ones its text needs. The Kosatka theme does
so.

### Minified files

Pages link the minified copy, `<name>.min.css`. The minified files are
built, not written, and are not kept in git. Build them after changing a
stylesheet:

```bash
./scripts/minify_css.sh
```

It minifies the engine's CSS, every theme's `static/css/*.css` and the
modules'; `./scripts/minify_js.sh` does the same for scripts. The Docker
image builds them itself. A theme installed from its repository leaves
the `.min` files out of its checksum, so building them on the site is
fine. Until a theme's file is minified the application serves its source
in its place (`sand.min.css` → `sand.css`); a web server serving
`/static/` itself (see [Running in production](../guides/production.md))
doesn't.

The asset URLs the template functions give (`themeCSS`, `staticCSS`…)
end with `?v=<version>`: `STATIC_ASSET_VERSION`, else the git commit the
binary was built from, else the working directory's. A new deploy gets
new URLs, and browsers fetch the new files.

## Scripts

Put the file in `static/js/` and link it in the theme's `footer.html`,
after the engine's libraries, with `themeJS` (the URL of its minified
copy):

```html
{{template "html_scripts" .}}
<script src="{{ themeJS "sand" }}" defer></script>
{{template "html_end" .}}
```

Write the script as a [behavior](./scripts.md#behaviors), so it also
works on markup that arrives later (lazy blocks, the next page of a
list). Short scripts that must run before the page paints (choosing the
colour scheme) belong inline in `theme_framework`. The shared helpers
(`Kosatka.t`, `Kosatka.escapeHtml`, the dialogs…), the `data-`
attributes the engine handles and the order the scripts load in are
described in [Page scripts](./scripts.md).

## Images, logo and icon

- **Images** of the theme go in `static/images/` (or anywhere in
  `static/`). Link them from templates with their full URL,
  `/static/themes/<theme>/images/…`, and from CSS relatively.
- **The logo** is the site's, not the theme's: it is uploaded on **Site
  information** (`/admin/config/system/site-information`). Templates get
  its URL as `.siteLogoURI` (empty when none is uploaded), and the *Logo*
  system block shows it in whatever region it is placed. A new site gets
  the engine's logo (`/static/images/kosatka-logo.svg`).
- **The icon** (favicon) is the manifest's
  [`favicon`](./manifest.md#icon). The site's own icon, uploaded on **Site
  information**, replaces it in every theme. The engine outputs the
  `<link rel="icon">` itself and redirects `/favicon.ico` to the current
  icon.
- **The screenshot** for the Appearance page is the manifest's
  `screenshot`, a 3:2 picture in `static/`.

## Translations

The theme's strings are written in English in its templates and
manifest; other languages are in `translations/<lang>.json`:

```json
{
  "strings": {
    "Sand": "Песок",
    "A warm light theme with a credit line in the footer.": "Тёплая светлая тема со строкой в подвале.",
    "Accent colour": "Акцентный цвет",
    "Credit line in the footer": "Строка в подвале",
    "Made of sand and patience": "Из песка и терпения",
    "light": "светлая",
    "@count grain": ["@count песчинка", "@count песчинки", "@count песчинок"]
  },
  "contexts": {
    "back to the page's start": {"Top": "Наверх"}
  }
}
```

- `strings` - source → translation. A `plural` string has one form per
  plural category the language uses for whole numbers, in the order
  zero, one, two, few, many, other (Russian has three: one - 1, 21…;
  few - 2-4, 22-24…; many - 0, 5-20, 25…).
- `contexts` - the strings of `tc "<context>" "<source>"`, by context:
  `{{tc "back to the page's start" "Top"}}`.
- What to translate: everything the theme's templates and scripts pass
  to `t`, `tc`, `tjs`, `plural`, `Kosatka.t`, `Kosatka.tc` and
  `Kosatka.formatPlural`, and in the manifest its `name`, `description`,
  `tags`, and the labels of its regions, settings and options.

A string is looked up in the theme's catalog first, then its base
theme's, then the engine's and the modules'. So a theme translates only
its own strings and reuses the engine's (*Edit*, *Save*…); it can also
replace an engine's or a module's translation by putting the same key in
its catalog.

No command lists a theme's untranslated strings: a theme installed
from its repository makes its strings known with its catalogs. A string
its templates use that none of its catalogs has is shown in English and
listed nowhere, so add each new string to every language's catalog at
once. On a site with the theme active, *Interface translations* →
*Strings* (*Show*: *Untranslated*) lists the theme's strings a
language's catalog lacks while another of its catalogs has them.

The catalogs are read at startup and when the theme changes: restart
the server after editing them.

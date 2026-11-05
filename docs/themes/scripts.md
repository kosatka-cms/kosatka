---
title: Page scripts
description: How JavaScript works on the site's pages - the order scripts load in, behaviors for markup loaded later, the shared helpers and data- attributes, and how themes and modules add their scripts.
sidebar:
  order: 5
---

This page is for theme and module authors who write JavaScript for the
site's pages. It covers what the engine loads and in what order, how a
script brings markup to life (including markup that arrives after the
page has loaded), the helpers every script can use, and the `data-`
attributes the engine's scripts handle without any code of yours.

## What loads, in what order

The HTML document is the engine's (`partials/html.html`, see
[Templates](./templates.md#the-page-and-the-html-document)). Its scripts
come in this order:

In `<head>` (`html_start`):

1. `<meta name="csrf-token">` and `csrf.js` - before any other script
   (see [CSRF](#csrf-tokens)).
2. The scripts in the theme's `theme_framework` hook, which comes with
   the styles (one that sets the colour scheme, say).
3. `window.USER_DATA` for a logged-in visitor (an inline script).
4. `/i18n/<lang>.js?v=…` - the page language's translations for scripts
   and `Kosatka.t`, `Kosatka.tc`, `Kosatka.formatPlural`. A plain script:
   it creates the `Kosatka` object.
5. `app.js`, the engine's script, with `defer`.
6. The modules' scripts, with `defer` - only those of the modules the
   page attached (see [A module's scripts](#a-modules-scripts)).
7. The tracking codes for `<head>` set on the Integrations page.

At the end of `<body>`:

8. `html_scripts`, in the theme's `footer.html`: the libraries -
   Bootstrap (`bootstrap.bundle.min.js`), PhotoSwipe and Swiper, as plain
   scripts.
9. The theme's scripts, which the theme links after `html_scripts`.
10. `html_end`: counting the content item's view and the tracking codes
    for the end of the page.

Plain scripts run as the browser reaches them. Scripts with `defer` run
in document order after the document is parsed and before the
`DOMContentLoaded` event, so a deferred theme script runs after `app.js`
and the modules' scripts, and all the libraries are there already.

The admin pages load `csrf.js`, the translations, `app.js` and their own
`admin.js` and `admin_bulk.js`; they don't use the theme's header,
footer or scripts.

## Behaviors

Markup reaches a page in two ways: with the page, or later - a block
loaded on scroll, the next page of a list loaded in place, the comments,
a module's fragment (a form or a poll in a block, a content item's
galleries). Code that looks for its elements once on page load never
sees the second kind. So the engine's, the modules' and the themes'
scripts attach to markup through **behaviors**.

A behavior is an object with an `attach(context)` function in
`Kosatka.behaviors`:

```js
Kosatka.behaviors.sandToggle = {
    attach: function (context) {
        Kosatka.once('sand-toggle', '[data-sand-toggle]', context).forEach(function (el) {
            el.addEventListener('click', function () {
                document.body.classList.toggle('sand-wide');
            });
        });
    }
};
```

- `Kosatka.attachBehaviors(context)` calls `attach` of every behavior.
  `app.js` calls it for `document` on `DOMContentLoaded`; code that puts
  HTML in the page calls it for the element it filled:

  ```js
  el.innerHTML = html;
  Kosatka.attachBehaviors(el);
  ```

  An error in one behavior is logged to the console and doesn't stop
  the others.
- `attach` looks only inside `context` (`context` itself can match), and
  it may be called again for markup it has seen.
  `Kosatka.once(id, selector, context)` returns the matching elements not
  yet handled under `id` and marks them (the `data-once` attribute lists
  the ids), so each element is set up once.
- Register a behavior before `DOMContentLoaded`: in a deferred script, or
  in a plain one at the end of `<body>`. A plain script there runs before
  the deferred `app.js`, so it starts with
  `Kosatka.behaviors = Kosatka.behaviors || {};` (`app.js` keeps what is
  already there). A behavior registered after `DOMContentLoaded` calls
  `Kosatka.attachBehaviors` for its markup itself.
- Markup that is on the page once and listens on `document` (a header
  widget, say) can stay plain code on `DOMContentLoaded`. Anything that
  may come in a fragment needs a behavior.

The engine and the modules put HTML in the page and run the behaviors
for it in these places: blocks loaded on scroll, pages of a list and its
glossary loaded in place, the comments and their pages, the *Form* and
*Poll* blocks, a content item's galleries, a block added to a blocks
field in the content form.

### The engine's and the modules' behaviors

| Behavior | Where | What |
|---|---|---|
| `lazyBlocks` | `app.js` | blocks loaded on scroll: fetches the content near the viewport, then runs the behaviors for it |
| `viewCarousels` | `app.js` | carousels (`[data-view-carousel]`, Swiper) |
| `contentGallery` | `app.js` | images in content open in PhotoSwipe (those shown smaller than their original: `a.content-image-link`); the lightbox restarts when HTML with a gallery arrives |
| `comments` | `app.js` | a content item's comments load when their section nears the viewport |
| `commentPager` | `app.js` | the comments' pages change in place |
| `editors` | `app.js` | `textarea[data-editor-autoload]` gets its editor at once |
| `textFormatPicker` | `app.js` | a text format select (`.text-format-picker`) puts the format's editor on its textarea, or takes it off |
| `userAutocomplete` | `app.js` | `input[data-user-autocomplete]` suggests users |
| `passwordStrength` | `app.js` | the password rules list (`partials/password_strength.html`) marks the rules met |
| `copyText`, `passwordToggle` | `app.js` | the copy, print and show-the-password [buttons](#buttons) |
| `pollBlocks`, `pollVote` | Polls module | the *Poll* block (`[data-poll-embed]`) loads its poll; voting without reloading the page |
| `nodeGalleries` | Galleries module | the galleries block on a content page |
| `webformBlocks`, `webformConditions` | Forms module | the *Form* block (`[data-webform-embed]`) loads its form; fields shown or hidden by conditions |

The table leaves out the behaviors of the content form (its fields, the
media picker, autosave) and the modules' other widgets (ratings,
reports, messages, their forms).

## Shared helpers

`app.js` and the translations script give every script these functions.
Use them instead of keeping your own copies.

### Translations

```js
Kosatka.t('Saved');
Kosatka.t('Error: @error', {'@error': message});
Kosatka.tc('verb', 'Search');
Kosatka.formatPlural(n, '@count comment', '@count comments');
```

- `Kosatka.t(source, args)` - the string in the page's language;
  placeholders (`@name`) are replaced from `args`.
- `Kosatka.tc(context, source, args)` - the same for a string with a
  context.
- `Kosatka.formatPlural(n, singular, plural, args)` - the plural form
  for `n`, with `@count` set to `n`. The forms come from the catalog's
  entry for `singular`.
- `Kosatka.lang` - the page's language.

Like the templates' `tjs`, these return plain text: the values put in
are not escaped. Escape the result when you build HTML from it.

The engine finds the strings scripts translate at startup: it scans the
`.js` files (not `*.min.js`) of the engine, the modules and the themes'
`static/`, and the inline scripts of templates, for calls with a
literal string - `Kosatka.t('…')` or `Kosatka.t("…")`. Only those
strings go into `/i18n/<lang>.js`, so pass the source as a literal, not
a variable. Their translations go in the theme's or module's
`translations/<lang>.json`, as for templates
([Translations](./assets-and-translations.md#translations)).

### Escaping HTML

A script that builds HTML in a string puts text and attribute values in
only through `Kosatka.escapeHtml(s)`: it escapes `& < > " '`, and `null`
or `undefined` become an empty string.

```js
list.innerHTML = items.map(function (item) {
    return '<li><a href="' + Kosatka.escapeHtml(item.url) + '">' +
        Kosatka.escapeHtml(item.title) + '</a></li>';
}).join('');
Kosatka.attachBehaviors(list);
```

### Dialogs

`Kosatka.confirm(message)`, `Kosatka.alert(message)` and
`Kosatka.prompt(message, value)` show the site's own dialog instead of
the browser's `confirm`, `alert` and `prompt`, which block the page and
can't be translated or styled. Each returns a Promise: `confirm`
resolves to `true` for OK, `prompt` to the text or `null`, `alert` once
it is closed. Escape and a click outside the dialog cancel. `confirm`'s
second argument names its OK button.

```js
Kosatka.confirm(Kosatka.t('Delete this file?'), Kosatka.t('Yes, delete')).then(function (yes) {
    if (yes) { /* … */ }
});
```

The dialog is a `<dialog class="kosatka-dialog">` (with
`.kosatka-dialog-message`, `.kosatka-dialog-input` and
`.kosatka-dialog-buttons`), styled by the engine's `components.css`; a
theme can restyle it: these rules are inside `:where()`, so a theme's
rule wins over them.

### Windows and toasts

`Kosatka.window(options)` opens a window over the page for a tool bigger
than a dialog: a head with `options.title` and a close button, the
`body` to fill and a foot for the buttons. Escape, a click beside the
window and the close button shut it; the focus then goes back where it
was (or `options.onClose` runs). `options.className` adds a class to
style the window's content.

```js
var win = Kosatka.window({title: Kosatka.t('Choose a colour'), className: 'my-colours'});
win.body.appendChild(list);
win.button(Kosatka.t('Cancel')).addEventListener('click', win.close);
var ok = win.button(Kosatka.t('Choose'), true);
```

`Kosatka.toast(message, type, ms)` shows a short note in the bottom
corner of the page that goes by itself after `ms` milliseconds (3
seconds by default); `type` is `info` (the default), `success`,
`warning` or `error`.

```js
Kosatka.toast(Kosatka.t('Saved'), 'success');
```

Both are styled by the engine's `overlays.css` (`.kosatka-window…`,
`.kosatka-toast`, `.kosatka-toast--<type>`); the toasts' rules are inside
`:where()`, so a theme can restyle them.

### Reordering by dragging

`Kosatka.sortable(container, options)` lets the visitor drag the items
of a list or a grid into a new order (the browser's drag and drop, no
library). The items are `options.items` (a selector) inside the
container; each, or its `options.handle`, has `draggable="true"`. A
dragged item goes before or after the one under the pointer - by its
top or bottom half, or with `axis: 'x'` (a grid) its left or right half.
`options.end(moved)` runs once it is dropped, e.g. to save the order.

```js
Kosatka.sortable(document.getElementById('my-list'), {
    items: '.my-item',
    end: function () { /* save the order */ }
});
```

The dragged item has the class `dragging` meanwhile. Items added to the
container later move too.

### Suggestions under an input

`Kosatka.autocomplete(input, options)` shows suggestions under a text
input; the visitor picks one with the mouse or the keyboard (the arrows
and Enter; Escape closes the list). `options.source(query)` returns the
items for what is typed - an array or a Promise of one - once
`options.minChars` characters are typed (1 by default) and the typing
pauses; `options.label(item)` is an item's text, `options.pick(item)`
runs when it is chosen. `options.empty` is the text when nothing matches,
`options.group(item)` the heading an item goes under.

```js
Kosatka.autocomplete(input, {
    source: function (q) {
        return fetch('/my/search?q=' + encodeURIComponent(q)).then(function (r) { return r.json(); });
    },
    label: function (item) { return item.title; },
    pick: function (item) { input.value = item.title; hidden.value = item.id; }
});
```

The list is `ul.kosatka-autocomplete` (its items
`.kosatka-autocomplete-item`, the marked one `[aria-selected="true"]`),
styled by `overlays.css` inside `:where()`.

### CSRF tokens

The site rejects a state-changing request (POST and the like) without the
session's CSRF token. `csrf.js` adds it, so scripts and forms don't need
to:

- every POST form gets a hidden `csrf_token` field - on load, when it is
  added to the page later, and on submit;
- every `fetch` and `XMLHttpRequest` to the site with a method other
  than GET, HEAD or OPTIONS gets the `X-CSRF-Token` header.

The token is in `<meta name="csrf-token">`. A guest who only reads has
none (no session, so the page can be cached): on the first POST form or
request, `csrf.js` asks `/api/csrf-token` for one, then sends it. So a
script simply calls `fetch`:

```js
fetch('/api/…', {method: 'POST', body: new FormData(form), credentials: 'same-origin'})
    .then(function (r) { return r.json(); });
```

### The editor

`Kosatka.loadEditor(textarea)` puts the TinyMCE editor on a textarea and
resolves with the editor; its settings are JSON in the textarea's
`data-editor-config`. TinyMCE itself loads on the first call. A page
with an editor includes `{{template "tinymce_user_files.html" .}}`
(`partials/tinymce_user_files.html`), which adds uploads, the file
picker and the theme's `theme_editor_css` stylesheets to the settings.

```html
<textarea name="content" data-editor-config="{{json (dict "preset" "comment" "height" 200)}}"></textarea>
```

`preset` starts from settings the engine's forms share; the other keys
override them:

| Preset | Toolbar | Used by |
|---|---|---|
| `comment` | bold, italic, lists, a link, the HTML code | comments |
| `text` | headings, bold, italic, alignment, lists, a link, an image, the HTML code | content fields without a text format, Views texts |

Every editor has no menu bar. A toolbar that lists `insertbreak` gets the
button that inserts `<!--break-->` (where the teaser ends). A script can
build the same settings with `Kosatka.editorConfig(settings)`.

With `data-editor-autoload` the editor is put on at once (the `editors`
behavior).

### The visitor

For a logged-in visitor the page sets `window.USER_DATA`: `isLoggedIn`,
`username`, `avatar` (a URL) and `creatableMaterials` (the content types
the user may create: `url`, `label`). For a guest it is not set.

The desktop user menu (`blocks/user_menu.html`) is rendered for the
visitor by the server: a guest's *Log in* / *Register* links, a signed-in
visitor's dropdown - one of them, never both (no script hides one).
The mobile menu (`app.js`) repeats it: a guest's *Log in* and
*Register* (when the page has a register link), or for a signed-in
visitor the profile, log out, *Create* and the items of the desktop user
menu (`#user-dropdown-menu`) marked `data-mobile-user-menu` - the
modules' items (their `user_menu` hook), for example *Messages*.

## Shared data- attributes

Behaviour that pages repeat is shared code in `app.js`; the template
only marks up the elements. Use these instead of inline `onclick`,
`onsubmit` or `onchange` and instead of copying a script between
templates. They work in markup added later, too.

### Confirmation

`data-confirm="<question>"` on a form, a button or a link: the action
waits for OK in the site's dialog, then goes ahead.

```html
<form method="post" action="/…/delete" data-confirm="{{t "Delete this item?"}}">
    <button type="submit" class="btn btn-danger">{{t "Delete"}}</button>
</form>
```

### Buttons

- `data-copy-text="<text>"` on a button copies the text to the
  clipboard; `data-copied="<words>"` is the button's label for a moment
  after.
- `data-print` on a button prints the page.
- `data-password-toggle="<input id>"` on a button shows and hides the
  password typed in that input (`aria-pressed`); `data-label-hide` is the
  button's `aria-label` while the password shows.

### Carousels

A container with `data-view-carousel` becomes a Swiper carousel. This is
what a View shown as a carousel renders, and a theme's template can use
it too. Its options:

| Attribute | What |
|---|---|
| `data-slides-per-view` | slides in a row on wide screens (default 1); narrower screens show fewer |
| `data-responsive="0"` | the same number of slides at every width |
| `data-space-between` | the gap between slides in px, at every width |
| `data-loop="1"` | loop |
| `data-autoplay="1"` | change slides every 5 seconds |
| `data-effect="fade"` | crossfade instead of sliding |
| `data-show-navigation="1"` | arrows: `.swiper-button-prev` and `.swiper-button-next` inside the container |
| `data-show-pagination="1"` | dots: `.swiper-pagination` inside the container |

```html
<div class="swiper" data-view-carousel data-slides-per-view="3" data-loop="1" data-show-navigation="1">
    <div class="swiper-wrapper">
        <div class="swiper-slide">…</div>
        <div class="swiper-slide">…</div>
    </div>
    <div class="swiper-button-prev"></div>
    <div class="swiper-button-next"></div>
</div>
```

The Swiper stylesheet is linked only on pages that show a carousel View.
A theme that puts a carousel in its own template links
`{{ staticCSS "swiper-bundle" }}` for that page itself.

### The lightbox

Links `a.content-image-link` inside `.node-content`, `.item-content`
or any container with `data-lightbox` open in the
PhotoSwipe image viewer. Each link needs the image's size in
`data-pswp-width` and `data-pswp-height`; `data-pswp-caption-html` is
its caption. A link with `data-lightbox-skip` is followed instead.

```html
<div data-lightbox>
    <a href="/files/photo.jpg" class="content-image-link" data-pswp-width="1600" data-pswp-height="1067">
        <img src="/files/photo-small.jpg" alt="">
    </a>
</div>
```

### Blocks loaded on scroll

A block placement with **Load on scroll** set
([Blocks](../guides/blocks.md)) renders as
`<div class="lazy-block" data-lazy-block="<URL>">` holding the theme's
placeholder (`block_placeholder_<region>`, see
[Templates](./templates.md#regions-and-blocks)). The `lazyBlocks`
behavior fetches the content when the place is within 400px of the
viewport, replaces the placeholder with it and runs the behaviors for
it. While it works, `data-lazy-state` is `loading`, then `loaded` or
`error` - something a theme can style.

### Forms

- **Comment form** - a hidden `[data-comment-form]` with
  `input[name=parent_id]`, a `textarea` with `data-editor-config`, and
  `[data-reply-info]` holding `[data-reply-author]` and
  `[data-cancel-reply]` buttons. A link with `data-comment-open` opens the
  form (and its editor); a link with `data-reply-to="<comment id>"
  data-author="<name>"` opens it as a reply. `#comment-form` in the
  address opens it on load.
- **User autocomplete** - `input[data-user-autocomplete]` suggests users
  after three characters (`Kosatka.autocomplete`). With
  `data-user-id-input="<hidden input id>"` it picks one user (the name in
  the input, the id in the hidden input, which gets a `change` event);
  with `data-user-list="<container id>"` and
  `data-user-field="<field name>"` it picks several (each a tag with a
  hidden input).

### Admin pages

`admin.js` handles a few more attributes on admin pages, for a module's
admin forms:

- `data-show-when="<field id>=<value>"` - the element is shown only while
  the field has that value; several values with `|`
  (`<id>=text|list`).
- `<select data-filter-by="<select id>">` - its options with
  `data-filter-value` are offered only while that select has that value.
- `<tbody data-sortable data-order-input="<hidden input id>">` - rows
  `tr[data-sort-id]` are dragged into a new order (by the row, or by its
  `.drag-handle`); the hidden input gets the ids, comma-separated.
- `<tbody data-tree-sortable …>` (a tree: `data-sort-id`,
  `data-parent-id`, `data-depth`) and `<tbody data-group-sortable …>`
  (fields with groups) - the same for trees and groups.

## Markup the engine's scripts look for

`app.js` finds some of the engine's markup by id or class. A theme that
replaces one of these templates keeps the names, or the feature stops
working:

| Feature | Names |
|---|---|
| live search (`blocks/search_form.html`) | `.block-search`, `#siteSearch`, `#search-main`, `.search_autocomplete_results` |
| user menu (`blocks/user_menu.html`) | `#user-menu-toggle` and `#user-dropdown-menu` inside `.user-menu-dropdown`; the *Create* submenu's `.dropdown-create-submenu`, `#user-create-submenu-toggle` and `#user-create-submenu`; `#logout-link` (logging out in one click, not through the logout page) |
| comments (`comments.html`) | `#comments` with `data-node-id`, the `.comments-content` the comments load into (and `.comments-loading`), its `.pagination`, the comment form's attributes above |
| a View's list loaded in place | a list container with `data-ajax-pagination="1"` (`#view-list-container` of a page, `#view-list-container-block-<id>` of a block, `.view-block-pages` around a theme template's block), the `.pagination` inside it; the glossary `#view-glossary` with `a.glossary-letter` |
| the lightbox | `a.content-image-link` in `.node-content`, `.item-content` or `[data-lightbox]` |
| back to top | `#back-to-top` (rendered by `html_end` when it is on) |

A theme's header can also use the engine's mobile menu: when the main
menu's `ul.menu.ul-depth-0` is inside `#main-menu`, a `#menu-icon` button
inside `#nav-wrap` opens and closes it on screens up to 768px wide, and
`app.js` adds the visitor's account links to it.

## A theme's scripts

A theme keeps its scripts in `static/js/`, served at
`/static/themes/<theme>/js/`, and links them in its `footer.html`, after
`html_scripts` and before `html_end`. `{{ themeJS "<name>" }}` is the
URL of `js/<name>.min.js`, versioned (`?v=`, a new value with every
deploy) and taken from the theme's base when the theme hasn't the file:

```html
{{template "html_scripts" .}}
<script src="{{ themeJS "sand" }}" defer></script>
{{template "html_end" .}}
```

- Use `defer`: the script then runs after `app.js`, and the libraries
  and the shared helpers are there.
- Write widgets as [behaviors](#behaviors), so they also work in blocks
  loaded on scroll and lists loaded in place.
- `scripts/minify_js.sh` writes `js/<name>.min.js` next to each
  `js/<name>.js` of every theme under `themes/`, an installed one
  included, as it does for the engine's and the modules' scripts, so a
  theme's repository needn't contain the `.min.js`. Until a script is
  minified the application serves its source in its place; a web server
  serving `/static/` itself doesn't, so run the script after each change.
- A theme's static files are served with long-lived caching: link them
  with `themeJS`, so every deploy changes their URL.
- A short script that must run before the page paints (choosing the
  colour scheme) goes inline in the `theme_framework` hook, in
  `partials/theme_head.html`
  ([Stylesheets](./assets-and-translations.md#stylesheets)). A value from
  the theme's settings can go into it:
  `var mode = {{themeSetting "scheme"}};`.

## A module's scripts

A module keeps its scripts in its `static/` and lists them in its
manifest's `JS`, without the extension
([The module SDK](../modules/sdk.md#the-manifest)). A page loads them
only when it shows something of the module: the module's templates call
`{{attach "<module>"}}`, and a block of the module attaches it itself
(also when it is loaded on scroll). The engine then links each script
deferred, after `app.js`:

```html
<script src="/static/modules/hello/hello.min.js?v=<version>" defer></script>
```

`scripts/minify_js.sh` builds the `.min.js` of the engine's modules; a
module from its own repository may ship only `hello.js`, which is served
in its place. Everything said above about behaviors and the shared
helpers applies to a module's scripts too.

## Minified files and versions

The engine's own scripts are linked minified, through
`{{ staticJS "<name>" }}` - `/static/js/<name>.min.js?v=<version>`. The
minified files are built, not written, and are not kept in git. After
changing the engine's or a module's script:

```bash
./scripts/minify_js.sh
```

The `?v=` is `STATIC_ASSET_VERSION`, else the git commit the binary was
built from, else the working directory's
([Configuration](../reference/configuration.md#other-variables)), so a
new deploy gets new URLs and browsers fetch the new files. The
translations script is versioned by its content instead.

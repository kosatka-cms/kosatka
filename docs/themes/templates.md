---
title: Templates
description: How the engine's templates are put together, how a theme replaces them, template suggestions, the data and functions templates get, and debugging.
sidebar:
  order: 3
---

Templates are Go's [html/template](https://pkg.go.dev/html/template)
files. The engine's are in `web/templates/`; a theme's are in its
`templates/` directory, with the same relative paths.

## How a theme replaces a template

Each template is looked up by its path relative to the templates root:
the theme first, then its base theme (for a subtheme), then the engine,
then the modules. To change a template, copy the engine's file into the
theme under the same path and edit it:

```bash
mkdir -p themes/sand/templates/errors
cp web/templates/errors/not_found.html themes/sand/templates/errors/
```

A module's template is replaced the same way: copy it from
`internal/modules/<module>/templates/` to the same path under the
theme's `templates/`. A subtheme gets its base theme's copies unless it
has its own, and a previewed theme shows its own.

Copy only what you change. Everything the theme leaves out keeps
following the engine, including its fixes in later releases.

## How a page is put together

A page template names the layout and defines the page's parts.
`home/index.html`, for example:

```html
{{template "layout.html" .}}

{{define "title"}}{{end}}
{{define "breadcrumb"}}{{end}}
{{define "content"}}…{{end}}
```

For each page the engine parses the page's template together with the
layout and the shared files: `layout.html`, `header.html`, `footer.html`,
`sidebar.html`, `pagination.html`, `comments.html`, the system blocks'
`blocks/*.html`, the `partials/` the engine uses, every file in
`teaser/`, `page/`, `fields/` and `component/`, and the modules' block
and partial templates. They form one set, so a `define` in any of them
can be called from any other: `{{template "name" .}}`.

Where to put a theme's own `define`s: in a file the engine loads with
every page. A new file in `teaser/`, `page/`, `fields/`, `component/` or
`partials/` (`partials/cards.html`, say) is loaded whole, with the theme's
base theme's; `header.html` and `footer.html` hold the page's frame
(a theme's own menu block markup, `menu_block_header`, say, goes in
`header.html`).

What is rendered apart from a page - a lazy block, a View block, the next
page of a list or of the comments - is parsed with fewer files: its own
(`partials/region_blocks.html`, `dynamic/list.html`, `comments.html`…),
`fields/` and the modules' blocks and partials. A `define` such a part
calls (from a theme's `dynamic/list.html`, say) goes in a `fields/` file,
which every render parses.

## The page and the HTML document

The HTML document is the engine's and is shared by every theme
(`partials/html.html`); the theme decides the layout around it.

| Template | File | What |
|---|---|---|
| layout | `layout.html` | the page: `{{template "header" .}}`, the theme's container with `{{template "main_column" .}}`, `{{moduleHook "page_end" .}}`, `{{template "footer" .}}` |
| `header` | `header.html` | starts with `{{template "html_start" .}}` (the `<head>`, `<body>`, the admin bar), then the site's header |
| `footer` | `footer.html` | the site's footer, ending with `{{template "html_scripts" .}}` and `{{template "html_end" .}}` |
| `main_column` | `partials/main_column.html` | the content row: breadcrumb, title, `content_top`, the page's `content`, `content_bottom`, and the sidebar column when its region has blocks for the page |
| `theme_framework`, `theme_head`, `theme_editor_css` | `partials/theme_head.html` | the [style hooks](./assets-and-translations.md#stylesheets) |

The engine's `layout.html`:

```html
{{template "header" .}}

<main id="main-content" class="container container-content">
    {{template "main_column" .}}
</main>

{{moduleHook "page_end" .}}
{{template "footer" .}}
```

Keep `moduleHook "page_end"`: the modules put their markup there (an
error-report form, say). Prefer calling `main_column` from your layout
to copying it: breadcrumbs, titles and the sidebar rule then stay the
same in every theme. A theme that wants another arrangement of the
content and sidebar replaces `partials/main_column.html`.

For an administrator `html_start` puts the admin bar at the top of the
window (fixed) and leaves room for it right after it: the page starts
below the bar, in the admin and on the site alike, and the room goes when
the bar is collapsed. A theme doesn't add room of its own. Its fixed or
sticky elements stop below the bar with
`top: var(--admin-bar-offset, 0px)` - the bar's height, `0px` for a
visitor or while the bar is collapsed.

On service pages (error pages, editor forms) `.servicePage` is true and
`main_column` leaves the sidebar out.

The admin (`/admin/…`) has its own layout (`admin/layout.html`) and
doesn't use the theme's header, footer or styles.

## The engine's templates

The ones a theme most often replaces:

| File | What |
|---|---|
| `layout.html`, `header.html`, `footer.html` | the page frame (above) |
| `partials/main_column.html` | the content row and sidebar |
| `partials/theme_head.html` | the style hooks |
| `sidebar.html` | the sidebar column (the `sidebar` region) |
| `home/index.html` | the front page |
| `dynamic/page.html` | a content item's page (built from parts, see [below](#content-pages)) |
| `dynamic/list.html` | a View's page and list |
| `taxonomy/term.html` | a term's page with teasers. A theme's copy keeps `{{moduleHook "term_actions" .}}` under the title (the modules' buttons, such as *Follow*) |
| `teaser/default.html` | a teaser card |
| `fields/date.html`, `fields/bool.html`, `fields/list.html` | how date, yes/no and list values are printed |
| `search/results.html` | search results |
| `blocks/logo.html`, `blocks/search_form.html`, `blocks/user_menu.html`, `blocks/last_comments.html`, `blocks/tag_cloud.html`, `blocks/copyright.html` | the system blocks (`block_logo`…). A theme's copy of `blocks/user_menu.html` keeps `{{moduleHook "user_menu" .}}` (the modules' items, such as *Messages*) and the number on the toggle, `.userMenuCount` |
| `partials/region_blocks.html` | how a region's blocks are output |
| `partials/block_placeholders.html` | the placeholders of lazy blocks (empty in the engine) |
| `partials/breadcrumbs.html` | the breadcrumb trail |
| `pagination.html` | the pager; each link carries its page number in `data-page` |
| `comments.html`, `partials/comment_list_item.html` | comments. A theme's copy of `comments.html` keeps `{{moduleHook "comments_actions" .}}` under the heading (such as *Subscribe to the comments*) and `{{moduleHook "comment_links" .}}` in each comment's links (such as *Report*) |
| `errors/not_found.html`, `errors/forbidden.html` | the 404 and 403 pages (unless the site chose content items for them on **Site information**) |
| `user/*.html` | sign-in, registration, the profile |
| `styleguide.html` | the [style guide](#the-style-guide) page |

## Template suggestions

In some places the engine picks a template by name, trying the most
specific first. A theme overrides one piece by defining the first name in
a file of its own; it doesn't copy the whole page.

| Where | Names tried, in order | Put it in |
|---|---|---|
| a teaser card | `teaser_<content type>`, `teaser_node`, `teaser_default` | `teaser/<content type>.html` |
| a content page | `page_<content type>`, `page_node`, `page_default` | `page/<content type>.html`, `page/node.html` |
| a content page's fields | `node_sections_<content type>`, `node_sections_node`, `node_sections_default` | a file in `page/` |
| an insert into a content page | `page_extra_<content type>` (none - nothing) | a file in `page/` |
| a date field's value | `field_date_<field>`, `field_date_default` | `fields/<name>.html` |
| a yes/no field's value | `field_bool_<field>`, `field_bool_default` | `fields/<name>.html` |
| a link field's links | `field_links_<field>`, `field_links_default` | `fields/<name>.html` |
| a date field's several dates | `field_dates_<field>`, `field_dates_default` | `fields/<name>.html` |
| a user field's people | `field_users_<field>`, `field_users_default` | `fields/<name>.html` |
| a number field's value | `field_number_<field>`, `field_number_default` | `fields/<name>.html` |
| a list field's chosen options | `field_list_<field>`, `field_list_default` | `fields/<name>.html` |
| an e-mail, a phone | `field_email_<field>`, `field_phone_<field>` (and `_default`) | `fields/<name>.html` |
| a block of a blocks field | `component_<field set>`, `component_node`, `component_default` | `component/<field set>.html` |
| a field's input in the content form | `field_input_<field type>`, `field_input_default` (empty) | `fields/<name>.html` |
| a field's "Label: value" line on the content page | `field_line_<field type>`, `field_line_default` (empty) | `fields/<name>.html` |
| a menu block in a region | `menu_block_<region>` (none - the engine's menu markup) | `header.html` or another loaded file |
| a lazy block's placeholder | `block_placeholder_<region>` (none - empty) | `partials/block_placeholders.html` |
| a View's list | `view_<name>`, when the View sets *Theme template* | `views/<name>.html` |

`<content type>` and `<field>` are machine names. A new site has the
content types `article` and `page`, so `teaser/article.html` with
`{{define "teaser_article"}}` changes the article cards only. The `_node`
names cover every content type (and field set) at once, so none can be
named `node` or `default`; the field templates (`field_…`) have no
`_node` step - they go by field or field type, not by content type.

Content types are never named in the engine's code: a new content type
gets the general templates, and its own look by a file in the theme.

The helpers behind this can be used in a theme's templates too:
`renderSuggestion "<prefix>" <type> <data>` (with the `_node` and
`_default` fallbacks) and `renderOptional "<prefix>" <type> <data>` (the
specific name only, else nothing).

### Teasers

A teaser is a content item's card in the engine's term pages
(`taxonomy/term.html`). It gets an `entity.TeaserItem`:

| Field | What |
|---|---|
| `.Node` | the item: `.Title`, `.URL`, `.Type`, `.CreatedAt`… |
| `.AltTitle` | the alternative title, if the type has one |
| `.TypeLabel` | the content type's name |
| `.Body` | the body text (HTML) |
| `.MainImage`, `.ImageStyle` | the type's first image field's file and its image style in lists (`{{img (styledPicture .MainImage .ImageStyle) .Node.Title}}`) |
| `.Image "<field>"`, `.Text "<field>"` | a field's image (or its default image), a text field's value |
| `.Fields`, `.Labels`, `.Terms`, `.Images` | the values, labels, terms and images of the fields, by machine name |
| `.DateFormats` | each date field's display format, for `fieldDate` |
| `.CommentCount` | the number of comments |
| `.ReadingTime` | the minutes reading the item takes, 0 when it has nothing to read (see below) |

`themes/sand/templates/teaser/article.html`:

```html
{{define "teaser_article"}}
<article class="node node-teaser sand-teaser">
    {{with .MainImage}}{{if .URI}}
    {{img (styledPicture . $.ImageStyle) ""}}
    {{end}}{{end}}
    <h3><a href="{{.Node.URL}}">{{or .AltTitle .Node.Title}}</a></h3>
    <p class="sand-meta">{{date .Node.CreatedAt ""}} · {{.TypeLabel}}</p>
    {{with plainPreview .Body 300}}<p>{{.}}</p>{{end}}
</article>
{{end}}
```

### Content pages

`dynamic/page.html` renders a content item through
`renderSuggestion "page" <type>`. The default page, `page_default`, is
assembled from parts a theme can reuse in its own layouts:

| Part | What |
|---|---|
| `node_moderation_notice` | the "awaiting moderation" notice |
| `node_status_class` | the wrapper's status class (` node-unpublished`…) |
| `node_header` | `node_title`, `node_actions` (edit, delete, the modules' actions) and the submitted line |
| `node_sections` | the fields, called as `(dict "Ctx" . "Sections" <sections> "Lead" <HTML>)` |
| `node_footer` | the `content_inline` region, `page_extra_<type>`, sharing, attached Views, comments |
| `node_body_value` | a body field's value, `(dict "Field" … "Value" … "Format" <its text format's ID>)` |

Data on a content page:

| Key | What |
|---|---|
| `.item.Node` | the item (as in teasers) |
| `.contentType` | its content type (`.Label`, `.ShowSubmitted`…) |
| `.displaySections` | the fields shown on the page, in display order, grouped into sections; `.displaySections.Without "<field>"` leaves fields out |
| `.fieldValues` | every field's value by machine name |
| `.fieldFormats` | a text value's text format (its ID) by machine name, for `processText` |
| `.fieldTerms` | a taxonomy field's terms by machine name (each `.Name`, `.Slug`, `.HasPublicPage`) |
| `.fieldImages` | an image field's `.File` and `.URI` (in the field's image style) by machine name, for fields shown on the page; a field with several images gives its first there and every one, in order, in `.All` (each with `.File` and `.URI`) |
| `.altTitle` | the alternative title |
| `.readingTime` | the minutes reading the item takes, 0 when it has nothing to read |
| `.commentCount` | the number of published comments; absent while the page doesn't show the engine's comments (they are hidden, or a module shows its own in their place) |

The reading time counts the item's long text fields shown on its page
(more than one line: a text area, HTML, the editor) at 200 words a
minute, a character of Chinese or Japanese at 500, rounded up to whole
minutes. A meta line with both:

```html
{{with .readingTime}}<span>{{plural . "@count min read" "@count min read"}}</span>{{end}}
{{with .commentCount}}<a href="#comments">{{plural . "@count comment" "@count comments"}}</a>{{end}}
```

In `node_sections` an image field is a `.dynamic-field-image` block; a
picture taller than wide (a cover, a poster) also has
`.dynamic-field-image-portrait`, so a theme can set what follows it
beside it with CSS alone - floating it left of the fields' card, for
instance:

```css
.dynamic-field-image-portrait:has(+ .item-characteristic) { float: left; width: 280px; }
.dynamic-field-image-portrait + .item-characteristic { display: flow-root; }
.dynamic-field-image-portrait + .item-characteristic + * { clear: both; }
```

A page of its own for articles that puts the image first and the other
fields after it, `themes/sand/templates/page/article.html`:

```html
{{define "page_article"}}
{{template "node_moderation_notice" .}}
<article class="node{{template "node_status_class" .}}">
    {{template "node_header" .}}
    {{with index .fieldImages "field_image"}}
    <img class="sand-cover" src="{{.URI}}" alt="">
    {{end}}
    {{template "node_sections" (dict "Ctx" . "Sections" (.displaySections.Without "field_image"))}}
    {{template "node_footer" .}}
</article>
{{end}}
```

To change only the fields and keep the rest of the page, define
`node_sections_<type>` (or `node_sections_node`) and call
`node_sections` from it. `Lead` puts markup into the first section (a
floated picture, say); a theme that passes `Lead` outputs the modules'
links in it itself (`{{template "node_module_links" .}}`), otherwise the
engine puts them after the fields.

Link a term only when `.HasPublicPage` is true: without public term
pages its address answers 404.

### Field values

Date and yes/no values go through `fields/*.html` wherever they are
printed: content pages, View lists, teasers. A theme defines
`field_date_<field>` or `field_bool_<field>` for one field:

```html
{{define "field_date_release"}}{{if not .IsZero}}<time class="release">{{.Formatted}}</time>{{end}}{{end}}
```

A date value is a `FieldDate`: `.Formatted` is the date in the field's
display format (with its time, or the period, when the field has them);
`.IsZero`, `.Day`, `.Month`, `.Year` and `.Format` of the start are
there for rules of your own, with `now` for the current time. A field
with times or an end also gives `.End` (zero - none; `.HasEnd`),
`.WithTime` (the start and the end are in the page's time zone then),
`.Pattern` and `.TimePattern`.

A field with several dates goes through `field_dates_<field>` (default -
`field_dates_default`: a list, each date through `field_date_*`): its
value has `.Field` and `.List` - `.List.Dates`, the dates to show (each a
`FieldDate` with `.Past` and `.Ongoing`), and `.List.More`, how many the
"nearest" layout leaves out. `fieldDatesFor field value "<layout>"` makes
the list in a template of your own.

A user field goes through `field_users_<field>` (default -
`field_users_default`: each user's avatar and name, a link to the
profile); its value is the users (`entity.User`: `.Username`, `.Slug` -
`userURL .Slug` for the profile's address, `.Avatar.URI`). A page
template has them in `.userFieldUsers` by field ID.

A number goes through `field_number_<field>` (`fieldNumber field value`:
`.Value`, and `.Formatted` - by the field's settings in the page's
language, its units around). An e-mail address - `field_email_<field>`
(`fieldEmail field value`: `.Address`; `.Hidden` with `.Reversed`, the
address backwards for the site's script, and `.Masked` for the page
without it); a phone - `field_phone_<field>` (`fieldPhone value`:
`.Number` as typed, `.Tel` - its `tel:` link). A yes/no
value is a `bool`.

A list field's chosen options go through `field_list_<field>` (default -
`field_list_default`, `fields/list.html`): its value is the options
(`entity.ListItems`), each with `.Value`, `.Label` and `.Index` - the
option's place among the field's options (1 - the first; 0 - an option
removed since). The default prints each as
`<span class="list-value list-value-<place>" data-list-value="<value>">`
and puts a `<span class="list-value-sep">, </span>` between them, so a
theme colours the options in their order with CSS alone (badges, say:
`.list-value-1` purple, `.list-value-2` green…) and hides the separators
of badges.

A blocks field's blocks go through `field_blocks` (`fields/blocks.html`:
one after another, or `field-blocks-view--grid`), each block through
`component_<field set>` - `component/role.html` with
`{{define "component_role"}}`, say; without one, `component_default`
shows the block's fields as the set's *Display* tab says, in a
`<div class="component component--<field set>">`. The block's value has
`.Type` (its set), `.ID`, `.Sections` and `.Ctx` - its values and terms
by machine name in `.Ctx.fieldValues`, `.Ctx.fieldTerms`, and what
`node_sections` takes: `{{template "node_sections" (dict "Ctx" .Ctx
"Sections" .Sections "Block" true)}}` lays its fields out as the
engine does (`"Block" true` - without the modules' links).

```html
{{define "component_role"}}
<figure class="role">
  {{template "node_sections" (dict "Ctx" .Ctx "Sections" .Sections "Block" true)}}
  <figcaption>{{index .Ctx.fieldValues "field_character"}}</figcaption>
</figure>
{{end}}
```

A field's compact "Label: value" line on the content page - in the
fields' card and in a group - is `field_line_<field type>`
(`fields/line.html`; `factBoxFieldLine` calls it): it gets `.Field`,
`.Value` and the type's resolved data (`.Terms`, `.FileLink` - a file
field's first file, `.FileLinks` - all of them in order, `.Users`,
`.Nodes`). Define one in a `fields/` file to show a type your way. The
page's data has them by field ID: `.fileFieldLinks`, `.fileFieldItems`.

The content form's inputs are `field_input_<field type>`
(`fields/input.html`): a theme or a module replaces a type's input by
defining it in a `fields/` file of its own. It gets the field's dict -
`.Field`, `.Value`, `.Prefix` (a block's inputs': before every name) and
the type's data (`.Options`, `.SelectedNodes`, `.Images`…) - and must
keep the inputs' names (`field_<id>` after the prefix, and the type's
own: `_url`, `_time`, `_keep`…), which the engine reads.

A link field shown as links goes through `field_links_<field>` (default -
`field_links_default`: one link, or a list of several); its value is
the links to show (only the first when the field shows "only the
first"), each with `.URL`, `.Title` and `.Text` (the title, else the
address). A field a module shows in its own way (a video player, say) is
the module's markup; the module's own template, when it has one, is
replaced the usual way.

### View templates

A list's row is `.view-list-item` (a grid's card `.list-grid-item`); a
sticky item's (*Sticky at the top of lists*) also has
`.view-list-item-sticky` (`.list-grid-item-sticky`), so a theme makes it
lead the list - Kosatka shows it larger, across the list -
while the other rows, on any page of the list, stay alike.

A View's display can name a *Theme template* (see
[Views](../guides/views.md)). With the name `cards`, the list is
rendered by `view_cards` from the theme's `views/cards.html` instead of
the display style; without the file the display style is used.

```html
{{define "view_cards"}}
<ul class="sand-cards">
    {{range .items}}
    <li><a href="{{.Node.URL}}">{{or .AltTitle .Node.Title}}</a></li>
    {{else}}
    <li>{{t "Nothing here yet"}}</li>
    {{end}}
</ul>
{{if .showPagination}}{{template "pagination.html" .}}{{end}}
{{end}}
```

- `.items` - content items, each an `entity.TeaserItem` as in teasers;
  `.comments` for a View of comments.
- `.pagination` and `.showPagination` - the pager. The template
  outputs it itself (`pagination.html` takes the whole data, as above).
- The display's *Text before the list* and *Text after the list* are
  output around the template's list, as with the display style.
- The file is rendered on its own, with `pagination.html`, the field
  templates and every `partials/*.html` of the engine and the theme
  (not the teasers or the page layout). Shared markup for View templates
  goes in a `partials/` file of the theme.

It works wherever the View shows: its page, a block, an attachment, a
profile tab.

### Regions and blocks

`region_blocks` outputs a region's blocks, checking each block's
visibility for the page:

```html
{{if regionHasBlocks "footer" .}}
<div class="site-footer-region">
    {{template "region_blocks" (dict "Region" (region "footer") "Ctx" .)}}
</div>
{{end}}
```

Every region used must be in the manifest's `regions`. A menu block
placed in a region can get the theme's own markup:

```html
{{define "menu_block_header"}}
<nav class="sand-nav" aria-label="{{.Menu.Label}}">
    {{template "menu_tree_items" (dict "Items" .Items "Remaining" .Remaining "Depth" 0)}}
</nav>
{{end}}
```

It gets `.Items` and `.Remaining` (for `menu_tree_items`), `.Menu`,
`.Placement` and `.Ctx` (the page's data).

A system block's markup is replaced like any template: a theme's
`blocks/logo.html` defines `block_logo`.

## Data on every page

| Key | What |
|---|---|
| `.siteName`, `.siteSlogan`, `.siteLogoURI` | from **Site information** (empty when not set) |
| `.user` | the signed-in user (`.Username`…), nil for a guest |
| `.isAdmin` | the user may use the admin |
| `.isHomepage` | the front page |
| `.currentPath` | the request's path |
| `.pageSEO` | the title, description, canonical URL… used by `html_start` |
| `.title`, `.breadcrumbs` | the page's heading and trail, on the pages that have them |
| `.servicePage` | an error page or a form: no sidebar |
| `.tokenInURL` | the address carries a secret (a `token` parameter, a `{token}` in the route): no tracking code |

A page adds its own keys: `.item` on content pages, `.items` and `.term`
on term pages, and so on. [Template debugging](#template-debugging)
shows which template rendered what, and the engine's template is the best
reference for its data.

## Template functions

Besides Go's built-in functions (`and`, `or`, `not`, `eq`, `index`,
`len`, `printf`, `with`, `range`…):

**Text and HTML**

| Function | What |
|---|---|
| `rawHTML s` | `s` output as is: only for HTML cleaned on save (formatted text fields, block bodies, comments) |
| `processContent s` | like `rawHTML`, with image styles and the lightbox applied to the pictures |
| `processText s formatID` | `processContent`, then the text filters the modules add and the format has on (a video link into a player, say): `{{processText (index .fieldValues "body") (index .fieldFormats "body")}}` |
| `textFilters s formatID` | only the format's text filters, for HTML otherwise shown as is |
| `embed "kind:name"` | what a tag `[[kind:name]]` embeds - a block, a View's list ([Embeds](../guides/embeds.md)) - as the page's visitor may see it: `{{embed "block:opening_hours"}}`. The names are the site's: not for a theme meant for other sites |
| `truncate s n` | the beginning of HTML `s`, `n` characters of text, tags closed (or the part before `<!--break-->`) |
| `truncateText s n` | plain text cut to `n` characters with "…" (escaped) |
| `plainPreview s n` | the same as `plainExcerpt` (the name teasers use) |
| `plainExcerpt s n` | HTML as plain text without its code blocks (`<pre>`; a text that is all code keeps it), cut at a word to `n` characters |
| `plainLead s n` | the first paragraph (block) of HTML as plain text, code blocks left out, cut at a word to `n` characters (`0` - the whole paragraph): a card's text from a body that goes on with headings and lists |
| `stripImages s` | `s` without `<img>` tags |
| `json v` | `v` as JSON, for an attribute (`data-ids="{{json .IDs}}"`); inside `<script>` print the value itself |
| `toString v` | `v` as a string |
| `dict k v …` | a map, for passing several values to a template |
| `add`, `sub`, `mul`, `isEven` | integer arithmetic |
| `sliceHead list n`, `sliceTail list n` | the first `n` items, the rest |
| `pathEscape s`, `queryEscape s` | escaping for a URL path or query |

The output of `plainPreview`, `plainExcerpt` and `truncateText` is plain
text: never pass it to `rawHTML` or `truncate`.

**Translation**

| Function | What |
|---|---|
| `t "Source" ["@name" value …]` | the string in the page's language: `{{t "Hello @name" "@name" .user.Username}}` |
| `tc "context" "Source" …` | `t` with a context, for a word that reads differently in different places |
| `plural n "@count item" "@count items"` | the form for a number |
| `tjs "Source" …` | `t` as plain text, for inline scripts and attributes |
| `lang` | the page's language code |
| `viewTitle .ViewDisplay` | a View display's title in the page's language (its page's `.title` is already) |
| `termName .Term` | a taxonomy term's name in the page's language (tags, the tag cloud); a term's page has `.term` in it already |
| `blockTitle .Block`, `placementTitle .Placement fallback` | a custom block's title, a placement's (`fallback` - a View's or a menu's label when it has none), in the page's language - what `region_blocks` shows in `<h2>` |
| `localURL path` | an address of the site for the page's language: with the language's prefix, as it is for the site's default language (`{{localURL "/"}}` is `/en/` on an English page when English isn't the site's default); another site's address and one with a language's prefix stay as they are. Use it for the links a template builds itself (the home link, a term's `{{localURL (print "/" .Slug)}}`) |
| `otherLanguage code` | the name of `code`'s language when content in it shows on a page in another language of a site of several (a list showing an original where there's no translation), else `""`: `{{with otherLanguage .Node.Langcode}}<span class="badge">{{.}}</span>{{end}}` - the engine's teaser and a View's title field mark it so |

**Dates**

| Function | What |
|---|---|
| `date t "<format>"` | a date in the page's time zone (the visitor's own, else the site's) in a format from **Date formats** (`""` - the default one, `"datetime"`); empty for no date |
| `fieldDate value "<format>"` | a date field's value as a `FieldDate` (for `field_date_*`); a time in hours and minutes, a period shortened |
| `fieldDateFor field value "<format>"` | the same by the field's own time format and period setting (`""` - the field's date format) |
| `siteTime t` | `t` in the visitor's or site's time zone, for `.Format` of your own |
| `now`, `currentYear` | the current time, the year |
| `copyrightYears "2015"` | "2015-<this year>" |

**Files, images and users**

| Function | What |
|---|---|
| `publicFileURL uri` | the URL of an uploaded file (`public://…`) |
| `resolvePublicURI uri` | the same, passing other paths through |
| `styledImage file "<style>"` | an image in an image style (its `.URI`) |
| `styledPicture file "<style>"` | an image in an image style for `img`: its address, its [2× copy](../guides/files-and-images.md#high-density-screens) and its size; `""` - the image itself |
| `lightboxImage file` | what the [lightbox](../guides/files-and-images.md#the-lightbox) opens of an image (its large copy, or the original): `.Src`, `.Width`, `.Height` for a link's `href` and `data-pswp-width`/`data-pswp-height` |
| `img picture alt "<name>" "<value>"…` | an `<img>` of a picture (`styledPicture`, or an item's `.Picture`): `src`, `srcset` (the 2× copy), `width` and `height` when known, `alt`, `loading="lazy"` and `decoding="async"`. More attributes come as name, value pairs and replace the ones it sets (`"loading" "eager"`), except `src`, `srcset` and `alt` |
| `humanFileSize n` | "245 KB" |
| `avatarPlaceholder` | the picture of a user without an avatar (`.Avatar.URI` is empty then) |
| `registrationOpen` | whether visitors may sign up (the account settings): a header's "Register" link goes inside `{{if registrationOpen}}` |
| `minPasswordLength` | the fewest characters of a password, for an input's `minlength` and its hint |
| `userURL slug` | a user's profile path |

**The theme and its assets**

| Function | What |
|---|---|
| `themeCSS "<name>"` | the URL of the theme's `css/<name>.min.css`, versioned |
| `themeJS "<name>"` | the URL of the theme's `js/<name>.min.js`, versioned |
| `themeCSSAsync "<name>"` | the `<link>` tags to load it without blocking the page |
| `themeStylesheets` | the URLs of the manifest's `stylesheets` (the chain's) |
| `themeSetting "<id>"` | a [setting](./manifest.md#settings)'s value |
| `staticCSS "<name>"`, `staticJS "<name>"` | the URL of the engine's `/static/css/<name>.min.css`, `/static/js/<name>.min.js` |
| `staticCSSAsync "<name>"` | non-blocking `<link>` tags for the engine's stylesheet |
| `moduleCSS "<module>" "<file>"`, `moduleJS …` | a module's asset URL |
| `attach "<module>"` | the page needs the module's CSS and JS |
| `moduleEnabled "<module>"`, `featureEnabled "<feature>"` | whether a module or a feature is on |

**Regions and rendering**

| Function | What |
|---|---|
| `region "<id>"` | a region ID for `region_blocks` |
| `regionHasBlocks "<id>" .` | the region has a block visible on this page |
| `renderSuggestion`, `renderOptional` | a template by [suggestion](#template-suggestions) |
| `renderTeaser item` | a teaser card |
| `renderBlock "<define>" data` | a `define`'s output as a value, to pass on |
| `moduleHook "<hook>" .` | the enabled modules' markup for a hook (`page_end`, `user_menu`, `comments_actions`, `comment_links`, `term_actions`…) |

The engine's own templates use a few more (`viewGroups`,
`renderRowTemplate`, `rewriteFieldValue`, `integrations`…); their
comments in `internal/domain/service/template_funcs.go` and `template.go`
say what they do.

## Template debugging

Turn it on with `templates_debug: true` in the config or
`TEMPLATES_DEBUG=1` (in Docker: `TEMPLATES_DEBUG=1 docker compose up
--build`). The pages then carry comments about the files that rendered
them:

```html
<!DOCTYPE html>
<!-- PAGE: taxonomy/term.html - theme: sand → kosatka → (engine) -->
…
<!-- SUGGESTIONS: teaser_article, teaser_node, teaser_default ✓ -->
<!-- BEGIN teaser_default - web/templates/teaser/default.html -->
<article class="node node-teaser">…</article>
<!-- END teaser_default -->
```

- `PAGE` - the page's template and the theme chain.
- `BEGIN` / `END` - around each `define` that outputs markup, with the
  file it came from (the theme's, a base theme's, the engine's or a
  module's). A `define` that outputs a URL or a script's value isn't
  wrapped: a comment there would break it.
- `SUGGESTIONS` - where a template is picked by suggestion: the names
  tried, and the one used (`✓`). To override that piece, define the first
  name in the theme.

It is for development only: a site that isn't `env: local` warns in its
log at startup when debugging is on. Locally with `env: local`,
templates are also read again on every request, so an edit shows on
reload.

To see the regions on a page, open any page as an administrator with
`?blocks_demo=1`: each region is framed with its name.

## The style guide

**Appearance → Style guide**
(`/admin/config/appearance/styleguide`) shows the engine's markup in the
site's layout and the active (or previewed) theme: headings and text as
in a content field, buttons of every kind, the editor with the same
text, form elements, alerts, badges, a table, tabs, the pager and a list
of three cards. Check a theme on it: elements of one kind should look
alike. The page is `styleguide.html`, which a theme can replace to show
its own components too.

---
title: A theme's demo site
description: demo/<lang>.yaml - the site a theme shows on its screenshot, installed with one command.
sidebar:
  order: 7
---

A theme can bring a demo site: the content, Views, blocks and menus that
show it as on its screenshot. `app install -demo-set <theme>` creates a
new site with them, in that theme. It takes Kosatka 0.1.5 or newer.

## The file

The demo site is `demo/<lang>.yaml` in the theme's directory, a file a
language:

```text
kosatka-theme-sand/
├── theme.yaml
├── demo/
│   ├── en.yaml       the demo site in English
│   └── ru.yaml       the same in Russian (optional)
├── templates/
└── static/
```

- The demo set is named by the theme's directory: `sand`.
- A site is installed in a language the theme has a file for; another
  `-language` is refused.
- The site gets the theme, unless the file's `site.theme` names another.
- The format is that of the engine's demo sets. Every key is described
  in [Demo sets](../contributing/demo-sets.md).

## What goes in it

A small front page: a View of the starter's articles in the theme's own
list template, a text block in one of the theme's regions on the front
page, a link in the main menu.

```yaml
site:
  slogan: Warm light and wide covers
  front_page_view: latest

nodes:
  - key: welcome
    type: article
    title: Welcome to the dunes
    ago: 2d
    fields:
      field_image: 1        # a generated picture
      field_body: |
        <p>The first article of the demo site.</p>
  - key: dunes
    type: article
    title: Dunes at noon
    ago: 5d
    fields:
      field_image: {file: images/dunes.jpg, alt: Dunes under a white sky}   # the theme's own, demo/images/dunes.jpg
      field_body: |
        <p>The second one, with the theme's own picture.</p>
  - key: about
    type: page
    title: About
    fields:
      field_body: <p>Who we are.</p>

views:
  - name: latest
    label: Latest
    display_type: block
    theme_template: cards   # the theme's views/cards.html
    page_size: 6
    fields:
      - {builtin: title, link_to: content}
    filters:
      - {builtin: content_type, operator: eq, value: article}
      - {builtin: status, operator: eq, value: "1"}
    sorts:
      - {builtin: created_at, direction: desc}

blocks:
  - region: hero            # a region of the theme's theme.yaml
    visibility: homepage
    weight: 1
    title: Sand and patience
    show_title: true
    body: |
      <p>A theme for slow reading.</p>
      <p><img src="/static/themes/sand/images/dunes.svg" alt=""></p>

menus:
  - name: main
    links:
      - {title: About, node: about}
```

What a demo site usually has:

- **Its content types and fields**, or the starter's: the `article` and
  `page` types, the `field_body` and `field_image` fields.
- **Blocks in the theme's regions**, by their ids in `theme.yaml`;
  `visibility: homepage` keeps a block on the front page, `weight`
  orders the blocks of a region.
- **Lists in the theme's templates**: a View's `theme_template` names a
  `views/<name>.html` of the theme ([View templates](./templates.md#view-templates)).
- **Pictures**: an image field takes a generated picture by its number,
  or a picture of the theme's own - `{file: images/dunes.jpg, alt: …}`, a
  path inside `demo/` (a `.jpg`, `.png` or `.webp`; the alternative text
  is the item's title unless given). Installing copies it into the
  site's files, as an upload: the site's image styles crop and scale it.
  Keep these pictures in `demo/`, not in `static/`: the site serves
  `static/`, and the demo's pictures are needed only by the install. A
  text block can show the theme's pictures from its `static/`
  (`/static/themes/<theme>/images/…`).
- **The modules' content** under `modules`: a form with its block in a
  region, a gallery on a content item (its pictures drawn by their
  titles, or the theme's own: `{title: …, file: images/…}`), a poll. The modules a demo site
  has content for are installed with it. A module of its own repository
  (a forum, say) is there only on a site built with it: on another, the
  install says so and leaves its part out.

## Installing and checking it

```bash
app theme check                      # in the theme's directory
app install -site-name "Sand" -admin-name admin -admin-email admin@example.com \
  -language en -demo-set sand        # in the project, with the theme in themes/
```

- `app theme check` loads the demo site as the install does and checks
  that its blocks and menus are in the theme's regions (see [Checking a
  theme](./distribute.md#checking-a-theme)).
- `app install` creates a new site on an empty database; the theme has
  to be in the project's `themes/` (installed with `app theme add`, or
  a clone of its repository).
- Everything it creates is ordinary content, Views, blocks and menus:
  the site's administrators change or delete them in the admin.
- An engine's demo set of the same name comes first.

## An example

[Keel](https://github.com/kosatka-cms/kosatka-theme-keel)'s
`demo/en.yaml` is a whole company's site: a studio's services, projects,
team, clients' quotes, contacts and a request form, the front page made
of the theme's bands.

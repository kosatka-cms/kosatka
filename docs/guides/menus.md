---
title: Menus
description: Creating menus, adding links to content, terms, Views and any address, and showing a menu in a region.
sidebar:
  order: 6
---

A menu is a tree of links. A site can have any number of them: the main
navigation, a footer menu, a menu for one section. A menu shows on the
site once it is placed as a block in a region. Menus also give the site
its breadcrumbs.

Menus are managed in the admin under **Structure → Menus**
(`/admin/config/menus`).

## The site's menus

A new site has one menu, *Main navigation* (machine name `main`), with a
*Home* link, already placed in the *Header* region. A site installed
with `-demo` also gets a footer menu.

To add a menu, enter its *Name* under *Add menu* and press *Add*. The
machine name is made from the name and never changes. The menu's
settings (the pencil icon):

| Setting | What it does |
|---|---|
| *Name* | the menu's name in the admin |
| *Description* | a note for administrators, not shown on the site |
| *Include in the sitemap* | the menu's links go into `/sitemap.xml`, all levels; disabled and broken links are skipped |

Deleting a menu deletes all its links.

## Add a link

1. On the menus list press *Links* next to the menu.
2. Under *Add link* enter the *Name* (the link's text) and choose the
   *Link type*:

| Link type | Target |
|---|---|
| URL | a path on the site (`/about`) or a full address (`https://example.com`), used as typed |
| Node | a content item: type part of its title and pick it from the list |
| Term | a taxonomy term: choose the vocabulary, then type part of the term's name |
| View | a published View page, from the list |

3. Press *Add*. The link goes to the end of the top level.

Links to content, terms and Views follow their target: when its address
changes, the menu link changes with it. When the target is deleted, the
link is marked *- broken link -* in the *Current URL* column; on the
site it stays in the menu but leads nowhere, so fix or delete it.

## Nesting and order

The links table shows the tree, children indented under their parent.

- Drag a row by ☰ to change its place among the links of its level; it
  moves with all its children.
- Choose a *Parent* in the row's list to put a link under another one,
  at any depth; *- no parent -* moves it to the top level.
- Press *Save order* to keep the changes.

Deleting a link doesn't delete its children: they move up to its level.

## Link settings

Open a link (the pencil icon) to change its name and target, and
*Enabled*: a disabled link, with all its children, isn't shown on the
site but is kept. Whether a link's children show right away (or on
hover) is up to the theme.

## Show a menu on the site

A menu is shown by a menu block. On `/admin/config/blocks`, under *Add
menu block*:

1. Choose the menu and the region.
2. Set the levels: *Initial level* and *Number of levels* (below).
3. Choose where it shows (*Everywhere*, *Front page only*, *Only on
   content types* - their machine names, comma-separated) and press
   *Add*.

The block's page has the same settings, plus a heading (*Title*, the
menu's name by default, and *Display title*), the roles and the other
visibility choices ([Blocks](./blocks.md#visibility)). One menu can be
placed in several regions with different settings.

| Setting | What it does |
|---|---|
| *Initial level* (1-10) | 1 shows the menu from the top. 2 and above show only the branch of the current page at that level, and nothing on pages outside the menu |
| *Number of levels* (0-9) | how many levels to show from the initial one; 0 - all |

For example, a section menu in the sidebar: place the main menu again in
the *Sidebar* with *Initial level* 2. On a page under *Products* it lists
the links under *Products*; on the front page it shows nothing.

How a menu looks (a bar, a drop-down, a mobile panel) is up to the
theme.

## Menus and breadcrumbs

Breadcrumbs are built from the menus: a page whose address is a menu
link - a View's page, a term page - gets that link's chain, for example
*Home › Products › Lamps*. Disabled links don't count. A content item
gets its place in the menu from a breadcrumb rule, even when its own
address is a menu link; without a matching rule its trail is *Home ›*
its title. See [Breadcrumbs](./urls-and-seo.md#breadcrumbs).

---
title: Ratings
description: Let visitors rate content with stars, show the rating on the content page and build top lists with Views.
sidebar:
  order: 9
---

The *Ratings* module (`ratings`) lets visitors rate content items with
stars: 1 to 5 by default, any scale from 3 to 10. A content page shows
the average, the number of votes and the visitor's own vote; Views sort
and filter by the rating and show it in a list's rows, so a top list is a
View sorted by the weighted rating.

A new site doesn't get the module unless you ask for it: it isn't one of
the standard modules. Enable it on the **Modules** page, or name it in
`app install -modules` with the other modules the site should have (or
`-modules all`). See [Using modules](../using-modules.md).

## What it adds

| What | Where |
|---|---|
| The settings | **Configuration → Ratings** (`/admin/config/ratings`) |
| A permission | *Rate content*, given to signed-in users on install |
| A switch in the content type form | *Content of this type can be rated*, in the *Modules* group of the type's *Settings* tab |
| A block | *The content's rating*, placed on install in the theme's `content_inline` region (*Inside content (content page)*), when it has one |
| Sort criteria of Views | *Rating (weighted)*, *Rating: average*, *Rating: number of votes* |
| Filters of Views | *Rating: average*, *Rating: number of votes* |
| A field of Views | *Rating* |

## Settings

| Setting | |
|---|---|
| *Scale* | 3 to 10 stars, 5 at first. Change it any time: each vote keeps its share of the scale, so 4 of 5 stars become 8 of 10 |
| *Who can rate* | users with the *Rate content* permission ([permissions of roles](../../guides/users-and-permissions.md)); *Guests can rate too* (off at first) lets guests vote as well, each told apart by their browser's session |
| *Own vote* | *Can be changed* and *Can be taken back*: whether a voter may change their vote and take it back. Both on at first |
| *Rated content types* | which types' content can be rated - the same switch as *Content of this type can be rated* in each content type's form. None at first. Switching a type off keeps its votes |

A guest who may not rate sees the rating and a link to log in.

## On a content page

The block *The content's rating* shows a rated item's average, the
stars filled to it and the number of votes, then the visitor's vote: a
star button per score. A vote is sent without leaving the page; *Take
back* removes it while votes can be taken back. Move the block to
another region, or remove it, on the **Blocks** page.

The page itself may be cached for guests: the block shows the average
and count from the page, and loads the visitor's own vote from
`/node/<id>/rating`. Without JavaScript the block shows only the average
and the count.

The module's look is its own `rating-*` classes; a theme changes the
stars' colours with `--rating-star-on` and `--rating-star-off` on
`.rating-block` and `.rating-summary`, or restyles the classes.

## A top list

Make a View of the rated content and sort it by *Rating (weighted)*:

1. Create a View (a page or a block), filter it by the content type and
   by `status` = `1`.
2. Add the sort criterion *Rating (weighted)*, descending. Its setting
   *Votes for full weight* (5 by default) is the threshold: while an
   item has fewer votes, its rating is pulled towards the site's
   average, so two fives don't outrank forty votes of 4.8. Each View has
   its own threshold.
3. Add the field *Rating* to show it in the rows: the stars with the
   average and the number of votes, or only one of them.
4. To leave out items with too few votes, add the filter *Rating: number
   of votes* `gte` some number.

The rating sorts count an item without votes as 0: last in descending
order, first in ascending. See
[Views](../../guides/views.md#criteria-of-modules).

## Demo content

With `app install -demo` and the module, articles are rated, the three
newest demo articles get the demo users' votes, and the sidebar gets a
*Top rated* list: a block View of the rated articles that have votes,
the best first by the weighted rating, each with its average. A demo set
gives its own instead, under `modules` → `ratings`: the content `types`
rated, the `votes` (a node's key and the users' `scores`) and, for the
list, its title as `block` - without it, no list. See [Demo
sets](../../contributing/demo-sets.md).

## Uninstalling

Uninstalling removes the votes and the settings, the block's placements,
*Rate content* from the roles, and the module's sort criteria, filters
and fields from every View.

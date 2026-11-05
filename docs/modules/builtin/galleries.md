---
title: Galleries
description: Attach image galleries to content items, show them on the content page and on gallery pages, and manage them from the admin.
sidebar:
  order: 5
---

The Galleries module (`galleries`) lets a content item have image
galleries: sets of uploaded pictures with a name, a description and
captions. They show on the item's page in a block, each on a page of its
own, and open in the site's lightbox.

A new site doesn't get the module unless you ask for it: it isn't one of
the standard modules. Enable it on the **Modules** page, or install the
site with it: the standard ones plus this one,
`-modules action_log,forms,not_found_log,private_messages,subscriptions,video_embed,galleries`,
or `-modules all`. See [Using modules](../using-modules.md). With
`-demo` too, the newest demo article gets a gallery of six pictures.

## What it adds

| What | Where |
|---|---|
| A content item's galleries | `/node/<id>/galleries`: the galleries as cards, with *+ Create new gallery* for editors |
| Editing a gallery | `/node/<id>/galleries/<gallery id>/edit` |
| A gallery's page | `/galleries/<gallery id>` |
| The overview | **Content → Galleries** (`/admin/content/galleries`) |
| Settings of the content type | two rows marked *Galleries* in the *Modules* group of the type's *Settings* tab |
| A panel of the content form | *Galleries*, among the item's settings |
| A *Galleries* button | on the content page, next to *Edit* |
| Links under the content | each gallery of more than one image: its name and the number of images |
| A block | *The content's galleries*, placed in *Inside content (content page)* (`content_inline`) on install |
| Image styles | *Gallery cover*, *Gallery thumbnail*, *Gallery medium* |

The image styles it creates (change them on `/admin/config/image-styles`,
see [Image styles](../../guides/files-and-images.md#image-styles)):

| Style | Name | Size, mode | Used for |
|---|---|---|---|
| *Gallery cover* | `gallery_cover` | 600×200, fill, top | the cover on the cards of `/node/<id>/galleries` |
| *Gallery thumbnail* | `gallery_thumbnail` | 215×144, fill, center | a new gallery's grid preview |
| *Gallery medium* | `gallery_medium` | 250×250, fill, top | a new gallery's larger preview |

A style of the same name that already exists is left as it is.

The cards of `/node/<id>/galleries` take the *Gallery cover* style's
proportions, and the cover is shown whole inside the card. A style taller
than wide (say 300×450) gives narrower, vertical cards; with the *fit*
mode the picture isn't cropped at all, only scaled down.

**Permissions.** The module has none of its own. Galleries follow the
permissions of the content they belong to ([Users and
permissions](../../guides/users-and-permissions.md)):

| Who | Can |
|---|---|
| Anyone | see the galleries of content that can have them, the galleries list and the gallery pages |
| Who may edit the content item (*Edit own*, *Edit any* of its type) | see the *Galleries* button, create galleries, upload, order, caption and delete images, set the cover and the main image |
| Who may delete the content item (*Delete any*) | delete its galleries |
| *Administer content* | switch galleries on and off in the content form |
| *Access administration (/admin)* | the overview, **Content → Galleries**, with its *Turn on* and *Turn off* |

## Let a content type have galleries

Galleries are switched on per content item. The content type sets the
starting point and the field for the main image.

1. Open the type's *Settings* tab (`/admin/config/content-types/<id>`,
   see [Content types](../../guides/content-types.md)).
2. In the *Modules* group, the rows marked *Galleries*:

   | Setting | What it does |
   |---|---|
   | *New content of this type can have galleries (default)* | New items of the type start with their switch on. Existing items keep their own setting |
   | *Field for the gallery's "Main content image"* | One of the type's image fields, or *- none (no button) -*. With a field chosen, the gallery editor gets the *Main content image* button that copies an image into that field (see below) |

3. Save.

## Turn galleries on for a content item

Each item has its own switch. Either:

- **In its form.** Open the *Galleries* panel among the item's settings
  and tick *This content can have galleries*. The panel is shown to
  editors with *Administer content*, like the other settings; an item
  someone else creates gets its type's default, and their edits leave
  the switch as it is.
- **In the overview.** **Content → Galleries** lists all content by
  title, 50 to a page. Filter by *Type* and *Show* (*All content* or
  *Content that can have galleries*). The *Galleries* column shows *on*
  or *off* and, for an item that has galleries, how many and how many
  images. *Turn on* and *Turn off* flip the switch; *Manage* (for an
  item with the switch on) opens the item's galleries.

With the switch off, the item's page has no *Galleries* button, no
links and no galleries block, and its galleries list
(`/node/<id>/galleries`) answers "forbidden" and a gallery's own page
(`/galleries/<id>`) "not found". Its galleries and images are kept:
turning the switch on brings them back. The galleries of an unpublished
item open only to those who may see the item.

## Create a gallery

1. On the content page, click *Galleries* (or *Manage* in the
   overview). The item's galleries are listed as cards, newest first.
2. Click *+ Create new gallery*, fill in *Name* (up to 255 characters)
   and *Description*, choose the preview styles (see below) and click
   *Create*. All three are optional; a gallery without a name is shown
   as *Gallery*.
3. Click *Edit* on its card.

## Add and arrange images

The gallery's edit page has the gallery's settings at the top, then
*Upload images* and the images.

- **Upload.** Under *Upload images*, choose one or more files in *Select
  files* (the formats the site's upload policy allows, named under the
  field) and click *Upload*. The files are sent one by one with a
  progress bar; a file that fails is listed with the reason, the others
  are kept. New images go to the end.
- **Order.** Drag the images; the new order is saved at once.
- **Caption.** ✎ (*Edit*) opens *Edit metadata*: a *Name* and a
  *Description* for the image. In the lightbox, they are shown as its
  caption.
- **Cover.** ☆ (*Set as cover*) makes the image the gallery's cover; the
  cover is marked ★ and *Cover*. It is the picture on the gallery's card
  (without one the card says *No cover*) and the image shared on social
  networks for the gallery's page (without one, the first image).
- **Main image of the content.** 🖼 (*Main content image*) copies the
  image into the type's field chosen in *Field for the gallery's "Main
  content image"*, replacing its value, after a confirmation. Without that
  field there is no button.
- **Delete.** × (*Delete*) removes the image from the gallery, after a
  confirmation.

*Save settings* saves the gallery's name, description and preview
styles; *View* opens its page; *Back to gallery list* returns to the
cards.

## Preview styles

Each gallery chooses its own two [image
styles](../../guides/files-and-images.md#image-styles), from all the
site's styles, when it is created and later on its edit page:

| Setting | What it does |
|---|---|
| *Preview (grid)* | the previews in the gallery's grid, on the content page and on the gallery's page. Default *Gallery thumbnail* |
| *Preview (larger)* | a larger preview of each image. Default *Gallery medium*. The module's own pages show only the grid preview; the larger one is there for a theme that shows it ([Templates](../../themes/templates.md)) |

The grid follows the grid style: a style taller than wide gives the
portrait grid, whose cells have the style's proportions. To make
previews of a new shape, add a style first, then choose it.

## Show galleries on the content page

The galleries on a content page are the system block *The content's
galleries*. Installing the module places it in *Inside content (content
page)* (`content_inline`) of the site's theme, if the theme has that
region. It shows only on the page of a content item that can have
galleries and has a gallery with images; elsewhere it is empty. It loads
when the visitor scrolls close to it.

For each gallery with images, the block shows its name (or *Gallery*)
and a row of previews. When the gallery has more images than the block
shows, the previews are a random selection, and the last one is dimmed
with "+N" and leads to the gallery's page. A click on a preview opens
the image in the lightbox.

On the block's page (**Structure → Blocks**, the pencil icon):

| Setting | What it does |
|---|---|
| *Previews of each gallery (0 - a row of its grid: 5, or 6 for the portrait grid)* | how many previews each gallery shows, up to 50 |

To show the galleries elsewhere on the page, move the block to another
region on the Blocks page, or to *Hidden* to show none (the links under
the content stay). Each theme has its own block layout; see
[Blocks](../../guides/blocks.md).

## The gallery's page

`/galleries/<id>` shows the gallery's description and all its images in
the grid, in their order; a click opens the lightbox, with the
captions. Its title is the gallery's name (or *Image gallery*); in the
browser's tab the content's title follows. The breadcrumb leads through
the content item and its galleries. Below, *Back to content* and *All
galleries of this content*. An editor sees *Edit* and, with the delete
permission, *Delete* at the top.

## A gallery in a text

The tag `[[gallery:<id>]]` (the number in the gallery's address,
`/galleries/<id>`) shows a gallery inside a text - its title and
previews, opened in the lightbox - any content's gallery, not only the
text's own ([Embeds](../../guides/embeds.md)). It shows only to those
the gallery's page opens for (they may see its content item, and the
item's switch is on), and only while the gallery has images.

## Delete a gallery

*Delete* on the gallery's card or page opens *Confirm gallery deletion*
with the gallery's name, cover and number of images. *Delete gallery*
removes the gallery and its images from it; this can't be undone.

Removing an image or a gallery doesn't delete the uploaded files from
the files directory. When the content item itself is deleted, its
galleries go with it, and so do their files, unless something else uses
them.

There is no switch to hide a single gallery. To hide all galleries of an
item, turn its switch off; to hide them from the content page only, move
the block.

## Upload limits

Gallery uploads follow the site's [upload
policy](../../guides/files-and-images.md#upload-policy): the maximum file
size, the allowed image formats, and the maximum image size, above which
an image is scaled down on upload. Galleries have no limits of their
own. A file larger than the web server accepts fails with "The file is
too large for the server": raise the server's limit if you need bigger
files (see [Running in production](../../guides/production.md)).

Images go to `galleries/<gallery id>/` in the site's files directory.

## Uninstall

Disabling the module hides the galleries, the button, the links, the
block and the overview; enabling it brings them back. Uninstalling (see
[Using modules](../using-modules.md#uninstall-a-module)) deletes all
galleries and their images, the content types' gallery settings, the
items' switches, the block's placements and the image files only
galleries used. The three image styles stay.

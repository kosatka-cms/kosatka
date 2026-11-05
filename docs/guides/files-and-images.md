---
title: Files and images
description: What the site accepts for upload, where files go, image styles and the users' own files.
sidebar:
  order: 12
---

Files reach the site through image and file fields of content, the text
editor, profile pictures, the site logo and modules (galleries, forms).
One site-wide policy sets the limits; fields, text formats and avatars
can set their own instead. Images are shown through image styles:
resized copies made on first request.

## Upload policy

`/admin/config/upload-settings` (*File upload policy*) applies to every
upload on the site:

| Setting | New site | |
|---|---|---|
| Maximum file size (MB) | 10 | the largest file accepted |
| Maximum image size (px) | 4000 | an image wider or taller than this is scaled down on upload and saved as JPEG |
| Allowed formats | JPEG, PNG, GIF, WebP | the image formats accepted. The format is read from the file's content, not its name |
| Documents in users' files | pdf doc docx xls xlsx ppt pptx odt ods odp txt rtf csv zip | the extensions, besides images, users may upload to their own files. Empty - images only |

Web pages, scripts and programs are never accepted, whatever the lists
say: such extensions are dropped when you save a list, and a file whose
content looks like HTML, SVG, a script or a program is refused.

### Limits of their own

These places have their own *Allowed file extensions* and *Max file
size (MB)*. Left empty, the site-wide value applies. A value set there
replaces the site-wide one, even a larger size, and an extension list
replaces the site-wide *Allowed formats*.

| Where | Settings |
|---|---|
| An *Image* field of a content type (the field's edit page, *Upload*) | extensions, size, *Max resolution* (larger images are scaled down), *Min resolution* (smaller ones are refused), *File directory* |
| A *File* field | extensions (empty - pdf doc docx xls xlsx zip txt), size, *File directory* |
| A text format (`/admin/config/text-formats`, *Upload via the file manager*) | extensions and size of images put into the editor with that format |
| Avatars ([Account settings](./users-and-permissions.md#account-settings), *Avatar upload*) | extensions and size of profile pictures |

## Where files go

Uploads are stored in the site's files directory and served from
`/uploads/` (both are set in [the
configuration](../reference/configuration.md#files-files)), or in an
S3-compatible bucket or an Azure container ([Files in object
storage](./file-storage.md)) at the same addresses. Inside it:

| What | Directory |
|---|---|
| An *Image* field | the field's *File directory*, by default `content/images` |
| A *File* field | the field's *File directory*, by default `content/files` |
| A user's own files (the *Files* tab, the editor) | `u<user id>/` |
| Profile pictures | `user_pictures/` |
| The site logo | `site/logo/` |

**File directory** is set per field and content type, so the same field
can store files in different places for different types. It may use
tokens:

| Token | Becomes |
|---|---|
| `[date:custom:Y]` | the year of the upload, `2026` |
| `[date:custom:m]` | the month, `01`-`12` |
| `[date:custom:d]` | the day, `01`-`31` |
| `[node:type]` | the content type's machine name, `article` |
| `[current-user:uid]` | the uploader's ID |

For example `content/[date:custom:Y]-[date:custom:m]` puts an upload of
October 2026 into `content/2026-10/`. Dates follow the site's time zone.
Each part of the path is transliterated to lowercase Latin letters,
digits, `-` and `_`. A directory with an unknown token, `..`, an empty
part or one of the engine's own directories at the start (`styles`,
`resize`, `imagefield_default_images`, `user_pictures`, `u<digits>`) is
refused. Changing the directory affects new uploads only; files already
uploaded stay where they are.

**File names** come from the uploaded name, transliterated:
`My Photo.PNG` becomes `my-photo.png`. If the name is taken, `_1`, `_2`…
is added; an existing file is never overwritten. A *File* field keeps
the original name for its download link.

**Default image or file.** An *Image* or *File* field can have a
default, shown on cards and the content page when an item has no value
of its own.

### Public and private files

Everything above is public: anyone with the link can open the file, and
in production the web server sends it directly (see [Running in
production](./production.md)). Files that must not be reachable by a
link, such as files sent with forms of the Forms module, go to the
private directory instead (`files.private_dir`, see [the
configuration](../reference/configuration.md#files-files)), or a
private bucket ([Files in object storage](./file-storage.md#private-files)).
The web server doesn't serve it; the site hands such a file out only after
checking the viewer's permission.

A *File* or *Image* field keeps its files there with its *Private
files* setting: the site then gives such a file - an image in each of
its styles too - only to those who may see a content item it is
attached to, checked on every download by the same rules as the item's
page, and to its uploader; never to someone with the link alone. See
[Security](./security.md#private-files). Files already uploaded stay
where they are, and the field's default file or image stays public.

When content is deleted, its files are deleted too unless something
else still uses them. Deleting a user together with their content also
deletes their uploads, picture and files folder.

## Image styles

An image style is a recipe for a resized copy of an image: a teaser
thumbnail, a full-width picture, an avatar. Content types' display
settings, Views fields and themes choose a style by name. The copy is
made the first time it is requested and kept on disk.

**Configuration → Image styles** (`/admin/config/image-styles`, in the
*Texts and files* group) lists the styles and adds new ones:

| Field | |
|---|---|
| name | the machine name, used in image addresses (`…/styles/<name>/…`). Can't be changed later |
| Name, Description | for the admin |
| Width, Height | the target size in pixels. Height `0` keeps the proportions (with *fit* or *resize*) |
| Mode | *fill* - cover the whole size and crop the rest; *fit* - fit inside the size, keeping the proportions; *resize* - scale to exactly the size |
| Anchor | which part to keep when *fill* crops: *the image's focal point* (see [Media library](#media-library); an image without one - the center), `center`, `top`, `bottom-left`… |
| Quality | 1-100, 85 by default: the copies of photos are compressed with losses at it (WebP or JPEG). A PNG or GIF original, or one with transparency, becomes a lossless WebP, as drawn graphics stay sharper and smaller that way |
| Format | *WebP* (the default) or *JPEG*, for feeds and services that don't take WebP |
| Sharp on high-density screens (2×) | a copy at double size too, see [High-density screens](#high-density-screens) (on the style's edit page) |

A new site has *Article card* and *Article picture* for its articles,
and *User avatar* and *User avatar (profile)* for profile pictures.

Saving a style deletes its old copies; they are made again with the new
settings. The copies are served as unchanging for a year
(`Cache-Control: immutable`), so their addresses carry a version of the
style's settings (`…/styles/<name>/<file>.webp?v=1a2b3c4d`): when the
size, mode, anchor, quality or format changes, the addresses change too,
and browsers and a CDN fetch the new copies instead of keeping the old
ones. Changing only the name or the description keeps them. Deleting a
style that is still used somewhere makes those images stop showing, so
change the places that use it first.

*Clear thumbnails* on the *Performance* page (`/admin/config/performance`)
deletes the copies of all styles; the originals are not touched.

A copy is made of an image up to 20,000 px a side and 50 megapixels, as
for uploads. A file that isn't such an image (another file, a damaged
one, a larger one) gets no copy: its address answers 404, the log has a
warning, and the file isn't tried again for ten minutes unless it
changes.

### High-density screens

Phones and many laptops have screens with two or three pixels per
point, where a copy of the style's own size looks soft. With *Sharp on
high-density screens (2×)* on (the style's edit page), the style makes a
copy at double size too, and a picture in it says so to the browser
(`srcset="… 1x, … 2x"`): such screens fetch the double one, the others
the usual one. A double copy takes several times the disk space
of a usual one. An original no larger than the usual copy gets none.

A double copy lives under `@2x/` of its style's directory
(`…/styles/article_card/@2x/content/images/a.webp`) and is made on its
first request, like the others. Every picture in a style also gets its
`width` and `height`, so the page doesn't jump while it loads.

A new site's *Article card* and *Article picture* have it on, and crop
at the focal point.

### The lightbox

A click on a picture - of an image field, of a gallery, of a text -
opens it enlarged in the lightbox. What opens is the image's copy in the
engine's *Lightbox* style (fitted in 2048 × 2048 without enlarging,
WebP, quality 85), not the original: a camera's photo of several
megabytes opens as a few hundred kilobytes, looking the same on a
screen. Change the style to change the size or the quality. An animated
GIF and an SVG open as they are (a copy would lose the animation, the
vector), as does every image when the style is deleted.

The original stays the file itself: downloads, the media library and
the users' files give it.

## Media library

*Content → Media library* (`/admin/content/media`, for those who may
administer content) shows every file uploaded to the site's fields -
images and documents of content, the content types' default images,
the modules' files such as gallery pictures - as tiles, the newest
first. The users' own folders are a separate place, see [Users'
files](#users-files).

Above the tiles: a search by the file's name or alt text, its kind
(*Images*, *Documents*), who uploaded it, and *Used nowhere*. Each tile
says in how many places its file is used.

A click on a tile opens the file's card beside them:

- its size, format, who uploaded it and when, and a link to the file;
- for an image, its **focal point** and **alternative text**, saved
  together. A click on the image puts the point where the subject is;
  the styles cropping at the focal point (their *Anchor*) keep it in the
  picture, and the card shows their crops as they will be. Changing the
  point drops the image's copies in every style: they are made again
  around it, at new addresses, so browsers and a CDN don't keep the old
  crop. The alternative text is one for every place the image is used,
  the text the content form's field shows too;
- **where it is used**: the content items with their field (marked when
  the item is in the trash), the content types it is the default image
  of, the modules;
- **Delete** - only for a file used nowhere: its row, the file and its
  image styles' copies. A file in use is deleted with the content that
  uses it, as before.

Files used nowhere are left behind, for example, when an image is
replaced in a content item's form. *Used nowhere* finds them, and *Delete
all unused* then deletes them all at once.

### Picking from the media library

An image or file field's *From the media library* in the content form
opens a window of the site's files the field may take: images for an
image field, of its extensions, no larger than its size limit and no
smaller than its least resolution; public files for a public field,
private ones for a field with *Private files* - of those, only your own
uploads, unless you may administer content. Search them by name or alt
text, pick one - its preview, size and in how many places it's used
show beside - and *Choose*: it takes the place of the field's image (or
file), or, for a field with several images, joins them as a new row.

The content item then uses the same file as the others: no copy is
made, and its alternative text is the file's - changing it in the form
changes it everywhere the image is used. Deleting the content item
keeps a file something else still uses.

The field's *Where files come from* setting decides what the form
offers: *Upload or the media library* (the default), *Upload only*, or
*The media library only* - the files the site already has, for example a
set of approved pictures. The picker isn't offered in a blocks field's
items.

The tiles are shown in the engine's *Media library* image style
(320×240); if it's deleted, the originals are shown instead.

## Images in the text editor

A text format that uses the visual editor (TinyMCE) lets writers add
images:

- the *Image* button opens a dialog whose browse button shows the
  writer's own files: pick one or upload a new one;
- an image pasted or dropped into the text is uploaded to the writer's
  files;
- the *Link* dialog can link to a file the same way, for example a PDF.

Uploads go to the writer's own folder and follow the text format's own
limits, or the upload policy where the format sets none. Only users with
*Upload documents* may upload files other than images.

On the page, an image of the site's files is shown at the size set in
the editor, from a preview of that size. A click opens it in the
[lightbox](#the-lightbox) when the original is larger than shown; an
image shown at its full size isn't a link.

## Users' files

Every signed-in user has a folder of their own. It opens at `/me/files`
(the *Files* tab of the profile) and in the editor's file dialogs. There
the user can:

- browse folders, as a list or as tiles with previews, and search by
  name;
- upload several files at once, with a button or by dragging them in;
- create folders, rename and delete files and empty folders;
- copy a file's link and view an image.

Images are always allowed. Other files need the *Upload documents*
permission (by default only the full-access role has it) and must be on
the *Documents in users' files* list. In a text, a document is inserted
as a link.

**Where a file is used.** Before a file is deleted or renamed, the site
looks for links to it in texts: content text fields, comments, private
messages, signatures, profile fields, blocks, Views' headers and footers,
and the texts of enabled modules. The user sees where the link would
break and confirms. The search reads the texts at that moment, so it
takes a few seconds.

**All users' files.** *Content → Users' files* (`/admin/content/files`) lists
the users who have files, the largest folders first, with the number
of files, their size and the last change, and a search by user name.
*Open* shows that user's folder in the same file browser. It needs the
*Administer users' files* permission.

---
title: Video embeds
description: A link to a YouTube, VK Video, Rutube or Vimeo video on its own line in a text shows as the video - a preview first, the player on a click.
sidebar:
  order: 11
---

The *Video embeds* module (`video_embed`) turns a link to a video into
the video itself. Paste the link on a line of its own in a text, and the
content page shows the video's preview with its title in that place; a
click starts the player right there. The saved text keeps just the link:
switching the module off, or changing its settings, changes how every
text shows its videos, old ones included.

A new site gets the module installed and enabled (it is one of the
standard modules of `app install`). On a site created before the engine
had it, it starts disabled: enable it on the **Modules** page (see
[Using modules](../using-modules.md)). Installing it turns its processing
on for the text formats with the visual editor.

## Which links

A link becomes a video when:

- it is a video of one of the services on (**Configuration → Video
  embeds**);
- it stands alone in its paragraph - the link's text or a link
  element, nothing else on the line;
- the value's text format has **Video links as players** ticked
  (*Text formats*, *Edit* - *Processing when shown*).

A link inside a sentence stays a link. So does a link to a site that
isn't on the list.

| Service | Links |
|---|---|
| YouTube | `youtube.com/watch?v=…`, `youtu.be/…`, `youtube.com/shorts/…` (vertical), `youtube.com/live/…` |
| VK Video | `vkvideo.ru/video…`, `vk.com/video…`, `vk.com/clip…` (vertical) |
| Rutube | `rutube.ru/video/…`, `rutube.ru/shorts/…` (vertical) |
| Vimeo | `vimeo.com/…` (off until switched on) |

A vertical video keeps its proportions: a narrower box in the middle of
the column, not stretched to its width.

## In the editor

The visual editor of a format with the processing on gets the **Video**
button: it asks for the link, checks it and puts it on a line of its
own. A link pasted on its own line works the same way. In the editor
such a link shows as a card above it - the service, the title and the
preview - so you see it will be a video; the card isn't part of the
text. The check (`/api/video-embed`) answers those who may create
content or work in the admin; for anyone else the editor shows no card,
and the site still shows the video.

## In a link field

A [link field](../../guides/content-types.md) can show its links as
videos: trailers of a film, say, one or several. On the content type's
*Display: detail page* tab, in the field's *How to show*, pick:

- **Video: player** - each video's player;
- **Video: preview, player on a click** - each video's preview with a
  play button.

With several links, *Several values* decides how they show: one after
another; the first large and the others as small previews in a row (a
click starts that one in its place); or only the first. A link that
isn't a video of a service on stays a link. A link's own title, when the
field has titles, stands for the video's. A [View](../../guides/views.md)
field of the link field has the same choice of its own - a card of a
list, say, shows only the first video's preview.

On the field's page, under *Allowed sites*, *Fill in* puts the services'
sites there, so the field takes links to them only. In the content form
a field shown as video tells under each link what it is: the video's
preview, title and service, or that it isn't a video link and will be
shown as a link.

With the module off the field shows as links; uninstalling it resets the
fields' choice to links.

## The preview and the player

The site asks the video's service about it once: the title, the preview
picture and the proportions (the service's oEmbed answer, or the Open
Graph tags of the video's page where the service has no oEmbed). A link
the content form checks - with the editor's **Video** button or card, or
under a link field - is asked about at once; any other within a minute
of the page first showing it, until then the box shows the service's
name. A service that doesn't answer is asked again a day later. When a
service tells nothing (VK Video may not tell a visitor who isn't signed
in), the video shows as a box with the service's name and the play
button.

The preview links to the video on its service: without JavaScript a
click opens it there.

## Settings

**Configuration → Video embeds** (`/admin/config/video-embed`, in the
*Texts and files* group):

| Setting | |
|---|---|
| *Services* | Which services' links become videos. |
| *The player on a click* | On (the default) - the preview and a play button until a click, then the player starts at once: the page loads faster, and the service learns nothing about the visitor before the click. Off - the player itself in the text. |
| *Previews copied to the site* | On (the default) - the site keeps a copy of each preview (in the *Video preview* image style, under `video_embed/` of the public files) and shows it, so the visitor's browser doesn't ask the service for it. Off - the preview is the service's picture. |

The YouTube player comes from `youtube-nocookie.com`, Vimeo's with *do
not track*.

## Themes

A video is the template `video_embed/embed.html` (the block
`video_embed`); a theme may have its own (see
[Templates](../../themes/templates.md)). Its classes are `video-embed`,
`video-embed--portrait`, `video-embed-play`, `video-embed-preview`,
`video-embed-title`, `video-embed-icon` and `video-embed-player`; the
proportions are `--video-embed-w` and `--video-embed-h`. There is no
caption under the video: a theme wanting one (the service's name, a link
to the video on it) adds it in its own template.

A link field shown as video is `video_embed/field.html` (the block
`video_embed_field`: `.Items`, each with `.Video` or, not a video,
`.Link`; `.Layout`), each video in it shown by `video_embed`. Its
classes are `video-embed-field`, `video-embed-field--featured`,
`video-embed-more` (the small previews) and `video-embed-link`.

A theme's copy of either template keeps `{{attach "video_embed"}}`: the
module's styles and the script starting the player.

## Uninstalling

Uninstalling the module takes its processing out of the text formats,
makes the link fields shown as video show as links and deletes the
previews' copies with its tables. The texts keep their links.

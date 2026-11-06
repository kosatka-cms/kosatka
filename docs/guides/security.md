---
title: Security
description: How Kosatka protects a site - uploads checked by their content, files served so they can't run, private files only for those who may see their content, safe HTML, sign-in protections - and what is up to you.
sidebar:
  order: 18
---

Security in Kosatka is built in rather than added by modules: what a
site accepts, how it serves files and who gets them work the same on
every site, without settings to remember. This page sums up the
protections, and what is left to the site's administrator.

## Uploads are checked by what they are

A file is judged by its content, never by its name or by what the
browser says it is:

- an image is recognized from its bytes (JPEG, PNG, GIF, WebP), and its
  stored extension follows the format found - an image named
  `photo.html` is stored as `photo.jpg`, never served as a page;
- an image declaring a huge size (a "decompression bomb": a small file
  that would take gigabytes of memory to open) is refused before it is
  decoded;
- web pages, SVG, scripts and programs are refused whatever the
  extensions a field allows: by their extension, and by what their
  content starts with - an HTML page renamed to `.pdf` doesn't get in;
- every field, text format and avatar has its own limits on types and
  size over the site-wide policy
  ([Files and images](./files-and-images.md#upload-policy)).

## Files are served so they can't run

The public files are data, never pages. Served by the site, or by nginx
set up as in [Running in production](./production.md#7-nginx-and-https),
they get:

- `X-Content-Type-Options: nosniff` - the browser takes the type as
  given, never guesses a script out of a file;
- a sandboxing `Content-Security-Policy` (on all but PDF, which browsers'
  viewers need without it) - even an HTML or SVG file that got there
  couldn't run a script or load anything;
- anything but images and PDF is downloaded, never opened in place;
- no folder listings, no dotfiles (`.htaccess`, `.env`).

A bucket that serves the files itself ([Files in object
storage](./file-storage.md), `public_url`) keeps anything but images and
PDF as downloads too, and lives on another domain than the site, out of
reach of its cookies.

## Private files

Some files must not be reachable by a link: documents attached to a
draft, files sent with forms, a section for members only. Kosatka keeps
them apart from the public files:

- **A file or image field can keep its files private** (the field's
  *Private files* setting). Such a file is stored in the private files -
  a directory the web server never serves, or a private bucket - and the
  site hands it out only to those who may see the content it is
  attached to, checked on every download by the same rules as the
  content's page: a file of an unpublished item opens only for those
  who may see unpublished content (and its editors), and opens for
  everyone once the item is published. The uploader can always open it.
- **A private image's styles are private too:** each derivative is made
  in the private files and served after the same check as the original.
  It is made from that very file only, never from another one with a
  similar name.
- **Files sent with forms** are always private: only those who may view
  the form's submissions download them.
- A link to a private file is worth nothing on its own: copied,
  forwarded, indexed or guessed, it gives no access to someone the
  content isn't shown to. A visitor is asked to sign in.
- Private files are sent with `Cache-Control: private, no-cache`, so no
  shared cache or CDN keeps a copy, and are never redirected to a
  bucket's public address.
- The site refuses to start with a private bucket that the public files'
  address could reach (the same bucket or container, one prefix inside
  the other).
- When the content is deleted, its private files go with it.

A file's place is decided when it is uploaded: switching a field to
private keeps its earlier files public.

## Unpublished content

Content that isn't public yet - a draft, an item waiting on moderation -
is shown only to those who may view unpublished content and, while it
waits on moderation, to its author. For everyone else it is missing
everywhere, not just from its page: the comments on it, the comment
lists, the forum, the blocks embedding it (a poll, a form, galleries)
and the editors' title search leave it out
([Moderation](./users-and-permissions.md#moderation)).

## Closed sections

A content type can be closed to some roles - members only, staff only,
signed-in visitors only - with its *Who may view* setting
([Content types](./content-types.md#type-settings)). The protection
doesn't rely on each list remembering it: every query the site makes
for content and comments leaves out the types the visitor may not view,
so its content is missing from:

- its pages (they answer "not found": the content isn't revealed to
  exist), the comments on it and their links;
- lists (Views), the front page, term pages, "similar content",
  profile tabs, blocks;
- search, the title search of the editors' forms;
- feeds and the sitemap - always made as for a guest, since any cache
  or bot may keep them;
- e-mails of the *Subscriptions* module: a subscriber is told only of
  content they may view;
- private files attached to it ([Private files](#private-files)).

Blocks kept in memory for all visitors are made as for a guest; those
that would show closed content are made again for the visitors who may
view it, kept apart for each kind of access, so nobody gets someone
else's. The public files of the closed content (its images in a public
image field) are still reachable by their address: use a private field
for files that must stay closed.

Who may edit any of the type's content, and who has the *View the
content of every type* permission, see it anyway. The types that come
with modules (forum topics, polls) can't be closed this way.

One item can be closed the same way, in its form's *Access* section
([One item's access](./content-types.md#one-items-access)): it is
missing from the same places for those without one of its roles - but
its author, those who may edit it and those who may view every type's
content see it.

## HTML is cleaned

Text typed into the site is cleaned on save: content and blocks by
their text format, comments and profile signatures by a fixed policy.
The default formats and that policy keep safe HTML only, without
scripts, event handlers, `javascript:` links or frames. A format
allowing `script` or `style` (needed for an ad snippet, say) can be used
only by those with full access ([Blocks](./blocks.md)).

## Sign-in and sessions

- Passwords are stored as bcrypt hashes, never as they are.
- Two-factor sign-in with an authenticator app, and recovery codes
  ([Users and permissions](./users-and-permissions.md#two-factor-sign-in)).
- Sign-in, registration and password forms, comments, the search and the
  modules' public forms (forms, polls, ratings, complaints, error
  reports) are rate limited per address (an IPv6 address counts as its
  /64 network); a CAPTCHA can be added
  ([CAPTCHA](./users-and-permissions.md#captcha)).
- An account takes ten sign-in tries, then one a minute, whoever makes
  them; an address gets three password reset mails, then one in ten
  minutes.
- Every state-changing request carries a CSRF token: another site can't
  post a form in a visitor's name.
- Session cookies are `HttpOnly` and `SameSite=Lax`, signed with a key the
  site refuses to start without in production. The site accepts a cookie
  for `session.max_age` at most, whatever the browser keeps.
- A new password (set in the profile or by a reset link) and two-factor
  sign-in turned on, off or reset end every other session of the
  account: a session opened with a stolen password or cookie is signed
  out on its next request. A blocked account is signed out the same way.
- A failed sign-in takes as long whether the login exists or not, and
  the password reset form answers at once either way: neither tells
  whether an address has an account.
- Two-factor codes: five wrong ones in ten minutes end the attempt; the
  count is kept by the site, per account.

## Secrets stay out of the config

The keys of a bucket or a storage account come from the environment
only: the config file's are ignored. The database password, the session
key and the secrets key may be in the config file too, but belong, as in
[Running in production](./production.md#4-the-config), in an env file
only the service reads. None of them is taken on the command line or on
a page of the admin, and `app config` prints the settings without them.

## Secrets in the database

Some secrets live in the database: the keys and passwords of mail,
CAPTCHA and social sign-in entered in the admin, and each user's
two-factor secret. With `SECRETS_KEY` set they are stored sealed
(AES-256-GCM), so a copy of the database - a backup, a dump for
debugging - doesn't give them away. The admin never shows a saved key:
its field is empty, and left empty it keeps the key.

- Set `SECRETS_KEY` to a long random string and restart: at start the
  site seals what it kept unsealed. `app install` prints one when none
  is set, `app sites add` writes one; the status report warns about a
  guessable key (an example value, too few different characters). Without it they are stored as they
  are, and the status report (**Reports → Status report**) warns.
- Keep the key with the backups: a database restored without it has
  secrets that don't open.
- To replace the key, set `SECRETS_KEY=new,old` and restart: the site
  seals everything again with the new key, then the old one can go.
- A secret the key doesn't open reads as none, and the status report
  names where: enter it again in the admin. Users set up two-factor
  sign-in again; their recovery codes still work.

A key set in the server's configuration (`SMTP_PASSWORD`, a mail
module's such as `RESEND_API_KEY`, `RECAPTCHA_*`, `*_OAUTH_*`) wins over the one saved in
the admin, and the admin page doesn't offer the field: someone with the
admin's password can't send the site's email - password resets
included - through an account of their own. Set the keys there when the
server is yours; the admin is for sites whose owners have no access to
it.

## Response headers

Every response carries:

- `X-Content-Type-Options: nosniff` - a browser takes a file for the type
  it is sent as, never guesses a page or a script in it;
- `X-Frame-Options: SAMEORIGIN` - another site can't show the site's
  pages in a frame (under its own buttons);
- `Referrer-Policy: strict-origin-when-cross-origin` - a link to another
  site tells it the site's address only, not the page's path and query;
- over HTTPS, `Strict-Transport-Security: max-age=31536000` - a browser
  that has seen the site over HTTPS keeps to it for a year (the site's
  own host only).

HTTPS behind a proxy is known from its `X-Forwarded-Proto`, believed
only from a [trusted proxy](../reference/configuration.md#the-server).
The files under `/static/` are served one by one: a folder's listing
and dotfiles answer 404.

## Who did what

The *Action log* module records the actions that matter - content
deleted or unpublished, users blocked, permissions and settings
changed, sign-ins - with who did them and when
([Action log](../modules/builtin/action-log.md)).

## What is up to you

- Serve the site over HTTPS only and set `session.secure: true`
  ([Running in production](./production.md#7-nginx-and-https)).
- Give each role only the permissions it needs; *full access* to as few
  people as possible.
- Keep the engine up to date: security fixes go into the latest release.
- Keep backups of the database and the files, and check you can restore
  them ([Backups](./production.md#10-backups)).

## Reporting a vulnerability

Please don't report security problems in public issues: see the
repository's security policy (`SECURITY.md`) for how to report one
privately.

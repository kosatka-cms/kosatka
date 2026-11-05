---
title: Building a site
description: The guides to setting up a Kosatka site in the admin and running it.
sidebar:
  label: Overview
  order: 0
---

These guides walk through building a site in the admin, part by part,
and running it in production. They assume a site is installed and you
are signed in as its administrator (see
[After installation](../getting-started/first-steps.md)).

A typical order:

1. [Content types and fields](./content-types.md) - what kinds of content
   the site has and what each holds.
2. [Taxonomy](./taxonomy.md) - the sections and tags that classify it.
3. [Views](./views.md) - the lists of content: pages, blocks, feeds.
4. [Blocks and regions](./blocks.md) - what goes where on the page.
5. [Embeds](./embeds.md) - a block or a View's list shown inside a text.
6. [Menus](./menus.md) - the site's navigation.
7. [Addresses and SEO](./urls-and-seo.md) - URL aliases, redirects,
   breadcrumbs, titles and descriptions, the sitemap.
8. [Search](./search.md) - what the site search finds.
9. [Users and permissions](./users-and-permissions.md) - roles, what each
   may do, registration, sign-in.
10. [Email](./mail.md) - how the site sends email and the texts of the
    emails it sends.
11. [Comments and spam](./comments.md) - comments waiting for approval,
    spam and its checks.
12. [Files and images](./files-and-images.md) - uploads and image styles.
13. [Files in object storage](./file-storage.md) - the uploads and image
    styles in an S3-compatible or Azure bucket instead of the server's
    disk.
14. [Languages](./languages.md) - the site's language and translations.
15. [Views and popular content](./statistics.md) - counting content views
    and the most viewed content.
16. [Running in production](./production.md) - a server, a web server in
    front, updates and backups.
17. [Several sites on one server](./multisite.md) - one engine for many
    sites, each with its own database; one command updates them all.
18. [Security](./security.md) - what the site protects by itself and
    what is up to you.

The look of the site belongs to its [theme](../themes/index.md), and
extra features to [modules](../modules/index.md); their guides come
separately.

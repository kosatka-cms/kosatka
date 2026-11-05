---
title: Kosatka
description: A content management system written in Go.
template: splash
hero:
  tagline: A content management system written in Go. Powerful tools for building any kind of site - a blog, a news portal, a community, a company site - in a convenient admin, without writing code.
  image:
    file: ./assets/logo.svg
  actions:
    - text: Get started
      link: getting-started/
      icon: right-arrow
    - text: Build a site
      link: guides/
      variant: secondary
    - text: GitHub
      link: https://github.com/kosatka-cms/kosatka
      variant: minimal
      icon: external
---

## Try it

```bash
git clone https://github.com/kosatka-cms/kosatka.git && cd kosatka
docker compose up --build
```

Open http://localhost:8080: a site with demo content and every module.
Sign in at http://localhost:8080/login as `admin` with the password
printed once in the log:

```bash
docker compose logs app | grep password
```

More in [Run in Docker](./getting-started/docker.md).

## What is here

- [Getting started](./getting-started/index.md) - what a site is made
  of, running Kosatka in Docker or from source, the first steps in the
  admin.
- [Building a site](./guides/index.md) - content types, taxonomy, Views,
  blocks, menus, addresses and SEO, search, users, files, languages, and
  running a site in production.
- [Themes](./themes/index.md) - writing a theme: the manifest, templates,
  assets, translations, and shipping it from its own repository.
- [Modules](./modules/index.md) - using the bundled modules and adding
  others, and writing a module with the public SDK.
- Reference - [the `app` commands](./reference/cli.md),
  [the configuration](./reference/configuration.md) and
  [the content model](./reference/content-model.md): the tables of
  content and what saving and deleting an item does.
- [Contributing](./contributing/index.md) - how each part of the engine
  works, for those who change it.

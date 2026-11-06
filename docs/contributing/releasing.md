---
title: Releasing
description: How a release is made - a vX.Y.Z tag on main - and what the release workflow builds from it.
sidebar:
  order: 5
---

A release is a `vX.Y.Z` tag on a commit of `main`. Pushing the tag runs
the release workflow (`.github/workflows/release.yml`); nothing is
uploaded by hand.

## Versions

Versions follow [semantic versioning](https://semver.org/). Before 1.0
a minor version (`v0.3.0`) may change what sites, themes or modules rely
on - its release notes say what; a patch version (`v0.3.1`) only fixes.
The binary reports its version on the status report
(*Reports → Status report*): the tag it was built from, or, for a build
between releases by `scripts/build_linux_release.sh` or the
`Dockerfile`, the last tag with the distance from it
(`v0.3.0-5-g1a2b3c4d`). A plain `go build` has none: "not a release
build".

## Making a release

1. Check that `main` is green: the test workflow passed on its last
   commit (or run `./scripts/check.sh -race -db` locally).
2. Tag the commit and push the tag:

   ```bash
   git switch main && git pull
   git tag -a v0.3.0 -m "v0.3.0"
   git push origin v0.3.0
   ```

3. Watch the *release* workflow in the repository's *Actions*. When it
   finishes, edit the release's notes on GitHub if they need more than
   the list of merged pull requests - what a site has to do when
   updating, above all.

The workflow starts only on a tag of three numbers
(`v[0-9]+.[0-9]+.[0-9]+`): `v0.3.0` does, `v0.3.0-rc1` doesn't.

## What the workflow builds

- **Archives** - `kosatka-<tag>-linux-amd64.tar.gz` and
  `kosatka-<tag>-linux-arm64.tar.gz` (`kosatka-v0.3.0-linux-amd64.tar.gz`),
  laid out as a release directory of [multisite](../guides/multisite.md):
  the binary `kosatka` (with the tag as its version), `web/templates`,
  `web/static` (minified, with the modules' static files from
  `app assets export`), `themes/`, `LICENSE` and `README.md`. The binary
  is for a Linux with glibc (Debian, Ubuntu and the like): the WebP
  encoder loads `libwebp` when the server has it, so the binary links
  dynamically. On Alpine, use the image.
- **The GitHub release** of the tag with the archives and `SHA256SUMS`;
  its notes are generated from the pull requests merged since the
  previous release.
- **The Docker image** - `ghcr.io/kosatka-cms/kosatka`, for
  `linux/amd64` and `linux/arm64`, tagged with the version (`0.3.0`),
  its minor line (`0.3`) and `latest`. It is the repository's
  `Dockerfile` (see [Run in Docker](../getting-started/docker.md)).

The tests aren't run again: they ran on the pull request and on `main`.

## Checks on every change

The test workflow (`.github/workflows/test.yml`) runs on every pull
request and push to `main`: golangci-lint, govulncheck, gitleaks, the
unit tests with the race detector, the integration tests on MySQL and
PostgreSQL, the check that the
migration files keep up with the ent schemas, the example module and
site, and a build of the Docker image. Locally,
`./scripts/check.sh -race -db` runs them all but the image's build;
golangci-lint, govulncheck and gitleaks only when they are installed.

gitleaks scans the whole history for secrets: an API key or a password
committed by mistake fails it, even when a later commit removes it. What
only looks like a secret - a test's made-up key, a demo set's item
keys - is listed in `.gitleaks.toml`,
each with the narrowest pattern that matches it.

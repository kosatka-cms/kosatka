---
title: Files in object storage
description: Keep the uploads and image styles in an S3-compatible bucket - AWS S3, Yandex Object Storage, Google Cloud Storage, Cloudflare R2, MinIO - or in Azure Blob Storage instead of the server's disk, with a storage package added to the site's binary.
sidebar:
  order: 13
---

By default the public files - uploads, image style derivatives, the
users' own files - are kept on the server's disk (`web/uploads`). They
can be kept in an S3-compatible bucket or an Azure Blob Storage
container instead:

- the files don't take the server's disk and aren't lost with it: the
  storage keeps them, replicated;
- a site moved to another server takes no files with it;
- the bucket, or a CDN in front of it, can serve the files itself,
  taking that traffic off the site.

The engine keeps the files on disk itself; a storage elsewhere is a Go
package of its own that the site adds to its binary, like a module from
its own repository:

| Storage | Package | `files.storage` |
|---|---|---|
| S3-compatible buckets | [`kosatka-storage-s3`](https://github.com/kosatka-cms/kosatka-storage-s3) | `s3` |
| Azure Blob Storage | [`kosatka-storage-azure`](https://github.com/kosatka-cms/kosatka-storage-azure) | `azure` |

Another storage can be written the same way: see [File
storages](../modules/sdk.md#file-storages) in the module SDK.

Any storage speaking the S3 API works:

| Storage | `endpoint` | `region` |
|---|---|---|
| AWS S3 | `https://s3.amazonaws.com` | the bucket's, e.g. `eu-central-1` |
| Yandex Object Storage | `https://storage.yandexcloud.net` | `ru-central1` |
| Google Cloud Storage (interoperability, HMAC keys) | `https://storage.googleapis.com` | `auto` |
| Cloudflare R2 | `https://<account id>.r2.cloudflarestorage.com` | `auto` |
| DigitalOcean Spaces | `https://<region>.digitaloceanspaces.com` | the region, e.g. `fra1` |
| Selectel, VK Cloud and others | the provider's S3 address | the provider's |
| MinIO, SeaweedFS (your own) | e.g. `http://127.0.0.1:9000`, with `path_style: true` | - |

Azure Blob Storage has an API of its own: see [Azure Blob
Storage](#azure-blob-storage).

The private files (sent with forms, of fields with *Private files*) can
go to a bucket of their own too (see [Private files](#private-files)).

## Set it up

1. Add the storage to the site's binary (the site then builds its own,
   see [Add a module from its own
   repository](../modules/using-modules.md#add-a-module-from-its-own-repository)):

   ```bash
   app module add github.com/kosatka-cms/kosatka-storage-s3
   go build -o app .
   ```

   The engine's Docker image has no storage of its own: a site keeping
   its files in a bucket runs an image of its own binary.
2. Create a bucket and a key with access to it (reading, writing, listing
   and deleting objects).
3. Set the storage in the site's config:

   ```yaml
   files:
     storage: s3
     s3:
       endpoint: https://storage.yandexcloud.net
       region: ru-central1
       bucket: my-site
       prefix: blog/                # optional: the bucket's folder for this site
       public_url: https://my-site.storage.yandexcloud.net/blog/   # optional, below
   ```

   and the key in its environment - never in the config file:

   ```bash
   FILES_S3_ACCESS_KEY=…
   FILES_S3_SECRET_KEY=…
   ```

4. Copy the existing files into the bucket (`app files migrate`, see
   [Existing files](#existing-files)) and restart the site.

The status report (Reports → Status report) shows the bucket under
*Public files* and checks that the site can write there.

A `storage` the binary doesn't have - its package not added, or taken
out - stops the site at startup with the setting's name: add the package
or set `storage` back. `app module remove` refuses to take out a storage
the site keeps its files in.

| Key | Variable | |
|---|---|---|
| `storage` | `FILES_STORAGE` | `local` (default), or a storage the binary has: `s3`, `azure` ([below](#azure-blob-storage)) |
| `s3.endpoint` | `FILES_S3_ENDPOINT` | the storage's address, `https://…` (`http://` - without TLS) |
| `s3.region` | `FILES_S3_REGION` | the bucket's region |
| `s3.bucket` | `FILES_S3_BUCKET` | the bucket |
| `s3.prefix` | `FILES_S3_PREFIX` | the bucket's folder for the site's files; several sites can share a bucket, each with its own |
| `s3.public_url` | `FILES_S3_PUBLIC_URL` | where the bucket or its CDN serves the files itself; empty - the site serves them |
| `s3.path_style` | `FILES_S3_PATH_STYLE` | `true` - `<endpoint>/<bucket>` addresses (MinIO, SeaweedFS, some providers) |
| - | `FILES_S3_ACCESS_KEY`, `FILES_S3_SECRET_KEY` | the key |

## How the files are served

The files' addresses don't change: pages, and the texts that link to
files, keep `/uploads/…`. A request for one goes to the site
(nginx finds no file on disk and passes it on, as for an image style not
made yet), and the site:

- **without `public_url`** reads the file from the bucket and serves it
  itself, with the same headers as from disk. The bucket can stay
  private. The files' traffic goes through the site.
- **with `public_url`** sends the visitor there (a redirect, kept by
  browsers for a day). The bucket - or the CDN in front of it - has to
  serve the files publicly. Files other than images and PDF are stored
  as downloads (`Content-Disposition: attachment`), and the bucket's
  address is another site than yours, so a file uploaded there can't
  act on your site's pages.

  The pages themselves link the originals - images, attached files, the
  logo - and the previews of images in texts at `public_url` directly,
  without the redirect. An image style's derivative keeps the site's
  address: it is made on its first request, which only the site can do.

An image style's derivative is made on its first request, as on disk:
the site reads the original from the bucket, stores the derivative next
to it (`styles/<style>/…`) and serves it, or sends the visitor to it.
Clearing the image styles (Configuration → Performance) removes them
from the bucket.

## Azure Blob Storage

1. Add the storage to the site's binary: `app module add
   github.com/kosatka-cms/kosatka-storage-azure`, as above.
2. Create a storage account and a container in it.
3. Set the storage in the site's config:

   ```yaml
   files:
     storage: azure
     azure:
       account: mysite
       container: files
       prefix: blog/              # optional
       public_url: https://mysite.blob.core.windows.net/files/blog/   # optional
   ```

   and the account's access key in its environment:

   ```bash
   FILES_AZURE_KEY=…
   ```

Everything else works as with a bucket: the addresses, `public_url` (the
container then needs anonymous read access for blobs, or a CDN in front
of it), the image styles, `app files migrate`.

| Key | Variable | |
|---|---|---|
| `storage` | `FILES_STORAGE` | `azure` |
| `azure.account` | `FILES_AZURE_ACCOUNT` | the storage account |
| `azure.container` | `FILES_AZURE_CONTAINER` | the container |
| `azure.prefix` | `FILES_AZURE_PREFIX` | the container's folder for the site's files |
| `azure.endpoint` | `FILES_AZURE_ENDPOINT` | the account's blob address; empty - `https://<account>.blob.core.windows.net` (the Azurite emulator: `http://127.0.0.1:10000/devstoreaccount1`) |
| `azure.public_url` | `FILES_AZURE_PUBLIC_URL` | where the container or its CDN serves the files itself |
| - | `FILES_AZURE_KEY` | the account's access key |

An empty folder is kept as a `.folder` blob in it (Azure advises against
blob names ending in `/`).

## Private files

The files that must not be reachable by a link - those sent with forms,
those of fields with *Private files* - are kept in the private directory
(`files.private_dir`), or in a bucket:

```yaml
files:
  private_storage: s3
  private_s3:
    bucket: my-site-private
    prefix: blog/          # optional
```

It is reached with `files.s3`'s `endpoint`, `region`, `path_style` and
keys (the public files may stay on disk). In Azure:
`private_storage: azure` and `private_azure.container` (and `prefix`), in
`files.azure`'s account. Keep it private: the site reads a file from it
and hands it out only after checking the viewer's permission, never
redirecting there. The site refuses to start when the private files
would be where the public files' address reaches - the public bucket
with the same prefix, one inside the other, or no prefix. Use another
bucket, or a prefix beside the public files' one.

| Key | Variable | |
|---|---|---|
| `private_storage` | `FILES_PRIVATE_STORAGE` | `local` (default), or a storage the binary has: `s3`, `azure` |
| `private_s3.bucket` | `FILES_PRIVATE_S3_BUCKET` | the private files' bucket |
| `private_s3.prefix` | `FILES_PRIVATE_S3_PREFIX` | its folder for the site's files |
| `private_azure.container`, `private_azure.prefix` | `FILES_PRIVATE_AZURE_CONTAINER`, `FILES_PRIVATE_AZURE_PREFIX` | the same in Azure |

`app files migrate -private` copies the private directory into it (see
below).

## Existing files

Switching a site with files to the bucket needs them in it first. With
the bucket set in the config (the site may still be running on its
disk), copy them:

```bash
app files migrate -dry-run      # what it would copy
app files migrate
```

```text
files: ./web/uploads -> s3://my-site/blog/
files: copied 12840 (3.2 GB), already there 0, left out 5120 (derivatives, dotfiles), failed 0
```

Then restart the site. The command:

- copies every file of the files directory (`files.public_dir`) to the
  same path in the bucket, under the prefix;
- skips a file the bucket has already with the same size, so a copy
  cut short - or one run again for the files uploaded meanwhile - goes
  on where it stopped;
- leaves out the image styles' derivatives and previews (the site makes
  them again; `-derivatives` copies them too) and dotfiles;
- never removes anything: remove the directory's files yourself once
  the site works from the bucket.

`-private` copies the private files (`files.private_dir` into the
bucket of `files.private_storage`). `-to-local` copies the other way,
from the bucket into the directory - for leaving the bucket. `-jobs <n>`
sets how many files are copied at once (default 8). Any S3 tool
(`aws s3 sync`, `rclone`) does the same job too.

Since the addresses stay the same, the texts linking to the files keep
working.

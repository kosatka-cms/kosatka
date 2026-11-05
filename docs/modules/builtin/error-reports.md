---
title: Error reports
description: Visitors select a mistake in the text and press Ctrl+Shift+E; the reports are listed under Content → Error reports and e-mailed to the site's address.
sidebar:
  order: 6
---

The *Error reports* module (`error_reports`) lets visitors point out
mistakes on the site: they select the wrong words on a page and press
<kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>E</kbd> (<kbd>Cmd</kbd>+<kbd>Shift</kbd>+<kbd>E</kbd>
on a Mac). A short form opens, and the report goes to the admin's
**Content → Error reports** list and to the site's e-mail address.

A new site doesn't get the module unless you ask for it
(`app install -modules error_reports,…` or `-modules all`). To add it
to a running site, enable **Error reports** on the **Modules** page (see
[Using modules](../using-modules.md)). The module adds no content types
and no permissions.

## What a visitor does

1. Select a fragment of text on any page of the site, at least 3
   characters.
2. Press <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>E</kbd>. With nothing
   selected, or a shorter selection, a message says *Select a text
   fragment on the page (at least 3 characters).* The shortcut does
   nothing while the cursor is in a text field.
3. The *Report an error* window shows:

| Field | |
|---|---|
| *Page* | the page's address (path and query), read-only |
| *Selected text* | the selection, read-only |
| *Comment* *(optional)* | what is wrong, up to 2000 characters (*Describe the error*) |

4. Press *Send*. On success the window closes and *Thank you! Your
   report has been sent.* appears briefly; otherwise the error is shown
   in the window and the visitor can try again.

Guests and signed-in users can both report. A signed-in user's report
records who sent it. The server accepts selections of 3 to 10,000
characters and only addresses on the site itself.

The form is on every public page of the site (not in the admin), as
long as the module is enabled. A theme with its own page layout must
keep the modules' end-of-page markup for it to work (see
[Templates](../../themes/templates.md)).

## The *Error report hint* block

Visitors won't guess the shortcut, so tell them. The module adds a
system block, *Error report hint*, that shows one line: *Found an error
in the text? Select it and press Ctrl+Shift+E.* It has no settings of
its own.

A new site doesn't place it anywhere. Add it on **Structure → Blocks**
with *Add system block*, choose a region (the footer or the sidebar
suits it) and, if you like, limit the pages it shows on (see
[Blocks and regions](../../guides/blocks.md)). Themes can style it through
the `error-report-hint` class.

## Reading the reports

**Content → Error reports** (`/admin/error-reports`) lists the reports,
newest first, 25 to a page. The menu item shows the number of new
reports - the ones not yet marked fixed. The *New* tab, open by default,
lists those; *All* lists every report, the fixed ones dimmed with the
date they were fixed:

| Column | |
|---|---|
| *Selection* | the selected text, shortened (hover to see it whole), and under it the comment, if any |
| *Page* | the page's address; click it to open the page in a new tab |
| *User* | the sender's name, linked to their profile, or *Guest*; under it the IP address |
| *Date* | when it was sent |
| | *Fixed* - the button that marks a new report fixed; for a fixed one, the date and *Back to the new ones* |

Fix the text in the content itself, then press *Fixed*: the report
leaves the new ones and the menu's number goes down. Marked fixed by
mistake, or not fixed after all - press *Back to the new ones* on the
*All* tab. Reports can't be edited or deleted one by one. The page is
open to every role with *Access administration (/admin)*.

### E-mail notifications

Each report is also e-mailed to the address in **Site information**
(*Email address*, `/admin/config/system/site-information`). The e-mail
is *New error report* on **Configuration → Mail → Emails** (see
[Email](../../guides/mail.md)): its subject and text, and a switch to
stop it. Besides the site's placeholders it has the report's:

| Placeholder | Replaced with |
|---|---|
| `[report:id]` | the report's number |
| `[report:date]` | when it was sent |
| `[report:author]` | the user who sent it, or *Guest* |
| `[report:page]` | the page's full address |
| `[report:text]` | the text selected |
| `[report:comment]` | the comment (a dash when none) |
| `[report:ip]` | the sender's IP address |
| `[report:admin-url]` | the list of reports in the admin |

Without an address, or without a working mail transport (the *Mail*
page, or [the configuration](../../reference/configuration.md#e-mail-email)),
the report is still saved, only no letter is sent.

## Spam protection

- **CAPTCHA.** When reCAPTCHA is set up on **Spam protection
  (reCAPTCHA)** (see [CAPTCHA](../../guides/users-and-permissions.md#captcha)),
  the report form is in its *Forms* list as *Error report (Ctrl+Shift+E)*
  and is ticked by default. The visitor then has to tick "I'm not a
  robot" before sending (*Please confirm that you are not a robot.*).
- **Rate limit.** One address can send 3 reports in a row, then about
  one every 6 seconds (10 a minute); extra requests are refused until
  then, and the form says so ("Too many requests. Please wait a little
  and try again.").
- Reports are accepted only from the site's own pages (the form carries
  the page's security token).

## Disabling and uninstalling

While the module is disabled, the form and the shortcut are gone from
the pages, the *Error report hint* block isn't shown, and **Content →
Error reports** is hidden. The saved reports stay.

Uninstalling (see [Using modules](../using-modules.md)) deletes the
*Error report hint* block's placements and the `error_reports` table
with all the reports. Back up the database first if you want to keep
them.

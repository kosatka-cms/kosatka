---
title: Email
description: How the site sends email, the emails it sends, their texts and placeholders, switching one off, a test email and the log of what was sent.
sidebar:
  order: 11
---

The site sends email about accounts (a confirmation link, a password
reset, a welcome), and its modules send their own: a private message,
the subscriptions' notifications, an error report. Everything about it
is on **Configuration → Mail** (`/admin/config/mail`), on three tabs.
The page is open to full administrators only: it holds the transport's
keys.

## Sending

The first tab sets how email leaves the site: the switch that turns all
of it off, the transport - an SMTP server, or a mail service's API that
a module adds - and the sender's address. Saved keys and passwords are
never shown again; leave a field empty to keep the saved one. When the
server's [configuration](../reference/configuration.md#e-mail-email)
sets a transport, it is the one in use: the page names it and offers
only the switch and the log.

The engine sends through SMTP itself; almost every mail service also
takes mail over SMTP. Its API is a module's:

| Service | Module | Its setting from the server's environment |
|---|---|---|
| [Resend](https://resend.com) | [`kosatka-module-resend`](https://github.com/kosatka-cms/kosatka-module-resend) | `RESEND_API_KEY` |

Such a module is added to the site's binary ([Add a module from its own
repository](../modules/using-modules.md#add-a-module-from-its-own-repository)),
and its transport is offered here while it is enabled; with the module
disabled, no email goes through it, and the page says so. Another
service's module can add one: see [Ways of sending
email](../modules/sdk.md#ways-of-sending-email) in the module SDK. *Delete the saved settings* removes the
ones saved here. *Mail log* - how many days
the log keeps an email (7, 30 - the default, 90 or 365), or *don't keep
a log*. *Send a test email* sends one to your own address, with the
settings in use (save the changes first).

## The site's emails

The *Emails* tab lists every email the site sends: the accounts' first,
then each enabled module's. A row shows when the email is sent, who
gets it and, under *Sent*, whether it goes out:

- **always** - an email the site can't work without: the invitation,
  the e-mail confirmation link, the password reset;
- **a switch** - the others: turn one off and the site stops sending
  it. The welcomes, the activation, the administrators' notice of a new
  registration and most modules' emails are of this kind.

*Last sent* - when it last went out; a click opens its entries in the
log.

| Email | When | To |
|---|---|---|
| Invitation | an administrator [invited](./users-and-permissions.md#managing-accounts) the person: they set their own password by the link | the user |
| Welcome (new user created by an administrator) | an administrator created the account with a password | the user |
| Email confirmation on registration | a visitor signed up and must confirm the address; also a changed address, or a social login's address its provider doesn't vouch for, where signing in needs a confirmed one | the user |
| Welcome (awaiting approval) | a visitor signed up; an administrator must approve the account | the user |
| Welcome (no approval required) | a visitor signed up and may sign in at once | the user |
| Account activation | an administrator approved the account | the user |
| Password recovery | the user asked for a link to set a new password | the user |
| Admin notification (new registration request) | someone signed up and awaits approval | the site's email (*Site information*) |

The modules' emails are described with the modules:
[private messages](../modules/builtin/private-messages.md#the-template),
[subscriptions](../modules/builtin/subscriptions.md#the-template),
[error reports](../modules/builtin/error-reports.md#e-mail-notifications),
[complaints](../modules/builtin/complaints.md#the-e-mail).

## An email's text

A click on an email opens its page:

- **Subject** and **Text** - plain text. A placeholder in square
  brackets is replaced with its value when the email is sent.
- **Placeholders** - the ones this email has; a click puts one into the
  subject or the text, where the cursor is.
- **How it looks** - the saved text with sample values: the site's
  name, your account, sample links.
- **Send a test email** - the email with those sample values to the
  address given, whether it is switched on or not.
- **Default text** - back to the built-in text in the site's language.
  A field left blank means the same.

The placeholders every email has:

| Placeholder | Replaced with |
|---|---|
| `[site:name]` | the site's name |
| `[site:slogan]` | the site's slogan |
| `[site:url-brief]` | the site's address |
| `[site:login-url]` | the sign-in page's address |

An email about a user also has `[user:name]`, `[user:email]` and
`[user:edit-url]` (their profile's form), and the link it carries:
`[user:confirm-email-url]`, `[user:one-time-login-url]` (a one-time
sign-in), `[user:reset-password-url]`; the invitation -
`[user:invite-url]` (its link), `[invite:by]` (who invited) and
`[invite:role]` (the role given); the administrators' notice has
`[site:people-pending-url]` (the accounts awaiting approval). A
module's email adds its own, listed on its page.

Links in an email need the site's address: set it in *Site
information* (or `public_base_url` in
[the configuration](../reference/configuration.md#the-server)), or the
account emails aren't sent.

## The log

The *Log* tab lists every email the site sent or tried to, newest
first: when, to whom, which email and its subject, and what came of it:

- **Sent** - the sending service took it (whether it reached the inbox
  is the service's to say: its dashboard, the SMTP server's log);
- **Failed** - the service refused it, with its answer, such as
  `550 mailbox unavailable`;
- **Not sent** - email was off or not configured.

Filter it by a part of the address, by the email and by the result. A
message of no template - a [form](../modules/builtin/forms.md)'s notice -
is listed as such. The log keeps no email's text, and entries older than
the *Mail log* period (the *Sending* tab) are deleted hourly.

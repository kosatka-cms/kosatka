---
title: Subscriptions
description: Users follow the comments of content and the terms they care about, and are told of what happens on the Notifications tab of their profile and by e-mail.
sidebar:
  order: 10
---

The *Subscriptions* module (`subscriptions`) tells signed-in users of
what happens where they care:

- a new comment on content they follow;
- a reply to their own comment;
- new content with a term (a tag, a section) they follow.

Each of these becomes a *notification*: on the *Notifications* tab of
their profile (`/me/notifications`), with the unread ones counted in the
user menu, and in an e-mail - at once or as a daily digest, as the user
chooses.

A new site gets the module installed and enabled (it is one of the
standard modules of `app install`). On a site created before the engine
had it, it starts disabled: enable it on the **Modules** page (see
[Using modules](../using-modules.md)). It has no permissions and no
blocks: once it is on, every signed-in user can use it.

## Following

### The comments of content

Above the comments of a published content item the module shows
**Subscribe to the comments**. A user who clicks it follows the item:
each new comment on it becomes a notification for them. The button then
reads *You are subscribed* with **Unsubscribe** next to it. A guest sees
*Log in to subscribe*, which brings them back after signing in.

A user follows content without clicking too:

- the content they create, once it is published;
- the content they comment on.

Both are on by default; each user can turn them off in their settings
(see [The user's settings](#the-users-settings)). Unsubscribing is
remembered: a user who unsubscribed from a content item isn't subscribed
again by commenting on it.

Only published content can be followed, and only comments on published
content are told of.

### Replies

A reply to a user's comment always tells that user (a *reply*), whether
or not they follow the content, unless they turned *Replies to my
comments* off. Someone told of a reply isn't told of the same comment
again as a follower.

### Terms

A term's page (a tag, a section - a vocabulary with public term pages)
has a **Follow** button; then it reads *You follow it* with **Stop**. A
user following a term is told of each new content item that has it,
when the item is published. An item published again later isn't told of
twice; its author isn't told of their own content.

Nobody is told of their own comments or content.

## The Notifications tab

A signed-in user opens their notifications from *Notifications* in the
user menu or from the *Notifications* tab of their own profile
(`/me/notifications`). Only its owner sees the tab: administrators don't
see other users'.

The left column lists the notifications, the newest first, 25 to a page:

| Notification | Reads |
|---|---|
| A reply | *ann replied to your comment on "Title"*, with the start of the reply |
| A comment | *ann commented on "Title"*, with the start of the comment |
| Several comments | *3 new comments on "Title"* and who wrote them: the unread comments of one content item are one line |
| New content | *New content with Sailing: "Title"* |

The unread ones are highlighted. Clicking a notification opens what it
tells of (the comment itself) and marks it read, with the other unread
comments of the same content. **Mark all as read** marks the rest.

The right column, *Subscriptions*, lists what the user follows (content
and terms, with the term's vocabulary) with a button to stop following
each, and a *Settings* link.

The user menu has a *Notifications* item with the number of unread
notifications, added to the menu's toggle; the admin header's user menu
has the same item. A theme that replaces the user menu template keeps
these through the menu's module hook (see
[Templates](../../themes/templates.md)).

Notifications are kept 90 days, then deleted.

## The user's settings

Each user decides on their profile's *Edit* page (`/me/edit`), in the
*Notifications* section:

| Setting | |
|---|---|
| *E-mails* | **At once** (the default): an e-mail of what happened, several things in a row in one e-mail; **Once a day**: a digest in the morning; **Never**: on the site only |
| *Replies to my comments* | on by default |
| *Subscribe me to the comments of content I create* | on by default |
| *Subscribe me to the comments of content I comment on* | on by default |

## The e-mails

The e-mails go out with the site's background jobs (see [Background
jobs](../../guides/content-types.md#background-jobs)):

- **At once**: every minute, one e-mail of what is waiting for the user;
- **Once a day**: a digest from 8:00 in the site's time zone, once a day;
- **Never**: nothing is sent, and nothing waits.

An e-mail of one thing has its own subject (*A new comment - Title*);
of several, *N new notifications*. Each item links to it through the
site (which marks it read), and, for a comment or new content, ends with
the link to unsubscribe from that content or term.

The unsubscribe link works without signing in: it opens a page that
asks first, and only its button unsubscribes (an e-mail client or a
scanner opening links unsubscribes nobody).

Blocked and pending accounts get no e-mails.

### The template

The e-mail is *Notifications* on **Configuration → Mail → Emails** (see
[Email](../../guides/mail.md)): its **Subject** and **Text**. It is
always sent - each user chooses how often in their profile. Besides the
placeholders of every e-mail (`[site:name]`, `[user:name]`,
`[user:edit-url]` and the others listed on its page; `[user:…]` is the
recipient), it has its own:

| Placeholder | Replaced with |
|---|---|
| `[subscriptions:subject]` | what the e-mail tells of: one thing's subject, or the number of notifications |
| `[subscriptions:list]` | the notifications, each with its links |
| `[subscriptions:url]` | the full address of the Notifications tab |

The built-in text, in the site's language, is:

```text
[subscriptions:subject] - [site:name]

Hello, [user:name]!

[subscriptions:list]

All your notifications: [subscriptions:url]
Turn the e-mails off or change how often they come in your profile: [user:edit-url]

-- [site:name] team
```

A blank **Subject** or **Text** means the built-in one: *Default text*
on its page brings it back.

### What sending needs

- A working mail transport on the *Mail* page (`/admin/config/mail`) or
  in the configuration, see [After
  installation](../../getting-started/first-steps.md#first-settings).
- The site's address: in Site information, or `public_base_url` in [the
  configuration](../../reference/configuration.md#the-server). The
  e-mails link to the site, so without an address they wait unsent.

## Deleting

Deleting a user deletes their subscriptions, notifications and
settings; in others' notifications they no longer appear as the author.
Deleting content, a comment or a term deletes the notifications of it,
and the subscriptions to the content or term.

## Disabling and uninstalling

While the module is disabled, its buttons, the *Notifications* tab and
its item in the user menu, the settings and all its pages are gone; no
notifications are made and no e-mails are sent. The subscriptions and
notifications stay in the database.

Uninstalling (see [Using modules](../using-modules.md#uninstall-a-module))
deletes every subscription and notification, the e-mail template and the
users' settings. It can't be undone.

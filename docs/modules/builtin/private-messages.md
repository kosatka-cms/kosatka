---
title: Private messages
description: Conversations between signed-in users on the Messages tab of their profile, with an e-mail about each new message.
sidebar:
  order: 8
---

The *Private messages* module (`private_messages`) lets signed-in users
write to each other. A message starts a conversation with a subject and
one or more recipients; everyone in it can reply. Conversations live on
the *Messages* tab of the user's profile (`/me/messages`), the user menu
shows how many have news, and each new message is e-mailed to the other
members.

It is one of the standard modules: a new site gets it installed and
enabled unless `app install -modules` leaves it out. A site without it
enables it on the **Modules** page (see [Using modules](../using-modules.md)).
It has no permissions and no blocks: once it is on, every signed-in user
can use it.

## The Messages tab

A signed-in user opens their conversations from *Messages* in the user
menu, or from the *Messages* tab of their profile: `/me/messages` (the
old address `/user/messages` redirects there). The tab's title is
*Private messages*, with a **Write a message** button.

| Part | What it does |
|---|---|
| *Active* / *Archived* | two tabs: the conversations in use (the default) and the archived ones |
| *Participant* | type at least three characters of a name and pick a user, then **Apply**: only conversations with that person. **Reset filter** (or ×) shows all again |
| The list | *Subject* (with the start of the last message under it), *Participants* (each a link to the profile) and *Updated*, the latest first, 25 to a page |
| *New* | a conversation with messages the user hasn't read yet is shown in bold with a *New* badge |
| The checkboxes | tick conversations (or *Select all*) and an **Archive** button appears; on the *Archived* tab it is **Unarchive** |

With no conversations, the tab says *You have no messages yet* and offers
**Write your first message**.

### Writing a message

**Write a message** opens *New message* (`/me/messages/new`):

| Field | |
|---|---|
| Recipients | type at least three characters of a username and pick from the suggestions (active accounts, by name only); add recipients one at a time, × removes one. Several recipients make a group conversation |
| Subject | required, up to 255 characters |
| Message | required. The text is plain; HTML in it is cleaned to safe formatting |

**Send** creates the conversation, with the author as one of its members,
and opens it. The members are fixed when the conversation starts: no one
can be added later.

A *Send private message* link opens the same form with the recipient
already filled in. The engine shows it next to the author's name on a
forum topic (when the [Forum](https://github.com/kosatka-cms/kosatka-module-forum) module is on), to signed-in
users other than the author.

Any signed-in user can write to any active account on the site; a
blocked or pending account can't be a recipient. There is no setting
that limits who may message whom. A message goes to 20 recipients at
most, and a user sends ten messages a minute at most (new conversations
and replies, a few at once): each recipient is mailed, and a
conversation isn't a mailing list.

### A conversation

A conversation (`/me/messages/thread/<number>`) shows its subject, the
*Participants*, and the messages from the oldest, each with its author's
picture and name and the date, 20 to a page. Opening it marks it read.
At the bottom are the reply box (**Send**) and **Archive conversation**.
*← Back* returns to the list.

Only the conversation's members can open it or reply; anyone else gets
"access denied". The page is not indexed by search engines.

### Archiving

Archiving hides a conversation from the user's *Active* tab, for that
user only: the others still see it as before. An archived conversation
can still be opened and answered from the *Archived* tab. To bring it
back, tick it there and click **Unarchive**.

A new message in a conversation the user archived leaves it archived: it
gets the *New* badge on the *Archived* tab, but it is not counted in the
user menu and no e-mail is sent about it.

### The unread count

The user menu has a *Messages* item with the number of conversations
that have unread messages (archived ones not counted), and the number is
added to the menu's toggle. The admin header's user menu has the same
item. A theme that replaces the user menu template keeps these through
the menu's module hook (see [Templates](../../themes/templates.md)).

## What administrators see

A user with the *Administer users* permission (see [Users and
permissions](../../guides/users-and-permissions.md#permissions)) also
sees the *Messages* tab on other users' profiles
(`/users/<name>/messages`): the list of that user's conversations
(*Active* and *Archived*, with the participant filter) with their
subjects, participants and the start of the last message. It is a list
only: no unread badge, no archiving, no writing, and the conversations
themselves open only for their members - an administrator who isn't one
gets "access denied".

On the **Modules** page, *What the module includes* counts the module's
*Conversations* and *Messages*.

## The new message e-mail

When someone writes a new conversation or replies, every other member
gets an e-mail, except:

- the author of the message;
- members who archived the conversation;
- members who turned the e-mail off in their profile;
- blocked and pending accounts.

The e-mails go out in the background, after the message is saved.

### The template

The e-mail is *New private message notification* on **Configuration →
Mail → Emails** (see [Email](../../guides/mail.md)): its **Subject** and
**Text**, and a switch that stops it for everyone. Besides the
placeholders of every e-mail (`[site:name]`, `[user:name]`,
`[user:edit-url]` and the others listed on its page; `[user:…]` is the
recipient), it has its own:

| Placeholder | Replaced with |
|---|---|
| `[pm:sender]` | the username of the message's author |
| `[pm:subject]` | the conversation's subject |
| `[pm:thread-url]` | the full address of the conversation |

The built-in text, in the site's language, is:

```text
New private message from [pm:sender] - [site:name]

Hello, [user:name]!

[pm:sender] has sent you a private message at [site:name].

Subject: [pm:subject]

To read the message and reply, follow the link:
[pm:thread-url]

If you don't want to receive these notifications, turn them off in your profile:
[user:edit-url]

-- [site:name] team
```

A blank **Subject** or **Text** means the built-in one: *Default text*
on its page brings it back.

### The user's switch

Each user decides on their profile's *Edit* page (`/me/edit`), with
*E-mail me about new private messages*. It is on until the user turns it
off.

### What sending needs

- A working mail transport on the *Mail* page (`/admin/config/mail`) or
  in the configuration, see [After
  installation](../../getting-started/first-steps.md#first-settings).
- The site's address: in Site information, or `public_base_url` in [the
  configuration](../../reference/configuration.md#the-server). The
  e-mail links to the conversation, so without an address no e-mail is
  sent at all.

## Deleting a user

In the user list (**People**), *Delete the account and all its content*
also deletes the user's private messages: the confirmation counts them
(*private messages: N*). Their messages and their place in each
conversation go; a conversation left with no messages is deleted, and
the other members' messages stay.

*Delete the account, keep the content* removes the user from their
conversations; the messages they wrote stay, without an author name.

## Disabling and uninstalling

While the module is disabled, the *Messages* tab, its item in the user
menu, the *Send private message* links and all `/me/messages` pages are
gone, and no e-mails are sent. The conversations stay in the database.

Uninstalling (see [Using modules](../using-modules.md#uninstall-a-module))
deletes every conversation and message, the e-mail template and each
user's e-mail switch, and removes the menu links to `/me/messages`. It
can't be undone.

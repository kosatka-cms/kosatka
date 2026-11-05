---
title: Polls
description: The Poll content type with voting by signed-in users, its results and vote log, and a Poll block that shows a chosen poll anywhere on the site.
sidebar:
  order: 2
---

The **Polls** module (`polls`) adds polls: a question with answer
options that signed-in users vote on. Each poll is a content item with
its own page, comments and results; a *Poll* block puts a chosen poll in
any region of the site.

A new site doesn't get it by default: enable it on the **Modules** page
(`/admin/config/modules`) or pass `-modules polls` (with the other
modules you want) to `app install`. See [Using modules](../using-modules.md).

## What it adds

| What | Where |
|---|---|
| The *Poll* content type | created from the *Create content* menu or at `/create/poll` |
| A poll's page | its own address, `/poll/<question>` by default |
| The vote log of a poll | `/node/<id>/poll/votes`, the *Votes* link on the poll's page |
| The *Poll* block | **Structure → Blocks**, *Add system block* |
| The *View poll votes* permission | **People → Permissions**, the general group |

The module has no settings page and no admin pages of its own. Polls are
listed with the rest of the content at `/admin/content` (filter by type
*Poll*).

## Permissions

| Permission | What it allows |
|---|---|
| *View poll votes* | the vote log of every poll: who voted for what and when |
| *Poll*: Create | creating polls; *Poll* appears in the user's *Create content* menu |
| *Poll*: Edit own, Edit any, Delete any | editing and deleting polls, as for any content type |

The *Poll* group is the usual set of per-type permissions
(`create poll content` and so on); see
[Users and permissions](../../guides/users-and-permissions.md#permissions).
Voting needs no permission: every signed-in user can vote. Guests see
the results and a *Log in to take part in the poll.* link.

## Create a poll

1. Open *Create content* → *Poll* (`/create/poll`).
2. Fill in the form:

| Field | What it does |
|---|---|
| *Question (title)* | the poll's question, required. It is the page heading and the block's heading |
| *Description* | optional text under the question. It accepts HTML, cleaned on save; there is no rich text editor here |
| *Maximum answers per participant* | 1 (the default) gives radio buttons: one answer. More gives checkboxes and the hint *Choose up to N options.* |
| *Available from (date and time, site time)* | before this moment the poll shows *The poll has not started yet.* and no vote form. Empty - open at once |
| *Available until (site time)* | after this moment voting stops and the poll shows *The poll has ended.*; the results stay. Empty - no end |
| *Results only (closed poll: no voting form)* | nobody can vote; the page shows the results and *Mode: results only.* |
| *Allow revoting (changing the selected options)* | on (the default): a voter can come back, change their choice and vote again. Off: one vote per user, then *You have already voted in this poll.* |
| *Answer options* | at least two. *Add option* adds a row, *Delete* removes one (not below two). Options are shown in this order |

3. Editors with the *administer content* permission also get the usual
   settings in the form's side column: *Publishing* (*Published* / *In
   moderation*), *Address* (*Slug (URL)*; empty - made from the
   question), *Meta tags*, *Comments* and *Author* (see
   [Content types](../../guides/content-types.md#creating-and-editing-content)).
   A poll has no URL pattern, no schedule and no front page or sticky
   flags. A poll created by an editor without *administer content* is
   published at once.
4. Press *Create*. You land on the poll's page.

The dates are entered in the site's time zone, or in the editor's own
when users may set one ([Account
settings](../../guides/users-and-permissions.md#account-settings)).
Voting is open only while the poll is published, not set to *Results
only* and inside its dates.

### Edit a poll

Open the poll and choose *Edit* (`/node/<id>/edit`). Every field can be
changed at any time, also after people have voted:

- Renaming an option keeps its votes.
- An option that already has votes can't be removed: it stays in the
  poll after saving. Options without votes are removed.
- Lowering *Maximum answers per participant* or turning *Allow
  revoting* off doesn't touch votes already cast; it applies to the next
  vote.

To close a poll early, set *Available until* to a past moment or tick
*Results only*.

## Voting and results

The poll's page shows the question, its author and date, the
description, the results, the vote form and the comments (unless its
comments are hidden).

The results are always visible, to everyone, before and after voting:
each option with its share and count, and the total (*Results (votes:
N)*). The total counts answers, not people: with several answers per
participant, a voter who ticks three options adds three.

A signed-in user picks options and presses *Vote*. The results update
without reloading the page, and their choice stays ticked. When
revoting is allowed, the form stays, showing their current choice;
voting again replaces it. Without JavaScript the form still works and
reloads the page.

A vote is refused with a message when no option is chosen, or more than
allowed (*Choose an answer.*), when the poll is closed (*You can't vote
in this poll.*) or after a vote that can't be changed (*You have already
voted in this poll.*). Votes from one IP address are rate-limited (about
30 a minute), which matters only when many people vote from one network.

## The vote log

Users with *View poll votes* (administrators always) see a *Votes* link
among the poll's actions. It opens the *Vote log*
(`/node/<id>/poll/votes`): a table of *Visitor*, *Date* (newest first)
and *Option*, one row per chosen option. *Back to poll* returns to the
poll. Without the permission the address answers "access denied"
(guests are sent to sign in).

## The Poll block

The block shows one poll anywhere on the site: its question linking to
its page, the results and the vote form, which works as on the page.

1. Open **Structure → Blocks** (`/admin/config/blocks`).
2. Under *Add system block* choose *Poll* and a region, and press *Add*.
3. The block's page opens. Choose the poll in *Shows*: the site's polls
   by question, newest first. Until one is chosen, the block shows
   nothing.
4. Set its title, roles and visibility as for any block
   ([Blocks](../../guides/blocks.md#visibility)), and press *Save*.

The block loads the poll separately from the page, so each visitor
sees their own vote even on a cached page. An unpublished or deleted
poll isn't shown in the block (except to users who may see unpublished
content).

A text can show a poll the same way with the tag `[[poll:<id>]]`, the
poll's content ID ([Embeds](../../guides/embeds.md)).

A site installed with `-demo` and this module gets a demo poll with
votes and comments, its block in the sidebar of the front page and a
link to it in the footer menu.

## Disable and uninstall

While the module is disabled, polls don't open, the *Poll* block isn't
shown and the permissions are hidden; the polls and votes are kept.

Uninstalling deletes every poll (with its comments and the files only
it used), all votes, the *Poll* block's placements, and *View poll
votes* and the *Poll* type's permissions in roles. See [Using
modules](../using-modules.md#uninstall-a-module).

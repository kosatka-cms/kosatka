---
title: Comments and spam
description: Who may comment, how a page loads its comments, comments waiting for approval, spam, and the settings that decide what happens to a suspicious comment.
sidebar:
  order: 10
---

Signed-in users comment where a content item's comments are open (each
item's comment mode, its type's default: see [Content
types](./content-types.md#type-settings)). Guests are asked to sign in
or register.

## How a page loads its comments

**Configuration → Comments** (`/admin/config/comments`), *Load comments*:

- *With the page* (a new site's choice): the first page of comments is
  in the content page's HTML. Search engines read them, they show
  without JavaScript, and a link to a comment (`?page=N#comment-ID`)
  opens it at once.
- *When scrolled to (AJAX)*: the page comes without them, and they load
  once the visitor scrolls to the comments. The page is lighter; behind
  a CDN that keeps guests' pages, new comments show at once instead of
  after the cache expires.

Either way, the comments' pager loads the next page in place.

## What happens to a new comment

A new comment goes through the site's checks, in order; the first one
that objects decides:

- **published** - the comment is on the site at once;
- **waiting for approval** - only its author sees it, marked *waiting
  for approval*, until someone approves it;
- a check may also refuse it outright: the author is told why.

Checks are set under **Configuration → Comments**
(`/admin/config/comments`, for roles that *Administer comments*):

| Setting | What it does |
|---|---|
| Bot trap | The comment form carries a field hidden from people, which bots fill, and the time it was shown: a form with the field filled, or sent sooner than a person could (3 seconds by default; 2, 5 or 10 to choose), is refused. People don't see it. The same trap guards registration and the web forms of the *Forms* module. On by default. |
| The same comment again | The same text by the same author once more within an hour, a day (the default) or a week is refused. Can be switched off. |
| New authors | An author's first comments wait for approval - the first one (the default), the first three, every one, or none. Once that many of theirs are published, their comments are published at once. A role with *Comment without approval* is never checked. |
| Links | A comment with more web addresses - in links or typed - than allowed waits (more than 2 by default; *no limit* turns it off). |
| Stop words | One per line, case aside, in the text and in the links' addresses; a part of a word counts (*casino* catches *casinos*). A comment with one waits for approval or is refused, as chosen. None by default. |
| Delete spam | Spam is deleted with its replies 7, 30 (the default) or 90 days after it was marked, or never. |

A **CAPTCHA** can be shown on the comment form too, to authors without a
published comment yet (those with one aren't bothered): tick *Comment
form* on the [CAPTCHA](./users-and-permissions.md#captcha) page; the
comment settings show whether it is on.

The refusals run first - the bot trap, then the same comment again -
and then the checks that hold a comment, in this order: stop words,
links, new authors. So the reason a comment waits is the most telling
one: a stop word before its links, its links before a new author.

The default holds a new author's first comment: a spammer's account has
none published, a regular commenter is checked once.

An author edits their comment while its content takes comments, and
the edited text goes through the same checks: refused, or waiting for
approval again. Someone who *Administers comments* edits any comment,
unchecked.

## Approving and spam

**Content → Comments** (`/admin/content/comment`) has a tab per status
with its count - *Published*, *Waiting for approval* and *Spam* - and
opens on the waiting ones when there are any; the number by its menu
item is how many wait. A waiting comment shows why it waits (*new
author*, *3 links*, *stop word "casino"*, *3 complaints* - hidden by the
[Complaints](../modules/builtin/complaints.md) module, …) and how many
published comments its author has. The actions below take *Administer
comments*.

- **Approve** publishes it: subscribers are told of it as of any new
  comment.
- **Spam** hides it from everyone, in the *Spam* tab; *Not spam* there
  publishes it. Marking several at once offers to **block their
  authors** too (not those who administer comments, nor yourself): a
  blocked user can't sign in, their other comments stay.
- **Delete** removes a comment with its replies.

Each row has *Approve* and *Spam* of its own; the checked rows get them,
and *Delete*, in bulk, after a confirmation page. Approving, marking
spam and deleting are in the [action
log](../modules/builtin/action-log.md).

Comments waiting for approval and spam are left out everywhere the site
shows comments: the content's comment list, the recent comments, the
counts, the forum, and [comment Views](./views.md#comment-views) that
don't filter by status themselves.

---
title: Action log
description: Who did what on the site - content deleted or unpublished, users blocked, roles' permissions and settings changed, sign-ins - with filters, and kept as long as you choose.
sidebar:
  order: 12
---

The *Action log* module (`action_log`) answers "who did this?": who
deleted a content item, unpublished it, blocked a user, gave a role a
permission, changed a setting, and who signed in. Each entry has the
time, who did it and from which address, the action, what it was done to
and, where it applies, what changed - what was and what is.

A new site gets the module installed and enabled (it is one of the
standard modules of `app install`). On a site created before the engine
had it, it starts disabled: enable it on the **Modules** page (see
[Using modules](../using-modules.md)). The log starts with the actions
done after that; nothing earlier is in it.

## Who sees it

The log tells who acted from which address, so it has a permission of
its own: *View the action log*. Give it to the roles that should see it
(an administrator with full access has it anyway). The same permission
opens the log's settings.

## What it records

| Group | Actions |
|---|---|
| Content | created, moved to the trash, restored from it, deleted for good, published, unpublished (moderation included), its author changed, its address changed (with the old and the new one) |
| Comments | deleted (with the replies to it), approved (published after waiting for approval), marked as spam |
| Users, roles and permissions | a user created by an administrator, blocked, unblocked, activated (a registration approved), deleted, given another role; their account changed on the profile form, by them or an administrator - the name and the e-mail (what they were and became), the time zone, the password, the signature, the picture, the profile fields; a password reset by e-mail; two-factor sign-in turned on or off by the user or reset by an administrator; a role created, renamed, deleted; a role's permissions changed - each permission granted or taken away |
| Settings and structure | a module enabled, disabled, uninstalled; any configuration page saved - content types and fields, Views, blocks, menus, text formats, the site's settings - named by its item in the admin menu |
| Sign-ins | the sign-ins of users with access to the admin area; failed attempts under any name, with the reason (no such user, a wrong password, an account blocked, awaiting approval or with its e-mail not confirmed, a wrong two-factor code) |

Edits of a content item's text and fields aren't in the log: they are in
its history (*History* on the item; see
[History of changes](../../guides/content-types.md#history-of-changes)).
Content of the modules' types - forms, polls - is in the
*Content* group like any other: created, published, unpublished, deleted,
a new author or address.

Other modules add their own groups of actions; their switches appear on
the log's settings while the module is on:

| Group | Actions |
|---|---|
| Forms | a form's submissions deleted (how many) |

Disabling the *Action log* module itself is recorded (it is still on
when the change is saved).

## Reading the log

**Reports → Action log** (`/admin/reports/action-log`) lists the
entries, the newest first, 50 to a page. The filter narrows it by who
(a part of their name), the group of actions, the object (a part of its
name, or its ID) and the dates (*From*, *To*).

- **Who** links to all of that user's actions.
- **The object** links to it while it exists; a deleted one keeps the
  name it had.
- **What changed** opens a table of the fields, what was and what is.

The admin's Overview (`/admin`) shows the log's five newest entries
under *Recent actions*, to those who may see the log.

Two links lead to a filtered log: a content item's form has an *Action
log* line in its side column (how many entries there are about it), and
the users list has a log icon beside each user's edit icon - what they did.

## Settings

**Reports → Action log → Settings** (`/admin/reports/action-log/settings`):

| Setting | |
|---|---|
| *Keep entries* | 30, 90 (the default), 180 days, a year, or forever. A background job deletes older entries once an hour |
| *Keep the IP address* | On by default. The address helps to see where a hijacked account acted from; it is personal data, so mention it in the site's privacy policy |
| *What to record* | A switch per group. A group turned off isn't recorded from then on; what was recorded stays until it gets old |

## For module authors

A module records its own actions through the SDK: it registers a group
and its action types and calls `RecordAction` where the action is done -
see [The module SDK](../sdk.md#hooks).

## Uninstalling

Uninstalling the module deletes its tables with all the entries.

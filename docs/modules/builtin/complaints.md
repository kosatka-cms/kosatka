---
title: Complaints
description: Signed-in visitors report content and comments (spam, insults, broken rules); moderators decide under Content → Complaints, and enough complaints hide the item until they do.
sidebar:
  order: 16
---

The *Complaints* module (`complaints`) lets visitors tell the
moderators about content and comments that break the site's rules. A
*Report* link sits under each comment and under content of the types you
choose; the complaint goes to the **Content → Complaints** queue, and
the first complaint about each content item or comment is e-mailed to
the site's address. When several people complain about the same thing,
it is hidden until a moderator decides.

The module is off on a new site. Turn it on on the **Modules** page (see
[Using modules](../using-modules.md)): its tables are created, and
every signed-in user gets the permission to complain.

## What a visitor does

1. A signed-in visitor presses *Report* under a comment, or *Report this
   content* under a content item. Their own comments and content have
   no link.
2. A window shows what they report (the comment's author and its first
   words, or the content's title), the reasons and a *Details* field.
3. They choose a reason and press *Send*. *Other* needs the details;
   with any other reason they are optional (up to 1000 characters).
4. *Thank you: the moderators will look at it.* appears briefly, and the
   link goes away.

Complaining twice about the same thing changes nothing: one person's
complaints count once. Only public content and published comments can
be reported. Guests don't see the links. The queue shows the reasons
and the notes, not who sent them, and the window tells the visitor that
the author won't know who complained.

## Permissions

| Permission | |
|---|---|
| *Report content and comments* | sends complaints. The module gives it to the *Signed-in user* role (`user`) when it is installed; take it away on **People → Permissions** to stop a role complaining |
| *Handle complaints* | the queue and the module's settings, together with *Access administration (/admin)* |

## The queue

**Content → Complaints** (`/admin/content/complaints`) has a row for
each content item or comment complained about, the latest complaint
first; the menu item shows how many wait. A row shows:

- what it is - *Comment* (its author, the content it is on and its
  first words) or *Content · type* (its title and author), linked to
  the page;
- the reasons with how many people gave each, the newest notes, when
  the latest complaint came;
- *Hidden by the complaints* when enough of them hid it;
- how many complaints there are.

The decisions take the row out of the queue:

| | Comment | Content |
|---|---|---|
| *Spam* | marked as spam (see [Comments](../../guides/comments.md#approving-and-spam)) | - |
| *Unpublish* | back to waiting for approval on **Content → Comments** | unpublished |
| *Delete* | deleted with its replies | moved to the trash (a module's type, such as a forum topic, has no trash: deleted for good) |
| *Keep* (*Show again* for a hidden one) | stays; a hidden one is published again | stays; a hidden one is published again |

*Block the author* opens the author's account, to block it there. A row
whose content or comment is gone already only has *Keep*. The
*Resolved* tab lists the decided ones with what was done, by whom and
when. A new complaint about something decided brings it back to the
queue; to hide it again, only the complaints after the decision count.
Deleting content removes its complaints and its comments' complaints.

## Settings

**Configuration → Complaints** (`/admin/config/complaints`):

| Setting | |
|---|---|
| *What may be reported* | *Comments* (on by default) and the content types whose items get *Report this content* (none by default) |
| *Reasons* | one per line; *Other*, with the details asked, is always added. Empty - *Spam or advertising*, *Insults*, *Breaks the site's rules* |
| *Hide automatically* | after 3 (the default), 5 or 10 complaints of different people, or never. A comment goes back to waiting for approval (the comments' queue gives *3 complaints*, say, as the reason), a content item is unpublished |
| *E-mail to the moderators* | a link to the e-mail's text |

### The e-mail

The first complaint about a content item or a comment - and the first
after a decision - is e-mailed to the address in **Site information**.
The e-mail is *New complaint* on **Configuration → Mail → Emails** (see
[Email](../../guides/mail.md)): its subject, text and a switch to stop
it. Besides the site's placeholders it has:

| Placeholder | Replaced with |
|---|---|
| `[complaint:about]` | what it is: *"Title"*, or *a comment by ann on "Title"* |
| `[complaint:reason]` | the reason |
| `[complaint:note]` | the details (a dash when none) |
| `[complaint:url]` | the page's full address |
| `[complaint:queue-url]` | the queue in the admin |

## Themes

The links are in the core's comment template (`comments.html`, the
`comment_links` hook) and under the content (`node_links`); the window
is added at the end of the page (`page_end`). A theme with its own
copies of these templates keeps the hooks (see
[Templates](../../themes/templates.md)). The module's classes start with
`complaints-`.

## Disabling and uninstalling

While the module is disabled, the links, the window, the queue and the
settings are gone; hidden content and comments stay hidden. Uninstalling
(see [Using modules](../using-modules.md)) deletes the complaints and
the settings (the `complaints`, `complaint_targets` and
`complaint_settings` tables) and the module's permissions.

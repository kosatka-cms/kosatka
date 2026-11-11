---
title: Users and permissions
description: Accounts, roles and what each may do, registration, two-factor sign-in, social login, CAPTCHA and moderation.
sidebar:
  order: 9
---

People on a Kosatka site are guests (not signed in) and users with an
account. What a user may do comes from roles: each role holds a set of
permissions, and each account has one role. Most of it is under
**Users** in the admin.

## Roles

A new site has two roles:

| Role | Machine name | What it is |
|---|---|---|
| Signed-in user | `user` | the base role. Every signed-in user has its permissions, whatever role the account has. A new site grants it only the *Plain text* format |
| Administrator | `admin` | full access: every permission, now and in the future. The first account gets it on install |

Guests have no permissions. They see published content and use the
public forms (sign-in, registration, password reset). In block
visibility conditions guests are the *Guest (not signed in)* role.

An account has one role of its own. Its permissions add to those of
the base role: a user whose role is *Editor* has the editor's
permissions plus the signed-in user's.

To add a role, open `/admin/users/roles` and fill in *Add role*:

| Field | |
|---|---|
| Name | what the admin shows, for example *Editor*. You can rename it later |
| Machine name | lowercase Latin letters, digits and `_`, starting with a letter, for example `editor`. Can't be changed later |

A new role has no permissions: grant them on the
[Permissions](#permissions) page. You can't delete the base role, the
role with full access, or a role that still has users (give them
another role first).

## Permissions

`/admin/users/permissions` is a table: a row per permission, a column
per role. Tick the boxes and press *Save permissions*. The full-access
role's column is always ticked and can't be changed.

**General** permissions:

| Permission | What it allows |
|---|---|
| Access administration (/admin) | opening the admin at all. Most admin pages need only this |
| Administer content: URL, meta, status and author in the form, "staff only" blocks | the publishing options, address, author and date in the content form; blocks shown to staff only |
| Administer users: accounts, user fields, account settings | the user list, creating and approving accounts, bulk actions, user fields, account settings, editing other users' profiles |
| Administer roles and permissions (lets a role grant itself any permission) | the Roles and Permissions pages. Give it only to people you trust with everything |
| Administer comments: edit and delete any | editing and deleting other people's comments, approving them, marking spam, the comment settings |
| Comment without approval (no new authors' check) | comments published at once, however new the author (see [Comments and spam](./comments.md)) |
| View unpublished content | unpublished content and disabled Views |
| View the content of every type, whoever its type is for | the content of types closed to other roles (a type's *Who may view*, see [Content types](./content-types.md#type-settings)) |
| Publish and unpublish | changing whether content is published |
| Moderation (moderation queue) | the *Pending moderation* list (`/admin/moderation`) |
| View the history of changes of content they may edit | a content item's *History* and its comparisons; see [History of changes](./content-types.md#history-of-changes) |
| Bring back an earlier version of content they may edit | bringing an earlier version of a content item back |
| Make copies of content they may edit (and create) | *Make a copy* among a content item's actions; see [Copying content](./content-types.md#copying-content) |
| Administer users' files: view and manage any user's files | the users' files in *Content → Users' files* |
| Upload documents to files (without it - images only) | uploading documents, not just images, to one's own files |
| Access the site in maintenance mode | using the site while it is in maintenance mode |
| Delete their own account (what happens to the content - account settings) | *Delete account* on the own profile's *Edit* tab (see [Deleting one's own account](#deleting-ones-own-account)); never for an administrator |

Enabled modules add their own permissions to this group (for example
*View poll votes* or *Rate content*); a disabled module's permissions
are hidden.

**Each content type** has its own group, including the types that come
with modules (such as forum topics or polls). A new type gets these
automatically:

| Permission | What it allows |
|---|---|
| Create | creating content of the type; the type appears in the user's *Create content* menu |
| Edit own | editing one's own content of the type, published or not (a draft); its author then sees it unpublished too. Publishing it takes another permission |
| Edit any | editing anyone's content of the type |
| Delete any | deleting content of the type |

**Text formats** has a row per format: a user may save a text field in
a format only with its permission. Give *Basic HTML* (or your own
formats) to the roles that should write formatted text.

### Who may manage whom

- Giving a role means giving its permissions, so the base role can be
  given by anyone with *Administer users*, any other role only by
  someone with *Administer roles and permissions*, and the full-access
  role only by a full-access user. A role you may not give, you may not
  take away either: the profile form doesn't offer it.
- An account - its profile, password, role, two-factor sign-in, its
  blocking and deletion - may be changed only by someone who may give
  its role: the base role's by anyone with *Administer users*, another
  role's only with *Administer roles and permissions* too, a full-access
  account only by a full-access user. Signing in as an account with
  more rights would give them away.
- A few pages are open to full-access users only, whatever the
  permissions say, because they hold secrets: *Mail*, *Social login*
  and *Spam protection (reCAPTCHA)*. On *Counters and codes* only the
  code fields, which run on every page, are theirs; others with access
  to the admin may set the counters. The *Site address* on *Site
  information* is theirs too: it is in the links the site mails, a
  password reset's among them. Likewise, only full-access users
  may use a text format that allows `<script>` or `<style>`.

## Managing accounts

`/admin/users` lists the accounts. Filter by status (*Active*,
*Pending approval*, *Blocked*) and role; sort by *Created* or *Last
access*. Pending accounts have an *Approve* button.

**Add user** (`/admin/users/create`) asks for the e-mail, the login,
how the password is set, the role and status, and the required profile
fields. The password:

- **Invite by email** (the default) - the person gets the *Invitation*
  e-mail with a link working for 7 days: by it they choose their login
  (the one typed, or one made of the address when left empty) and set
  their password, and are signed in. Until then the list shows them as
  *Invited*, with the link's last day, and *Send again* (a new link; the
  old one stops working) and *Withdraw* (the account, never used, is
  deleted). An expired link - *Invitation expired*: send it again.
- **Set a password now** - you type it (at least 6 characters -
  characters, not bytes) and tell it yourself; the person gets the
  *Welcome (new user created by an administrator)* e-mail with a
  one-time sign-in link.

With email off, or without the site's address (*Site information*), the
account is created but the invitation isn't sent: the list says so; fix
that and press *Send again*.

**Bulk actions** - tick accounts, press the action's button and confirm
on the next page:

| Action | |
|---|---|
| Delete the account, keep the content | the account goes, its content stays |
| Delete the account and all its content | also deletes its content, uploads, picture and files folder |
| Block | the user is signed out and can't sign in |
| Unblock | makes the account active again |

You can't block or delete your own account this way.

To change one account, open its profile and press *Edit*
(`/user/<id>/edit`). Under *Administrative settings* are its *Status*
(*Active*, *Blocked*, *Pending approval*) and *Roles*.

### Blocking

A blocked user is signed out on their next request and can't sign in
again. Their profile page answers "not found" to everyone but users
with *Administer users*, and a password reset sends nothing to them.
Their content stays on the site.

## Account settings

`/admin/config/account` (*Account settings*) decides how accounts are
created.

**Who can register accounts?**

| Option | |
|---|---|
| Administrators only | no public registration: accounts are created at `/admin/users/create`; the sign-in page and the user menu offer no registration, and `/user/register` says it is disabled |
| Visitors (default) | anyone can register at `/user/register` |
| Visitors, but administrator approval is required | the account waits as *Pending approval*; the site's e-mail (Site information) is told, and an administrator approves it at `/admin/users` |

A new site starts with *Visitors, but administrator approval is
required*.

Other settings:

| Setting | |
|---|---|
| Require email verification when a visitor creates an account | on - only accounts with a confirmed address can sign in: a visitor who registers gets a link (valid 24 hours) and signs in after following it; where approval is required, approving the account confirms its address instead. Off (the default) - they can sign in right away |
| Users may set their own time zone | adds a *Time zone* field to the profile; times are shown to the user in it |
| Block registration from email domains | one domain per line; blocks the domain and its subdomains. Applies to public registration (and to new accounts of social login), not to accounts an administrator creates |
| Avatar upload | the allowed extensions and the maximum size of profile pictures, instead of the [upload policy](./files-and-images.md#upload-policy)'s; empty - the policy's |
| When users delete their own account | *Keep their content without an author* (the default; their comments are deleted) or *Delete their content too*. See [Deleting one's own account](#deleting-ones-own-account) |

The texts of the account emails - the invitation, the welcomes, the
activation, the password recovery, the e-mail confirmation, the
administrators' notice of a new registration request - are on the
*Mail* page: see [Email](./mail.md).

## Profile fields

`/admin/users/fields` (*User fields*) adds your own fields to the
profile, for example *City* or *Website*. Each is a text field with:

| Field | |
|---|---|
| Name | the label on the profile and in its form |
| Machine name | lowercase Latin letters, digits and `_`, starting with a letter; can't be changed |
| Max length | 1 to 65535 characters (255 by default) |
| Weight | the order on the profile: lower comes first |
| Required | asked at registration and can't be left empty |

Deleting a field deletes its values for all users. Social login asks for
none of them: a user who signs in with Google, VK or Yandex while a
required field is empty is sent to fill it in first.

## Sign-in, passwords and profiles

- **Sign in** at `/login` with the e-mail or the username: a card in the
  middle of the page, without the sidebar; the eye in the password field
  shows what was typed. The social networks' buttons, when configured,
  stand side by side under it. The other sign-in pages - registration,
  the forgotten password and the new one, the two-factor code, logging
  out - are the same kind of card.
- **Forgotten password**: `/user/password` sends a reset link (valid 24
  hours) to the address, if an account has it. The page says the same
  whether it does or not. A password set by the link confirms the
  address.
- **A new e-mail address** in the profile isn't confirmed until its
  confirmation link is followed: where the site signs in confirmed
  addresses only, the link is mailed to it, and signing in waits for it.
- **Own profile**: `/me`; its *Edit* tab (`/me/edit`) changes the
  picture, username, time zone, signature and the profile fields
  (*Profile*), the e-mail and the password (*Sign-in and password*); an
  enabled module adds its own settings in the side column, under
  *Save*. Changing the e-mail or the password asks for the current
  password; the other details save without it.
- **Private messages** - the *Private messages* module (switched on and
  off on the Modules page): the *Messages* tab of the own profile,
  `/me/messages`; an administrator of users sees the list of
  conversations on others' profiles too, but opens only those they take
  part in. A link to it and the number of unread conversations are in
  the user menu. A new message is e-mailed to the other members, unless
  one turned *E-mail me about new private messages* off on their *Edit*
  page.
- **Notifications** - the *Subscriptions* module: the *Notifications*
  tab of the own profile, `/me/notifications`, with what the user follows
  (the comments of content, terms); new comments, replies and new
  content are told of there, in the user menu and by e-mail, as set in
  the *Notifications* section of the *Edit* page. See
  [Subscriptions](../modules/builtin/subscriptions.md).
- **Files**: `/me/files`, see
  [Files and images](./files-and-images.md#users-files).

### Deleting one's own account

By default only an administrator deletes accounts (*Users*, with the
user's content or keeping it without an author). A site where people
should be able to leave on their own grants their role the permission
*Delete their own account*. Their profile's *Edit* tab then ends with
*Delete account*: their password, a tick that it can't be undone, and a
last question. The account is blocked at once and deleted; the user is
signed out and lands on the sign-in page with *Your account is deleted*.

What stays is set in *Account settings*, *When users delete their own
account*:

- *Keep their content without an author* - the profile and the comments
  are deleted; the content, and the user's messages in other people's
  conversations, stay without an author.
- *Delete their content too* - the content, the comments, the private
  messages, the uploaded files, the picture and the files folder go too,
  in the background, as with an administrator's *delete with content*.

An administrator's account (the role with full access) is never deleted
by its owner: another administrator deletes it. Someone who signed up
with Google, VK or Yandex has no password of their own: they set one
with *Forgot your password?* first.

## Two-factor sign-in

With two-factor sign-in on, signing in takes a code from an
authenticator app on the phone as well as the password (Google
Authenticator, Microsoft Authenticator, Yandex Key, 1Password,
Bitwarden - any app for time-based codes). A leaked password alone no
longer opens the account.

**Turning it on.** *Edit* in the profile → *Sign-in and password* →
*Two-factor sign-in* → *Turn on* (`/me/two-factor`): scan the QR code
with the app (or type the key it shows), then enter the six-digit code
the app shows. The page then shows ten **recovery codes** once - each
signs in once instead of a code from the app, when the phone is lost.
Keep them apart from the phone: download, copy or print them. Adding the
same key to a password manager as well keeps a lost phone from locking
you out.

**Signing in.** After the right password the site asks for the code
(`/login/code`); *No access to the app?* takes a recovery code instead.
A code is taken once; after five wrong codes, or ten minutes, the
sign-in starts over. The code is asked for on every way in: the
password, the one-time sign-in link of a welcome or activation e-mail,
the e-mail confirmation link and social login (a password reset signs
nobody in: the new password is used at `/login`). Failed codes are in
the [action log](../modules/builtin/action-log.md) as failed sign-ins.

**On its page** while it's on: since when, how many recovery codes are
left, *New recovery codes* (the old ones stop working) and *Turn off* -
each asks for a code from the app.

**Required for roles.** **Account settings** (`/admin/config/account`),
*Two-factor sign-in*: tick the roles whose users must use it -
*Signed-in user* makes it every account's, whatever its own role. Such
a user without it is sent to turn it on after
signing in and can't open anything else until they do; so is a user the
site can't check (the database is down, say). They can't turn it off.
The users list marks who has it on (*2FA*) and who must but hasn't.
*The site's name in the app* - how the app names the site (empty - the
site's name).

**Lost the phone and the codes.** An administrator of users opens the
user's form (`/user/<id>/edit`) and clicks *Reset two-factor sign-in*:
it's turned off (the action log records it), and the user signs in with
the password and turns it on again. Without anyone to do that - the only
administrator lost theirs - reset it in the database, with access to the
server:

```sql
DELETE FROM user_recovery_codes WHERE user_id = (SELECT id FROM users WHERE username = 'name');
DELETE FROM user_two_factor WHERE user_id = (SELECT id FROM users WHERE username = 'name');
```

The authenticator's key is kept in the database as it is (the recovery
codes only as hashes), as the app keeps it: protect the database's
backups like the site's passwords.

## Social login

`/admin/config/oauth` (*Social login*, full access only) lets visitors
sign in and register with an account on Google, VK or Yandex.

1. Open the provider's console (the *Provider's console* link under
   each provider) and register an app for your site.
2. Give it the *Redirect URI* shown on the page. It is built from the
   site address in *Site information*, for example
   `https://www.example.com/auth/google/callback`, so set the address
   first.
3. Copy the app's *Client ID* and *Client secret* into the form, keep
   *Enabled* on and press *Save*.
4. Check the sign-in in a private browser window.

The provider then shows on the sign-in and registration pages. A saved
secret is never shown again; leave the field empty to keep it, or tick
*Delete the saved keys*. Switching a provider off keeps the keys; its
users then sign in with a password (or reset it).

How the accounts come together:

- A provider account connected to an account of the site signs in to
  it.
- A signed-in user connects a provider in the profile (*Edit* →
  *Sign-in and password* → *Social networks* → *Connect*), and
  disconnects it there - unless the account has no e-mail address of
  its own (VK may give none), as it would have no way in left. A
  provider account connected to another account of the site isn't taken
  over, and one provider connects one account of it.
- A visitor whose provider account has the e-mail of an existing account
  isn't signed in to it by the address, whatever the provider says of
  it: whoever registered the address here first would get the
  provider's sign-ins. They sign in with the password and connect the
  provider in the profile.
- Otherwise a new account is made, following the registration settings:
  none with *Administrators only*, a pending one with approval, none
  from a blocked e-mail domain. The new account is connected to the
  provider at once, a pending one too: until it is approved, signing in
  with the provider shows the "awaiting approval" page; once approved,
  it signs the user in. Its address counts as confirmed when the
  provider vouches for it (Google and Yandex do; VK doesn't say). Where
  the site signs in confirmed addresses only, an account with an address
  the provider doesn't vouch for gets the confirmation link first; when
  the provider gives no address, no account is made, and the sign-in
  page asks the visitor to register with the form and connect the
  provider afterwards.

The keys can also come from the server's configuration (see [the
configuration](../reference/configuration.md#captcha-and-oauth)): a
provider's keys there win over the ones saved in the admin, and its
section offers only the switch.

## CAPTCHA

`/admin/config/captcha` (*Spam protection (reCAPTCHA)*, full access
only) adds Google reCAPTCHA v2, the "I'm not a robot" checkbox, to the
chosen forms.

1. In the [reCAPTCHA admin console](https://www.google.com/recaptcha/admin)
   create keys of type v2, "I'm not a robot", for your domain.
2. Enter the *Site key* and *Secret key*; keep *Enable reCAPTCHA* on (it
   is by default).
3. Under *Forms*, tick where it appears: the login form, registration
   and password reset (these three are on by default), the comment form
   (only for authors without a published comment), and forms of enabled
   modules.

Without keys no form shows the CAPTCHA. Keys can also come from the
server configuration, like the OAuth ones: they win over the ones saved
here, and the page doesn't offer the key fields.

Registration is guarded by the bot trap too, without anyone noticing: a
field only bots fill and the time the form took (see [Comments and
spam](./comments.md)).

## Moderation

**Content.** Each content type sets what happens to new content with
*Publishing* under *New content* in its settings (see [Content
types](./content-types.md#type-settings)):

| New content | |
|---|---|
| Published at once | everyone sees it |
| On moderation - its author sees it, the others after approval | it waits in the moderation queue |
| Unpublished | only those who may view unpublished content see it |

An author without *Administer content* gets the type's default and
can't change it. Content waiting for approval is listed at
`/admin/moderation` (*Pending moderation*), open to roles with
*Moderation*. To approve an item, open it and clear *In moderation* in
its publishing options (this needs *Administer content*), or tick items
in the queue and press *Remove from moderation*. The queue's bulk
actions are open to *Moderation* alone; *Publish* and *Unpublish* show
for who may publish, and deleting takes *Delete any* of the item's
type.

Content that isn't public - unpublished, or waiting on moderation - is
left out wherever the site shows content, not just its page: its
comments and the comment lists, the forum's lists and counts, the
title search of the editors' forms and the blocks that embed it (a
poll, a form, galleries) show only public content. Those who may see
it (its author while it waits or while they may edit it, those who may
view unpublished content) open it by its page; moderators find it in
the queue.

**Comments.** Signed-in users can comment where a content item's
comments are open; authors can edit their own. A new author's first
comments wait for approval unless their role may *Comment without
approval*. *Administer comments* lets a role edit, delete, approve and
mark as spam any comment, and set the checks - see [Comments and
spam](./comments.md).

**Bulk actions** on content and comments ask the same permissions as
the single ones, not only *Access administration*: deleting an item
takes *Delete any* of its type (the others are reported and left),
publishing and unpublishing *Publish and unpublish* or *Administer
content*, putting into and out of moderation *Administer content* or
*Moderation*, every action on comments *Administer comments*.

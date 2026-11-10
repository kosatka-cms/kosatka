---
title: Forms
description: Build forms with your own questions, collect the submissions, get them by e-mail and export them as CSV.
sidebar:
  order: 7
---

The **Forms** module (`forms`) lets editors build forms with their own
questions - feedback, applications, questionnaires - without touching
code. Each form is a piece of content with its own page; visitors fill it
in, and the answers are kept as *submissions* you can read, analyse,
export and delete.

A new site gets the module installed and enabled (it is one of the
standard modules of `app install`). On a site created before the engine
had it, it starts disabled: enable it on the **Modules** page (see
[Using modules](../using-modules.md)).

## What it adds

- **The *Form* content type.** A form is created from the *Create
  content* menu (`/create/form`) and lives at its own address, by
  default `/form/<title>` (for example `/form/contact-us`). It is edited
  and deleted like other content. Its type isn't set up under **Content
  types**: everything about a form is in its own editor.
- **Content → Forms** (`/admin/forms`): every form with its number of
  submissions and the time of the last one, and links to *Submissions*
  and, once it has some, *Analysis* and *CSV*; the pencil (*Edit*), and
  the bin (*Delete*) for a form you may delete: it asks first, then
  deletes the form with its submissions and their files and comes back
  to the list. A *Create form* button is shown to those who may create
  forms.
- **The *Form* block**, which shows a chosen form anywhere on the site.
- **Permissions** (**Users → Permissions**, see
  [Users and permissions](../../guides/users-and-permissions.md#permissions)):

| Permission | What it allows |
|---|---|
| *View and export form submissions* | the **Content → Forms** page, a form's submissions, analysis, CSV export and the files sent with it |
| *Delete form submissions* | deleting submissions (with the first permission, which opens the pages) |
| *Form*: *Create*, *Edit own*, *Edit any*, *Delete any* | creating, editing and deleting forms themselves |

## Create a form

1. Open **Content → Forms** and press *Create form* (or choose *Form* in
   the *Create content* menu).
2. Enter the **Title** and, if you like, the **Text above the form**
   (plain text; line breaks are kept).
3. Under **Questions**, fill in the first question and choose its type
   (see below). Press *Add question* for each next one. The ↑ and ↓
   buttons move a question, × removes it.
4. Set the rest of the form (all optional):

| Field | What it does |
|---|---|
| *Submit button text* | the button's label. Empty - *Send* |
| *Message after sending* | shown on the form's page after a submission (plain text). Empty - *Thank you, the form has been sent.* A *Send once more* link follows it |
| *Page after sending* | a path on this site (starting with `/`, for example `/thanks`) to open after sending instead of the message |
| *Notify of submissions* | e-mail addresses, separated by commas, that get every submission (see [E-mail notifications](#e-mail-notifications)) |
| *Limits* | see [Limits and closing](#limits-and-closing) |

5. Press *Create* (*Save* when you edit a form). The form's page opens.

Editors with *Administer content* also see the usual content settings
in the form's side column: *Publishing*, *Address* (empty - made from
the title), *Meta tags*, *Comments* (a new form starts with comments
hidden) and *Author*. A form created by someone without that permission
is published at once, with comments hidden.

A form needs a title and at least one question. The editor refuses to
save a question without its text or type, a choice question without
options, an invalid address in *Notify of submissions*, or a *Page after
sending* that isn't a path on the site, and lists what to fix.

*Preview* under *Create* / *Save* opens the form in a new tab as
visitors will see it, before saving: the questions in order, the hints,
the required marks, *Show only if* and *Fill from the address
parameter* working. Its send button is off; nothing is saved, and the
editor stays as it was in its own tab. What saving would refuse is
listed over the page (see [Preview before
saving](../../guides/content-types.md#preview-before-saving)).

## Question types

Each question has its text (*Question*), its type and a *Required*
checkbox; the arrows move it up or down, the cross deletes it. Some
types have settings of their own, shown when you choose the type. The
rarer settings are under **More**: an optional *Hint under the
question*, [Show only if](#show-a-question-only-sometimes) and [Fill
from the address parameter](#fill-a-question-from-the-address). *More*
opens by itself on a question where one of them is set.

| Type | The visitor sees | Settings and checks |
|---|---|---|
| *Single line* | a one-line text field | - |
| *Text* | a multi-line text area | - |
| *E-mail* | an e-mail field | must be a valid address |
| *Drop-down list* | a list with *- choose -* first | *Options, one per line* |
| *One of the options* | radio buttons | *Options, one per line* |
| *Several of the options* | checkboxes | *Options, one per line*; any number may be ticked |
| *Consent (a checkbox to tick)* | a checkbox that must be ticked to send the form | always required; an optional *Link to the privacy policy* (`/privacy` or a full `https://…` address), shown as a *privacy policy* link next to it |
| *File* | a file upload | *Allowed extensions* (see [Files](#files-sent-with-forms)) |
| *Number* | a number field (a decimal comma is accepted) | *Minimum* and *Maximum*, each optional |
| *Date* | a date picker | a real date |
| *Phone* | a phone field | digits with `+ ( ) - .` and spaces, 5 to 15 digits |

An answer may be up to 10,000 characters. A choice must be one of the
options. When a check fails, the form is shown again with what the
visitor entered and a message under the question.

### Show a question only sometimes

From the second question on, a question has **Show only if** (under
*More*): choose an
earlier question and, in *has the answer*, the answer that shows this
one (for a choice question, the option's text exactly). Leave *has the
answer* empty to show the question once the earlier one has any answer.
For *Several of the options*, the question shows when that option is
among the ticked ones.

While a question is hidden it is not required, not checked and not
stored, and the questions that depend on it are hidden too. The page
shows and hides questions as the visitor answers; the server checks the
same rules when the form is sent. A condition can only point to an
earlier question: move a question above the one it depends on and the
condition is dropped.

### Fill a question from the address

A question (not a file, not a consent) can take its answer from the
address the form is opened with: type a parameter's name in **Fill from
the address parameter** (under *More*), say `product`. Then a link to the form with
`?product=Tank RVS-200` opens it with the question answered. The visitor
can still change the answer, and the server checks it as any other.

So a product's, a service's or a specialist's page links to one form -
*Request a price*, *Book a visit* - with its own name in the address,
and every request says what it was about. It works on the form's own
page and in a form shown in a block or embedded in a text: the address
of the page the visitor is on is used.

| Question type | The parameter's value |
|---|---|
| a line, a text, an e-mail, a phone, a number, a date | put in as it is |
| a list, *One of the options* | the option with exactly this text, if there is one |
| *Several of the options* | the options named: repeated (`?topic=Ads&topic=Other`) or separated by commas |

A question that already has an answer - the form came back with an
error - keeps it. The parameter's name is Latin letters, digits, `_`
and `-`.

### Changing a form that has submissions

Changing a question's text or hint keeps its answers. Removing a
question deletes its answers from every submission (and the files sent
with it). Answers given to an option you have since renamed or removed
stay, and the analysis lists them as *no longer an option*.

## Limits and closing

| Setting | What it does |
|---|---|
| *One submission per user (visitors must log in to send the form)* | a guest sees *Log in to fill in the form.* with a *Log in* link; a user who has sent it sees *You have already sent this form.* |
| *Close after so many submissions* | the form closes when it has this many submissions. Empty - no limit |
| *Close at (site time)* | the form closes at this date and time, in the site's time zone |
| *The form is closed (takes no submissions)* | closes the form by hand |

A closed form keeps its page and text but shows *The form is closed.*
instead of the questions. An unpublished form, or one in moderation, is
seen only by those who may see such content, and shows *The form is not
available.*

## E-mail notifications

With addresses in *Notify of submissions*, each address gets an e-mail
for every submission: the subject *New submission: <form title>*, the
answered questions as `Question: answer` lines (a consent as *Yes*, a
file by its name; files are not attached), the page it was sent from
(see [Submissions](#submissions)) and a link to the form's submissions.
The e-mail is in the site's language. It is sent in the background, so
the visitor doesn't wait for it, and only if the site can send e-mail:
set up the transport on **Configuration → Mail** (see
[Email](../../guides/mail.md#sending)).

## Files sent with forms

A *File* question takes one file. **Allowed extensions** lists them, for
example `pdf, docx, jpg` (commas, spaces or semicolons; dots and case
don't matter). Empty - documents and pictures: pdf, doc, docx, odt, rtf,
txt, xls, xlsx, ods, jpg, jpeg, png, gif and webp. Web pages, SVG
images, scripts and programs are refused under any name.

The largest file is the *Maximum file size* of the site's
[upload policy](../../guides/files-and-images.md#upload-policy) (10 MB if
it can't be read). The form tells the visitor the allowed types and the
size, and a file that is too large is dropped in the browser before
sending.

Sent files go to the site's private directory
([public and private files](../../guides/files-and-images.md#public-and-private-files)),
never served as they are. They open only from the submissions, for those
with *View and export form submissions*, and always download under their
original name. A file is deleted with its submission, with its question
and with its form. They can be kept in a private bucket instead of the
directory ([Files in object storage](../../guides/file-storage.md#private-files)).
The site must be able to write there: the status report
(`/admin/reports/status`) says if it can't.

## Show a form in a block

To put a form in a sidebar, a footer or on other pages, add the system
block *Form* on **Structure → Blocks** and choose the form under
*Shows* on the block's page (see [Blocks](../../guides/blocks.md)). Until
a form is chosen, the block shows nothing.

The block shows only the questions and the button: the form's title and
*Text above the form* are not in it, so give the block a title if it
needs one. The form is loaded into the page by JavaScript, so a cached
page still shows each visitor what applies to them (logged in, already
sent, closed).

A form is sent without leaving the page - in a block, embedded in a text
or on its own page: the visitor stays where they are, the errors appear
by the questions with what they typed kept, and after sending the form
gives way to its message. A form with a *Page after sending* goes to that
page. Without JavaScript (or should sending fail) the form is sent the
usual way: the visitor lands on the form's own page, with the errors or
its message.

A text can show a form the same way, a contact form inside a page, say,
with the tag `[[form:<id>]]`, the form's content ID ([Embeds](../../guides/embeds.md)).

## Submissions

Those with *View and export form submissions* see a *Submissions (N)*
link on the form's page, and the links on **Content → Forms**.

- **The list** (`/node/<id>/form/submissions`) shows 50 submissions a
  page, newest first (*Newer* / *Older*): the number, the date, the user
  (*Guest* for visitors who weren't logged in) with the IP address and
  the page it was sent from, and a column per question. Files are
  download links.
- **One submission** - click its number: every question with its
  answer, the sender, *Sent from the page* and *Newer* / *Older* links.
- **Analysis** (`/node/<id>/form/analysis`): for each question, how many
  answered it (*Answered: N (P%)*); for a choice question, how many chose
  each option, with a bar. Percentages of *Several of the options* can
  add up to more than 100%.
- **Download CSV**: every submission in one file, `<form>-submissions.csv`
  (UTF-8, opens in spreadsheet programs). Columns: number, *Date*,
  *User*, *IP*, *Page*, then one per question; several answers are joined by
  `; `, a consent is *Yes*, a file is its name. A cell starting with
  `=`, `+`, `-` or `@` gets a leading apostrophe so it isn't run as a
  formula.

**The page it was sent from** is the site's page the visitor sent the
form from: the page a form in a block or a text is on, or - for the
form's own page - the page of the site that linked to it (the product
whose *Request a price* was clicked). It is kept as the page's path; an
address of another site is never stored. When the visitor came from
elsewhere, or the browser ran no scripts and sent no referrer, it's
empty.

With *Delete form submissions* you can also:

- delete one submission (*Delete* in its row or on its page);
- tick several and press *Delete selected*;
- delete every submission older than a number of days (*Delete
  submissions older than … days*, 365 by default) - for example to keep
  personal data no longer than you need.

Deleted submissions go with their files and can't be restored. Deleting
a form deletes all its submissions and files too.

Submission, analysis and file pages are hidden from search engines.

## Spam protection

- **CAPTCHA.** On *Spam protection (reCAPTCHA)* (`/admin/config/captcha`,
  see [CAPTCHA](../../guides/users-and-permissions.md#captcha)) tick
  *Forms (the forms module)*. Every form then shows the "I'm not a
  robot" checkbox, on its page and in blocks; a visitor who doesn't pass
  it sees *Confirm that you are not a robot.*
- **Bot trap.** Every form carries a field hidden from people, which
  bots fill, and the time it was shown: one with the field filled or
  sent sooner than a person could is refused with *The form was sent too
  quickly…*. It is the site's bot trap, set on *Configuration →
  Comments* (see [Comments and spam](../../guides/comments.md)).
- **Rate limit.** Sending forms is limited per IP address: a short burst
  of three, then about ten a minute. Further attempts are refused until
  the limit recovers, with "Too many requests. Please wait a little and
  try again." in the visitor's language.

## Uninstalling

Disable the module on the **Modules** page, then *Uninstall* (see
[Using modules](../using-modules.md#uninstall-a-module)). This deletes
every form with its submissions and sent files, the *Form* block
placements, the module's permissions in roles and its tables. Export
the submissions you need as CSV first.

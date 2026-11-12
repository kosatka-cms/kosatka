---
title: Views and popular content
description: Counting content views, how long the views of each day are kept, the Popular content report and sorting lists by recent views.
sidebar:
  order: 15
---

A Kosatka site can count how often its content is viewed. The counts
feed the *Popular content* report, the Overview's *Popular today* card,
the *Page views* field of lists and the Views' sorts by views.

## Turning counting on

*Configuration → Statistics* (`/admin/config/statistics`) has the switch
*Count content views*, on for a new site. While it is on, each content
page sends a small request when it is opened, and the view is counted
three ways: in all, today, and in that day's views. Only published
content that isn't waiting on moderation is counted, and one view of an
item from the same address in 30 minutes: reloading a page doesn't
inflate the figures. Neither do crawlers: search engines open pages in
a browser that runs their scripts, from many addresses, so a view sent
by a client that calls itself a bot, crawler or spider (or a headless
browser, or one that leaves out its name) isn't counted.

Turning counting off keeps what was counted; the figures just stop
growing.

## How long the views of each day are kept

The views of each day are what the 7- and 30-day figures add up. The
setting *Keep views by day* on the same page decides how long they stay:
30 days, 90 days (the default), 365 days, or for ever - even the
shortest keeps the 30-day figures complete. Older days are deleted
hourly; the total count of each item stays.

## The Popular content report

*Reports → Popular content* (`/admin/reports/popular`) lists the
published content viewed most, the most viewed first, 50 to a page:

| Tab | What it counts |
|---|---|
| *Today* | the views of today, in the site's time zone |
| *7 days*, *30 days* | the views of the last 7 or 30 days, today included |
| *All time* | every view since counting began |

Each row shows the item's place, its title (a link to it) and type, its
views in the period and its total views. The *Type* filter narrows the
list to one content type. The Overview's *Popular today* card shows the
first five of today and links to the report.

## Lists sorted by recent views

A View can sort its items by *Page views today*, *Page views, 7 days*
or *Page views, 30 days* - for example a sidebar block "Popular this
week". These sorts leave out the items not viewed in their period, so
such a list may be short or empty on a quiet site; add a second
criterion (say, *Creation date*) to order the items with equal views.
See [Views](./views.md#sorting).

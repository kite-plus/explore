Explore gathers the public feeds of independent blogs and shows their latest posts, newest first. It is the discovery side of [Kite Plus](https://github.com/kite-plus), and it works with any blog system.

## What we show

Each post shows its title, its publish date, an excerpt of at most 140 characters and, when its feed has one, a thumbnail. Clicking a title takes you straight to the author's own site: the link is the original address, with no redirect page in between and no tracking parameters added.

The status beside a post comes from a link check. Click “Awaiting check” to check a post immediately. “Possibly unavailable” means the source returned 404 or 410 to the checker; readers may get a different result, and the original link remains available.

Explore **keeps neither post content nor images**. The posts here are only a cache of what each blog's feed says right now: when an author edits or deletes a post, the change shows up here after the next fetch.

## How blogs are listed

Authors enter their blog's address on the [submission page](/en/submit). We check its feed right away, and a maintainer reviews blogs that pass. We list:

- personal and independent blogs;
- with a working RSS, Atom or JSON Feed;
- updated within the last 12 months.

Marketing, content farms, and generated or copied content are not listed.

## How to leave

Leaving is always easier than joining. [Open an issue](https://github.com/kite-plus/explore/issues) to be removed, or have your site answer our crawler with `410 Gone`, or disallow `KiteExplore` in robots.txt; any of these counts as leaving. The crawler is described [here](/bot).

## Privacy

You can read everything on Explore without an account. When you browse anonymously, we set no cookies, run no analytics scripts, and pages make no requests to third parties: Explore's own server fetches post thumbnails and blog icons for you.

The server logs only the kind of request and its outcome, such as “someone opened a blog page”, but not which blog, and never your IP address, browser details or filters. To stop abuse, the server counts requests per IP address briefly in memory; the counts go into no log and no database. Your choice of dark or light theme stays in your own browser and is never sent to us.

With an account, we keep your email, your name and a hash of your password (never the password itself), plus the blogs you follow and claim; a report you file is kept with its reason and your account so maintainers can act on it. Signing in sets one cookie, used only to keep you signed in, which ends after 30 days or when you sign out. We do not record which posts you view, click or read, and recommendations will not use personal behavior.

You can delete your account at any time on your [account page](/en/account): the account, its sign-ins, follows and claimed blogs go with it, and reports you filed stay, no longer linked to you.

## Take everything with you

The feeds of all listed blogs can be [exported as OPML](/blogs.opml) into any feed reader, and the home page itself has a [feed](/feed.xml) you can subscribe to.

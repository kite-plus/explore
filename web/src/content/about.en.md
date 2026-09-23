Explore gathers the public feeds of independent blogs and shows their latest posts, newest first. It is the discovery side of [Kite Plus](https://github.com/kite-plus), and it works with any blog system.

## What we show

Each post shows its title, its publish date and an excerpt of at most 140 characters. Clicking a title takes you straight to the author's own site: the link is the original address, with no redirect page in between and no tracking parameters added.

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

Reading Explore needs no account. We set no cookies, run no analytics scripts, and pages make no requests to third parties. Access logs record the path and the outcome of a request, never your IP address.

## Take everything with you

The feeds of all listed blogs can be [exported as OPML](/blogs.opml) into any feed reader, and the home page itself has a [feed](/feed.xml) you can subscribe to.

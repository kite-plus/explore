# Changelog

All notable changes to Explore are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

[简体中文](CHANGELOG.zh-CN.md)

## [Unreleased]

### Added

- Notices and ads that Explore publishes itself. Maintainers write them under 公告与推广 in the admin console: a title, a short summary, a link or a plain-text body, a place in the latest stream (pinned on top, or after the Nth post), who sees it (everyone, or Chinese or English pages) and when. They show on the latest stream's first page without a tag filter, in the same row as a post, with a warm "Notice" capsule or an outlined "Ad" capsule where a post gives its time. One without a link has its own page at `/notices/{id}`. Ads are always marked, and nothing is counted: Explore records no views or clicks, and ad links carry only `utm_source`. The upgrade runs one migration, which adds the `notices` table.

### Changed

- The streams no longer split posts under Today, Yesterday and other day headings: posts run in one list, newest first, and each says when it came out, such as "3 hours ago" or a date after a week. The next page simply follows on. Recommended keeps its order, each day's best first, so neighbouring posts can jump in time. The iOS app lists posts the same way.

## [0.1.13] - 2026-10-06

A small release: a post shared from the iOS app or the site now opens a page of its own on Explore, and feeds that put each post's whole web page in its description get proper excerpts and covers. The upgrade runs no migration and needs no change to `.env`, `docker-compose.yaml` or the `Caddyfile`.

### Added

- A share page for posts at `/p/{id}` (`/en/p/{id}` in English). Links and QR codes shared from the iOS app used to open the transition page, which asks the reader to confirm an address it did not open itself. The share page names the blog and the post, shows the date, excerpt, cover and tags, and offers Read the post straight away, with a few more posts from the same blog below. It never redirects on its own, shows only what lists already show, and stays out of search engines through `noindex` and `robots.txt`. It reads the post from the new `GET /api/v1/entries/{id}`, which follows the blog page's visibility rules, so a post hidden from lists cannot be reached through it either.
- Link previews of every page carry an image, the Explore mark; a shared post previews as an article with its cover when it has one.

### Fixed

- A feed that puts each post's whole web page in its description gave excerpts that began with the page's title and the site's menu, and the site's logo as every post's cover. Such an excerpt now comes from the page's main content, and the cover from the `og:image` the page names. A cached excerpt is rebuilt the next time its blog's feed changes.

## [0.1.12] - 2026-10-05

A small release: the recommended stream now takes posts scoring 11 or more, about ten a day. The upgrade runs no migration and needs no change to `.env`, `docker-compose.yaml` or the `Caddyfile`.

### Changed

- The recommended stream takes posts scoring at least 11 out of 15, about ten a day, one in six or seven posts. Over 19 days of measured scores, the provisional bar of 10 let in 21 a day, a third of all posts.

## [0.1.11] - 2026-10-05

The recommended stream chooses posts a new way: the model scores each post for depth, originality and value instead of picking one of four levels, the stream keeps the higher scorers, and each day lists its best posts first. Excerpts also lose the markup some feeds left in them. The upgrade runs one migration, after which the newest 10,000 posts are scored again over about eight hours; `docker-compose.yaml` and the `Caddyfile` stay as they are.

### Changed

- Posts are scored instead of rated: the model marks depth, originality and value from 1 to 5 each, and the score is their sum, from 3 to 15; advertising, sponsored and test posts score 0. The recommended stream takes posts scoring at least 10 for now, and the threshold will follow the scores' spread once the newest posts are all scored. The four levels rarely gave their top one and let more than half of all posts in. Posts keep their tags while they are scored again.
- The recommended stream lists each day's posts best first, instead of keeping standout posts at the top for two days, which put day headings out of order. Days follow the reader's time zone: `GET /api/v1/entries?order=recommended` takes a `tz` parameter, an IANA time zone name, and the web passes Beijing time on Chinese pages and UTC on English ones.
- With DeepSeek, `EXPLORE_TAGGER_EXTRA_BODY={"thinking":{"type":"disabled"},"temperature":0}` keeps scores steady: at the default temperature, a post scored twice got the same score only 24 times in 40.

### Fixed

- Excerpts could show markup as text: the insides of inline SVG icons and diagrams, such as `<g fill="none" …>` and `<path d="…">`, the MathML that KaTeX writes, such as `<semantics><mrow>`, and custom elements such as `<mjx-container>`. They are left out now, keeping the text inside custom elements, while a tag-like word in plain text, such as the `<T>` in a title about generics, is still kept. A cached excerpt is rebuilt the next time its blog's feed changes.

## [0.1.10] - 2026-10-05

Tagging, and the recommended stream that rests on its ratings, can now run on any OpenAI-compatible model API, such as DeepSeek, as well as on Anthropic's. It also copes with failures without stalling, and only the newest 10,000 posts get tags and ratings. The upgrade runs no migration; `.env` gains optional tagger settings while the old ones keep working, and `docker-compose.yaml` and the `Caddyfile` stay as they are.

### Added

- Tagging can use any OpenAI-compatible chat completions API, such as OpenAI, DeepSeek or a local model server, besides the Anthropic API. `EXPLORE_TAGGER_PROVIDER` picks `anthropic` (the default) or `openai`, `EXPLORE_TAGGER_BASE_URL` points at the service and `EXPLORE_TAGGER_API_KEY` holds its key; the old `EXPLORE_ANTHROPIC_API_KEY` still works. A local server that takes no key may go without one. `EXPLORE_TAGGER_EXTRA_BODY` adds fields a service needs to every request, such as `{"thinking":{"type":"disabled"}}` to turn off DeepSeek's thinking mode.

### Changed

- Only the newest 10,000 posts by publish date get tags and ratings. Older posts, mostly ones found through sitemaps, are rarely reached in a stream and would have taken days and most of the model's bill; the window moves on as new posts come in, and posts that leave it keep their tags.

### Fixed

- A post whose answer was cut short was rated skip and never asked about again. It is now tried again later, and Claude models get more room to think before they answer.
- A post the model kept failing on held up tagging for every other post. It now waits six hours on its own while the rest go on; tagging pauses only when several posts in a row fail.

## [0.1.9] - 2026-10-05

User management grows from a bare list into a place to look after accounts: find and sort them, see what hangs off each one, and handle what readers ask for, a forgotten password included, since Explore has no self-service reset. Every account also gets a numeric ID in sign-up order. The upgrade runs three database migrations; `.env`, `docker-compose.yaml` and the `Caddyfile` stay as they are.

### Added

- User management can filter accounts by status and role, with counts beside each option, sort them by ID, sign-up, last activity, follows or claimed blogs, and find one by email, name or ID (`12` or `#12`); the page keeps all of it in its address. A last-active column shows when each account was last used, recorded at most once every five minutes and never with an address or device.
- Clicking an account opens its details: live sessions, the blogs it follows and claims, claims still being verified, the reports it filed, and whether its password is a temporary one. From there or a row's menu, an admin can edit the name and email, reset the password, sign the account out everywhere, release a claimed blog, change its admin access, or delete it after typing its email. Disabling asks for a reason, kept with the admin who did it and shown on the account. Disabling, restoring and signing out also work on a selection of accounts.
- Resetting a password suggests a random temporary one that is easy to read out, signs the account out everywhere and shows the password once more to copy. A reader who signs in with it lands on the account page and is asked to change it.
- Readers can change their password on the account page; before, only the admin profile page could.
- Every account has a numeric ID in sign-up order, shown on the account page with the join date, such as "ID 12 · Joined Sep 5, 2026". Existing accounts are numbered by when they signed up. IDs never change, and only deleted accounts leave gaps. `/api/v1/me` returns it as `number`, along with `created_at` and `temporary_password`.
- Reporters in the takedown list link to their accounts in user management.

### Fixed

- Disabling or demoting the only active admin was refused with "This account already exists or the resource is already owned." It now says to give admin access to another account first, and an admin acting on their own account is told so instead of "The request is malformed."

## [0.1.8] - 2026-10-04

A small fix: the blog system Explore found on a blog's pages when listing it, such as Astro or Kite, is no longer reset to unknown by later fetches.

### Fixed

- A blog whose feed does not name the system that made it lost the one its pages named when it was listed: every fetch set it back to unknown. Only a feed that names a system changes it now.

## [0.1.7] - 2026-10-04

A small release for sites behind a CDN and for the submit page: behind Cloudflare, rate limits can count readers instead of Cloudflare's addresses, and a blog that fails the check is told what to fix instead of shown a bare code.

### Added

- Explore can run behind a CDN such as Cloudflare without its rate limits counting the CDN instead of readers: list the CDN's address ranges in `EXPLORE_CDN_RANGES`, and Caddy takes the reader's address from the `X-Forwarded-For` the CDN sends, ignoring whatever a reader wrote there. `.env.example` carries Cloudflare's ranges ready to uncomment. It needs this version's `docker-compose.yaml` and `Caddyfile`: fetch them again as in the README's first step, then set the variable in `.env`.

### Fixed

- When a blog did not pass the check on the submit page, the form listed each problem's bare code, such as `feed_not_found`, instead of the hint saying what to fix. It now shows the hint in the page's language, with the code beside it.

## [0.1.6] - 2026-10-04

This release brings new posts in sooner and makes them easier to read: blogs can ask to be fetched right after they publish, subscriptions move in and out as OPML, Recommended has posts at last, and the stream pages are rebuilt around the posts, on phones too. It also fixes rate limits that, behind the reverse proxy every deployment uses, counted the site rather than each reader.

### Added

- A blog can have Explore fetch it right after publishing. `POST /api/v1/ping` takes a JSON `{"url": ...}`, or the XML-RPC `weblogUpdates.ping` and `weblogUpdates.extendedPing` that WordPress sends to its update services, and brings a listed blog's next fetch forward to no sooner than 5 minutes after its last one. A ping cannot list a blog, and the answer is the same for any address. The About page shows authors how to set it up.
- Readers can take their subscriptions with them and bring them in. The account page exports the blogs you follow as OPML, and imports an OPML file from another reader: blogs Explore lists are followed at once, matched by feed or site address, and the rest are listed with a link that fills in the submission form.
- The Recommended tab has posts. When the tagger files a post under its tags, the same model call now rates the writing as standout, solid, brief or skip, from the title and excerpt alone and never from anyone's reading. Recommended shows solid and standout posts, at most one per blog a day, newest first, with standout posts kept at the top two days longer, and filters by language and tag like Latest. Without a model configured it says why and links to Latest. Upgrading has every cached post tagged again, newest first, so it gets a rating; this costs about as much as tagging the cache did the first time.

### Changed

- The stream pages lead with the posts. The large heading and its fixed text are gone; the tabs and the language filter, now a switch whose thumb slides to the new choice, sit in a toolbar that stays under the header in one row at any width, phones folding the language switch and the tags into a Filter button, and posts are grouped by the day they came out. Each post shows its blog and time first, and a link still to be checked is quiet grey text instead of an amber badge. Wide screens get a sidebar with a note on the stream, the tags and the blogs listed last. `GET /api/v1/blogs` takes `order=newest` for the latter.
- Switching streams, the blog language or a tag no longer reloads the page: only the list and what goes with it are swapped in place, the address still changes so it can be shared, and back and forward work the same way. The blog directory's language switch does the same.
- The header stays at the top of the window on every page, so the logo is always in view, and on phones its links fold into a menu. It shows a signed-in reader's avatar and name, drawn on the server so it no longer flashes Sign in first, and opens a menu with Following, My account, Admin for admins, and Sign out. The language switch and the theme toggle move to the footer.
- A reader who is signed in already skips the sign-in page, and the page's text has more room.
- Clicking a post's title no longer drops the reader straight onto another site: the new tab shows Explore's page for under a second, naming the blog and the post, then goes on. The link is still the post's own address, for search engines, copied links and other clicks, and the destination stays in the address's fragment, which never reaches the server. Opened from anywhere but an Explore page, the page lists the destination and waits for a click.

### Fixed

- Behind a reverse proxy, the API's rate limits counted the site's own server rather than each reader: one busy minute could turn every reader away, and the five submissions an hour were shared by everyone. The site now passes each reader's address on as the proxy gave it; the proxy and the site must be listed in `EXPLORE_TRUSTED_PROXIES`, as before. With `EXPLORE_ALLOW_PRIVATE_NETWORKS` on, for local development where every request comes from one address, reads are limited as loosely as submissions.
- A page whose data the API turned away for asking too often said the page did not exist. It now says Explore is briefly unavailable, with a 503 that tells search engines to come back.
- The crawler could crash while reading a post's page when the part before `<body>` held invalid UTF-8 or one of a few special characters, such as the Kelvin sign. This came with 0.1.4.

## [0.1.5] - 2026-09-25

This release tidies the site's navigation: Latest, Recommended and Following sit under one Discover item, and the footer's source link becomes GitHub with its mark. Releases are complete now too: every version gets a GitHub Release with native builds for Linux, macOS and Windows.

### Added

- Every version is published as a GitHub Release with native builds: the `explore` program behind the API, the crawler and database migrations, for Linux, macOS and Windows on amd64 and arm64, and the site's bundle for Node.js 22, so Explore runs without Docker too. The README has the steps.

### Changed

- The header merges Latest and Following into one Discover item. The Latest, Recommended and Following tabs sit under it and share the heading Discover, so switching tabs changes only the line under the heading and the posts.
- The footer's source code link is named GitHub and carries GitHub's mark.

## [0.1.4] - 2026-09-25

This release polishes the reading pages: a cover that fails to load no longer shows a broken image, a post whose link checked out shows a green check, and links into blogs name Explore as their source. Posts found through sitemaps get their titles and dates right.

### Added

- Links from Explore into a blog, a post's title and the Visit the blog button on a blog's page, carry `utm_source=<this site's host>`, so the author's analytics can tell a visit came from Explore even when the browser sends no referrer. A `utm_source` the author set stays as it is, and feed addresses are left alone.

### Fixed

- A cover that fails to load, for example one over 2 MB, goes away with its frame, and the post reads like one without a cover instead of showing a broken image.
- A post whose link checked out showed a brown check on a beige circle; it now shows a green check in a circle, like the icons of the other states.
- WordPress posts found through sitemaps no longer keep the blog's name at the end of their titles: blog names now match with curly and straight quotes alike, and the page's `og:site_name` counts as a name too. They get their dates as well: a page is read up to `<body>`, at most 1 MB, and the date of a JSON-LD WebPage node counts. Posts already stored are read again after the upgrade.

## [0.1.3] - 2026-09-25

Explore reads more than feeds now: covers and excerpts from article pages, and older posts from sitemaps. Accounts are complete too: readers can delete their account and change their password, the admin has a profile page, and setup no longer asks for a setup code.

### Added

- Older posts come from sitemaps. A feed carries only recent posts, so older ones are found in the sitemap that robots.txt names or that sits at a usual address, picked out by the shape of the feed's post links, and read from the `<head>` of their pages for title, date, cover and excerpt. A blog keeps at most 1000 of them, and no post body is read or stored.
- When a feed gives no image or cuts the excerpt short, such as WordPress's "[…]", the post's page is read once for the `og:image` and `og:description` in its `<head>`, honoring the page's robots meta tag.
- The posts on a blog's page can be paged through instead of showing only the latest batch.
- Entry lists show each blog's favicon.
- Readers can delete their account on the account page; the only admin cannot.
- The password can be changed. It asks for the current one, and signs out every other device while this one stays signed in.
- The admin has a profile page for the name and the password. The account menu lives only at the foot of the sidebar, and its avatar is the name's first letter on a color picked from the email.
- Sign-up asks for the password twice.

### Changed

- The reader sign-in and sign-up page uses the admin sign-in's layout.
- Setup no longer asks for a setup code. Until an admin exists, whoever finishes setup first becomes the admin, so set a fresh install up right after deploying it.
- The privacy notes on the about page cover accounts.

## [0.1.2] - 2026-09-25

The home page is now the whole stream, paged back to the first post, under three tabs: Latest, Recommended and Following.

### Added

- The home page has Latest, Recommended and Following tabs. Latest lists every post, newest first, with at most 3 per blog a day so no blog floods it. Recommended says it is coming soon. Following holds the posts of the blogs you follow.
- Switching between light and dark reveals the new theme as a circle growing from the button, or switches at once when the system asks for reduced motion.

### Changed

- The home page no longer stops at the last 30 days; it pages back to the first post.

## [0.1.1] - 2026-09-25

The web image carries only what it runs, and is much smaller.

### Changed

- The `explore-web` image no longer ships `node_modules`: the dependencies are bundled into `dist` at build time, so the image holds Node.js and the build.

## [0.1.0] - 2026-09-25

The first release of Explore: it gathers the public feeds of independent blogs and shows their posts by publish date.

### Added

- Reading: the posts of every listed blog in one stream, filtered by language or topic; a directory with a page for each blog; the stream as RSS at `/feed.xml` and the blog list as OPML at `/blogs.opml`. The interface is in Chinese and English, with a dark mode.
- Accounts, optional: follow blogs for a stream of your own, claim your blog with a DNS TXT record, and report a problem from a blog's page. Reading needs no account.
- Joining: authors submit a blog and its feed is checked on the spot, with what to fix if it fails. `explore check <url>` runs the same check from a terminal.
- Crawling: follows robots.txt, makes conditional requests and respects Retry-After; posts that link outside the blog's own site are dropped, and post links are checked on a schedule.
- Admin console: review submissions, manage blogs and posts, handle takedowns and users, watch the crawl queue, and switch registration, submissions and crawling on or off. A fresh install creates its first admin in a setup wizard.
- Deployment: every release publishes two images for amd64 and arm64, `ghcr.io/kite-plus/explore` and `ghcr.io/kite-plus/explore-web`, with Docker Compose and Caddy files.

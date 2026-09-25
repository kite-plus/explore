# Changelog

All notable changes to Explore are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

[简体中文](CHANGELOG.zh-CN.md)

## [Unreleased]

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

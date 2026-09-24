<h1 align="center">Explore</h1>

<p align="center">Discover what people publish.</p>

<p align="center">
  <strong>English</strong> · <a href="README.zh-CN.md">简体中文</a>
</p>

Explore gathers the public feeds of independent blogs and shows their latest posts as one stream, each linking straight back to the author's site. Any blog that publishes RSS, Atom or JSON Feed can join, starting with WordPress, Halo, Hugo and Hexo; [Kite](https://github.com/kite-plus/kite) joins through the same open standards.

Explore keeps no post content. It stores the list of blogs; titles, dates and short excerpts are a cache that can be emptied and rebuilt from the feeds at any time. Readers get no cookies, no tracking and no third-party requests.

Explore is the discovery side of [Kite Plus](https://github.com/kite-plus).

## Features

- **Reading**: the latest posts from every listed blog in one stream, filtered by language or topic; a directory with a page for each blog; the stream as RSS at `/feed.xml` and the whole blog list as OPML at `/blogs.opml`. The interface is in Chinese and English.
- **Accounts, optional**: follow blogs for a stream of your own, claim your blog, report a problem. Reading needs no account.
- **Joining**: authors submit a blog and its feed is checked on the spot, with what to fix if it fails. `explore check <url>` runs the same check from a terminal.
- **Crawling**: follows robots.txt, makes conditional requests and respects Retry-After; posts that link outside the blog's own site are dropped.
- **Admin console**: review submissions, manage blogs and posts, handle takedowns and users, watch the crawl queue, and switch registration, submissions and crawling on or off.

## Status

0.1.0 is the first release. Until 1.0, a new release may still change configuration or behavior. The design lives in [docs/design](docs/design/README.md), written in Chinese.

## Deploy

Every release publishes two images for amd64 and arm64: `ghcr.io/kite-plus/explore` (the API, the crawler and database migrations) and `ghcr.io/kite-plus/explore-web` (the site). A server needs Docker with Compose, ports 80 and 443 open, and a domain whose DNS record points at it; Caddy gets the HTTPS certificate. No source code is needed.

1. Download the deployment files into an empty directory:

   ```bash
   mkdir explore && cd explore
   base=https://raw.githubusercontent.com/kite-plus/explore/v0.1.0/deploy
   curl -fsSLO "$base/docker-compose.yaml"
   curl -fsSLO "$base/Caddyfile"
   curl -fsSL "$base/.env.example" -o .env
   ```

2. Edit `.env`: set `EXPLORE_VERSION=0.1.0`, a `POSTGRES_PASSWORD` (for example from `openssl rand -hex 24`) and `EXPLORE_PUBLIC_URL`. The file explains the optional settings.

3. Start it:

   ```bash
   docker compose up -d
   ```

4. Open `/admin` on your domain. Until the first admin account exists, it shows a setup wizard that asks for a one-time code, which proves you can read the server's logs:

   ```bash
   docker compose logs serve | grep setup_code
   ```

   Enter the code, create the admin account, and choose whether to open registration and submissions.

**Upgrading**: set `EXPLORE_VERSION` to the new release, then run `docker compose pull && docker compose up -d`. The database is migrated before the new version starts.

**Backups**: everything Explore keeps is in PostgreSQL. The post cache can be left out, since the crawler refills it:

```bash
docker compose exec -T postgres pg_dump -U explore --exclude-table-data=entries explore > explore.sql
```

To put a reverse proxy you already run in place of Caddy, see [docs/design/project-layout.md](docs/design/project-layout.md#10-部署).

## Development

You need Go 1.26 and Docker, plus Node 22 and pnpm for the frontend.

```bash
make db-up
export EXPLORE_TEST_DATABASE_URL='postgres://explore:explore@127.0.0.1:5433/explore?sslmode=disable'
make check      # formatting, vet, import rules, lint and tests
make web-test   # build the frontend and test it against a stub API
```

Checking a blog's feed needs no database:

```bash
make build
./bin/explore check https://blog.example.com/
```

Running the API and the worker locally is described in [docs/design/project-layout.md](docs/design/project-layout.md#9-本地开发).

## Get involved

Have an idea about how discovery should work? [Open an issue](https://github.com/kite-plus/explore/issues) to start the conversation.

## License

Explore is licensed under the [Apache License 2.0](LICENSE). The admin console is adapted from [satnaing/shadcn-admin](https://github.com/satnaing/shadcn-admin), which is MIT licensed; its license is kept in [web/src/admin/LICENSE-shadcn-admin.txt](web/src/admin/LICENSE-shadcn-admin.txt).

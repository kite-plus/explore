<h1 align="center">Explore</h1>

<p align="center">Discover what people publish.</p>

<p align="center">
  <strong>English</strong> · <a href="README.zh-CN.md">简体中文</a>
</p>

Explore gathers the public feeds of independent blogs and shows their latest posts as one stream, each linking straight back to the author's site. Any blog that publishes RSS, Atom or JSON Feed can join, starting with WordPress, Halo, Hugo and Hexo; [Kite](https://github.com/kite-plus/kite) joins through the same open standards.

Explore keeps no post content. It stores the list of blogs; titles, dates and short excerpts are a cache that can be emptied and rebuilt from the feeds at any time. Readers get no cookies, no tracking and no third-party requests.

Explore is the discovery side of [Kite Plus](https://github.com/kite-plus).

## Status

The feed checker, the backend (API, worker and PostgreSQL) and the bilingual web frontend are built; explore.kite.plus is not live yet. The design lives in [docs/design](docs/design/README.md), written in Chinese.

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

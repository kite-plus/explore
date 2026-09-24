<h1 align="center">Explore</h1>

<p align="center">发现大家发布的内容。</p>

<p align="center">
  <a href="README.md">English</a> · <strong>简体中文</strong>
</p>

Explore 汇集独立博客公开的订阅源，把它们的最新文章按时间排成一条信息流，每篇都直接链接回作者自己的网站。任何提供 RSS、Atom 或 JSON Feed 的博客都可以加入，首批支持 WordPress、Halo、Hugo 和 Hexo；[Kite](https://github.com/kite-plus/kite) 也通过同样的开放标准接入。

Explore 不保存文章正文。它只保存博客清单；标题、日期和简短摘要都是缓存，随时可以清空，再从订阅源重建。读者这边没有 Cookie、没有追踪，也没有第三方请求。

Explore 是 [Kite Plus](https://github.com/kite-plus) 负责「发现」的部分。

## 当前状态

订阅源检查、后端（API、worker 和 PostgreSQL）和中英双语的网页前端都已完成，explore.kite.plus 还没有上线。设计文档在 [docs/design](docs/design/README.md)。

## 开发

需要 Go 1.26 和 Docker；前端另需 Node 22 和 pnpm。

```bash
make db-up
export EXPLORE_TEST_DATABASE_URL='postgres://explore:explore@127.0.0.1:5433/explore?sslmode=disable'
make check      # 格式、vet、依赖规则、lint 和测试
make web-test   # 构建前端，并用模拟的 API 测试
```

检查一个博客的订阅源不需要数据库：

```bash
make build
./bin/explore check https://blog.example.com/
```

在本机运行 API 和 worker 的方法见 [docs/design/project-layout.md](docs/design/project-layout.md#9-本地开发)。

## 部署

每个 `v*` 标签都会发布两个镜像：`ghcr.io/kite-plus/explore` 和 `ghcr.io/kite-plus/explore-web`。服务器上只需要 Docker，以及 `deploy/` 里的 `docker-compose.yaml`、`Caddyfile` 和照 `.env.example` 填好的 `.env`，然后运行 `docker compose up -d`。详见 [docs/design/project-layout.md](docs/design/project-layout.md#10-部署)。

## 参与其中

对内容发现有想法？欢迎[提交 Issue](https://github.com/kite-plus/explore/issues) 一起讨论。

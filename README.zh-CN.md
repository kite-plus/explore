<h1 align="center">Explore</h1>

<p align="center">发现大家发布的内容。</p>

<p align="center">
  <a href="README.md">English</a> · <strong>简体中文</strong>
</p>

Explore 汇集独立博客公开的订阅源，把它们的最新文章按时间排成一条信息流，每篇都直接链接回作者自己的网站。任何提供 RSS、Atom 或 JSON Feed 的博客都可以加入，首批支持 WordPress、Halo、Hugo 和 Hexo；[Kite](https://github.com/kite-plus/kite) 也通过同样的开放标准接入。

Explore 不保存文章正文。它只保存博客清单；标题、日期和简短摘要都是缓存，随时可以清空，再从订阅源重建。读者这边没有 Cookie、没有追踪，也没有第三方请求。

Explore 是 [Kite Plus](https://github.com/kite-plus) 负责「发现」的部分。

## 功能

- **阅读**：所有收录博客的最新文章汇成一条信息流，可以按语言或主题筛选；博客目录里每个博客都有自己的页面；信息流另有 RSS（`/feed.xml`），整份博客清单可以导出为 OPML（`/blogs.opml`）。界面支持中文和英文。
- **账号（可选）**：关注博客，得到自己的订阅流；认领自己的博客；举报问题。不登录也能阅读全部内容。
- **加入**：作者提交博客后当场检查订阅源，没通过时会说明要改什么。命令行的 `explore check <url>` 做的是同一套检查。
- **抓取**：读取每个博客的订阅源，并从站点地图补上更早的文章，文章页只读 `<head>` 里的元数据；遵守 robots.txt 和 robots meta，使用条件请求，尊重 Retry-After；链接指向博客自身网站以外的文章会被丢弃。
- **管理后台**：审核投稿，管理博客和文章，处理下架申请和用户，查看抓取队列，随时开关注册、投稿和抓取。

## 当前状态

0.1.0 是第一个发布的版本。1.0 之前，新版本仍可能调整配置或行为。设计文档在 [docs/design](docs/design/README.md)。

## 部署

每次发版都会为 amd64 和 arm64 发布两个镜像：`ghcr.io/kite-plus/explore`（API、抓取和数据库迁移）和 `ghcr.io/kite-plus/explore-web`（网站）。服务器需要装好 Docker 和 Compose，开放 80 和 443 端口，并有一个解析到这台服务器的域名，HTTPS 证书由 Caddy 自动申请。不需要源码。

1. 把部署文件下载到一个空目录：

   ```bash
   mkdir explore && cd explore
   base=https://raw.githubusercontent.com/kite-plus/explore/v0.1.5/deploy
   curl -fsSLO "$base/docker-compose.yaml"
   curl -fsSLO "$base/Caddyfile"
   curl -fsSL "$base/.env.example" -o .env
   ```

2. 编辑 `.env`：设置 `EXPLORE_VERSION=0.1.5`、`POSTGRES_PASSWORD`（可以用 `openssl rand -hex 24` 生成）和 `EXPLORE_PUBLIC_URL`。其余可选项在文件里有说明。

3. 启动：

   ```bash
   docker compose up -d
   ```

4. 马上打开域名下的 `/admin`。还没有管理员账号时，这里显示安装向导：创建管理员账号，再选择是否开放注册和投稿。第一个完成安装的人就是管理员，所以站点启动后请立即完成这一步。

**升级**：把 `EXPLORE_VERSION` 改成新版本，再运行 `docker compose pull && docker compose up -d`。新版本启动前会先迁移数据库。

**备份**：Explore 的数据都在 PostgreSQL 里。文章缓存可以不备份，抓取程序会重新填满：

```bash
docker compose exec -T postgres pg_dump -U explore --exclude-table-data=entries explore > explore.sql
```

想用服务器上已有的反向代理代替 Caddy，见 [docs/design/project-layout.md](docs/design/project-layout.md#10-部署)。

### 不用 Docker

每个版本的 [Release](https://github.com/kite-plus/explore/releases) 都附有原生构建：`explore-<版本>-<系统>-<架构>` 是服务端、抓取器和数据库迁移共用的程序，有 Linux、macOS 和 Windows 的 amd64、arm64 版本；`explore-web-<版本>.tar.gz` 是前台。另外需要 PostgreSQL 16、Node.js 22 和一个反向代理：

1. 设置 `EXPLORE_DATABASE_URL` 和 `EXPLORE_PUBLIC_URL`，运行 `explore migrate up` 迁移数据库。每次升级后也要先运行一次。
2. 让 `explore serve`（默认监听 `127.0.0.1:8080`）和 `explore worker` 常驻运行。设置 `EXPLORE_TRUSTED_PROXIES=127.0.0.1`，隔着反向代理和前台，限流也能认出读者的真实地址。
3. 在解压出的 `explore-web-<版本>` 目录里运行 `node dist/server/entry.mjs`，设置 `EXPLORE_API_URL=http://127.0.0.1:8080`、`EXPLORE_PUBLIC_URL`、`HOST=127.0.0.1` 和 `PORT=4321`。
4. 反向代理把 `/api/*`、`/feed.xml`、`/blogs.opml`、`/healthz` 和 `/readyz` 转给 serve，其余转给前台，写法见 `deploy/Caddyfile`。

其余设置见 `deploy/.env.example`。

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

## 参与其中

对内容发现有想法？欢迎[提交 Issue](https://github.com/kite-plus/explore/issues) 一起讨论。

## 许可证

Explore 以 [Apache License 2.0](LICENSE) 授权。管理后台移植自采用 MIT 许可的 [satnaing/shadcn-admin](https://github.com/satnaing/shadcn-admin)，它的许可文本保留在 [web/src/admin/LICENSE-shadcn-admin.txt](web/src/admin/LICENSE-shadcn-admin.txt)。

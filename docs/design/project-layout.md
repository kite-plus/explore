# 工程结构

> 状态：设计中 · 最近更新：2026-09-23
> E0、E1 开工时的代码骨架约定。与 Kite 一致的地方（Go 版本、命令行、lint、Makefile、import 守护）直接沿用 Kite 的做法。

---

## 1. 目录

```text
explore/
├── cmd/explore/        main package; only calls internal/cli
├── internal/
│   ├── cli/            cobra commands: serve, worker, check, survey, migrate, version
│   ├── config/         environment configuration
│   ├── policy/         the numbers from architecture.md section 6
│   ├── model/          domain types shared across packages
│   ├── feed/           parse RSS / Atom / JSON Feed into one model
│   ├── normalize/      links, identity, dates, excerpts, snapshot
│   ├── fetch/          polite HTTP client, SSRF guard, robots.txt
│   ├── check/          feed discovery and source checks
│   ├── store/          PostgreSQL queries and transactions (pgx)
│   ├── worker/         scheduling, snapshot sync, tagging, daily maintenance
│   ├── tagger/         file posts under the tag list with a Claude model
│   ├── publicfeed/     /feed.xml and /blogs.opml
│   └── api/            Gin router, handlers, middleware
├── migrations/         goose SQL migrations, embedded into the binary
├── testdata/
│   ├── feeds/          fixture feeds per blog system
│   └── schema/         column list golden file
├── scripts/            check-imports.sh
├── deploy/             Dockerfile, docker-compose.yaml
├── docs/design/
├── .golangci.yml       same linters as Kite
├── Makefile
└── go.mod              module github.com/kite-plus/explore
```

- 前端在 E2 开工时放进 `web/`，结构见 [frontend.md §9](frontend.md#9-目录)，现在不建目录。
- `api/openapi.yaml` 在 E1 接口定稿后加入（[api.md §7](api.md#7-与前端的约定)）。
- `store` 的 SQL 直接写在 Go 代码里，用 pgx 执行，没有代码生成步骤（§5）。

---

## 2. 包的职责与依赖

### 2.1 职责

| 包 | 职责 | 允许依赖的内部包 |
|---|---|---|
| `policy` | [architecture.md §6](architecture.md#6-抓取与展示规则) 的数值常量，代码里唯一的出处 | 无 |
| `i18n` | 选择给人看的文字用中文还是英文（`Accept-Language`、`--lang`） | 无 |
| `model` | `Blog`、`Entry`、`Submission`、`CheckReport` 等领域类型和枚举 | 无 |
| `feed` | 解析订阅源（[worker.md §4](worker.md#4-解析internalfeed)） | 无 |
| `normalize` | 规范化，纯函数（[worker.md §5](worker.md#5-规范化)） | `policy`、`model`、`feed` |
| `publicfeed` | 生成 `/feed.xml` 和 OPML，纯函数 | `model` |
| `fetch` | 礼貌抓取、SSRF 防护、robots.txt（[worker.md §3](worker.md#3-抓取internalfetch)） | `policy` |
| `check` | 发现订阅源、检查、生成报告（[worker.md §6](worker.md#6-检查internalcheck)） | `policy`、`model`、`i18n`、`feed`、`normalize`、`fetch` |
| `store` | 数据库访问、事务、迁移入口；子包 `storetest` 给每个集成测试一个独立的 schema | `policy`、`model`、`migrations` |
| `worker` | 调度、同步、打标签的循环、每日维护 | `policy`、`model`、`feed`、`normalize`、`fetch`、`store` |
| `tagger` | 用 Claude 模型给文章打标签，项目里唯一调用模型的包（[worker.md §11](worker.md#11-打标签internaltagger)） | `model` |
| `api` | Gin 路由与处理函数 | `policy`、`model`、`i18n`、`check`、`store`、`publicfeed` |
| `config` | 读取环境变量 | 无 |
| `cli` | 命令入口，负责组装 | 全部 |

### 2.2 依赖规则

`scripts/check-imports.sh` 在 `make check` 里强制执行这些规则，写法照搬 Kite 的同名脚本。规则靠文档是守不住的，一次 import 就能打破它。

1. **纯逻辑包**（`policy`、`model`、`i18n`、`feed`、`normalize`、`publicfeed`）不依赖 `fetch`、`check`、`store`、`worker`、`api`、`cli`、`config`，也不依赖 Gin 和 pgx。它们没有 I/O，全部用夹具做黄金文件测试。
2. **只有 `api` 可以依赖 Gin**：Web 框架留在最外层，换框架只动一个包。
3. **只有 `store`（及其子包）可以依赖 pgx**：SQL 都在一个包里，schema 守护和 review 都只看这一处。**只有 `tagger` 可以依赖 Anthropic 的 SDK**：调用模型的地方只有一个，换模型或换服务商只动一个包；worker 通过接口使用它，由 `cli` 接起来。
4. **`check` 不依赖 `store`**：`explore check` 不需要数据库也能运行，作者和 E0 都要用。
5. **`api` 和 `worker` 互不依赖**，只通过数据库协作，所以可以分开部署、分开重启。

---

## 3. 命令

| 命令 | 作用 | 需要数据库 |
|---|---|---|
| `explore serve` | 启动 API | 是 |
| `explore worker` | 启动抓取和每日维护 | 是 |
| `explore check <url> [--json] [--lang zh-CN]` | 检查一个博客首页或订阅地址，输出报告；提示默认用英文 | 否 |
| `explore survey <file> [--concurrency N]` | 对清单里的每个博客运行检查，再测量订阅源（体积、条件请求、日期、摘要长度等），每个博客输出一行 JSON；加 `--report` 则把这些记录按博客系统汇总成 Markdown 表，不联网。用于 E0 实测（[worker.md §10](worker.md#10-e0-实测)） | 否 |
| `explore migrate up` | 执行数据库迁移 | 是 |
| `explore version` | 输出版本和构建信息 | 否 |

---

## 4. 配置

服务通过环境变量配置：

| 变量 | 默认值 | 说明 |
|---|---|---|
| `EXPLORE_DATABASE_URL` | 无；`serve`、`worker`、`migrate` 必填 | PostgreSQL 连接串 |
| `EXPLORE_HTTP_ADDR` | `127.0.0.1:8080` | API 监听地址 |
| `EXPLORE_PUBLIC_URL` | `https://explore.kite.plus` | 对外地址：User-Agent 里的说明页、`/feed.xml` 里的链接 |
| `EXPLORE_TRUSTED_PROXIES` | 空 | 反向代理和 `web` 服务的地址，逗号分隔，让限流拿到读者的真实地址。`web` 在服务端代读者调用提交接口（[frontend.md §6](frontend.md#6-提交流程)） |
| `EXPLORE_ADMIN_TOKENS` | 空 | 维护者令牌，`名字:SHA-256` 逗号分隔；为空时不注册管理接口 |
| `EXPLORE_WORKER_CONCURRENCY` | `16` | 同时抓取的博客数 |
| `EXPLORE_TAGGER_MODEL` | 空 | 打标签用的模型，例如 `claude-opus-5`；和下一项一起设置才会打标签（[accounts.md §5.3](accounts.md#53-模型与费用-待定)） |
| `EXPLORE_ANTHROPIC_API_KEY` | 空 | Claude API 的密钥；只能和上一项一起设置 |
| `EXPLORE_TAGGER_EFFORT` | 空 | 可选的 `effort`：`low`、`medium`、`high`、`xhigh`、`max`。Claude Opus 5 和 Claude Sonnet 5 建议 `low`；Claude Haiku 4.5 不支持，留空 |
| `EXPLORE_ALLOW_PRIVATE_NETWORKS` | `false` | 放行内网地址和非标准端口，只用于测试和本地开发。开发机上的代理开着 fake-IP 模式时也需要打开（[worker.md §3.3](worker.md#33-ssrf-防护)） |
| `EXPLORE_LOG_LEVEL` | `info` | `debug`、`info`、`warn`、`error` |

**抓取与展示规则的阈值不做成配置**。它们是产品规则，写在 architecture.md §6，代码里定义在 `internal/policy`，改动走文档和 code review，而不是改一个环境变量。

生成维护者令牌的哈希：

```bash
printf %s "$TOKEN" | shasum -a 256
```

---

## 5. 技术选型与版本

| 方面 | 选择 |
|---|---|
| Go | 与 Kite 相同：`go 1.26.4`，`toolchain go1.26.8`。工具链固定到补丁版本，理由见 Kite 的 `go.mod` 注释 |
| Web 框架 | `github.com/gin-gonic/gin` |
| 数据库驱动 | `github.com/jackc/pgx/v5` |
| SQL | 手写，用 pgx 执行。查询里有窗口函数、`SKIP LOCKED` 和批量 upsert，直接写 SQL 比 ORM 清楚。不用 sqlc：它的 PostgreSQL 解析器要 cgo 构建，每台开发机和 CI 都得另装；而查询只有二十来条，每条都有集成测试 |
| 迁移 | `github.com/pressly/goose/v3`，SQL 文件嵌入二进制 |
| 命令行 | `github.com/spf13/cobra`，与 Kite 一致 |
| 订阅源解析 | `github.com/mmcdole/gofeed` |
| HTML 转纯文本 | `golang.org/x/net/html`：要的是纯文本而不是净化后的 HTML，块级元素之间要补空格 |
| 域名、编码与语言 | `golang.org/x/net` 的 `publicsuffix`、`html/charset`；`golang.org/x/text` 的 `language`、`width` |
| robots.txt | `github.com/jimsmart/grobotstxt`：Google 官方 robots.txt 解析器的移植，与 RFC 9309 一致 |
| 日志 | 标准库 `log/slog`，JSON 输出 |

依赖版本在 E1 初始化 `go.mod` 时取当时的稳定版并固定。

---

## 6. HTTP 服务约定

- **用 `gin.New()`，不用 `gin.Default()`**：默认的 Logger 中间件会把客户端 IP 写进日志，违背 [architecture.md §10](architecture.md#10-成功指标)。自己写日志中间件，只记方法、路由模板（`c.FullPath()`，不带查询参数）、状态码和耗时。
- 生产环境 `gin.SetMode(gin.ReleaseMode)`。
- `SetTrustedProxies` 只信任 `EXPLORE_TRUSTED_PROXIES`；`c.ClientIP()` 只用于内存里的限流。
- Recovery 中间件返回 `internal` 错误（[api.md §5](api.md#5-错误码)），堆栈只写日志。
- 处理函数保持薄：解析参数，调用 `store` 或 `check`，把结果映射成响应。错误到错误码的映射集中在一处。
- `http.Server` 设置 `ReadHeaderTimeout` 等超时；提交接口单独给 30 秒的上下文超时。
- 不设 Cookie，不用 Session；管理接口只认 Bearer 令牌。

---

## 7. 测试

| 层 | 做法 | 需要数据库 |
|---|---|---|
| 纯逻辑包 | 表驱动测试加黄金文件，`go test ./... -update` 更新 | 否 |
| `fetch`、`check` | `httptest` 模拟服务器，测试时放行回环地址 | 否 |
| `store`、`worker`、`api` | 集成测试连真实的 PostgreSQL，由 `EXPLORE_TEST_DATABASE_URL` 指定，没设置就跳过 | 是 |
| 不变量 | 清空恢复、schema 列清单（[data-model.md §6](data-model.md#6-schema-守护)） | 是 |

- 夹具的来源规则见 [worker.md §9](worker.md#9-测试)：不提交真实博客的内容。
- 每个集成测试使用独立的 schema，互不干扰，可以并行。

---

## 8. Makefile

沿用 Kite 的目标名，另加数据库和代码生成相关的目标：

| 目标 | 作用 |
|---|---|
| `build` | 构建 `bin/explore` |
| `test`、`test-race`、`cover` | 测试 |
| `fmt`、`vet`、`lint` | 格式、静态检查、golangci-lint |
| `check-imports` | 运行 `scripts/check-imports.sh`（§2.2） |
| `check-tidy` | `go.mod` 是否整洁 |
| `check` | `fmt vet check-imports check-tidy lint test`，提交前必跑 |
| `db-up`、`db-down` | 启动或停止本地开发用的 PostgreSQL |
| `migrate` | 对本地数据库执行 `explore migrate up` |
| `docker` | 构建镜像 |
| `web`、`web-check`、`web-test` | 构建前端、类型检查、对桩 API 跑 HTML 测试，与 Kite 的同名目标对应；从 `api/openapi.yaml` 生成类型的 `web-gen` 等 OpenAPI 写好后再加 |
| `docker-web` | 构建前端镜像 |
| `clean` | 清理构建产物 |

---

## 9. 本地开发

```bash
make db-up
export EXPLORE_DATABASE_URL='postgres://explore:explore@127.0.0.1:5433/explore?sslmode=disable'
make migrate
go run ./cmd/explore worker
go run ./cmd/explore serve
```

`worker` 和 `serve` 各开一个终端。开发用的 PostgreSQL 映射到本机 5433 端口，避免和本机已有的 PostgreSQL 冲突。

检查一个博客不需要数据库。检查本机上的博客（例如本地的 Halo）要放行内网地址：

```bash
go run ./cmd/explore check https://blog.example.com/
EXPLORE_ALLOW_PRIVATE_NETWORKS=true go run ./cmd/explore check http://127.0.0.1:8090/
```

---

## 10. 部署

| 服务 | 镜像与命令 | 说明 |
|---|---|---|
| `postgres` | `postgres:16-alpine` | 数据卷持久化；备份时排除 `entries` 的数据（[data-model.md §5](data-model.md#5-数据保留)） |
| `migrate` | `explore migrate up` | 每次发布先跑一次，跑完退出 |
| `serve` | `explore serve` | API；可以多实例 |
| `worker` | `explore worker` | 抓取；V1 一个实例 |
| `web` | 前端镜像（`web/Dockerfile`），Node 服务 | 只在服务端调用 `serve`（[frontend.md §10](frontend.md#10-开发测试与部署)）。compose 给网络固定了 `10.89.0.0/24`，`serve` 默认信任它，限流才能拿到读者的地址 |

后端的四个服务用同一个镜像（`deploy/Dockerfile`）、不同的命令，编排在 `deploy/docker-compose.yaml`，配置从 `deploy/.env` 读取（照 `deploy/.env.example` 填写，不进版本库）；`web` 是单独的镜像。反向代理（Caddy 或 Nginx）为 explore.kite.plus 终止 TLS：`/api/`、`/feed.xml`、`/blogs.opml`、`/healthz`、`/readyz` 转给 `serve`，其余转给 `web`，并按页面的 `Cache-Control` 缓存。反向代理的访问日志同样不记录客户端 IP。

CI（`.github/workflows/ci.yml`）用 GitHub Actions 运行格式检查、`vet`、依赖守护、`check-tidy`、lint 和带 `-race` 的测试，附带一个 PostgreSQL 服务容器并设置 `EXPLORE_TEST_DATABASE_URL`，让集成测试和不变量测试都能跑。

---

## 11. 开发顺序

按里程碑（[architecture.md §9](architecture.md#9-里程碑)）从不需要数据库的部分做起：

**E0 实测**（不需要数据库）

1. 初始化：`go.mod`、`.golangci.yml`（从 Kite 复制）、`Makefile`、`scripts/check-imports.sh`、`cmd/explore` 和 `internal/cli`。
2. `policy`、`model`。
3. `feed`、`normalize`，以及各系统的夹具和黄金文件测试。
4. `fetch`（包括 SSRF 防护和 robots.txt）、`check`、`explore check`。
5. 对四个系统各至少 10 个真实博客批量运行，写出质量报告，回填 `[待定]`。

**E1 后端**

6. `migrations/00001_init.sql`（[data-model.md §2](data-model.md#2-表结构-设计中)）、`store`，以及 schema 列清单测试。
7. `worker`：领取、同步事务、退避、每日维护，以及清空恢复测试。
8. `publicfeed`、`api`：公开接口、提交、管理、`/feed.xml`、OPML。
9. `deploy/`：Dockerfile、docker-compose.yaml；CI。

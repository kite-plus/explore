# API 设计

> 状态：设计中 · 最近更新：2026-09-23
> 服务：`explore serve`（Gin）。读者看到的页面由独立的前端在服务端渲染（[architecture.md §8](architecture.md#8-前端与-seo)），前端只调用本文的公开接口。

---

## 1. 约定

- **路径**：接口都在 `/api/v1` 下。v1 内只加字段、不改语义；破坏性变更走 `/api/v2`。订阅与导出（§3）不在 `/api` 下，因为它们的地址本身就是给阅读器用的。
- **格式**：JSON，UTF-8；字段名用 snake_case；时间用 RFC 3339 的 UTC 形式（`2026-09-20T02:00:00Z`）；数据库里的 `bigint` 在 JSON 里写成字符串，避免 JavaScript 丢精度。
- **列表**：统一返回 `{"data": [...], "next_cursor": "..."}`，`next_cursor` 为 `null` 表示没有更多。`limit` 默认 30，最大 100。游标是不透明字符串（`(published_at, id)` 的 base64url 编码），客户端原样传回，不要解析。
- **错误**：HTTP 状态码加 `{"error": {"code": "...", "message": "..."}}`。`code` 是稳定的机器可读值（§5），`message` 给人看。
- **缓存**：公开的 GET 接口返回 `Cache-Control: public, max-age=60` 和弱 `ETag`，支持 `If-None-Match`，前端和 CDN 可以直接缓存。
- **限流**：按客户端地址在内存里计数，超出返回 `429` 和 `Retry-After`。地址不写日志、不入库。
- **读者侧不设 Cookie**。CORS 不开放：浏览器从不直接调用 `/api/v1`，前端在服务端调用（[frontend.md §5](frontend.md#5-数据获取)）。
- **语言**：给人看的文字只有错误的 `message` 和检查报告的 `hint`，按 `Accept-Language` 返回简体中文或英文，默认英文；含这类文字的响应带 `Vary: Accept-Language`。`code` 与语言无关，客户端只按 `code` 做判断。时间流、目录、博客页的响应不含这类文字，与语言无关（[frontend.md §3.5](frontend.md#35-接口返回的文字)）。

---

## 2. 公开接口

### 2.1 `GET /api/v1/entries`

首页时间流。规则见 [architecture.md §6.6](architecture.md#6-抓取与展示规则)，查询见 [data-model.md §4.1](data-model.md#41-首页时间流)。

| 参数 | 说明 |
|---|---|
| `cursor` | 上一页返回的 `next_cursor` |
| `limit` | 默认 30，最大 100 |
| `lang` | 按博客语言的前缀筛选，例如 `zh` 匹配 `zh-CN` 和 `zh-TW` |

```json
{
  "data": [
    {
      "id": "8412",
      "title": "Hello, world",
      "url": "https://blog.example.com/posts/hello/",
      "excerpt": "First paragraph of the post, cut to 140 characters…",
      "published_at": "2026-09-20T02:00:00Z",
      "blog": {
        "host": "blog.example.com",
        "name": "Example Blog",
        "site_url": "https://blog.example.com/",
        "language": "zh-CN"
      }
    }
  ],
  "next_cursor": "MjAyNi0wOS0yMFQwMjowMDowMFp8ODQxMg"
}
```

作者关闭摘要时，`excerpt` 为 `null`。`url` 是订阅源里的原始链接，前端必须原样输出，不能改写成跳转地址（[architecture.md §6.3](architecture.md#6-抓取与展示规则)）。

### 2.2 `GET /api/v1/blogs`

博客目录，只含可见的博客，排序见 [data-model.md §4.2](data-model.md#42-博客目录与博客页)。参数同 §2.1。

```json
{
  "data": [
    {
      "host": "blog.example.com",
      "name": "Example Blog",
      "site_url": "https://blog.example.com/",
      "feed_url": "https://blog.example.com/atom.xml",
      "language": "zh-CN",
      "generator": "hexo",
      "last_published_at": "2026-09-20T02:00:00Z"
    }
  ],
  "next_cursor": null
}
```

前端用它生成 `sitemap.xml`，`last_published_at` 就是 `lastmod`。

### 2.3 `GET /api/v1/blogs/{host}`

一个博客的信息和它在缓存里的全部文章（包括没有日期、日期不可信、超过 30 天的），按发布时间倒序，没有日期的排最后。

```json
{
  "blog": { "host": "blog.example.com", "name": "Example Blog", "...": "..." },
  "entries": [
    {
      "id": "8412",
      "title": "Hello, world",
      "url": "https://blog.example.com/posts/hello/",
      "excerpt": "…",
      "published_at": "2026-09-20T02:00:00Z"
    }
  ]
}
```

博客不存在或当前不可见（被暂停、7 天没有成功抓取、收到退出信号）时返回 `404 not_found`。

### 2.4 `POST /api/v1/submissions`

提交一个博客。检查同步执行，最长 30 秒。

```json
{
  "site_url": "https://blog.example.com/",
  "feed_url": null,
  "note": "A personal blog about Go and databases."
}
```

按顺序判断：

| 情况 | 响应 |
|---|---|
| 地址不是 `http(s)` 地址，或主机是 IP 地址、没有点的单段主机名（如 `localhost`）；`EXPLORE_ALLOW_PRIVATE_NETWORKS` 打开时不检查主机形式 | `400 invalid_url` |
| 主机在 `excluded_hosts` 里 | `403 excluded` |
| 主机已经收录 | `409 already_listed` |
| 主机已有待审核的提交 | `409 already_pending`，响应里带那条提交的 `id` |
| 检查不通过 | `422 check_failed`，响应里带 `check_report`；**不写入**任何数据 |
| 检查通过 | `201`，写入一条待审核提交 |

```json
{
  "id": "0f6b1c1e-8a47-4f55-9a61-3d2b6f1f7c0a",
  "status": "pending",
  "check_report": { "passed": true, "...": "..." }
}
```

`check_report` 的结构见 [worker.md §6.3](worker.md#63-报告格式)。每个客户端地址每小时最多提交 5 次 `[待定]`。

### 2.5 `GET /api/v1/submissions/{id}`

查询提交进度。返回 `id`、`status`、`host`、`site_url`、`feed_url`、`check_report`、`review_note`、`created_at`、`reviewed_at`。维护者的名字（`reviewed_by`）不对外。

---

## 3. 订阅与导出

| 路径 | 内容 |
|---|---|
| `GET /feed.xml` | 首页时间流的 RSS 2.0，最新 50 篇，规则与 §2.1 相同（不按语言筛选），`Cache-Control: public, max-age=300` |
| `GET /blogs.opml` | 全部可见博客的 OPML 2.0，导入阅读器即可带走整份清单 |
| `GET /healthz` | 进程存活即返回 `200` |
| `GET /readyz` | 能连上数据库才返回 `200` |

`/feed.xml` 的每一项：

```xml
<item>
  <title>Hello, world</title>
  <link>https://blog.example.com/posts/hello/</link>
  <guid isPermaLink="true">https://blog.example.com/posts/hello/</guid>
  <pubDate>Sun, 20 Sep 2026 02:00:00 +0000</pubDate>
  <description>First paragraph of the post, cut to 140 characters…</description>
  <source url="https://blog.example.com/atom.xml">Example Blog</source>
</item>
```

`<source>` 是 RSS 2.0 专门用来标明"这一项来自哪个订阅源"的元素。作者关闭摘要时省略 `<description>`。

`/blogs.opml` 的每个博客一行：

```xml
<outline type="rss" text="Example Blog" title="Example Blog"
         xmlUrl="https://blog.example.com/atom.xml" htmlUrl="https://blog.example.com/"/>
```

---

## 4. 管理接口

**认证**：`Authorization: Bearer <token>`。令牌配置在 `EXPLORE_ADMIN_TOKENS` 里，格式是 `名字:令牌的 SHA-256`，多个用逗号分隔（[project-layout.md §4](project-layout.md#4-配置)）。服务端对收到的令牌求哈希后做常量时间比较；名字记为审核人。没有配置任何令牌时，管理接口整体不注册，访问返回 `404`。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/admin/submissions?status=pending` | 提交列表 |
| POST | `/api/v1/admin/submissions/{id}/approve` | 通过并创建博客；可以覆盖 `name`、`language`、`feed_url`、`extra_domains`、`show_excerpt`。新博客立即进入抓取 |
| POST | `/api/v1/admin/submissions/{id}/reject` | 拒绝，必须填 `review_note`，作者查询进度时能看到 |
| GET | `/api/v1/admin/blogs?health=unhealthy` | 全部博客及其抓取状态（`last_error`、`consecutive_failures` 等） |
| POST | `/api/v1/admin/blogs` | 维护者直接收录，同样先运行检查 `[待定]`，取决于 [architecture.md §13](architecture.md#13-待确认的问题) 的问题 3 |
| PATCH | `/api/v1/admin/blogs/{host}` | 修改 `name`、`language`、`feed_url`、`extra_domains`、`show_excerpt`；暂停或恢复（`status`、`status_note`）。改动影响规范化结果的字段时强制重建（[data-model.md §3](data-model.md#3-同步事务)） |
| DELETE | `/api/v1/admin/blogs/{host}?exclude=opt_out` | 移除博客，文章级联删除；`exclude` 为 `opt_out` 或 `blocked` 时同时写入排除名单。处理作者的退出申请用 `opt_out` |
| POST | `/api/v1/admin/blogs/{host}/fetch` | 立即抓取，返回 `202` |
| GET | `/api/v1/admin/excluded-hosts` | 排除名单 |
| DELETE | `/api/v1/admin/excluded-hosts/{host}` | 解除排除，例如作者想重新加入 |
| POST | `/api/v1/admin/check` | 对任意地址运行检查并返回报告，不写入任何数据 |

---

## 5. 错误码

| `code` | HTTP | 场景 |
|---|---|---|
| `invalid_request` | 400 | 请求体或参数格式错误 |
| `invalid_url` | 400 | 不是公网的 `http(s)` 地址 |
| `invalid_cursor` | 400 | 游标无法解析 |
| `unauthorized` | 401 | 管理接口的令牌缺失或无效 |
| `excluded` | 403 | 博客已退出或被屏蔽 |
| `not_found` | 404 | 资源不存在或不可见 |
| `already_listed` | 409 | 博客已经收录 |
| `already_pending` | 409 | 已有待审核的提交；响应里带 `submission_id` |
| `not_pending` | 409 | 审核一条已经审核过的提交 |
| `check_failed` | 422 | 检查不通过，响应里带 `check_report` |
| `rate_limited` | 429 | 超出限流，带 `Retry-After` |
| `internal` | 500 | 服务端错误，细节只写日志 |

---

## 6. 发布即出现（E3）`[待定]`

- `POST /api/v1/ping`，请求体 `{"url": "..."}`，地址可以是博客首页或订阅地址。对应一个已收录的博客时，把它的 `next_fetch_at` 设为现在，同一博客 5 分钟内最多触发一次。无论是否对应已收录的博客都返回 `202`：ping 不能新增博客，也不能写入任何内容。
- `POST /xmlrpc`：实现 `weblogUpdates.ping` 和 `weblogUpdates.extendedPing`，让 WordPress 的"更新服务"可以直接通知 Explore，效果与上一条相同（E3 验证兼容性）。

---

## 7. 与前端的约定

- 页面和接口一一对应：`/` 对应 §2.1，`/blogs` 对应 §2.2，`/blogs/{host}` 对应 §2.3，提交页对应 §2.4 和 §2.5。
- **没有单篇文章的接口，也就没有文章页**（[architecture.md §8](architecture.md#8-前端与-seo)）。
- 前端（Astro，见 [frontend.md](frontend.md)）只在服务端调用接口；接口的缓存头让前端和 CDN 可以直接复用响应。
- 前端代读者调用提交接口时带上 `X-Forwarded-For`；`web` 服务要列在 `EXPLORE_TRUSTED_PROXIES` 里，限流才能拿到读者的真实地址。
- 接口还没有写成 OpenAPI 3.1（`api/openapi.yaml`）`[待定]`。在那之前，前端的类型手写在 `web/src/lib/types.ts`，与本文保持一致；写好后改为生成，与本文冲突时先改本文。

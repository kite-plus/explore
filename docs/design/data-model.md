# 数据模型

> 状态：设计中 · 最近更新：2026-09-23
> 原则见 [architecture.md §0.1](architecture.md#0-两个核心判断)：只存"有哪些博客"和"谁订阅了什么"，不存"博客写了什么"。本文把它落到 PostgreSQL 上。
> 迁移文件落地后，表结构以 `migrations/` 为准；本文同步更新。

---

## 1. 数据分类

| 表 | 性质 | 能不能清空 | 备份 |
|---|---|---|---|
| `blogs` | 真相源：收录了哪些博客，附带抓取状态 | 不能 | 要备份 |
| `entries` | 缓存：各博客订阅源此刻的样子 | 能，下一轮抓取恢复 | 不备份 |
| `submissions` | 流程记录：提交与审核 | 能，按保留期清理 | 只备份待审核的 |
| `excluded_hosts` | 真相源：已退出或被屏蔽的主机名 | 不能 | 要备份 |

读者账号、会话、订阅、博客归属和未来 OIDC 关联表已实现，结构见 [accounts.md](accounts.md)。账号数据是真相源，不能清空，必须备份；文章标签仍是 `entries` 里的可重建缓存。维护者可使用显式授权的本站管理员账号或配置的访问令牌。

数据库里**永远不出现**的东西：文章正文、HTML、图片文件；读者的浏览记录、点击记录、IP；账号之外的任何读者信息。图片地址只是可丢弃的元数据。账号本身只有 [accounts.md §1](accounts.md#1-原则怎么改) 列出的几项。§6 的 schema 守护会在有人加列时让 CI 失败。

---

## 2. 表结构 `[设计中]`

约定：主键用 `bigint` 自增（对外暴露的提交用 `uuid`）；时间一律 `timestamptz`；枚举用 `text` 加 `CHECK`，比 PostgreSQL 的 enum 类型好迁移；应用负责维护 `updated_at`。

### 2.1 `blogs`

```sql
CREATE TABLE blogs (
    id                   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    host                 text        NOT NULL UNIQUE,
    name                 text        NOT NULL CHECK (char_length(name) <= 100),
    description          text        NOT NULL DEFAULT '' CHECK (char_length(description) <= 240),
    description_checked_at timestamptz,
    site_url             text        NOT NULL,
    feed_url             text        NOT NULL,
    language             text        NOT NULL,
    generator            text        NOT NULL DEFAULT 'unknown',
    status               text        NOT NULL DEFAULT 'active'
                                     CHECK (status IN ('active', 'paused')),
    status_note          text,
    show_excerpt         boolean     NOT NULL DEFAULT true,
    extra_domains        text[]      NOT NULL DEFAULT '{}',
    default_tags         text[]      NOT NULL DEFAULT '{}' CHECK (cardinality(default_tags) <= 3),

    etag                 text,
    last_modified        text,
    body_hash            bytea,
    fetch_interval       interval    NOT NULL DEFAULT '60 minutes',
    next_fetch_at        timestamptz NOT NULL DEFAULT now(),
    last_fetched_at      timestamptz,
    last_succeeded_at    timestamptz,
    consecutive_failures integer     NOT NULL DEFAULT 0,
    last_error           text,
    gone_since           timestamptz,

    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX blogs_due ON blogs (next_fetch_at) WHERE status = 'active';
```

| 字段 | 说明 |
|---|---|
| `host` | 站点地址的主机名：小写，国际化域名转成 punycode，不去掉 `www.`。对外用它标识博客（`/blogs/{host}`）。一个主机只能收录一个博客 |
| `description`、`description_checked_at` | 首页 `meta description`，缺失时取订阅源描述；worker 每 7 天检查一次，最长 240 字；没有可用描述时保留上次值 |
| `language` | BCP 47 标签（如 `zh-CN`），由提交时声明或取订阅源的 `<language>`，写入前规范化（`zh_cn` 存为 `zh-CN`），缺失或无法识别时为 `und`。`lang` 筛选按前缀匹配，靠的就是这个形式 |
| `generator` | 从订阅源识别的博客系统：`wordpress`、`halo`、`hugo`、`hexo`、`typecho`、`jekyll`、`ghost`、`kite`、`other`、`unknown`。用于统计和检查提示，**不影响**抓取与展示（[architecture.md §0.2](architecture.md#0-两个核心判断)） |
| `status` | 只由维护者改：`active` 正常抓取和展示；`paused` 既不抓取也不展示，原因写在 `status_note` |
| `show_excerpt` | 作者关闭摘要时为 `false`，此时 `entries.excerpt` 不入库 |
| `extra_domains` | 文章链接允许出现的额外域名（[architecture.md §6.3](architecture.md#6-抓取与展示规则)） |
| `default_tags` | 维护者给这个博客设的默认标签，打标签时给模型当提示，不直接贴到文章上。修改后，这个博客的文章会重新打标签（[accounts.md §5.2](accounts.md#52-怎么打)） |
| `etag` … `last_error` | 抓取状态，属于缓存性质，丢了只是多一轮完整下载 |
| `gone_since` | 第一次收到退出信号（`410 Gone`，或 robots.txt 明确禁止）的时间；恢复正常后清空（[worker.md §3.4](worker.md#34-结果分类)） |

博客健康不单独存状态，而是由抓取字段推导：

```text
visible = status = 'active'
      AND gone_since IS NULL
      AND last_succeeded_at > now() - interval '7 days'
```

连续 7 天抓不到的博客自动从页面上消失，一旦抓取恢复就自动回来，不需要维护者介入。

### 2.2 `entries`

```sql
CREATE TABLE entries (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    blog_id       bigint      NOT NULL REFERENCES blogs (id) ON DELETE CASCADE,
    identity      text        NOT NULL CHECK (char_length(identity) <= 500),
    url           text        NOT NULL CHECK (char_length(url) <= 2000),
    title         text        NOT NULL CHECK (char_length(title) <= 300),
    excerpt       text                 CHECK (char_length(excerpt) <= 140),
    image_url     text                 CHECK (char_length(image_url) <= 2000),
    published_at  timestamptz,
    date_trusted  boolean     NOT NULL,
    synced_at     timestamptz NOT NULL DEFAULT now(),
    categories    text[]      NOT NULL DEFAULT '{}' CHECK (cardinality(categories) <= 10),
    tags          text[]      NOT NULL DEFAULT '{}' CHECK (cardinality(tags) <= 3),
    tagged_at     timestamptz,
    link_status   text        NOT NULL DEFAULT 'unknown'
                              CHECK (link_status IN ('unknown', 'available', 'unavailable')),
    link_checked_at timestamptz,
    link_next_check_at timestamptz NOT NULL DEFAULT now(),

    UNIQUE (blog_id, identity),
    CHECK (published_at IS NOT NULL OR NOT date_trusted)
);

CREATE INDEX entries_stream   ON entries (published_at DESC, id DESC) WHERE date_trusted;
CREATE INDEX entries_by_blog  ON entries (blog_id, published_at DESC NULLS LAST);
CREATE INDEX entries_tags     ON entries USING gin (tags);
CREATE INDEX entries_untagged ON entries (published_at DESC NULLS LAST, id DESC) WHERE tagged_at IS NULL;
CREATE INDEX entries_link_due ON entries (link_next_check_at, id);
```

- **没有正文字段**，`excerpt` 在数据库层面限制在 140 字以内：不是"约定不存"，而是"存不进去"。140 是 [architecture.md §6.5](architecture.md#6-抓取与展示规则) 的 `[待定]` 值，E0 改动它需要一个迁移。
- `image_url` 只缓存订阅源给出的图片地址，不保存图片文件；代理加载图片时仅在内存中短时缓存。作者关闭摘要时也不展示缩略图。
- `identity` 的取法见 [worker.md §5](worker.md#5-规范化)。
- `date_trusted` 由规范化计算（没有日期、零值日期、"日期不可信"时为 `false`）；未来时间不在入库时判断，而在查询时比较 `now()`，因为它会随时间变化。
- 每个博客最多 20 行（[architecture.md §0.1](architecture.md#0-两个核心判断) 的 N）。1,000 个博客约 2 万行，完全不需要分区。
- `categories` 是订阅源里这篇文章自带的分类，只给打标签当线索，不展示；去掉了 WordPress 的 `Uncategorized` 这类占位分类。
- `tags` 是 Explore 从标签表里给文章打的标签（[accounts.md §5](accounts.md#5-文章标签)），`tagged_at` 为空表示还没打。它们和其他列一样是缓存：同步时标题没变就保留，标题变了就清空重打；清空 `entries` 后全部重打。
- `link_status`、`link_checked_at` 和 `link_next_check_at` 是原文链接的检测缓存。订阅源更新链接时重置检测状态；链接不变时保留。worker 每个博客每轮最多检查一篇，避免集中请求同一个站点。读者主动检测只认领尚未检查的文章，和 worker 共用 `link_next_check_at` 租约，避免重复请求源站。

### 2.3 `submissions`

```sql
CREATE TABLE submissions (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    host          text        NOT NULL,
    site_url      text        NOT NULL,
    feed_url      text        NOT NULL,
    note          text        CHECK (char_length(note) <= 500),
    check_report  jsonb       NOT NULL,
    status        text        NOT NULL DEFAULT 'pending'
                              CHECK (status IN ('pending', 'approved', 'rejected')),
    review_note   text,
    reviewed_by   text,
    reviewed_at   timestamptz,
    blog_id       bigint      REFERENCES blogs (id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX submissions_one_pending_per_host ON submissions (host) WHERE status = 'pending';
```

- 用 `uuid` 作对外 ID，避免被按序号遍历别人的提交。
- 只保存检查**通过**的提交；检查不通过时接口直接返回报告，什么也不写（[api.md §2.4](api.md#24-post-apiv1submissions)）。
- 不收集提交者的联系方式。作者凭提交时拿到的 ID 查询进度。
- `check_report` 只存代码和计数，不存提示文字；提示在接口返回时按请求的语言生成（[worker.md §6.3](worker.md#63-报告格式)）。
- `reviewed_by` 是审核它的管理员账号的邮箱。

### 2.4 `excluded_hosts`

```sql
CREATE TABLE excluded_hosts (
    host        text        PRIMARY KEY,
    reason      text        NOT NULL CHECK (reason IN ('opt_out', 'blocked')),
    note        text,
    created_at  timestamptz NOT NULL DEFAULT now()
);
```

作者退出（`opt_out`）或维护者屏蔽（`blocked`）后，主机名记在这里，之后针对它的提交一律拒绝。这是兑现"退出比加入容易"的必要记录：只存一个主机名，要永久保留。作者想重新加入时，由维护者删掉这一行。

---

## 3. 同步事务

抓取并规范化成功后，worker 在**一个事务**里把快照写进 `entries`，使它永远等于"该博客订阅源此刻的样子"：

```sql
SELECT id FROM blogs WHERE id = @blog_id FOR UPDATE;

INSERT INTO entries (blog_id, identity, url, title, excerpt, published_at, date_trusted)
SELECT @blog_id, s.identity, s.url, s.title, s.excerpt, s.published_at, s.date_trusted
FROM unnest(@identities::text[], @urls::text[], @titles::text[],
            @excerpts::text[], @published::timestamptz[], @trusted::boolean[])
     AS s (identity, url, title, excerpt, published_at, date_trusted)
ON CONFLICT (blog_id, identity) DO UPDATE
SET url          = excluded.url,
    title        = excluded.title,
    excerpt      = excluded.excerpt,
    published_at = excluded.published_at,
    date_trusted = excluded.date_trusted,
    synced_at    = now();

DELETE FROM entries
WHERE blog_id = @blog_id
  AND NOT (identity = ANY (@identities::text[]));

UPDATE blogs
SET etag = @etag, last_modified = @last_modified, body_hash = @body_hash,
    last_fetched_at = now(), last_succeeded_at = now(),
    consecutive_failures = 0, last_error = NULL, gone_since = NULL,
    fetch_interval = @fetch_interval, next_fetch_at = @next_fetch_at,
    updated_at = now()
WHERE id = @blog_id;
```

三种结果，三种写法：

| 抓取结果 | 写入 |
|---|---|
| 成功且有变化 | 上面的完整事务 |
| 没有变化（`304` 或响应体哈希相同） | 只更新 `blogs` 的抓取字段，`entries` 不动 |
| 失败（包括**解析失败**） | 只更新失败字段，`entries` 不动 |

解析失败**绝不能**当成"空快照"处理，否则一次服务器故障就会清空这个博客的全部文章。只有成功解析、而且订阅源里确实没有可用文章时，才会删到零行。

**清空之后怎么恢复**：一个博客在 `entries` 里一篇文章都没有时，worker 抓取它不带条件请求头，也不比较响应体哈希，直接完整重建。否则服务器会回 `304`（或哈希相同），缓存就永远是空的。所以单独清空 `entries`（`store.ClearCache`）就能自愈，不需要同时清抓取字段。

修改会影响规范化结果的字段（`show_excerpt`、`extra_domains`、`feed_url`）时，同时清空 `etag`、`last_modified`、`body_hash`，并把 `next_fetch_at` 设为 `now()`，强制下一轮完整重建。

---

## 4. 查询

### 4.1 首页时间流

规则见 [architecture.md §6.6](architecture.md#6-抓取与展示规则)：日期可信、不在未来、同一博客每天最多 3 篇，不限时间范围。

```sql
WITH visible AS (
    SELECT e.id, e.blog_id, e.title, e.url, e.excerpt, e.published_at,
           row_number() OVER (
               PARTITION BY e.blog_id, date_trunc('day', e.published_at AT TIME ZONE 'UTC')
               ORDER BY e.published_at DESC, e.id DESC
           ) AS rank_in_day
    FROM entries e
    JOIN blogs b ON b.id = e.blog_id
    WHERE b.status = 'active'
      AND b.gone_since IS NULL
      AND b.last_succeeded_at > now() - interval '7 days'
      AND e.date_trusted
      AND e.published_at <= now() + @future_tolerance::interval
)
SELECT id, blog_id, title, url, excerpt, published_at
FROM visible
WHERE rank_in_day <= @per_blog_per_day
  AND (published_at, id) < (@cursor_published_at, @cursor_id)
ORDER BY published_at DESC, id DESC
LIMIT @page_size;
```

- **先算每天的上限，再套游标**：上限在 `visible` 里对全体数据计算，翻页时结果才前后一致。第一页不带游标条件。
- "每天"按 UTC 划分 `[待定]`。
- 参数值来自 `internal/policy`，不在 SQL 里写死。
- **不设时间窗口**：缓存本身有上限（博客数 × 20 篇），对全部缓存排序和翻页的代价随之有界。

### 4.2 博客目录与博客页

- 目录：可见的博客，按最近一篇可信日期的文章倒序（没有文章的排最后），游标为 `(coalesce(last_published_at, '-infinity'), id)`。`last_published_at` 同时供前端生成 `sitemap.xml` 的 `lastmod`。
- 博客页：该博客在 `entries` 里的全部文章，包括没有日期和日期不可信的，按发布时间倒序、没有日期的排最后。

---

## 5. 数据保留

| 数据 | 保留 |
|---|---|
| `entries` | 不需要清理任务：同步事务让它始终等于订阅源的当前快照 |
| `submissions` | 待审核的一直保留；已通过或已拒绝的，90 天 `[待定]` 后由 worker 的每日维护任务删除 |
| `excluded_hosts` | 永久 |
| `blogs` | 移除博客时删除这一行，`entries` 随之级联删除 |

备份时排除缓存，本身就是 §0.1 的一个演示：

```bash
pg_dump --exclude-table-data=entries "$EXPLORE_DATABASE_URL" > explore.sql
```

---

## 6. schema 守护

两条集成测试，从第一个迁移起就在 CI 里跑：

1. **列清单黄金文件**：从 `information_schema.columns` 读出所有表的列，与 `testdata/schema/columns.txt` 比较。任何加列都会让测试失败，必须同时更新这个文件，于是在 code review 里一目了然。给 `entries` 加正文字段、给任何表加读者信息，都过不了这一关。
2. **清空恢复**（[architecture.md §0.1](architecture.md#0-两个核心判断) 的可验证条款，`internal/worker` 的 `TestClearingTheCacheLosesNothing`）：用本地 HTTP 服务提供夹具订阅源，跑一轮 worker，记下首页时间流、目录和博客页；`TRUNCATE entries` 之后再跑一轮，输出必须完全相同。测试服务器对旧的 ETag 仍会回 `304`，所以它同时证明了 worker 在缓存为空时不发条件请求。

---

## 7. 迁移

- 工具用 `goose` `[设计中]`：SQL 文件放在 `migrations/`，命名为 `00001_init.sql` 这样的递增序号，通过 `embed` 打进二进制，由 `explore migrate up` 执行。
- 生产环境只向前迁移。`-- +goose Down` 段可以留空，回滚靠新的迁移。
- 查询手写在 `internal/store` 里，用 pgx 执行，没有代码生成步骤。每条查询都由集成测试在真实的 PostgreSQL 上验证（[project-layout.md §7](project-layout.md#7-测试)）。
- 改表结构的迁移与本文同一个 PR 提交；与本文冲突时，先改文档。

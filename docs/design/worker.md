# 抓取与规范化（worker）

> 状态：设计中，阈值待 E0 回填 · 最近更新：2026-09-23
> 规则的产品含义见 [architecture.md §6](architecture.md#6-抓取与展示规则)，本文写实现。文中的数值都来自那里，代码里只在 `internal/policy` 定义一次。

---

## 1. 流水线

```text
claim due blogs
      |
      v
    fetch ----- unchanged (304 or same body hash) ----> reschedule
      |
      v
    parse (RSS / Atom / JSON Feed)
      |
      v
    normalize (links, identity, dates, excerpt, snapshot)
      |
      v
    sync snapshot in one transaction ----> reschedule
```

每一步失败都停在原地、只记录失败、按退避重新调度，**不会**碰已有的 `entries`（[data-model.md §3](data-model.md#3-同步事务)）。

`explore check`（§6）复用同一套 fetch、parse、normalize，只是不写数据库，而是输出一份报告。

---

## 2. 调度

### 2.1 领取到期的博客

```sql
UPDATE blogs
SET next_fetch_at = now() + @lease::interval
WHERE id IN (
    SELECT id FROM blogs
    WHERE status = 'active' AND next_fetch_at <= now()
    ORDER BY next_fetch_at
    LIMIT @batch
    FOR UPDATE SKIP LOCKED
)
RETURNING id, host, feed_url, etag, last_modified, body_hash,
          fetch_interval, consecutive_failures, show_excerpt, extra_domains;
```

- 领取时先把 `next_fetch_at` 推后一个租期（10 分钟）。worker 在抓取中途崩溃的话，租期一过这个博客会被重新领取。
- `SKIP LOCKED` 让多个 worker 可以同时领取而互不重复。V1 只跑一个 worker，将来加 worker 不需要改任何东西。
- worker 每 30 秒领取一批，批大小等于并发数（`EXPLORE_WORKER_CONCURRENCY`）。每个博客一个主机名，同一主机天然只有一个请求在进行。

### 2.2 下次抓取时间

| 情况 | 新的 `fetch_interval` | `next_fetch_at` |
|---|---|---|
| 有变化 | 基础间隔 60 分钟 | now + 间隔 |
| 没有变化 | min(间隔 × 1.5, 6 小时) | now + 间隔 |
| 失败（第 k 次连续失败） | 不变 | now + min(60 分钟 × 2^(k−1), 24 小时) |
| `429` / `503` 带 `Retry-After` | 不变 | now + `Retry-After`，限制在 60 分钟到 24 小时之间 |
| 维护者要求立即抓取 | 不变 | now |

所有延迟都乘一个 0.9 到 1.1 之间的随机系数，避免大批博客在同一时刻被请求。

---

## 3. 抓取（`internal/fetch`）

### 3.1 请求

```http
GET /rss.xml HTTP/1.1
User-Agent: KiteExplore/0.1 (+https://explore.kite.plus/bot)
Accept: application/rss+xml, application/atom+xml, application/feed+json, application/xml;q=0.9, text/xml;q=0.9, */*;q=0.8
If-None-Match: "..."
If-Modified-Since: ...
```

- `Accept` 必须包含 `application/rss+xml` 或 `*/*`：Halo 按 `Accept` 匹配订阅路由（[architecture.md §5.1](architecture.md#5-接入的博客系统)）。
- 不手动设置 `Accept-Encoding`，让 Go 的 `Transport` 自动协商并解压 gzip。
- WordPress 同时收到 `If-None-Match` 和 `If-Modified-Since` 时要求两者都匹配；两个值都来自同一台服务器的上一次响应，正常情况下会同时匹配。

### 3.2 限制

| 项 | 值 |
|---|---|
| 连接 / TLS 握手 / 等待响应头 / 总时长 | 5 秒 / 5 秒 / 10 秒 / 15 秒 |
| 响应体 | ≤ 5 MiB，读到第 5 MiB + 1 字节即判定 `too_large` |
| 重定向 | ≤ 5 次，只允许 `http`、`https` |
| 端口 | 只允许 80、443 `[设计中]` |
| 代理 | 不读环境变量里的代理设置 |

### 3.3 SSRF 防护

订阅地址来自外部提交，重定向又可能把请求带到任何地方，所以在**建立连接的那一刻**检查对方 IP（`net.Dialer.Control`），而不是只检查 URL：

- 拒绝回环、私有网段（`10/8`、`172.16/12`、`192.168/16`、`fc00::/7`）、链路本地（`169.254/16`、`fe80::/10`，包括云厂商的元数据地址）、运营商级 NAT（`100.64/10`）、组播和未指定地址，也包括它们的 IPv4 映射形式。
- 每次连接都检查，所以对重定向和 DNS 重绑定同样有效。
- `EXPLORE_ALLOW_PRIVATE_NETWORKS=true` 可以关掉这项检查，只用于测试和本地开发（例如检查 `http://127.0.0.1:8090/` 上的本地 Halo）。

### 3.4 结果分类

| 结果 | 处理 |
|---|---|
| `200` | 做变化检测（§3.6），有变化就进入解析 |
| `304` | 没有变化 |
| `301`、`308` 永久重定向 | 跟随。新地址与博客属于同一个可注册域名时，自动更新 `feed_url`；否则写入 `last_error` 等维护者处理 |
| `410 Gone` | 记录 `gone_since`，博客立即不再展示；之后任何一次成功都会清空它。持续 7 天后，每日维护任务移除博客并写入 `excluded_hosts`（`opt_out`） |
| `429`、`503` 带 `Retry-After` | 按 §2.2 推迟 |
| 其他 `4xx`、`5xx`，超时，TLS 错误，`too_large`，解析失败 | 失败：`consecutive_failures + 1`，写 `last_error`，按 §2.2 退避 |
| robots.txt 明确禁止 | 不发请求，与 `410 Gone` 相同处理：作者表达了不想被抓取，这是退出信号。`last_error` 为 `robots_disallowed` |
| robots.txt 返回 `5xx` 或网络不通 | 不发请求，按普通失败处理：这是服务器故障，不是作者退出 |

连续 7 天没有成功的博客会自动从页面上消失，恢复后自动回来（[data-model.md §2.1](data-model.md#21-blogs)）。

### 3.5 robots.txt

- 每个主机缓存 24 小时，只放在内存里。
- 按 RFC 9309 解析：有针对 `KiteExplore` 的规则组就用它，没有就用 `*` 组。
- robots.txt 返回 `4xx` 时视为允许全部；返回 `5xx` 或网络不通时视为全部禁止（RFC 9309 的要求），但按普通失败处理，不当作退出（§3.4）。
- 检查（§6）会把 `robots_disallowed` 报给作者：作者主动提交了博客，却被自己的 robots.txt 挡住，是常见的困惑。

### 3.6 变化检测

1. `304`：没有变化。
2. 否则计算响应体的 SHA-256，与 `body_hash` 相同就算没有变化。Halo 和 Typecho 不支持条件请求，靠这一步省掉解析。本地 Halo 2.26.1 连续两次请求的响应完全相同 `[EV]`，这一步对它有效。
3. 否则进入解析。

---

## 4. 解析（`internal/feed`）

- 用 `gofeed` 统一解析 RSS 2.0、RSS 1.0、Atom 和 JSON Feed，转换成与格式无关的模型：

  ```go
  type Feed struct {
      Title, SiteURL, Language, Generator string
      Items                               []Item
  }

  type Item struct {
      ID, Link, Title    string
      Summary, Content   string
      Published, Updated *time.Time
  }
  ```

- 非 UTF-8 编码（例如 GBK）的处理方式在 E0 验证：靠 `gofeed` 自身的支持，还是在解析前统一转码。
- 日期解析要宽松，E0 用 Halo 的一位数日期（`Thu, 3 Sep 2026 01:02:03 GMT`）验证。
- 解析失败只报错，**绝不**当作空订阅源。
- `Content`（全文）只在内存里停留到规范化结束，用来在没有 `Summary` 时生成摘要，之后立即丢弃。

---

## 5. 规范化

`internal/normalize` 是纯函数：输入解析结果、博客配置和当前时间，输出条目和问题计数，不做任何 I/O，所以可以用夹具做黄金文件测试。

### 5.1 逐条处理

1. **链接**：去掉首尾空白；相对链接按订阅地址解析成绝对地址；只接受 `http`、`https`。主机必须与博客属于**同一个可注册域名**（用 public suffix list 计算），或在 `extra_domains` 里；否则丢弃，记 `link_off_domain`。没有链接的条目丢弃，记 `no_link`。
   - 博客是 `www.example.com`：允许 `example.com`、`blog.example.com`。
   - 博客是 `alice.github.io`：`github.io` 是公共后缀，只允许 `alice.github.io` 及其子域名，`bob.github.io` 不行。
2. **身份键**：`guid` 或 `id` 去掉首尾空白后非空就用它（超过 500 字符时改用它的 SHA-256）；否则用规范化后的链接（协议和主机小写，去掉默认端口和 `#` 片段）。身份键只用来去重，永远不当链接访问。
3. **标题**：去掉 HTML 标签、还原实体、合并空白。为空时丢弃，记 `no_title` `[待定]`；超过 300 字截断并加省略号。
4. **发布时间**：`Published`，没有就用 `Updated`。早于 1990 年的（包括 Hugo 输出的 `0001-01-01`）视为没有。
5. **摘要**：只在 `show_excerpt` 为真时生成。来源是 `Summary`，为空时用 `Content`；先用 `bluemonday` 去掉全部 HTML、还原实体、合并空白，**再**截断到 140 字（省略号计入 140）。绝不在去掉标签之前截断，Hexo 插件截断 HTML 的问题就出在这里（[architecture.md §5.1](architecture.md#5-接入的博客系统)）。
6. **去重**：同一快照里身份键重复的，保留第一次出现的那条。

### 5.2 整个快照

7. **日期可信**：有发布时间才可能可信。把所有有效条目按"精确到分钟的发布时间"分组，某一组超过 5 篇 `[待定]`，这一组全部标为不可信。这一步在截断之前、对全部有效条目做，才能发现 Hugo 那种几百篇的订阅源里的问题。
8. **截断**：按发布时间倒序排列（没有时间的排最后，同时间按订阅源里的顺序），只保留前 20 篇。
9. 未来时间**不在这里**处理：它随时间变化，由查询时比较 `now()`（[data-model.md §4.1](data-model.md#41-首页时间流)）。

输出：最多 20 条 `model.Entry`，以及各问题代码的计数（供检查报告和 `last_error` 使用）。

---

## 6. 检查（`internal/check`）

`internal/check` 不依赖数据库。它服务四个地方：命令行 `explore check <url>`、提交接口、维护者的检查接口，以及 E0 批量实测。

### 6.1 找到订阅源

输入可以是博客首页，也可以直接是订阅地址。

1. 请求输入地址。响应能按订阅源解析，就直接用它（`direct`）。
2. 否则当作 HTML 读取（≤ 1 MiB），找 `<link rel="alternate">`，类型是 `application/rss+xml`、`application/atom+xml` 或 `application/feed+json`，按页面上的顺序取第一个能解析的（`autodiscovery`）。
3. 否则按下表依次尝试站点根目录下的默认地址，取第一个能解析的（`candidate`）：

| 顺序 | 路径 | 常见于 |
|---|---|---|
| 1 | `/feed/` | WordPress，开启伪静态的 Typecho |
| 2 | `/rss.xml` | Halo、Kite |
| 3 | `/atom.xml` | Hexo |
| 4 | `/index.xml` | Hugo |
| 5 | `/feed.xml` | Jekyll、Halo |
| 6 | `/rss/` | Ghost |
| 7 | `/index.php/feed/` | 没开伪静态的 Typecho |

逐个串行请求，遵守同样的礼貌规则（User-Agent、robots.txt、限制）。

### 6.2 问题清单

有任何 `error` 级问题，检查就不通过；`warning` 和 `info` 只提示。提示语按识别出的博客系统给出具体做法。

| 代码 | 级别 | 含义 | 提示 |
|---|---|---|---|
| `feed_not_found` | error | 找不到订阅源 | Hexo：安装 `hexo-generator-feed`；Halo：确认订阅插件（plugin-feed）已启用；其他：提交时直接填写订阅地址 |
| `robots_disallowed` | error | robots.txt 禁止 `KiteExplore` 抓取订阅地址 | 在 robots.txt 里放行订阅地址 |
| `http_error` | error | 订阅地址返回非 2xx | —— |
| `too_large` | error | 订阅源超过 5 MiB | Hugo：设置 `services.rss.limit` |
| `parse_error` | error | 不是合法的 RSS、Atom 或 JSON Feed | —— |
| `links_off_domain` | error | 超过一半 `[待定]` 的文章链接不在博客的域名内 | Hexo：改 `_config.yml` 的 `url`；Hugo：改 `baseURL`；Halo：改 `halo.external-url`；WordPress：设置 → 常规 → 站点地址（URL）；Jekyll：改 `_config.yml` 的 `url` |
| `no_valid_items` | error | 没有一篇可用的文章 | —— |
| `stale` | error | 近 12 个月没有更新 | —— |
| `some_links_off_domain` | warning | 少数文章链接不在博客的域名内，这些文章会被跳过 | 博客确实跨多个域名的，提交时说明 |
| `no_dates` | warning | 有文章没有发布时间，它们不会出现在首页 | Hugo：在 front matter 里写 `date` |
| `dates_untrusted` | warning | 大量文章的发布时间相同 | Hexo：给每篇文章写上 `date` |
| `includes_non_posts` | warning | 订阅源里混有独立页面 | Hugo：改用 `/posts/index.xml` |
| `no_conditional_get` | info | 服务器不支持条件请求 | —— |
| `redirected` | info | 订阅地址永久重定向到了新地址 | 改用新地址 |

`includes_non_posts` 的判断 `[设计中]`：Hugo 站点用的是 `/index.xml`，其中有条目的路径只有一级（如 `/about/`），而且 `/posts/index.xml` 存在。

提示有简体中文和英文两个版本，和检查逻辑一起放在 `internal/check` 里，上表的"提示"列是中文版。命令行默认输出英文，`--lang zh-CN` 输出中文；接口按 `Accept-Language` 选择（[api.md §1](api.md#1-约定)）。代码不随语言变化。

### 6.3 报告格式

命令行的 `--json` 输出和 API 返回的是同一个结构（下例的 `hint` 是中文版）：

```json
{
  "input_url": "https://blog.example.com/",
  "feed_url": "https://blog.example.com/atom.xml",
  "discovered_by": "autodiscovery",
  "format": "atom",
  "generator": "hexo",
  "http": { "status": 200, "etag": true, "last_modified": true },
  "items": {
    "total": 20,
    "valid": 18,
    "trusted_dates": 18,
    "latest_published_at": "2026-09-20T02:00:00Z"
  },
  "problems": [
    {
      "code": "some_links_off_domain",
      "severity": "warning",
      "count": 2,
      "hint": "博客确实跨多个域名的，提交时说明"
    }
  ],
  "passed": true
}
```

`hint` 不入库：`submissions.check_report` 只存上面除 `hint` 以外的字段，接口返回时再按请求的语言，根据 `code` 和 `generator` 补上 `hint`。同一条提交用中文或英文查看，提示都是对应的语言。

---

## 7. 每日维护任务

worker 每天运行一次（多个 worker 时用 PostgreSQL 的 advisory lock 保证只有一个在跑）：

- 删除超过保留期的已处理提交（[data-model.md §5](data-model.md#5-数据保留)）；
- 移除 `gone_since` 超过 7 天的博客，写入 `excluded_hosts`；
- 输出一条健康摘要日志：各状态的博客数、7 天内没有成功抓取的博客数。

---

## 8. 日志

- 用 `log/slog` 输出 JSON。每次抓取一条：主机名、结果、状态码、耗时、字节数、是否有变化、新增和删除的条目数。
- worker 不接触读者，天然没有读者数据；API 的日志规则见 [project-layout.md §6](project-layout.md#6-http-服务约定)。
- 指标暴露（例如 Prometheus）`[待定]`，E3 做健康页时一起定。

---

## 9. 测试

- **夹具**：`testdata/feeds/<系统>/<场景>.xml`。Halo 和 Hugo 的从本地实例生成，WordPress 和 Hexo 的按官方模板手写。**不提交真实博客的内容**。
- **黄金文件**：`normalize` 对每个夹具的输出存成 `.golden.json`，用 `-update` 参数更新。
- **抓取**：用 `httptest` 模拟各种状态码、重定向、超大响应、慢响应和 robots.txt；SSRF 防护单独测试（包括重定向到内网地址的情况）。
- **不变量**：清空恢复测试和 schema 列清单测试（[data-model.md §6](data-model.md#6-schema-守护)）。

---

## 10. E0 要回答的问题

E0 对 WordPress、Halo、Hugo、Hexo 各至少 10 个真实博客运行 `explore check`，用数据回答下面的问题，结果回填到 architecture.md 的 `[待定]` 和本文：

| 问题 | 影响的规则 |
|---|---|
| 订阅源体积的分布，5 MiB 够不够 | §3.2 |
| 各系统支持条件请求的比例 | §2.2 的退避参数 |
| 摘要取多长读起来合适 | 摘要 140 字 |
| 每个博客保留 20 篇是否够用 | 快照截断 |
| 同一分钟发布超过几篇才算日期不可信 | §5.2 |
| 链接不在域名内的比例，是配置错误还是合理的跨域 | `links_off_domain` 的阈值 |
| 非 UTF-8 编码和不规范日期有多常见 | §4 |
| 没有标题的条目有多常见 | `no_title` |

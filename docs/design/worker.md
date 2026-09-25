# 抓取与规范化（worker）

> 状态：E1 已实现，阈值已按 E0 实测回填（§10） · 最近更新：2026-09-23
> 规则的产品含义见 [architecture.md §6](architecture.md#6-抓取与展示规则)，本文写实现。文中的数值都来自那里，代码里只在 `internal/policy` 定义一次。

---

## 1. 流水线

```text
claim due blogs
      |
      v
    fetch ----- unchanged (304 or same body hash) ----> refresh description if due ----> reschedule
      |
      v
    parse (RSS / Atom / JSON Feed)
      |
      v
    normalize (links, identity, dates, excerpt, snapshot)
      |
      v
    sync snapshot in one transaction ----> refresh blog description (weekly) ----> reschedule
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
RETURNING id, host, site_url, feed_url, description_checked_at, etag, last_modified, body_hash,
          fetch_interval, consecutive_failures, show_excerpt, extra_domains;
```

- 领取时一并返回这个博客在 `entries` 里有没有文章（§3.6 用它）。
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
| 连接 / TLS 握手 / 等待响应头 / 总时长 | 5 秒 / 5 秒 / 10 秒 / 30 秒。总时长原是 15 秒：从海外下载一个 1.6 MB 的订阅源实测要 13 秒，时过时不过 `[EV]` |
| 响应体 | ≤ 5 MiB，读到第 5 MiB + 1 字节即判定 `too_large` |
| 重定向 | ≤ 5 次，只允许 `http`、`https` |
| 端口 | 只允许 80、443 `[设计中]` |
| 代理 | 不读环境变量里的代理设置 |

### 3.3 SSRF 防护

订阅地址来自外部提交，重定向又可能把请求带到任何地方，所以在**建立连接的那一刻**检查对方 IP（`net.Dialer.Control`），而不是只检查 URL：

- 拒绝回环、私有网段（`10/8`、`172.16/12`、`192.168/16`、`fc00::/7`）、链路本地（`169.254/16`、`fe80::/10`，包括云厂商的元数据地址）、运营商级 NAT（`100.64/10`）、组播和未指定地址，也包括它们的 IPv4 映射形式。
- 每次连接都检查，所以对重定向和 DNS 重绑定同样有效。
- `EXPLORE_ALLOW_PRIVATE_NETWORKS=true` 可以关掉这项检查（连同端口限制），只用于测试和本地开发（例如检查 `http://127.0.0.1:8090/` 上的本地 Halo）。
- 被拒绝时，报告里写明是哪个地址。开发机上的代理工具如果开着 fake-IP 模式，会把所有域名解析到 `198.18.0.0/15` 这类保留网段，于是每个博客都被拒绝 `[EV]`；在这样的机器上检查，同样要打开上面的开关。生产服务器上的 DNS 返回真实地址，不受影响。

### 3.4 结果分类

| 结果 | 处理 |
|---|---|
| `200` | 做变化检测（§3.6），有变化就进入解析 |
| `304` | 没有变化 |
| `301`、`308` 永久重定向 | 跟随。新地址与博客属于同一个可注册域名时，自动更新 `feed_url`；否则写入 `last_error` 等维护者处理 |
| `410 Gone` | 记录 `gone_since`，博客立即不再展示；之后任何一次成功都会清空它。持续 7 天后，每日维护任务移除博客并写入 `excluded_hosts`（`opt_out`） |
| `429`、`503` 带 `Retry-After` | 按 §2.2 推迟 |
| 其他 `4xx`、`5xx`，超时，TLS 错误，`too_large`，解析失败 | 失败：`consecutive_failures + 1`，写 `last_error`，按 §2.2 退避 |
| robots.txt 里针对 `KiteExplore` 的规则组禁止了订阅地址 | 不发请求，与 `410 Gone` 相同处理：作者明确拒绝了 Explore，这是退出信号 |
| 只有 `*` 组禁止了订阅地址 | 不发请求，按普通失败处理：7 天后自动不再展示，但不移除，也不写排除名单。这常是写给搜索引擎的 SEO 模板，不是作者要退出；E0 在 Typecho 博客上遇到过 `[EV]` |
| robots.txt 返回 `5xx` 或网络不通 | 不发请求，按普通失败处理：这是服务器故障，不是作者退出 |

连续 7 天没有成功的博客会自动从页面上消失，恢复后自动回来（[data-model.md §2.1](data-model.md#21-blogs)）。

### 3.5 robots.txt

- 每个主机缓存 24 小时，只放在内存里。
- 重定向的每一跳都要过 robots.txt：先请求的地址允许、跳到的地址禁止，照样不抓。robots.txt 自己的重定向除外。E0 遇到过 `/index.php/feed/` 跳到被禁止的 `/feed/`，检查通过了，抓取时却被拒绝 `[EV]`。
- 按 RFC 9309 解析：有针对 `KiteExplore` 的规则组就用它，没有就用 `*` 组。
- robots.txt 返回 `4xx` 时视为允许全部；返回 `5xx` 或网络不通时视为全部禁止（RFC 9309 的要求），但按普通失败处理，不当作退出（§3.4）。
- 检查（§6）会把 `robots_disallowed` 报给作者：作者主动提交了博客，却被自己的 robots.txt 挡住，是常见的困惑。

### 3.6 变化检测

0. 博客在 `entries` 里没有文章时跳过这一节：不带条件请求头，也不比较哈希，直接解析重建（[data-model.md §3](data-model.md#3-同步事务)）。
1. 条件请求**只带一个校验值**：上次响应有 `Last-Modified` 就发 `If-Modified-Since`，否则发 `If-None-Match`。压缩层会改写 `ETag`（Apache 加 `-gzip` 后缀，nginx 和 Cloudflare 把它变成弱校验值），而 WordPress 收到两个校验值时要求两个都匹配才回 `304`。E0 在 300 个同时有两种校验值的订阅源上对比过：两个都发，269 个回 `304`；只发 `If-Modified-Since`，282 个；只发 `If-None-Match`，271 个。WordPress 的差别最大，46 个里分别是 28、39、29 `[EV]`。
2. `304`：没有变化。
3. 否则计算响应体的 SHA-256，与 `body_hash` 相同就算没有变化。Halo 和 Typecho 不支持条件请求，靠这一步省掉解析：E0 实测的 14 个 Halo 订阅源没有一个回 `304`，有 `Last-Modified` 的也用了 `CST` 这类非 GMT 的时区 `[EV]`。本地 Halo 2.26.1 连续两次请求的响应完全相同 `[EV]`，这一步对它有效。
4. 否则进入解析。

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

- 非 UTF-8 编码由 `gofeed` 按文档自己声明的编码转码，GBK 已验证 `[EV]`。E0 的 1,256 个订阅源里只有 1 个声明了别的编码（ISO-8859-1）。
- 日期解析要宽松：Halo 的一位数日期（`Thu, 3 Sep 2026 01:02:03 GMT`）能正确解析 `[EV]`。写了日期却解析不了的条目记 `DateUnreadable`，E0 里有这种条目的订阅源只有 5 个。
- 解析失败只报错，**绝不**当作空订阅源。
- `Content`（全文）只在内存里停留到规范化结束，用来在没有 `Summary` 时生成摘要，之后立即丢弃。
- 订阅源的 `Description` 可作为博客介绍的后备文本；优先使用首页的 `<meta name="description">`，再尝试 `og:description` 和 `twitter:description`。worker 每 7 天最多读取一次首页，使用同一套 robots、重定向、大小和地址限制，描述清理成纯文本并截到 240 字。首页或订阅源没有可用描述时保留上一次的值；`304` 不影响描述刷新。

---

## 5. 规范化

`internal/normalize` 是纯函数：输入解析结果、博客配置和当前时间，输出条目和问题计数，不做任何 I/O，所以可以用夹具做黄金文件测试。

### 5.1 逐条处理

1. **链接**：去掉首尾空白；相对链接按订阅地址解析成绝对地址；只接受 `http`、`https`。主机必须与博客属于**同一个可注册域名**（用 public suffix list 计算），或在 `extra_domains` 里；否则丢弃，记 `link_off_domain`。没有链接的条目丢弃，记 `no_link`。
   - 博客是 `www.example.com`：允许 `example.com`、`blog.example.com`。
   - 博客是 `alice.github.io`：`github.io` 是公共后缀，只允许 `alice.github.io` 及其子域名，`bob.github.io` 不行。
2. **身份键**：`guid` 或 `id` 去掉首尾空白后非空就用它（超过 500 字符时改用它的 SHA-256）；否则用规范化后的链接（协议和主机小写，去掉默认端口和 `#` 片段）。身份键只用来去重，永远不当链接访问。
3. **标题**：去掉 HTML 标签、还原实体、合并空白。为空时丢弃，记 `no_title`；超过 300 字截断并加省略号。E0 里有无标题条目的订阅源占 2%，多是 Hugo 的零散页面，个别是全是短笔记的博客，它们不适合只列标题的时间流。
4. **发布时间**：`Published`，没有就用 `Updated`。早于 1990 年的（包括 Hugo 输出的 `0001-01-01`）视为没有。
5. **摘要**：只在 `show_excerpt` 为真时生成。来源是 `Summary`，为空时用 `Content`；先用 `golang.org/x/net/html` 提取纯文本（去掉脚本和样式的内容，块级元素之间补一个空格，不是 HTML 元素的标签如 `<T>` 保留原样）、合并空白，**再**截断到 140 字（省略号计入 140）。绝不在去掉标签之前截断，Hexo 插件截断 HTML 的问题就出在这里（[architecture.md §5.1](architecture.md#5-接入的博客系统)）。
6. **缩略图地址**：只在 `show_excerpt` 为真时提取。优先取正文或摘要 HTML 中第一张未标为极小尺寸的图片，再取订阅源的独立图片字段；只保留合法的 HTTP(S) 地址，图片字节不入库。展示时由本站图片接口校验并转发（[api.md §2.1.2](api.md#212-get-apiv1entriesidimage)）。
7. **去重**：同一快照里身份键重复的，保留第一次出现的那条。

### 5.2 整个快照

7. **日期可信**：有发布时间才可能可信。把所有有效条目按"精确到分钟的发布时间"分组，某一组超过 5 篇，这一组全部标为不可信（阈值的依据见 §10.4）。这一步在截断之前、对全部有效条目做，才能发现 Hugo 那种几百篇的订阅源里的问题。
8. **截断**：按发布时间倒序排列（没有时间的排最后，同时间按订阅源里的顺序），只保留前 20 篇。
9. 未来时间**不在这里**处理：它随时间变化，由查询时比较 `now()`（[data-model.md §4.1](data-model.md#41-首页时间流)）。

输出：最多 20 条 `model.Entry`，以及各问题代码的计数（供检查报告和 `last_error` 使用）。

---

## 6. 检查（`internal/check`）

`internal/check` 不依赖数据库。它服务四个地方：命令行 `explore check <url>`、提交接口、维护者的检查接口，以及 E0 批量实测。

### 6.1 找到订阅源

输入可以是博客首页，也可以直接是订阅地址。

1. 请求输入地址。响应能按订阅源解析，就直接用它（`direct`）。
2. 否则当作 HTML 读取，找 `<link rel="alternate">`，类型是 `application/rss+xml`、`application/atom+xml` 或 `application/feed+json`，按页面上的顺序取第一个能解析的（`autodiscovery`）。
   - 页面上没有订阅链接、只有 `<meta http-equiv="refresh">` 时，先跟过去一次（≤ 1 MiB），在那个页面上找。多语言的 Hugo 站点把根目录跳到 `/zh/`、`/en/`，订阅链接在语言目录的页面上 `[EV]`。
3. 否则按下表依次尝试站点根目录下的默认地址，取第一个能解析的（`candidate`）：

| 顺序 | 路径 | 常见于 |
|---|---|---|
| 1 | `/feed/` | WordPress，开启伪静态的 Typecho，部分 Halo |
| 2 | `/?feed=rss2` | 使用"朴素"固定链接的 WordPress，主题又没有声明订阅链接 `[EV]` |
| 3 | `/rss.xml` | Halo、Kite |
| 4 | `/atom.xml` | Hexo |
| 5 | `/index.xml` | Hugo |
| 6 | `/feed.xml` | Jekyll、Halo |
| 7 | `/rss/` | Ghost |
| 8 | `/index.php/feed/` | 没开伪静态的 Typecho |

逐个串行请求，遵守同样的礼貌规则（User-Agent、robots.txt、限制）。找到了订阅源却被 robots.txt 禁止或超过 5 MiB 时，报 `robots_disallowed` 或 `too_large`，而不是笼统的 `feed_not_found`。

页面声明的订阅地址打不开时，按原因分开：返回 `4xx`，或者返回的不是订阅源，就当作没有，继续往下找，最后报 `feed_not_found`。Hexo 主题没装订阅插件时，页面照样声明 `/atom.xml`，这正是要提示去装插件的情况。超时、`5xx`、订阅源格式损坏，则是服务器的问题，报 `http_error` 或 `parse_error`，细节里写明是哪个地址。默认地址打不开什么也不说明，大多数网站本来就没有。

记下首页经过重定向后的最终地址：它落在另一个网站、文章链接也都指向那里时，报 `site_moved`（§6.2）。

### 6.2 问题清单

有任何 `error` 级问题，检查就不通过；`warning` 和 `info` 只提示。提示语按识别出的博客系统给出具体做法。

| 代码 | 级别 | 含义 | 提示 |
|---|---|---|---|
| `feed_not_found` | error | 找不到订阅源 | Hexo：安装 `hexo-generator-feed`；Halo：确认订阅插件（plugin-feed）已启用；其他：提交时直接填写订阅地址 |
| `robots_disallowed` | error | robots.txt 禁止 `KiteExplore` 抓取订阅地址 | 在 robots.txt 里放行订阅地址 |
| `http_error` | error | 订阅地址返回非 2xx | —— |
| `too_large` | error | 订阅源超过 5 MiB | Hugo：设置 `services.rss.limit` |
| `parse_error` | error | 不是合法的 RSS、Atom 或 JSON Feed | —— |
| `links_off_domain` | error | 超过一半的文章链接不在博客的域名内，而且订阅源声明的站点地址也不在博客的域名内：站点地址配错了，或者博客搬了家、旧地址没有重定向 | 搬了家就提交新地址；否则 Hexo：改 `_config.yml` 的 `url`；Hugo：改 `baseURL`；Halo：改 `halo.external-url`；WordPress：设置 → 常规 → 站点地址（URL）；Jekyll：改 `_config.yml` 的 `url` |
| `site_moved` | error | 输入的地址重定向到了另一个网站，超过一半的文章链接也指向那里：博客搬了家，提交的是旧地址。细节里是新地址 | 提交新地址 |
| `no_valid_items` | error | 没有一篇可用的文章 | —— |
| `stale` | error | 近 12 个月没有更新 | —— |
| `links_elsewhere` | error | 超过一半的文章链接指向别的网站，但订阅源声明的站点地址是对的：订阅源本来就在链接外部（例如 gohugo.io 的订阅源是 GitHub 上的发布说明 `[EV]`） | Explore 只收录发布在博客本身的文章 |
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

## 10. E0 实测

2026-09-23 完成，结论已回填到 architecture.md §6 和本文。原始记录只留在跑实测的机器上，这里只写汇总。

### 10.1 方法

- 工具是 `explore survey`（[project-layout.md §3](project-layout.md#3-命令)）：对清单里的每个博客运行检查，再测量订阅源，包括体积、声明的编码、带着校验值再请求一次时是否回 `304`、条目数、落在时间流窗口里的条目数、同一分钟最多几篇、没有日期和日期解析不了的条目、域外链接、无标题条目、摘要和正文的长度。`--report` 把记录按博客系统汇总成表。
- 样本：
  - [中文独立博客列表](https://github.com/timqian/chinese-independent-blogs)里的全部 1,484 个博客，只给首页地址，顺带测自动发现；
  - [Kagi Small Web](https://github.com/kagisearch/smallweb) 里随机抽的 300 个英文博客（固定随机种子，去掉 Substack、Medium、wordpress.com 这类托管平台），直接给订阅地址。
- 博客系统按订阅源的 `generator` 识别，没有时看首页的 `<meta name="generator">`：WordPress 178 个、Halo 15、Hugo 224、Hexo 227、其他 455（Jekyll 82、Ghost 29、Typecho 26 等）、未识别 685（多数是连不上的站点）。首批四个系统都超过了每个 10 个的要求。
- 下面"回 `304`"一行是两个校验值一起发时测的，只发一个的对比见 §3.6。

### 10.2 单个站点

| 站点 | 系统 | 结果 |
|---|---|---|
| 本地 Halo 2.26.1 | Halo | 通过；靠默认地址找到 `/rss.xml`，不支持条件请求。用 `127.0.0.1` 访问时，文章链接写的是外部访问地址 `localhost`，报 `links_off_domain` |
| 本地 Hugo 0.157 站点 | Hugo | 通过；报 `no_dates`（没写日期的文章）和 `includes_non_posts`（首页订阅源混有 About） |
| wordpress.org/news | WordPress | 通过；自动发现 `/news/feed/`，10 篇，支持 `Last-Modified` |
| hexo.io | Hexo | 通过；自动发现 `/atom.xml`，20 篇，支持 `ETag` |
| ruanyifeng.com/blog | 其他 | 通过；订阅源托管在 FeedBurner，但文章链接指回博客本身 |
| gohugo.io | Hugo | 不通过：订阅源是 GitHub 上的发布说明，报 `links_elsewhere`。由此把"站点地址配错"和"本来就链接外部"分成了两个问题代码 |

### 10.3 结果

| | WordPress | Halo | Hugo | Hexo | 全部 |
|---|---|---|---|---|---|
| 博客 | 178 | 15 | 224 | 227 | 1,784 |
| 找到订阅源 | 175 | 14 | 219 | 206 | 1,256 |
| 通过检查 | 127 | 12 | 133 | 120 | 853 |
| 靠默认地址找到 | 26 | 10 | 21 | 20 | 211 |
| 体积：中位数 / p90 / 最大（KiB） | 62 / 271 / 2,346 | 121 / 476 / 486 | 99 / 1,261 / 4,545 | 212 / 907 / 4,681 | 89 / 712 / 4,681 |
| 回 `304` | 101 | 0 | 216 | 200 | 997 |
| 条目数：中位数 / 最大 | 10 / 227 | 20 / 116 | 27 / 1,024 | 20 / 276 | 18 / 1,161 |
| 30 天内超过 20 篇 | 0 | 0 | 0 | 1 | 9 |
| 输出全文 | 116 | 9 | 115 | 133 | 763 |
| 自带摘要的中位数（字） | 107 | 1,522 | 332 | 79 | 118 |
| 有没日期的条目 | 0 | 0 | 50 | 3 | 68 |
| 同一分钟超过 5 篇 | 0 | 0 | 15 | 2 | 45 |
| 大部分链接在域外 | 6 | 0 | 16 | 22 | 77 |

"全部"一列含其他系统和未识别的。没通过的原因（一个博客可能不止一个）：

| 原因 | 博客数 | 说明 |
|---|---|---|
| `http_error` | 332 | 连接被重置 109、超时 99、TLS 46、403 26、404 26。中文列表里不少博客已经下线；403 多是防爬挑战页 |
| `stale` | 315 | 近 12 个月没有更新 |
| `feed_not_found` | 164 | 其中 21 个是没装 `hexo-generator-feed` 的 Hexo；还有以 `200` 返回的防爬挑战页，只能报这个 |
| `no_valid_items` | 88 | 随链接问题一起出现，或订阅源是空的 |
| `links_off_domain` | 38 | 站点地址配错或搬家后旧站没有重定向 |
| `site_moved` | 38 | 旧地址重定向到了新域名 |
| `robots_disallowed` | 26 | 抽查的都是站长明确的规则，例如只放行搜索引擎，或禁止抓订阅地址 |
| `too_large` | 6 | 5.4 到 9.4 MiB，都输出全文、包含全部文章 |

### 10.4 结论

| 问题 | 数据 | 结论 |
|---|---|---|
| 订阅源体积，5 MiB 够不够 | 中位数 89 KiB，p90 712 KiB；1 MiB 以上 81 个（6%），超过 5 MiB 的 6 个（0.5%），4 个是 Hugo | 保持 5 MiB，超过时提示减少条目 |
| 各系统支持条件请求的比例 | 89% 的订阅源带校验值，79% 回 `304`：Hugo 99%、Hexo 97%、WordPress 58%、Halo 0 | 只发一个校验值（§3.6），对比样本里 WordPress 回 `304` 的比例从 61% 升到 85%。Halo 靠响应体哈希。轮询间隔不变 |
| 摘要取多长 | 订阅源自带的摘要，中文中位数约 100 字，英文约 260 个字符；140 字截断 36% 的中文摘要、71% 的英文摘要 | 暂时保持 140 字，交给 architecture.md §13 第 4 问 |
| 每个博客保留 20 篇够不够 | 30 天窗口里超过 20 篇的 9 个（0.7%），其中 2 个日期不可信 | 够用，保持 20 |
| 同一分钟超过几篇算日期不可信 | 有 2 篇同一分钟的订阅源 316 个，3 篇 134 个，超过 5 篇 45 个。这 45 个里约一半是 CI 构建时间或批量发布（例如 253 篇里 250 篇同一分钟），另一半落在同一天零点，是只写了日期的旧文批量导入 | 保持 5：2、3 篇同一分钟的，多是只写日期、同一天发的正常文章 |
| 链接不在域名内：配错还是合理跨域 | 有域外链接的 86 个订阅源两极分化：77 个 90% 以上在域外，9 个不到一半，中间没有。77 个里 38 个是博客搬了家；其余多是站点地址配错（Hexo 默认的 `example.com`、`yoursite.com`，自定义域名的站点还写着 GitHub Pages 地址），或搬家后旧站没有重定向 | 阈值保持一半。新增 `site_moved`；`links_off_domain` 的提示补上"搬了家就提交新地址" |
| 非 UTF-8 编码和不规范日期 | 声明其他编码的 1 个，有解析不了的日期的 5 个 | 不改规则 |
| 没有标题的条目 | 有这种条目的订阅源 25 个（2%），15 个是 Hugo | 保持丢弃 |

检查本身的这些改动也来自这次实测：跟随首页的 meta refresh（多语言 Hugo）；默认地址加上 `/?feed=rss2`；新增 `site_moved`；发现过程中订阅源超限或被 robots.txt 禁止时报具体原因；条件请求只带一个校验值。

### 10.5 以后

- 换一份清单再跑 `explore survey`，就是一次新的实测。E3 调轮询间隔和健康阈值前再跑一次。
- Typecho（26 个）和 Ghost（29 个）的样本还少，接入 §5.2 的系统之前单独补测。

---

## 11. 打标签（`internal/tagger`）

worker 里除了抓取，还有一个每分钟跑一轮的打标签循环。标签表、模型和费用见 [accounts.md §5](accounts.md#5-文章标签)。

1. 取出还没打标签（`tagged_at` 为空）的文章，新的在前，每轮最多 20 篇（`policy.TagsPerMinute`）。清空缓存后要重打的上万篇文章就这样慢慢补上，模型的费用也有了上限。
2. 逐篇把标题、摘要、文章自带的分类、博客的语言和维护者给博客设的默认标签发给模型，不发正文。固定的指令和标签表放在请求最前面，打上缓存标记。结构化输出把答案限定在标签表以内；结构化输出不支持数组长度，最多 3 个由代码截断。
3. 写回时核对标题：取出之后标题变了的，留给下一轮。
4. 模型拒答或输出被截断，都当作没有合适的标签，不再重试。接口出错时本轮停下，下次等待的时间逐次翻倍，最长一小时：密钥不对、服务中断、额度用完，下一篇多半也会失败。
5. 没配置模型和密钥时不启动这个循环，文章都不带标签，按标签筛选得到的是空列表。

`internal/tagger` 是唯一调用模型的包，这条依赖规则由 `scripts/check-imports.sh` 守住。worker 只认一个接口，由 `cli` 把两者接起来，所以 worker 的测试用的是假的实现，`tagger` 的测试用的是假的接口服务器。

## 12. 读文章页

订阅源不带缩略图，或摘要被截成「[…]」时，worker 读一次这篇文章的页面补上。读页有自己的循环，每 20 秒一轮（`policy.PageRoundEvery`），和站点地图的文章（§13）共用：

1. 取出订阅源缺图、缺摘要或摘要被截断，且 `page_next_check_at` 已到期的文章，每个博客最多一篇，每轮最多 30 篇（`policy.PageChecksPerRound`），领取时写入两分钟的租约。这一轮没轮到的博客，再各取一个待读的站点地图地址，同样最多 30 个，两类名额分开，补图的积压不会饿住站点地图。一个博客一轮最多被请求一页，也就是每分钟最多三页。
2. 按文章链接发一次 GET，遵守 robots.txt 和 SSRF 防护，只下载开头最多 256 KB（`policy.PageHeadBytes`），读到 `<body>` 就停，正文不解析也不保存。
3. 从 `<head>` 里取 `og:image`（其次 `twitter:image`），相对地址按页面地址补全，只收 http 和 https；取 `og:description`（其次 `description`、`twitter:description`），和订阅源摘要一样去掉 HTML、截成 ≤ 140 字。针对所有抓取器或 `KiteExplore` 的 robots meta：`noindex` 两样都不取，`nosnippet` 不取描述，`noimageindex` 不取图片。
4. 读到了（哪怕什么都没取到）、页面返回 404 这类确定结果、被 robots.txt 拒绝、不是 HTML，都算读过，`page_next_check_at` 置空，链接不变就不再读。网络出错、`429`、`5xx` 六小时后重试。
5. 展示时订阅源自己的图片和完整摘要优先；同一博客多篇文章共用的页面图片或描述是主题给的默认值，不展示（`store` 里的 `shownImage`、`shownExcerpt`）。

清空 `entries` 后，这些字段由这个循环按同样的规则重新读回，每个博客每分钟一篇，比订阅源本身慢一些，但结果一致（`TestClearingTheCacheLosesNothing`）。

## 13. 站点地图

订阅源只带最近的十几二十篇文章，更早的从博客的站点地图（sitemaps.org 协议）找到。站点地图只给地址，标题、发布时间、封面和摘要都从文章页 `<head>` 读出来，和 §12 一样不读正文。

1. **找站点地图**：每个可见博客每天读一次（`policy.SitemapCheckEvery`），读页循环里每分钟最多领 4 个博客。先看 robots.txt 的 `Sitemap:` 行，这份 robots.txt 和抓取规则共用缓存；没写的，依次试 `/sitemap.xml`、`/sitemap_index.xml`、`/wp-sitemap.xml`，找到一个就停。索引文件会展开，一个博客最多读 20 个文件（`policy.SitemapMaxFiles`），gzip 压缩的也能读，单个文件解压后最多 10 MB。只认博客自己站点（带不带 `www.` 都算）上的文件和地址。都找不到时七天后再试。
2. **挑出文章**：站点地图里还有首页、标签页、分类页和普通页面。拿订阅源里现有文章的链接当样本（`internal/sitemap` 的 `Learn`）：按路径层数、结尾斜杠、扩展名和查询参数分组，所有样本一致的路径段保留原样，数字按年、月日、编号归类，其余当通配，最后一段是文章自己的，从不保留原样。于是 `/posts/a/`、`/posts/b/` 学成「posts/任意」，`/tags/go/` 就不会被选中。没有样本时全部候选。选中的地址按 `lastmod` 从新到旧，最多留 1,000 个（`policy.SitemapMaxPosts`），写入 `sitemap_urls`；`lastmod` 变了就重新到期，不再列出的地址连同读出的文章一起删除。
3. **读文章页**：在 §12 的读页循环里，每个博客每轮最多一个地址，从新到旧。订阅源里已有的文章（`url_key` 相同）不读，等它挤出订阅源再读。页面要自称文章（`og:type` 为 `article`，或 JSON-LD 里有 Article、BlogPosting），或者给出发布时间，并且有标题，才算一篇文章。标题取 `og:title`，其次 `<title>`，去掉前后的博客名；发布时间取 `article:published_time`，其次 JSON-LD 的 `datePublished`，拿不到就当没有日期，不进首页时间流。robots meta 的 `noindex` 或 `none` 表示不要收录，这一页就不收。
4. **写入**：读出的文章以 `source = 'sitemap'`、`sitemap:` 开头的 `identity` 写进 `entries`，封面和摘要放在 `page_image_url`、`page_excerpt`，链接状态记为可访问，30 天后才再检查链接（`policy.SitemapLinkCheckInterval`）。写入前再看一次订阅源里是否已有这篇，有就不写；订阅源同步时也会删掉它已经覆盖的站点地图行，并让对应地址在它挤出订阅源后重新到期。

清空缓存会同时清空 `sitemap_urls`，读站点地图和读文章页按同样的规则重新补回（`TestSitemapsBringInOlderPosts`）。

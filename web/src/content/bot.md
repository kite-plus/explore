## KiteExplore

```text
User-Agent: KiteExplore/<version> (+https://explore.kite.plus/bot)
```

**中文** · KiteExplore 是 [Explore](/) 的抓取器。它读取博客作者主动提交、经过审核的博客的公开订阅源（RSS、Atom 或 JSON Feed）和站点地图，用来展示文章的标题、发布时间、一句短摘要和缩略图，读者点击后回到你的网站阅读。它不持久保存文章正文和图片文件。

它还会定时检查订阅源给出的原文链接是否有响应；读者也可以对尚未检测的文章发起一次检查。检查先发送 HEAD；需要确认时发送只请求首字节的 GET，不读取文章正文。检查结果只是提示，源站对抓取器和读者可能返回不同状态。

订阅源里的文章没有图片，或者摘要被截断时（例如 WordPress 的「[…]」），它会读一次这篇文章的页面，只取 `<head>` 里的 `og:image` 和 `og:description` 作为缩略图和摘要：读到 `<body>` 就停止下载，最多读 1 MB，不读取正文。页面里写给所有抓取器或 `KiteExplore` 的 robots meta 会被遵守：`noindex` 两样都不取，`nosnippet` 不取摘要，`noimageindex` 不取图片。

订阅源只带最近的文章，更早的从博客的站点地图找到：先看 robots.txt 里的 `Sitemap:`，没有时依次试 `/sitemap.xml`、`/sitemap_index.xml`、`/wp-sitemap.xml`。站点地图里看上去像文章的地址，同样只读页面的 `<head>`，取标题、发布时间、缩略图和摘要；页面写了 `noindex` 的不收录。站点地图不再列出的文章，Explore 也随之移除。

博客目录还会读取博客首页的简短描述，并按需读取 favicon，用本站图片接口展示头像；找不到图标时显示博客名称首字。文章缩略图和 favicon 都只在内存中短暂缓存。

为了给文章打标签，我们会把订阅源里的文章标题、短摘要、分类和博客语言发送给模型服务；只保存返回的标签，不发送正文或读者数据。

- **频率**：每个订阅源大约 60 分钟请求一次；连续没有更新时逐步放宽到 6 小时。服务器返回 `429` 或 `503` 时，按 `Retry-After` 推迟。
- **原文链接**：定时检查每篇最多每 24 小时一次；无法确认时 6 小时后重试。后台每分钟最多检查 8 篇，同一博客每轮最多 1 篇。读者只能提前检查尚未检测的文章，重复点击不会重复访问源站。
- **文章页**：每篇只读一次，链接变了、或站点地图给出新的修改时间才再读；每 20 秒一轮，同一博客每轮最多 1 篇，也就是每分钟最多 3 篇；出错时 6 小时后重试。
- **站点地图**：每个博客每天读一次，最多 20 个文件；找不到时 7 天后再试。
- **条件请求**：会带上次的 `Last-Modified`（没有时带 `ETag`），没有变化时你的服务器只需回 `304`。
- **robots.txt**：遵守 RFC 9309。有针对 `KiteExplore` 的规则组就用它，没有就用 `*` 组。
- **来自 Explore 的访问**：读者从 Explore 点进你网站的链接带 `utm_source=explore.kite.plus`（已经带了 `utm_source` 的链接不变），统计工具里能看到这些访问来自 Explore。链接里没有任何能识别读者的信息。
- **退出**：在 robots.txt 里写上 `User-agent: KiteExplore` 和 `Disallow: /`，或者对抓取器返回 `410 Gone`，或者[提交一个 Issue](https://github.com/kite-plus/explore/issues)。你的博客会从 Explore 移除，之后也不会再被收录。

**English** · KiteExplore is the crawler of [Explore](/en/). It reads the public feeds (RSS, Atom or JSON Feed) and sitemaps of blogs whose authors submitted them and that passed review. It uses them to show each post's title, publish date, short excerpt and thumbnail, and readers click through to your site to read. It does not persist post content or image files.

It also checks whether post links respond, and readers may start a check for a post that has not yet been checked. It sends HEAD first and, when needed, a GET requesting only the first byte; it does not read post bodies. The result is only a hint because a site may respond differently to the crawler and to readers.

When a post in the feed has no image or a cut excerpt, such as WordPress's “[…]”, it reads that post's page once and takes only `og:image` and `og:description` from its `<head>` as the thumbnail and excerpt. It stops downloading at `<body>`, reads at most 1 MB, and never reads the post itself. A robots meta tag for all crawlers or for `KiteExplore` is honored: `noindex` takes neither, `nosnippet` no excerpt, `noimageindex` no image.

A feed carries only recent posts, so older ones come from the blog's sitemap: the one robots.txt names with `Sitemap:`, or else `/sitemap.xml`, `/sitemap_index.xml` or `/wp-sitemap.xml`. For each address that looks like a post, it again reads only the page's `<head>` for the title, publish date, thumbnail and excerpt, and skips pages marked `noindex`. A post the sitemap no longer lists leaves Explore too.

The blog directory also reads a short description from a blog's home page and fetches its favicon on demand to show the avatar through a local image endpoint. If no icon is available, it shows the blog's initial. Post thumbnails and favicons are cached briefly in memory.

To tag posts, we send their feed titles, short excerpts, categories and blog language to a model service. We store only the resulting tags; we do not send full posts or reader data.

- **Frequency**: about one request per feed every 60 minutes, stretching to 6 hours while nothing changes. A `429` or `503` from your server delays it by `Retry-After`.
- **Post links**: scheduled checks run at most once every 24 hours per post, or retry after 6 hours when the result is uncertain. The worker checks at most eight links per minute and one per blog in each round. Readers can check an unchecked post early; repeated clicks do not fetch it again.
- **Post pages**: each is read once, and again only if its link changes or the sitemap gives it a new modification time. Rounds run every 20 seconds with at most one page per blog, so at most three per blog a minute, retrying after 6 hours on errors.
- **Sitemaps**: read once a day per blog, at most 20 files; when there is none, it tries again after 7 days.
- **Conditional requests**: it sends the last `Last-Modified`, or the `ETag` when there is none, so your server can answer `304` when nothing changed.
- **robots.txt**: follows RFC 9309, using a group for `KiteExplore` when there is one and the `*` group otherwise.
- **Visits from Explore**: links readers follow from Explore to your site carry `utm_source=explore.kite.plus` (links that already set `utm_source` stay as they are), so your analytics can tell these visits came from Explore. Nothing in the link identifies the reader.
- **Leaving**: add `User-agent: KiteExplore` with `Disallow: /` to robots.txt, answer the crawler with `410 Gone`, or [open an issue](https://github.com/kite-plus/explore/issues). Your blog is removed from Explore and will not be listed again.

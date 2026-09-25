## KiteExplore

```text
User-Agent: KiteExplore/<version> (+https://explore.kite.plus/bot)
```

**中文** · KiteExplore 是 [Explore](/) 的抓取器。它读取博客作者主动提交、经过审核的博客的公开订阅源（RSS、Atom 或 JSON Feed），用来展示文章的标题、发布时间、一句短摘要和缩略图，读者点击后回到你的网站阅读。它不持久保存文章正文和图片文件。

它还会定时检查订阅源给出的原文链接是否有响应；读者也可以对尚未检测的文章发起一次检查。检查先发送 HEAD；需要确认时发送只请求首字节的 GET，不读取文章正文。检查结果只是提示，源站对抓取器和读者可能返回不同状态。

订阅源里的文章没有图片，或者摘要被截断时（例如 WordPress 的「[…]」），它会读一次这篇文章的页面，只取 `<head>` 里的 `og:image` 和 `og:description` 作为缩略图和摘要：只下载页面开头最多 256 KB，读到 `<body>` 就停，不读取正文。页面里写给所有抓取器或 `KiteExplore` 的 robots meta 会被遵守：`noindex` 两样都不取，`nosnippet` 不取摘要，`noimageindex` 不取图片。

博客目录还会读取博客首页的简短描述，并按需读取 favicon，用本站图片接口展示头像；找不到图标时显示博客名称首字。文章缩略图和 favicon 都只在内存中短暂缓存。

为了给文章打标签，我们会把订阅源里的文章标题、短摘要、分类和博客语言发送给模型服务；只保存返回的标签，不发送正文或读者数据。

- **频率**：每个订阅源大约 60 分钟请求一次；连续没有更新时逐步放宽到 6 小时。服务器返回 `429` 或 `503` 时，按 `Retry-After` 推迟。
- **原文链接**：定时检查每篇最多每 24 小时一次；无法确认时 6 小时后重试。后台每分钟最多检查 8 篇，同一博客每轮最多 1 篇。读者只能提前检查尚未检测的文章，重复点击不会重复访问源站。
- **文章页**：每篇只读一次，链接变了才再读；后台每分钟最多读 8 篇，同一博客每轮最多 1 篇；出错时 6 小时后重试。
- **条件请求**：会带上次的 `Last-Modified`（没有时带 `ETag`），没有变化时你的服务器只需回 `304`。
- **robots.txt**：遵守 RFC 9309。有针对 `KiteExplore` 的规则组就用它，没有就用 `*` 组。
- **退出**：在 robots.txt 里写上 `User-agent: KiteExplore` 和 `Disallow: /`，或者对抓取器返回 `410 Gone`，或者[提交一个 Issue](https://github.com/kite-plus/explore/issues)。你的博客会从 Explore 移除，之后也不会再被收录。

**English** · KiteExplore is the crawler of [Explore](/en/). It reads the public feeds (RSS, Atom or JSON Feed) of blogs whose authors submitted them and that passed review. It uses them to show each post's title, publish date, short excerpt and thumbnail, and readers click through to your site to read. It does not persist post content or image files.

It also checks whether post links respond, and readers may start a check for a post that has not yet been checked. It sends HEAD first and, when needed, a GET requesting only the first byte; it does not read post bodies. The result is only a hint because a site may respond differently to the crawler and to readers.

When a post in the feed has no image or a cut excerpt, such as WordPress's “[…]”, it reads that post's page once and takes only `og:image` and `og:description` from its `<head>` as the thumbnail and excerpt. It downloads at most the first 256 KB, stops at `<body>`, and never reads the post itself. A robots meta tag for all crawlers or for `KiteExplore` is honored: `noindex` takes neither, `nosnippet` no excerpt, `noimageindex` no image.

The blog directory also reads a short description from a blog's home page and fetches its favicon on demand to show the avatar through a local image endpoint. If no icon is available, it shows the blog's initial. Post thumbnails and favicons are cached briefly in memory.

To tag posts, we send their feed titles, short excerpts, categories and blog language to a model service. We store only the resulting tags; we do not send full posts or reader data.

- **Frequency**: about one request per feed every 60 minutes, stretching to 6 hours while nothing changes. A `429` or `503` from your server delays it by `Retry-After`.
- **Post links**: scheduled checks run at most once every 24 hours per post, or retry after 6 hours when the result is uncertain. The worker checks at most eight links per minute and one per blog in each round. Readers can check an unchecked post early; repeated clicks do not fetch it again.
- **Post pages**: each is read once, and again only if its link changes. The worker reads at most eight pages per minute and one per blog in each round, retrying after 6 hours on errors.
- **Conditional requests**: it sends the last `Last-Modified`, or the `ETag` when there is none, so your server can answer `304` when nothing changed.
- **robots.txt**: follows RFC 9309, using a group for `KiteExplore` when there is one and the `*` group otherwise.
- **Leaving**: add `User-agent: KiteExplore` with `Disallow: /` to robots.txt, answer the crawler with `410 Gone`, or [open an issue](https://github.com/kite-plus/explore/issues). Your blog is removed from Explore and will not be listed again.

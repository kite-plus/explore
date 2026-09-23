## KiteExplore

```text
User-Agent: KiteExplore/<version> (+https://explore.kite.plus/bot)
```

**中文** · KiteExplore 是 [Explore](/) 的抓取器。它只读取博客作者主动提交、经过审核的博客的公开订阅源（RSS、Atom 或 JSON Feed），用来展示文章的标题、发布时间、一句短摘要和缩略图，读者点击后回到你的网站阅读。它不持久保存文章正文和图片文件。

博客目录还会读取博客首页的简短描述，并按需读取 favicon，用本站图片接口展示头像；找不到图标时显示博客名称首字。文章缩略图和 favicon 都只在内存中短暂缓存。

为了给文章打标签，我们会把订阅源里的文章标题、短摘要、分类和博客语言发送给模型服务；只保存返回的标签，不发送正文或读者数据。

- **频率**：每个订阅源大约 60 分钟请求一次；连续没有更新时逐步放宽到 6 小时。服务器返回 `429` 或 `503` 时，按 `Retry-After` 推迟。
- **条件请求**：会带上次的 `Last-Modified`（没有时带 `ETag`），没有变化时你的服务器只需回 `304`。
- **robots.txt**：遵守 RFC 9309。有针对 `KiteExplore` 的规则组就用它，没有就用 `*` 组。
- **退出**：在 robots.txt 里写上 `User-agent: KiteExplore` 和 `Disallow: /`，或者对抓取器返回 `410 Gone`，或者[提交一个 Issue](https://github.com/kite-plus/explore/issues)。你的博客会从 Explore 移除，之后也不会再被收录。

**English** · KiteExplore is the crawler of [Explore](/en/). It reads only the public feeds (RSS, Atom or JSON Feed) of blogs whose authors submitted them and that passed review. It uses them to show each post's title, publish date, short excerpt and thumbnail, and readers click through to your site to read. It does not persist post content or image files.

The blog directory also reads a short description from a blog's home page and fetches its favicon on demand to show the avatar through a local image endpoint. If no icon is available, it shows the blog's initial. Post thumbnails and favicons are cached briefly in memory.

To tag posts, we send their feed titles, short excerpts, categories and blog language to a model service. We store only the resulting tags; we do not send full posts or reader data.

- **Frequency**: about one request per feed every 60 minutes, stretching to 6 hours while nothing changes. A `429` or `503` from your server delays it by `Retry-After`.
- **Conditional requests**: it sends the last `Last-Modified`, or the `ETag` when there is none, so your server can answer `304` when nothing changed.
- **robots.txt**: follows RFC 9309, using a group for `KiteExplore` when there is one and the `*` group otherwise.
- **Leaving**: add `User-agent: KiteExplore` with `Disallow: /` to robots.txt, answer the crawler with `410 Gone`, or [open an issue](https://github.com/kite-plus/explore/issues). Your blog is removed from Explore and will not be listed again.

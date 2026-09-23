# 前端（web）

> 状态：公开页面、本站账号、订阅流与博客认领已实现 · 最近更新：2026-09-24
> 不能动摇的约束见 [architecture.md §8](architecture.md#8-前端与-seo)；接口见 [api.md](api.md)。本文写前端怎么满足它们。
> 本站账号、订阅流和账户页已实现，接口与未来 OIDC 迁移见 [accounts.md](accounts.md)。文章标签和首页标签筛选已实现。

---

## 1. 选型

| 方面 | 选择 |
|---|---|
| 框架 | Astro，`output: 'server'`：全部页面按需渲染。说明页也不预渲染：Node 适配器对预渲染的文件一律返回 `max-age=0`，按需渲染才能设置缓存头，CSP 也统一走响应头（§8） |
| 运行 | `@astrojs/node` 适配器，输出一个独立的 Node 服务，打进 Docker |
| 多语言 | 中英双语，用 Astro 自带的国际化路由（§3） |
| 样式 | Tailwind CSS v4，与 Kite 后台一致 |
| 组件 | shadcn/ui（官方支持 Astro）和 lucide 图标，与 Kite 后台一致 |
| 交互组件 | React（`@astrojs/react`），只用在"岛"里（§4） |
| 包管理与运行时 | pnpm、Node 22，与 Kite 相同 |
| 接口类型 | 目前手写在 `web/src/lib/types.ts`；`api/openapi.yaml` 写好后改为生成，与 Kite 后台从 `openapi.json` 生成的做法相同 `[待定]` |

### 1.1 为什么是 Astro

Explore 的页面几乎都是链接列表，交互很少。Astro 为这类站点设计：

- **默认不发 JavaScript**：页面在服务端渲染成纯 HTML，爬虫拿到的就是最终内容。基础页面只有主题切换脚本；含文章列表的页面另有状态检测与文章流式加载脚本（§4）。
- **严格 CSP 是自带功能**：`security.csp` 是稳定功能，会给 Astro 生成的脚本和样式自动加哈希，按需渲染的页面同样适用（§8）。
- **能沿用 Kite 后台的经验**：React 19、Tailwind v4、shadcn/ui 都能直接用，只是换一种用法（§4）。
- **部署简单**：一个普通的 Node 服务，和 Gin、PostgreSQL 放在同一个 Docker Compose 里，不依赖任何托管平台。

### 1.2 没有选的方案

| 方案 | 为什么不选 |
|---|---|
| 纯前端渲染的 SPA（例如 Kite 后台那样的 Vite + React） | 搜索引擎拿不到完整 HTML。百度执行 JavaScript 的能力远不如 Google，收录风险很大 |
| Next.js | SEO 能力足够，但每个页面都要带 React 运行时去水合，对链接列表网站偏重。更关键的是 CSP：[官方文档](https://nextjs.org/docs/app/guides/content-security-policy)写明，用 nonce 做严格 CSP 时所有页面都只能动态渲染，静态优化、ISR 和 CDN 缓存都会失效；不用 nonce 的替代方案 SRI 还是实验功能 |
| Nuxt、SvelteKit | 技术上可行，但会在 Go 和 React 之外再多一套技术栈 |
| Ant Design、MUI 这类组件库 | 靠 CSS-in-JS 在运行时往页面里插样式，和"默认零 JS"、严格 CSP 都冲突，服务端渲染时提取样式也麻烦 |

---

## 2. 页面与路由

下表是中文页面的路由；英文页面在前面加 `/en`，例如 `/en/blogs/{host}`（§3.1）。

| 路由 | 内容 | 数据 | 渲染 | `Cache-Control` | 搜索引擎 |
|---|---|---|---|---|---|
| `/` | 首页时间流，含可用的文章缩略图；可按博客语言和文章标签筛选 | `GET /api/v1/entries`、`GET /api/v1/tags` | 按需 | `public, max-age=60` | 无筛选时收录 |
| `/blogs` | 博客目录，含 favicon 和简短介绍 | `GET /api/v1/blogs` | 按需 | `public, max-age=300` | 收录 |
| `/blogs/{host}` | 一个博客的介绍、最新文章、文章标签和订阅地址 | `GET /api/v1/blogs/{host}`、`GET /api/v1/tags` | 按需 | `public, max-age=300` | 收录 |
| `/about` | 收录规则、退出方式、隐私说明 | —— | 按需，不调用 API | `public, max-age=86400` | 收录 |
| `/submit` | 提交博客的表单 | —— | 按需 | `no-store` | `noindex` |
| `/submissions/{id}` | 提交进度 | `GET /api/v1/submissions/{id}` | 按需 | `no-store` | `noindex` |

不分语言的地址：

| 路由 | 内容 | 渲染 |
|---|---|---|
| `/bot` | 抓取器说明，中英对照的单页：用途、频率、退出方式、联系方式。User-Agent 指向这里，看到它的站长可能说任何语言 | 按需，`public, max-age=86400` |
| `/sitemap.xml` | 两种语言的首页、目录、说明页和全部博客页 | 按需，`public, max-age=3600` |
| `/robots.txt` | 允许抓取，声明 sitemap 地址，禁止提交相关的页面 | 按需，`public, max-age=86400`；写成接口而不是静态文件，sitemap 地址才能跟着 `EXPLORE_PUBLIC_URL` |

- **带筛选或翻页的列表页**（`?lang=`、`?tag=`、`?cursor=`，可以组合）一律 `noindex, follow`：时间流一直在变，翻页后的内容没有收录价值，但爬虫仍会顺着链接去作者的博客。
- `/feed.xml`、`/blogs.opml`、`/api/` 不经过前端，由反向代理直接转给 Gin（§10）。
- **没有文章页**。文章只以指向原文的链接出现（[architecture.md §8](architecture.md#8-前端与-seo)）。
- 匿名读者没有 Cookie，公开页面对所有人相同；两种语言又是不同的地址，所以公开页面都可以被反向代理和 CDN 直接缓存。登录后的请求带会话 Cookie，返回个人化的页面，不缓存（[accounts.md §8](accounts.md#8-前端)）。

---

## 3. 多语言

界面中英双语。原则只有一条：**翻译界面，不翻译内容。**

### 3.1 URL

| 语言 | 前缀 | 例子 |
|---|---|---|
| 简体中文（默认） | 无 | `/`、`/blogs/blog.example.com` |
| 英文 | `/en` | `/en/`、`/en/blogs/blog.example.com` |

- 用 Astro 自带的国际化路由：`defaultLocale: "zh"`、`locales: ["zh", "en"]`、`routing.prefixDefaultLocale: false`（也是默认值），默认语言在根路径，其他语言加前缀。
- 中文放在根路径 `[设计中]`：首批接入的系统和读者以中文为主，最常用的地址最短。这个选择上线后再改，会影响已收录的页面，要调换就在 E2 开工前调换。
- **不按浏览器语言自动跳转，也不按它改变页面内容**。同一个地址对所有人（包括搜索引擎爬虫）返回相同的 HTML：缓存才能生效，爬虫也才能看到两种语言的页面。Astro 本身也不会自动跳转。
- 页眉放一个语言切换链接，指向另一种语言的同一个页面。

### 3.2 告诉搜索引擎

每个页面在 `<head>` 里声明两种语言的对应关系。Astro 不会自动生成，基础布局用 `EXPLORE_PUBLIC_URL` 加上本地化路径统一输出。没有用 `astro:i18n` 的 `getAbsoluteLocaleUrl`：它依赖构建时的 `site` 配置，而对外地址要在运行时读取，同一个镜像才能部署到任何域名。

```html
<link rel="alternate" hreflang="zh" href="https://explore.kite.plus/blogs/blog.example.com">
<link rel="alternate" hreflang="en" href="https://explore.kite.plus/en/blogs/blog.example.com">
<link rel="alternate" hreflang="x-default" href="https://explore.kite.plus/en/blogs/blog.example.com">
```

- `x-default` 指向英文版：浏览器语言既不是中文也不是英文的读者，英文更可能读得懂。
- canonical 指向当前语言的页面自己，两种语言之间不互相 canonical。
- `<html lang>`：中文页是 `zh-CN`，英文页是 `en`。Open Graph 的 `og:locale` 分别是 `zh_CN`、`en_US`，并用 `og:locale:alternate` 声明另一种。
- sitemap 同时列出两种语言的页面。

### 3.3 界面语言与博客语言分开

- 博客的标题和摘要**永远保持原文**，不做机器翻译。翻译会改变作者写下的内容，Explore 也就不再只是源站的索引（[architecture.md §0.1](architecture.md#0-两个核心判断)）。
- 每篇文章按所属博客声明的语言加 `lang` 属性。例如中文界面里的英文文章是 `<article lang="en">`，读屏软件和搜索引擎都能正确识别混排的内容。
- 博客语言筛选用查询参数 `?lang=`，对应接口的 `lang`，与界面语言无关：英文界面也可以只看中文博客。
- 首页标签筛选用 `?tag=`，取值是标签表里的英文短名。切换博客语言时保留标签，切换标签时保留博客语言；翻页同时保留两种筛选。
- 默认不筛选，两种界面都展示全部博客 `[待定]`，E2 看首批博客的语言分布再定。

### 3.4 文案

- 界面文案放在 `web/src/i18n/zh.ts` 和 `en.ts`。`en.ts` 的类型由 `zh.ts` 推导，少翻译一个键就编译失败。
- 说明页这类长文本，每种语言一个 Markdown 文件。`/bot` 是中英对照的单页，只有一个文件。
- 时间：`<time datetime>` 里写 ISO 时间；显示时，7 天内用相对时间（"3 小时前"、"3 hours ago"），更早的显示日期。中文按北京时间，英文按 UTC `[设计中]`。
- 接口返回的提示文字按语言返回（§3.5）。

### 3.5 接口返回的文字

- 接口里给人看的文字只有两类：错误的 `message`，和检查报告里的 `hint`。前端调用接口时，用 `Accept-Language` 带上当前页面的语言（`zh-CN` 或 `en`），接口按它返回（[api.md §1](api.md#1-约定)）。页面只按 `code` 做判断，不解析这些文字。
- 读者页面用到的接口（时间流、目录、博客页）不含这类文字，响应与语言无关，缓存不受影响。

---

## 4. 组件规则

**默认静态，需要交互才用岛。**

同一个 React 组件在 Astro 里有两种用法：

- **不加 `client:*` 指令**：只在服务端渲染成 HTML，不向浏览器发送任何 JS。
- **加 `client:*` 指令**：成为一个"岛"，在浏览器里激活，可以交互；这个页面会因此加载 React 运行时。

```astro
---
import { ButtonLink } from "@/components/ui/button";
import SubmitForm from "@/components/submit-form";
---

<ButtonLink href="/submit">提交博客</ButtonLink>
<SubmitForm client:visible />
```

第一行只输出一个带按钮样式的 `<a>`；第二行的表单才会在浏览器里运行。

规则：

1. React 组件默认**不加**指令。展示型组件（Button、Card、Badge、Table、Separator 等）都这样用。
2. 必须交互时才加指令。优先用 `client:visible` 或 `client:idle`，只有首屏必须立即可用的才用 `client:load`。
3. **能用 HTML 解决的交互不用 JS**：导航和折叠用 `<details>`，翻页、筛选和语言切换用链接，表单先做成普通的 HTML 表单。
4. 每个岛是独立的 React 实例，彼此不共享 Context。依赖 Provider 的组件（Toast、主题等）要和用它的组件放在同一个岛里；跨岛共享状态用 nanostores。
5. 不引入 CSS-in-JS 组件库（§1.2）。
6. 不加载外部字体：用系统字体栈。中文网络字体动辄几 MB，还会产生第三方请求。
7. 博客目录和博客页的大头像通过本站 `/api/v1/blogs/{host}/favicon` 加载源站 favicon；缺失时显示博客名首字和由主机名算出的颜色。文章列表的小头像仍使用首字。浏览器不向源站发图片请求。
8. **离开 Explore 的链接在新标签页打开**（原文、博客首页、订阅源、GitHub），读者看完还能回到信息流。提示要轻：文字后面一个淡色的 ↗，给读屏软件一段隐藏的"在新标签页打开"，首页说明里写一句；不用悬停提示，也不弹窗。组件里用 `ExternalLink.astro`，Markdown 页面的外链由 `ExternalLinks.astro` 统一改写。不加 `noreferrer`，作者的统计里仍能看到来自 Explore 的访问（[architecture.md §6.3](architecture.md#63-链接跳回源站的保证)）。站内链接照常在本页打开。

| 场景 | 做法 | 浏览器里的 JS |
|---|---|---|
| 按钮、卡片、徽章、表格 | shadcn/ui 组件，不加指令 | 无 |
| 链接样式的按钮 | `<ButtonLink href="…">`，代替 `asChild`，服务端渲染不需要 slot 原语 | 无 |
| 导航菜单、折叠内容 | `<details>` | 无 |
| 翻页、博客语言筛选、文章标签筛选、界面语言切换 | 筛选用链接；翻页支持渐进增强流式加载（前 2 次自动追加，之后点击加载更早文章），无 JS 时仍为普通链接 | 流式加载脚本只在文章列表页面加载 |
| 提交表单 | 第一版用普通 HTML 表单；需要即时校验和加载状态时改成岛 | 无，或只在提交页有 |
| 深色模式切换 | 页头一个按钮，配一小段内联脚本（`src/scripts/theme.js`）。Astro 只给它打包的脚本加哈希，所以这段脚本的哈希在 `astro.config.mjs` 里算出来，写进 CSP | 每个页面约 2 KB，gzip 后不到 1 KB |
| 文章状态检测 | “待检测”在有 JS 时变成按钮，点击后调用本站 `/api/v1/entries/{id}/check`，正在检查时轮询结果；无 JS 时保持静态状态。脚本哈希写进 CSP | 只在文章列表页面加载 |

端到端测试守住这条规则：公开页面只有主题脚本，文章列表页面再有状态检测与流式加载脚本；各段内容均与源文件逐字相同（§10）。

首页把文章流入口与筛选放在同一条横向导航上。“发现”按发布时间展示全部可见文章，“订阅”进入仅包含当前用户订阅博客文章的个人流。博客语言用紧凑切换组，文章标签放进原生 `<details>` 菜单，选择后仍通过链接保留另一项筛选条件。

深色模式：

- 没有选择时跟随系统设置。点一下切到另一种；切到的恰好是系统当前的配色时，就忘掉选择，重新跟随系统，所以不需要"跟随系统"这第三个选项。
- 选择存在浏览器的 `localStorage` 里，不是 Cookie，也不会发给服务器。脚本在 `<head>` 里、首次绘制之前运行，保存过的选择不会闪一下别的配色。
- 配色用 CSS 的 `light-dark()` 写在同一组变量里，脚本只切换 `<html>` 上的 `data-theme`，由 `color-scheme` 决定用哪一半。不支持 `light-dark()` 的旧浏览器（Safari 17.5 之前）固定用浅色，也不显示按钮。
- 浏览器关了 JavaScript 时按钮不显示，页面照样跟随系统。

---

## 5. 数据获取

- **读取列表时只在服务端调用 API**：页面在服务端渲染时调用 Gin。提交表单先提交给前端服务，由它转发（§6）；读者主动检测文章时，浏览器请求同源 `/api/v1/entries/{id}/check`，开发环境由 Astro 转发，部署环境由反向代理转发 Gin。
- API 地址来自环境变量 `EXPLORE_API_URL`，部署时指向内网的 `serve`。
- 接口类型目前手写，与 api.md 保持一致；OpenAPI 写好后改为生成（§1）。
- 游标原样透传：`/?cursor=X` 对应 `GET /api/v1/entries?cursor=X`。
- 首页与博客页在服务端读取 `GET /api/v1/tags`，按界面语言显示标签名；标签表在前端服务进程内缓存一小时。标签表暂时取不到时，页面仍显示文章，但不显示没有对应名称的标签。
- 文章缩略图从 `image_url` 指向的本站接口懒加载；本地开发时由 Astro 的同名路由转发给 Gin，部署时反向代理直接把 `/api/` 交给 Gin。图片加载失败不影响标题、摘要和原文链接。
- 首页的 `lang` 和 `tag` 原样传给时间流接口；未知标签由接口返回 `400`，前端显示真正的 `404` 页面。
- **前端不做任何数据规则**：摘要截断、过滤、每日上限都在后端（[architecture.md §6](architecture.md#6-抓取与展示规则)），前端只负责展示。
- 错误处理：
  - API 返回 `404` 时，页面也返回真正的 `404`，而不是一个显示"没找到"的 `200` 页面（搜索引擎称之为"软 404"）。
  - API 超时（3 秒 `[待定]`）或返回 `5xx` 时，页面返回 `503` 并带 `Retry-After`，不缓存。故障期间，搜索引擎会稍后重试，而不会以为内容消失了。

---

## 6. 提交流程

1. `/submit`（英文 `/en/submit`）是一个普通的 HTML 表单（`method="post"`），字段有博客地址、订阅地址（可选）和备注。
2. 表单提交给前端服务，由它在服务端调用 `POST /api/v1/submissions`，并用 `Accept-Language` 带上页面语言（§3.5）。检查最长要 30 秒，浏览器自带的加载状态在第一版够用。
3. 按接口的结果处理：

| 接口结果 | 页面 |
|---|---|
| `201` | `303` 重定向到同一语言的 `/submissions/{id}` |
| `422 check_failed` | 重新显示表单，逐条列出检查报告里的问题和修复提示 |
| `409 already_pending` | 重定向到已有的那条提交 |
| `409 already_listed` | 提示已收录，链接到 `/blogs/{host}` |
| `403 excluded` | 说明该博客已退出或被屏蔽，以及如何联系维护者 |

4. **防跨站提交**：Astro 的 `security.checkOrigin` 默认开启，会检查按需渲染页面收到的表单 `POST` 的 `Origin` 头。
5. **限流**：前端服务代读者调用接口时带上 `X-Forwarded-For`，并把 `web` 服务加进 Gin 的 `EXPLORE_TRUSTED_PROXIES`，这样提交接口的限流拿到的是读者的真实地址（仍然只在内存里用，[api.md §1](api.md#1-约定)）。

---

## 7. SEO

- **每页**：`<title>`、`<meta name="description">`、canonical（基于 `EXPLORE_PUBLIC_URL` 的绝对地址）、Open Graph 信息，都用当前页面的语言。两种语言的对应关系见 §3.2。标题、描述写成纯文本。Open Graph 图片 `[待定]`：放一张站内的静态图即可，还没有做。
- **博客页**：标题形如"{博客名} - Explore"；描述由它最近几篇文章的标题组成，内容会随订阅源更新。
- **状态码**要真实：`404` 就是 `404`，故障就是 `503`（§5）。
- **sitemap.xml** 由前端写接口生成：Astro 官方的 [sitemap 集成](https://docs.astro.build/en/guides/integrations-guide/sitemap/)不支持按需渲染的动态路由，列不出博客页。每个博客页的 `lastmod` 取接口返回的 `last_published_at`。
- **robots.txt**：允许抓取，写上 `Sitemap:` 地址，禁止两种语言的提交页和进度页。
- **外链**：指向原文的链接是普通的 `<a href>`，不加 `nofollow`。
- **性能**：没有 JS、没有外部字体、只有一个 CSS 文件，Core Web Vitals 基本不用额外优化。
- **上线后**：把 sitemap 提交到百度搜索资源平台和 Google Search Console；站点验证用的 meta 标签通过环境变量注入。
- 结构化数据（JSON-LD）`[待定]`。

---

## 8. CSP 与隐私

- 启用 Astro 的 `security.csp`：自动为 Astro 生成的脚本和样式计算哈希。按需渲染的页面由 Astro 以**响应头**下发策略（只有预渲染页面才用 `<meta>`），所以 `frame-ancestors 'none'` 直接写进 `directives` 就能生效 `[EV]`。不使用 `<ClientRouter />`（视图过渡），它和这个功能不兼容。
- 策略以 `default-src 'self'` 为底，脚本和样式只认哈希，图片只允许本站和 `data:`。其他指令通过 `security.csp` 的 `directives` 追加。主题脚本是内联的，Astro 不替它算哈希，由 `astro.config.mjs` 读取源文件算好，放进 `scriptDirective.hashes`。
- **页面不输出内联 `style` 属性**：哈希管不到它，而 Astro 的 `directives` 也不接受 `style-src-attr`。所以博客头像的颜色用一组固定的 Tailwind 类按主机名挑选，而不是计算出颜色值；Markdown 的代码高亮也关掉了，因为 Shiki 用内联 `style` 上色。测试会检查每个公开页面都没有 `style` 属性（§10）。
- `Referrer-Policy: strict-origin-when-cross-origin` 是浏览器的默认值，反向代理可以再显式下发一次。
- 匿名访问不设 Cookie，登录后只有一个会话 Cookie（[accounts.md §2.3](accounts.md#23-会话)）；不接任何统计脚本，不加载外部字体和图片。深浅色的选择只存在读者自己的浏览器里（§4）。

---

## 9. 目录

```text
web/
├── src/
│   ├── pages/          routes; English pages live under pages/en/
│   ├── layouts/        base layout with head, SEO and hreflang tags
│   ├── views/          page bodies shared by the zh and en routes
│   ├── components/     Astro components and React islands
│   │   └── ui/         shadcn/ui components
│   ├── i18n/           zh.ts and en.ts interface strings
│   ├── content/        long-form pages in both languages
│   ├── lib/            API client, loaders, types, formatting
│   └── styles/         Tailwind entry
├── public/             favicon
├── test/               HTML tests against a stub API
├── astro.config.mjs
├── components.json     shadcn/ui config
├── package.json
└── Dockerfile
```

`web/` 在 E2 开工时创建，和后端在同一个仓库里：接口改动和页面改动可以在同一个 PR 里完成。

---

## 10. 开发、测试与部署

**配置**

| 变量 | 默认值 | 说明 |
|---|---|---|
| `EXPLORE_API_URL` | `http://127.0.0.1:8080` | Gin API 的内网地址 |
| `EXPLORE_PUBLIC_URL` | `https://explore.kite.plus` | canonical、hreflang、sitemap、Open Graph 用的对外地址 |
| `HOST`、`PORT` | `@astrojs/node` 的默认值 | 前端服务的监听地址 |
| `EXPLORE_SITE_VERIFICATION_BAIDU`、`EXPLORE_SITE_VERIFICATION_GOOGLE` | 空 | 搜索引擎站点验证的 meta 标签 |

**开发**：`pnpm dev`，`EXPLORE_API_URL` 指向本地的 `explore serve`（[project-layout.md §9](project-layout.md#9-本地开发)）。

**测试**：`pnpm test` 用 `node:test` 启动构建好的前端和一个桩 API（`test/stub-api.mjs`），直接请求 HTML 做断言，不需要浏览器：

- 标题、canonical、robots 和 hreflang 标签正确，两种语言的页面互相对应；
- `404` 与 `503` 的状态码正确；
- 公开页面只有主题切换脚本，文章列表页面另有状态检测与流式加载脚本；内容与源文件一致，哈希在 CSP 里；没有内联 `style` 属性、不发 Cookie，并带着 CSP 响应头；
- 主题脚本本身在 `test/theme.test.mjs` 里用一个模拟的页面测试：跟随系统、保存选择、切回系统、存储不可用、其他标签页的选择；
- 页面不引用任何外部域名的资源；指向站外的链接都在新标签页打开并带提示，站内链接不带 `target`；
- 文章标签、标签表缓存、语言与标签组合筛选、翻页保留筛选，以及带筛选页面的 `noindex, follow`；
- 有图文章输出本站缩略图地址，不让浏览器直接请求源站图片；无图文章仍是纯文字条目；
- 提交流程的各种结果，以及跨站提交被拒绝。

**部署**：`web` 服务和 `serve`、`worker`、`postgres` 在同一个 Compose 里（[project-layout.md §10](project-layout.md#10-部署)）。反向代理把 `/api/`、`/feed.xml`、`/blogs.opml`、`/healthz`、`/readyz` 转给 `serve`，其余转给 `web`，并按页面的 `Cache-Control` 缓存。

---

## 11. 管理后台（E3）

管理后台不需要 SEO，交互多。当前采用 Astro 的整页 React 岛：

管理页面挂在 `/admin`，与读者页面一起部署。

当前后台支持具有 `is_admin` 权限的本站账号登录，也保留配置的 Bearer Token 作为运维入口。匿名读者侧仍不设置 Cookie。实现细节见 [admin-frontend.md](admin-frontend.md)。

---

## 12. 待定

| 问题 | 说明 |
|---|---|
| 根路径放中文还是英文 | 当前设计是中文（§3.1）。上线后再改会影响已收录的页面，要调换就在 E2 开工前 |
| 默认是否按界面语言筛选博客 | 当前设计是不筛选（§3.3），E2 看首批博客的语言分布再定 |
| 结构化数据 | 可以上线后补，不影响已有页面 |

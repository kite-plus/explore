# 统一身份、订阅与标签

> 状态：本站账号、订阅流及其公开地址、OPML 导入导出、推荐评分和域名认领已实现；OIDC 和应用登录（§1.1）待接入 · 最近更新：2026-10-10
> 跨站身份和评论的架构见 [identity-and-comments.md](identity-and-comments.md)。

## 0. 边界

现阶段 Explore 提供邮箱和密码登录，密码使用 bcrypt 哈希保存，30 天服务端会话只存令牌哈希。后续统一身份服务接入 OIDC 时，通过 `user_identities` 将 `(issuer, subject)` 关联到现有 `users.id`；不按邮箱自动合并账号，因此订阅和博客归属不需要迁移。评论服务仍独立保存评论及审核状态。

读者不登录也能看全部公开内容，匿名访问零 Cookie。Explore 不记录浏览、点击和阅读行为，不根据个人行为推荐。

## 1. 登录与会话

- OIDC 尚未接入。接入时使用 Authorization Code + PKCE，并校验 `state`、`nonce`、ID Token 签名、`iss`、`aud` 和有效期。
- 统一身份键是 `(issuer, subject)`；邮箱、昵称、头像只用于展示。不同客户端的 `sub` 若因 pairwise 配置而不同，须在身份服务中配置一致的关联标识，不能按邮箱推断同一用户。
- 过渡阶段 Explore 接收本站密码；最少 12 字符，bcrypt 哈希存储。暂未提供邮箱验证或自助找回密码，部署时应说明这个限制；忘记密码的读者联系站点，由管理员在后台重置为临时密码，读者登录后在账号页改掉（[admin-operations.md](admin-operations.md#数据与权限边界)）。接入统一身份后，新注册只用 Kite Plus 账号；已有的本站账号保留密码登录，读者用密码登录一次、关联 Kite Plus 账号之后才关掉。账号不按邮箱自动关联，所以不能提前关掉密码登录，否则没关联的账号就再也登不进来。过渡期何时结束、到期仍未关联的账号怎么处理 `[待定]`。
- OIDC 成功后 Explore 建立自己的服务端会话；Cookie 限本站域名，设 `HttpOnly`、`Secure`、`SameSite=Lax`。接入 OIDC 时 Cookie 改名为 `__Host-explore_session`，防止 `kite.plus` 下的兄弟子域覆盖它（[identity-and-comments.md §3](identity-and-comments.md#3-协议与数据)）；本地 http 开发环境设不了这个前缀，沿用旧名。数据库只存会话令牌哈希，并记下 ID Token 里的 `sid`。退出 Explore 删除本站会话；读者在身份服务退出时，身份服务发来 back-channel logout，Explore 删除同一 `sid` 的会话。
- 登录回调只建立会话，不直接做订阅等数据变更。登录后跳回本站相对路径，再由读者确认订阅。
- 管理员是具有 `is_admin` 的本站账号。第一个管理员由安装向导创建（[project-layout.md §10](project-layout.md#10-部署)），之后由已有管理员在后台授予，或由服务器操作者运行 `explore users promote-admin EMAIL`。普通账号不能访问管理接口。
- 删除 Explore 资料会删除本站订阅和会话。删除统一身份账号后，身份服务推送用户删除事件，Explore 删除对应的本站账号；账号被停用或锁定时，Explore 撤销它的会话、暂停登录，重新启用后恢复。推送可能丢失，Explore 另外定时对账（[identity-and-comments.md §3](identity-and-comments.md#3-协议与数据)）。

### 1.1 应用登录 `[设计中]`

Kite 应用（iOS、Android）接入统一身份时，不在身份服务里登记自己的客户端，由 Explore 服务端代办登录（[identity-and-comments.md §3](identity-and-comments.md#3-协议与数据) 第 6 条）。身份服务只认识 Explore 这一个机密客户端，退出和删号事件都在服务端处理；应用拿到的仍是 Explore 的会话令牌，不持有身份服务的任何令牌。

```text
App      生成 verifier，challenge = SHA-256(verifier)
App → B  系统登录页打开 /auth/app/start?challenge=…
E → I    建应用登录交易，跳到身份服务（Explore 自己的 state、nonce、PKCE）
I → E    已登录则立即回跳，否则先登录或注册；/auth/callback 校验，交易记下用户和 sid
E → App  跳到 https://explore.kite.plus/app/auth/done?code=…，应用收到授权码
App → E  POST /api/v1/auth/app/exchange {code, verifier} → 会话令牌
```

B 是系统登录页，E 是 Explore 服务端，I 是 `id.kite.plus`。

1. **系统登录页**：iOS 用 `ASWebAuthenticationSession`，Android 用 Custom Tabs，不用应用内的 WebView：读者看得到真实地址，Passkey 和密码管理器照常可用，应用也拿不到密码。不开 `prefersEphemeralWebBrowserSession`：系统登录页和 Safari（Android 上是 Chrome）共用 Cookie，读者在浏览器里登录过 `id.kite.plus`，应用里点「登录」就直接完成。iOS 打开前会弹出「“Kite”想要使用“kite.plus”登录」的系统提示，这是系统行为，无法去掉。
2. **回调用应用认领的 https 地址**：`https://explore.kite.plus/app/auth/done`。iOS 用 Associated Domains 加 `ASWebAuthenticationSession.Callback.https`（iOS 17.4 起，应用最低 iOS 26），Android 用 App Links（`autoVerify`）；Explore 为此发布 `/.well-known/apple-app-site-association` 和 `/.well-known/assetlinks.json`，前者要 Apple 开发者账号的 Team ID，后者要 Android 发布证书的 SHA-256 指纹。不用 `explore://`：任何应用都能注册同一个 scheme，再自己发起一次登录、用自己的 verifier 兑换，读者以为在登录 Kite，会话却给了它（[RFC 8252](https://www.rfc-editor.org/rfc/rfc8252) §7.2、§8.6）。应用没装或认领没生效时，浏览器会真的打开这个地址，页面只提示「请在 Kite 应用中完成登录」，不显示也不使用授权码，授权码到期作废。
3. **建交易**：`GET /auth/app/start?challenge=…`，challenge 是 43 个字符的 base64url。Explore 建交易（模式 `app`，30 分钟有效，注册可能要等验证邮件），和网页登录一样用一个只在登录流程里用的 Cookie 把交易绑在发起它的浏览器上，然后跳到身份服务。读者在应用里选「换一个账号」时带上 `switch=1`，Explore 向身份服务请求 `prompt=select_account`（两种实现是否都支持，I0 核对）。
4. **回调**：和网页登录共用 `/auth/callback`，按交易模式分开。`app` 模式校验通过后不设 Cookie 会话，而是生成授权码（32 字节随机数，只存哈希，2 分钟有效，只能用一次），跳到第 2 条的回调地址。
5. **兑换**：`POST /api/v1/auth/app/exchange {code, verifier}`。`SHA-256(verifier)` 必须等于交易的 challenge，授权码未过期、未用过；在一个事务里把授权码标为已用并签发会话，返回 `{token, expires_at, user}`。任何一项不对都返回同一个 `400 invalid_grant`，与登录共用限流。授权码出现在地址里，可能留在系统日志里，没有 verifier 就换不出会话；攻击者把自己账号的授权码塞给读者的应用（登录 CSRF）也不成，因为应用手里的 verifier 对不上。应用只在自己发起了登录、手里有 verifier 时才处理回调。
6. **会话**：和网页同一张 `sessions` 表，时长相同，记下 `sid` 和来源（`web`、`ios`、`android`）。应用请求改用 `Authorization: Bearer <令牌>`，不再手写 `explore_session` Cookie：接入 OIDC 时网页的 Cookie 改名为 `__Host-explore_session`，旧名不能再接受，否则兄弟子域写一个旧名的 Cookie 就能把读者登录成别人。浏览器不会自动带 `Authorization` 头，所以 Bearer 请求不需要 CSRF 头。应用先发一版改用 Bearer，再切换登录方式。
7. **退出**：应用里「退出」删除 Explore 会话和钥匙串里的令牌，不退出 `id.kite.plus`，所以再点「登录」会直接回到同一个账号，要换账号用第 3 条的「换一个账号」。读者在浏览器里退出 `id.kite.plus` 时，身份服务的 back-channel logout 按 `sid` 删除会话，应用里这次登录也一并结束，规则和网页一致（§1）。
8. **删除账号**：App Store 要求支持注册的应用能在应用内删除账号，必须删掉整个账号，不能只是停用；需要到网站完成时，给出直达删除页面的链接（审核指南 5.1.1(v) 和 Apple 的删号说明，2026-10 核实）。所以「我」页面保留现有的删除 Explore 资料（§1），另加「删除 Kite Plus 账号」，直接打开 `id.kite.plus` 的删号页面；删号事件再回到 Explore，删除本站账号。
9. **Apple 登录**：身份服务的登录页提供 GitHub 登录，按审核指南 4.8，应用必须同时提供一种只收集姓名和邮箱、允许隐藏邮箱、不做广告追踪的登录方式，所以应用切换之前，身份服务要先上线 Apple 登录（[identity architecture.md §2](https://github.com/kite-plus/identity/blob/main/docs/design/architecture.md#2-登录方式)）。
10. **按服务器提供的方式登录**：应用可以连自己部署的 Explore。`GET /api/v1/auth/options` 返回 `{"password": true, "oidc": {"name": "Kite Plus 账号"}}`（没有配置 OIDC 时 `oidc` 为 `null`），应用据此显示密码登录、统一登录或两者都有。应用的 Associated Domains 只登记了 `explore.kite.plus`，所以连其他服务器时回调退回 `explore://app/auth/done`；第 2 条说的冒充风险只在读者手动填了别的服务器时存在。
11. **过渡**：Cookie 改名之后，还在手写 `explore_session` 的老版本应用会掉线，所以改用 Bearer 的那一版要在 identity 的 I3 之前发出。已有本站账号的读者按 §1 的过渡期在网页账号页关联 Kite Plus 账号，应用里不做关联；过渡期内应用保留密码登录（第 10 条的 `password` 为 `true`）。

## 2. 订阅

- 订阅对象是博客，不是单篇文章或标签。博客页和目录页提供订阅入口。
- 订阅列表可以逐个取消，也可以导出为 OPML，带去任何 RSS 阅读器。
- 可以导入其他阅读器导出的 OPML。按每个订阅的订阅地址和站点地址找正在展示的已收录博客：主机名相同、只差 `www.`、是博客的额外域名，或订阅地址完全相同；找到的直接订阅，没找到的列出来，读者可以去提交。导入不新增博客，也不访问文件里的任何地址。
- 订阅流可以发布到一个私密地址，给自己博客上的博友圈页面读取：`/f/{token}.json`（JSON Feed 1.1）和 `/f/{token}.xml`（RSS 2.0）是订阅流最新的 50 篇，`/f/{token}.opml` 是订阅的博客（[api.md §3](api.md#3-订阅与导出)）。账号页默认不发布；发布后显示三个地址，可以复制，也可以换新地址（原来的立即失效）或停止发布。账号页写明：知道地址的人都能看到你订阅了哪些博客。账号被停用时地址不再返回内容，删除账号时地址一起删除。地址是随机值，不含读者的任何信息，日志里也不出现。
- 博客退出时删除其订阅。
- 每人最多 1,000 个订阅 `[待定]`。订阅人数是否公开展示 `[待定]`。

## 3. 三条信息流

| | 最新 | 推荐 | 订阅 |
|---|---|---|---|
| 地址 | `/` | `/recommended` | `/following` |
| 内容 | 全部收录博客 | 全部收录博客 | 当前用户订阅的博客 |
| 排序 | 发布时间倒序 | 按天倒序，同一天里分数高的在前（§3.1） | 发布时间倒序 |
| 范围 | 缓存里的全部（每博客最多 20 篇），按页往下翻 | 缓存里分数达到门槛的文章，按页往下翻 | 缓存里的全部（每博客最多 20 篇） |
| 要登录 | 否 | 否 | 是 |
| 防刷屏 | 同一博客每天最多 3 篇 | 同一博客每天最多 1 篇 | 不限 |
| 缓存与收录 | 可缓存、可收录 | 可缓存、无筛选时收录 | `private, no-store`、`noindex` |
| 筛选 | 标签、语言 | 标签、语言 | 标签、语言 |

三条流都不使用个人行为数据（§0）：最新和订阅按发布时间排序，推荐按文章自己的评分。

### 3.1 推荐评分

推荐流只看文章本身：不看任何人的点击和阅读，也不看博客的名气。

- **评分**：打标签时（§4）模型顺带从三方面给文章打分，每项 1 到 5 分，三项相加得 3 到 15 分：
  - **深度**：1 一两句话或随手记；2 短文、例行的周记月记、更新日志；3 讲清一件事的完整文章，比如教程、评测、随笔；4 细致深入的长文；5 透彻的深挖、研究或论证充分的长篇。
  - **原创**：1 转载、新闻、公告、优惠信息、几乎没有点评的链接列表；2 转述或翻译别人的内容；3 常见主题的笔记和教程；4 亲身经历、自己的项目或清楚的观点；5 原创的发现、来之不易的经验或新的视角。
  - **价值**：读者点开能得到多少，不论话题：1 几乎没有，比如站务、测试；2 主要对认识作者的人有意义；3 对关心这个话题的人有用或好读；4 明显值得一读，实用、有见地或打动人；5 会想收藏或转给别人。

  广告和赞助内容、测试或占位文章、只是指向别处的文章记 0 分。大多数文章每项 2 到 3 分，标题和摘要看得出来时才给 4 或 5；只评写作本身，不评话题和语言。
- **入选**：分数达到 `policy.RecommendedMinScore`（11，按线上实测的分数分布定，每天约 10 篇，见 [recommendation.md](recommendation.md) 的 R0）、日期可信、不在未来、原文链接没有疑似失效的文章。同一博客每天最多 1 篇，先取分高的，同分取新的。
- **排序**：按天倒序，同一天里分数高的在前，同分时新的在前。"天"按读者的时区划分（[api.md](api.md) 的 `tz` 参数）：网页和 iOS 都按同一个时区给文章分日期，日期标题不会乱序。
- **没有模型时**：没配置模型就没有评分，推荐流为空，推荐页说明原因并链接到最新。

这样设计的理由：

- 评分只用 Explore 本来就展示的标题、摘要和分类，和打标签是同一次模型调用，不多花一次请求，也不读正文。
- 拆成三项再相加，比让模型直接挑一档稳：v0.1.10 的四档评分实测几乎从不给最高档，一半以上落在第二档，推荐流接近"最新减掉短文"（[recommendation.md](recommendation.md) 的 R0）。分数也让同一天的文章能排出先后。
- 存分数，门槛写在代码里：按分布调门槛只改常量，不用重评；只有评分规则变了才重评。
- 不按博客打分：小博客的一篇好文章和大博客的一样排。
- 不用随时间衰减的"热度"分：那样排序时时在变，游标翻页会重复或漏掉文章。这里的位置只由发布日期和分数决定，翻页前后一致，新的一天只会出现在前面。
- 评分和标签一样是缓存：标题没变就保留，标题变了或清空缓存后重评。模型结果可能有细小差异，这是缓存重建一致性的例外（§4）。

推荐流的下一步（开通、校准和排序调整）见 [recommendation.md](recommendation.md)，设计中。

## 4. 文章标签

标签表由 Explore 维护，定义在 `internal/model/tags.go`，每个标签有 URL 短名、中英文名和给模型看的说明。每篇文章打 0 到 3 个标签，没有合适的就不打。

Worker 独立循环分类，不阻塞抓取，只给按发布时间最新的 1 万篇文章打标签和评分（[worker.md §11](worker.md#11-打标签internaltagger)）；更早的文章不带标签，也就不出现在按标签筛选和推荐流里。输入只有标题、短摘要、订阅源分类、博客语言和维护者设置的默认标签；不发送正文或读者数据。模型输出限定在标签表内，同一次调用还给出推荐评分（§3.1）。同一篇文章且标题未变时保留已有标签；清空文章缓存后重打。模型结果可能有细小差异，这是缓存重建一致性的例外。

模型接口有两种：Anthropic Messages API，以及任何 OpenAI 兼容的 Chat Completions 接口（OpenAI、DeepSeek 等，也可以是本机的开源模型服务）。部署时设置接口类型、地址、密钥和模型（[project-layout.md §4](project-layout.md#4-配置)）。Anthropic 接口用结构化输出把回答限定在标签表内，OpenAI 兼容接口只要求回答是 JSON；两种回答都按同样的规则校验，表外的标签丢掉，认不出的评分算 `skip`。上线前用样本实测质量、token 和费用（[recommendation.md](recommendation.md) 的 R0）。维护者可设置博客默认标签；逐篇人工修改暂不做。

最新已支持 `?tag=frontend`，并可与语言筛选组合；订阅沿用同一参数。带筛选页面 `noindex, follow`。可收录的标签落地页 `[待定]`。博客目录不按文章标签筛选。

## 5. 数据模型

已新增的读者数据：

```sql
users (id, number unique, email, password_hash, display_name, is_admin,
       created_at, disabled_at, disabled_reason, disabled_by, last_seen_at,
       feed_token unique)
user_identities (issuer, subject, user_id, linked_at,
                 primary key (issuer, subject))
sessions (token_hash primary key, user_id -> users on delete cascade,
          created_at, expires_at)
subscriptions (user_id -> users on delete cascade,
               blog_id -> blogs on delete cascade, created_at,
               primary key (user_id, blog_id))
blog_owners (blog_id primary key, user_id -> users on delete cascade, verified_at)
blog_claim_challenges (blog_id, user_id, token_hash, expires_at)
```

用户看到的 ID 是 `number`：按注册顺序递增的数字，账号页显示为「ID 12 · 2026年9月5日加入」，后台列表和详情也叫 ID，`/api/v1/me` 返回 `number` 和 `created_at`。`id` 列是 UUID，只在内部使用：其他表和统一身份服务都按它关联账号，接口路径里的账号也用它，后台详情里标为 UUID。`number` 由数据库序列生成（`GENERATED ALWAYS AS IDENTITY`），不能修改，可用于以后按注册先后安排的活动。注册时先确认邮箱没被占用再插入，重复注册不会消耗 ID；空号只来自删除的账号，以及两个人同时用同一邮箱注册这种极少见的情况。迁移 `00018_user_number.sql` 按注册时间给已有账号回填 1、2、3…，新账号从最大值之后继续。

`feed_token` 是订阅流公开地址里的随机值（32 字节，base64url），没有发布时为空，由迁移 `00022_feed_token.sql` 加入。它按原样保存而不是只存哈希：读者要在账号页再次看到地址，而它能透露的订阅本来就存在同一个库里。

不建 `login_codes` 或 GitHub 凭据表。清空 `entries` 不丢订阅；订阅流用 `subscriptions` 过滤文章并复用游标分页。博客认领要求用户在 `_explore-claim.<host>` 发布随机值对应的 DNS TXT 记录，验证值有效期 30 分钟；一个博客只能有一个已验证归属。

## 6. 接口与前端

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/auth/register` | 创建本站账号并建立会话 |
| POST | `/api/v1/auth/login` | 邮箱密码登录 |
| POST | `/api/v1/auth/logout` | 删除 Explore 会话 |
| GET | `/api/v1/auth/options` | 服务器提供哪些登录方式（§1.1）`[设计中]` |
| GET | `/auth/app/start` | 应用登录：建交易并跳到身份服务（§1.1）`[设计中]` |
| POST | `/api/v1/auth/app/exchange` | 应用登录：用授权码和 verifier 换会话令牌（§1.1）`[设计中]` |
| GET、PATCH、DELETE | `/api/v1/me` | 本站资料、昵称、删除账号（唯一可用的管理员除外） |
| PUT | `/api/v1/me/password` | 验证当前密码后修改密码，撤销其他会话，清除管理员重置留下的临时标记；账号页「修改密码」使用 |
| GET | `/api/v1/me/entries` | 订阅流 |
| GET、POST、DELETE | `/api/v1/me/following-feed` | 订阅流的公开地址：查看、换新地址（没有发布时就是开始发布）、停止发布 |
| GET | `/f/{token}.json`、`.xml`、`.opml` | 发布出去的订阅流和订阅清单，不需要登录 |
| GET | `/api/v1/me/subscriptions` | 订阅列表 |
| GET | `/api/v1/me/subscriptions.opml` | 导出订阅的 OPML |
| POST | `/api/v1/me/subscriptions/import` | 导入 OPML，订阅其中已收录的博客 |
| PUT、DELETE | `/api/v1/me/subscriptions/{host}` | 订阅和取消 |
| GET | `/api/v1/me/blogs` | 已认领博客 |
| POST | `/api/v1/me/blog-claims/{host}` | 生成 DNS TXT 验证记录 |
| POST | `/api/v1/me/blog-claims/{host}/verify` | 验证并认领博客 |

`/login` 是本站账号登录与注册页；`/following`、`/account` 均有英文页面。页头的登录状态由服务端按请求带的会话画出，画了读者名称的页面 `private, no-store`，匿名页面照常公开缓存（[frontend.md §5](frontend.md#5-数据获取)）；个人页面一律 `private, no-store`；已登录的读者打开 `/login` 直接跳到原本要去的页面。所有已登录写操作要求会话对应的 CSRF 请求头。

## 7. 部署与待定

统一身份服务单独部署，提供稳定的 HTTPS issuer。Explore 配置 `EXPLORE_OIDC_ISSUER`、`EXPLORE_OIDC_CLIENT_ID`、`EXPLORE_OIDC_CLIENT_SECRET`；密钥只放服务端。应用登录另需在 `explore.kite.plus` 发布 `apple-app-site-association` 和 `assetlinks.json`（§1.1），现在的 Caddyfile 只把 `/api/*` 等地址交给 serve，其余都给 web；`/auth/*`、`/app/auth/done` 和这两个文件由谁提供，接入时定，Caddyfile 随之修改。每个环境使用独立客户端和准确的回调白名单。生产身份服务需要数据库备份、邮件渠道、TLS 和管理员初始化。

第一条端到端链路是身份服务 → Explore 登录与订阅；第二条是身份服务 → 统一评论服务 → 通用博客嵌入组件。二者使用同一身份服务，数据与 Cookie 分开。外部博客只有主动嵌入组件或接入兼容适配器，才有统一评论登录体验。

已定：身份服务 `id.kite.plus`、评论服务 `comments.kite.plus`，都部署在境外（[identity-and-comments.md §2](identity-and-comments.md#2-组件与责任)）；账号删除后历史评论的处理由评论服务负责（[comments login.md §7](https://github.com/kite-plus/comments/blob/main/docs/design/login.md#7-账号删除与停用)）。

待定：订阅上限与是否展示人数；标签落地页与隐私政策文字。

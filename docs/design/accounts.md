# 统一身份、订阅与标签

> 状态：本站账号、订阅流、OPML 导入导出、推荐评分和域名认领已实现；OIDC 待接入 · 最近更新：2026-10-03
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

## 2. 订阅

- 订阅对象是博客，不是单篇文章或标签。博客页和目录页提供订阅入口。
- 订阅列表可以逐个取消，也可以导出为 OPML，带去任何 RSS 阅读器。
- 可以导入其他阅读器导出的 OPML。按每个订阅的订阅地址和站点地址找正在展示的已收录博客：主机名相同、只差 `www.`、是博客的额外域名，或订阅地址完全相同；找到的直接订阅，没找到的列出来，读者可以去提交。导入不新增博客，也不访问文件里的任何地址。
- 博客退出时删除其订阅。
- 每人最多 1,000 个订阅 `[待定]`。订阅人数是否公开展示 `[待定]`。

## 3. 三条信息流

| | 最新 | 推荐 | 订阅 |
|---|---|---|---|
| 地址 | `/` | `/recommended` | `/following` |
| 内容 | 全部收录博客 | 全部收录博客 | 当前用户订阅的博客 |
| 排序 | 发布时间倒序 | 排序时间倒序：发布时间加评分的提前量（§3.1） | 发布时间倒序 |
| 范围 | 缓存里的全部（每博客最多 20 篇），按页往下翻 | 缓存里评为扎实或出色的文章，按页往下翻 | 缓存里的全部（每博客最多 20 篇） |
| 要登录 | 否 | 否 | 是 |
| 防刷屏 | 同一博客每天最多 3 篇 | 同一博客每天最多 1 篇 | 不限 |
| 缓存与收录 | 可缓存、可收录 | 可缓存、无筛选时收录 | `private, no-store`、`noindex` |
| 筛选 | 标签、语言 | 标签、语言 | 标签、语言 |

三条流都不使用个人行为数据（§0）：最新和订阅按发布时间排序，推荐按文章自己的评分。

### 3.1 推荐评分

推荐流只看文章本身：不看任何人的点击和阅读，也不看博客的名气。

- **评分**：打标签时（§4）模型顺带给文章评一档。`standout`（出色）：原创、深入、多数读者会觉得值得一读，比如技术深挖、研究、有见地的长文、详细的经验总结。`solid`（扎实）：主题清楚、有实际内容，比如教程、评测、随笔、项目记录、讲得好的个人经历。`brief`（简短）：短札记、近况、例行公告、更新日志、几乎没有点评的链接列表。`skip`（不推荐）：测试或占位文章、广告推广、只是指向别处的文章，或者信息太少无法判断。只评写作本身，不评话题和语言；拿不准时选低一档。
- **入选**：评为 `solid` 或 `standout`、日期可信、不在未来、原文链接没有疑似失效的文章。同一博客每天最多 1 篇，先取评分高的，同分取新的。
- **排序**：按"排序时间"倒序。`solid` 的排序时间就是发布时间；`standout` 再加 2 天 `[待定]`，在推荐流顶部多留两天。
- **没有模型时**：没配置模型就没有评分，推荐流为空，推荐页说明原因并链接到最新。

这样设计的理由：

- 评分只用 Explore 本来就展示的标题、摘要和分类，和打标签是同一次模型调用，不多花一次请求，也不读正文。
- 不按博客打分：小博客的一篇好文章和大博客的一样排。
- 不用随时间衰减的"热度"分：那样排序时时在变，游标翻页会重复或漏掉文章。排序时间把评分折算成固定的提前量，效果相近，排序却固定不变，新文章只会出现在前面。
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
       created_at, disabled_at, disabled_reason, disabled_by, last_seen_at)
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

不建 `login_codes` 或 GitHub 凭据表。清空 `entries` 不丢订阅；订阅流用 `subscriptions` 过滤文章并复用游标分页。博客认领要求用户在 `_explore-claim.<host>` 发布随机值对应的 DNS TXT 记录，验证值有效期 30 分钟；一个博客只能有一个已验证归属。

## 6. 接口与前端

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/auth/register` | 创建本站账号并建立会话 |
| POST | `/api/v1/auth/login` | 邮箱密码登录 |
| POST | `/api/v1/auth/logout` | 删除 Explore 会话 |
| GET、PATCH、DELETE | `/api/v1/me` | 本站资料、昵称、删除账号（唯一可用的管理员除外） |
| PUT | `/api/v1/me/password` | 验证当前密码后修改密码，撤销其他会话，清除管理员重置留下的临时标记；账号页「修改密码」使用 |
| GET | `/api/v1/me/entries` | 订阅流 |
| GET | `/api/v1/me/subscriptions` | 订阅列表 |
| GET | `/api/v1/me/subscriptions.opml` | 导出订阅的 OPML |
| POST | `/api/v1/me/subscriptions/import` | 导入 OPML，订阅其中已收录的博客 |
| PUT、DELETE | `/api/v1/me/subscriptions/{host}` | 订阅和取消 |
| GET | `/api/v1/me/blogs` | 已认领博客 |
| POST | `/api/v1/me/blog-claims/{host}` | 生成 DNS TXT 验证记录 |
| POST | `/api/v1/me/blog-claims/{host}/verify` | 验证并认领博客 |

`/login` 是本站账号登录与注册页；`/following`、`/account` 均有英文页面。页头的登录状态由服务端按请求带的会话画出，画了读者名称的页面 `private, no-store`，匿名页面照常公开缓存（[frontend.md §5](frontend.md#5-数据获取)）；个人页面一律 `private, no-store`；已登录的读者打开 `/login` 直接跳到原本要去的页面。所有已登录写操作要求会话对应的 CSRF 请求头。

## 7. 部署与待定

统一身份服务单独部署，提供稳定的 HTTPS issuer。Explore 配置 `EXPLORE_OIDC_ISSUER`、`EXPLORE_OIDC_CLIENT_ID`、`EXPLORE_OIDC_CLIENT_SECRET`；密钥只放服务端。每个环境使用独立客户端和准确的回调白名单。生产身份服务需要数据库备份、邮件渠道、TLS 和管理员初始化。

第一条端到端链路是身份服务 → Explore 登录与订阅；第二条是身份服务 → 统一评论服务 → 通用博客嵌入组件。二者使用同一身份服务，数据与 Cookie 分开。外部博客只有主动嵌入组件或接入兼容适配器，才有统一评论登录体验。

已定：身份服务 `id.kite.plus`、评论服务 `comments.kite.plus`，都部署在境外（[identity-and-comments.md §2](identity-and-comments.md#2-组件与责任)）；账号删除后历史评论的处理由评论服务负责（[comments login.md §7](https://github.com/kite-plus/comments/blob/main/docs/design/login.md#7-账号删除与停用)）。

待定：订阅上限与是否展示人数；标签落地页与隐私政策文字。

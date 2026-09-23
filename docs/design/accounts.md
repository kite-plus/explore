# 统一身份、订阅与标签

> 状态：本站账号、订阅流和域名认领已实现；OIDC 待接入 · 最近更新：2026-09-24
> 跨站身份和评论的架构见 [identity-and-comments.md](identity-and-comments.md)。

## 0. 边界

现阶段 Explore 提供邮箱和密码登录，密码使用 bcrypt 哈希保存，30 天服务端会话只存令牌哈希。后续统一身份服务接入 OIDC 时，通过 `user_identities` 将 `(issuer, subject)` 关联到现有 `users.id`；不按邮箱自动合并账号，因此订阅和博客归属不需要迁移。评论服务仍独立保存评论及审核状态。

读者不登录也能看全部公开内容，匿名访问零 Cookie。Explore 不记录浏览、点击和阅读行为，不根据个人行为推荐。

## 1. 登录与会话

- OIDC 尚未接入。接入时使用 Authorization Code + PKCE，并校验 `state`、`nonce`、ID Token 签名、`iss`、`aud` 和有效期。
- 统一身份键是 `(issuer, subject)`；邮箱、昵称、头像只用于展示。不同客户端的 `sub` 若因 pairwise 配置而不同，须在身份服务中配置一致的关联标识，不能按邮箱推断同一用户。
- 过渡阶段 Explore 接收本站密码；最少 12 字符，bcrypt 哈希存储。暂未提供邮箱验证或密码重置，部署时应说明这个限制。接入统一身份后隐藏账号密码入口，并迁移已有账号的登录方式。
- OIDC 成功后 Explore 建立自己的服务端会话；Cookie 限本站域名，设 `HttpOnly`、`Secure`、`SameSite=Lax`。数据库只存会话令牌哈希。退出 Explore 删除本站会话；全局退出需单独的 OIDC logout 机制。
- 登录回调只建立会话，不直接做订阅等数据变更。登录后跳回本站相对路径，再由读者确认订阅。
- 管理员是显式授予 `is_admin` 的本站账号，命令 `explore users promote-admin EMAIL` 由服务器操作者执行。旧的 `EXPLORE_ADMIN_TOKENS` 保留为运维入口；普通账号不能访问管理接口。
- 删除 Explore 资料会删除本站订阅和会话。删除统一身份账号后的跨服务数据清理须有独立流程。

## 2. 订阅

- 订阅对象是博客，不是单篇文章或标签。博客页和目录页提供订阅入口。
- 列表可取消、导出 OPML、导入 OPML；导入只匹配已收录博客。博客退出时删除其订阅。
- 每人最多 1,000 个订阅 `[待定]`。订阅人数是否公开展示 `[待定]`。

## 3. 两条信息流

| | 推荐流 | 订阅流 |
|---|---|---|
| 地址 | `/` | `/following` |
| 内容 | 全部收录博客 | 当前用户订阅的博客 |
| 要登录 | 否 | 是 |
| 时间范围 | 近 30 天 | 缓存里的全部（每博客最多 20 篇） |
| 防刷屏 | 同一博客每天最多 3 篇 | 不限 |
| 缓存与收录 | 可缓存、可收录 | `private, no-store`、`noindex` |
| 筛选 | 标签、语言 | 标签、语言 |

推荐流按发布时间排序，不使用个人行为数据。

## 4. 文章标签

标签表由 Explore 维护，定义在 `internal/model/tags.go`，每个标签有 URL 短名、中英文名和给模型看的说明。每篇文章打 0 到 3 个标签，没有合适的就不打。

Worker 独立循环分类，不阻塞抓取。输入只有标题、短摘要、订阅源分类、博客语言和维护者设置的默认标签；不发送正文或读者数据。模型输出限定在标签表内。同一篇文章且标题未变时保留已有标签；清空文章缓存后重打。模型结果可能有细小差异，这是缓存重建一致性的例外。

使用 Claude API，部署时同时设置 `EXPLORE_TAGGER_MODEL` 和 `EXPLORE_ANTHROPIC_API_KEY`。上线前用样本实测质量、token 和费用。维护者可设置博客默认标签；逐篇人工修改暂不做。

推荐流已支持 `?tag=frontend`，并可与语言筛选组合；订阅流沿用同一参数。带筛选页面 `noindex, follow`。可收录的标签落地页 `[待定]`。博客目录不按文章标签筛选。

## 5. 数据模型

已新增的读者数据：

```sql
users (id, email, password_hash, display_name, is_admin, created_at)
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

不建 `login_codes` 或 GitHub 凭据表。清空 `entries` 不丢订阅；订阅流用 `subscriptions` 过滤文章并复用游标分页。博客认领要求用户在 `_explore-claim.<host>` 发布随机值对应的 DNS TXT 记录，验证值有效期 30 分钟；一个博客只能有一个已验证归属。

## 6. 接口与前端

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/auth/register` | 创建本站账号并建立会话 |
| POST | `/api/v1/auth/login` | 邮箱密码登录 |
| POST | `/api/v1/auth/logout` | 删除 Explore 会话 |
| GET、PATCH、DELETE | `/api/v1/me` | 本站资料、昵称、删除本站数据 |
| GET | `/api/v1/me/entries` | 订阅流 |
| GET | `/api/v1/me/subscriptions` | 订阅列表与 OPML 导出 |
| PUT、DELETE | `/api/v1/me/subscriptions/{host}` | 订阅和取消 |
| GET | `/api/v1/me/blogs` | 已认领博客 |
| POST | `/api/v1/me/blog-claims/{host}` | 生成 DNS TXT 验证记录 |
| POST | `/api/v1/me/blog-claims/{host}/verify` | 验证并认领博客 |

`/login` 是本站账号登录与注册页；`/following`、`/account` 均有英文页面。匿名页面仍可公开缓存，页头由客户端查询会话状态；个人页面一律 `private, no-store`。所有已登录写操作要求会话对应的 CSRF 请求头。

## 7. 部署与待定

统一身份服务单独部署，提供稳定的 HTTPS issuer。Explore 配置 `EXPLORE_OIDC_ISSUER`、`EXPLORE_OIDC_CLIENT_ID`、`EXPLORE_OIDC_CLIENT_SECRET`；密钥只放服务端。每个环境使用独立客户端和准确的回调白名单。生产身份服务需要数据库备份、邮件渠道、TLS 和管理员初始化。

第一条端到端链路是身份服务 → Explore 登录与订阅；第二条是身份服务 → 统一评论服务 → 通用博客嵌入组件。二者使用同一身份服务，数据与 Cookie 分开。外部博客只有主动嵌入组件或接入兼容适配器，才有统一评论登录体验。

待定：身份和评论服务的最终域名与部署地；全局账号删除后历史评论的处理；订阅上限与是否展示人数；标签落地页与隐私政策文字。

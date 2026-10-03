# Kite Plus 统一身份与跨博客评论

> 状态：架构决定；域名和部署地已定。身份服务本身的规划在 [identity 仓库](https://github.com/kite-plus/identity/blob/main/docs/design/architecture.md)，评论服务的详细设计在 [comments 仓库](https://github.com/kite-plus/comments/blob/main/docs/design/README.md) · 最近更新：2026-09-26

本文只记跨项目的约定：各应用怎么接入身份服务（§3），以及三个项目的分工和先后。身份服务本身（选型、登录方式、部署、运维）在 identity 仓库，评论服务自己的设计在 comments 仓库，这里都不重复。

## 1. 目标与范围

读者在 Kite Plus 身份服务登录一次后，可以在 Explore 订阅博客，也可以在主动接入 Kite Plus 评论的多个博客发表评论。博客作者保留原文、页面和接入选择权。独立博客的 RSS/Atom/JSON Feed 收录规则完全不依赖评论接入。

这里的「登录一次」指各应用经同一 OIDC 身份服务执行顶层跳转或弹窗时复用身份服务会话。浏览器仍为每个应用维护独立会话。在别人的博客上，评论组件以「每个站点点一次登录」为基线：身份服务已登录时这一下不用输入任何东西，但不做打开页面就自动认出读者，原因见 [comments login.md §1](https://github.com/kite-plus/comments/blob/main/docs/design/login.md#1-目标与边界)。OIDC 不会自动登录未接入的 WordPress、Halo、Giscus 或其他评论系统。

## 2. 组件与责任

```text
                         ┌───────────────────────────────┐
                         │ id.kite.plus                  │
                         │ Kite Plus 账号（OIDC Provider）│
                         │ 注册、登录、账号合并、退出      │
                         └───────┬───────────────┬───────┘
                                 │ OIDC          │ OIDC
                        ┌────────▼─────────┐ ┌───▼────────────────┐
                        │ explore.kite.plus │ │ comments.kite.plus │
                        │ 博客发现、订阅     │ │ 评论、审核、站点    │
                        └──────────────────┘ └───┬────────────────┘
                                                  │ 嵌入组件／API
                                         ┌────────▼──────────────┐
                                         │ Kite、Hugo、Hexo、     │
                                         │ WordPress、Halo 等博客 │
                                         └───────────────────────┘
```

- **身份服务**部署在 `id.kite.plus`，issuer 是 `https://id.kite.plus`，一经公开不能更换。读者看到的名字是「Kite Plus 账号」。
- **评论服务**部署在 `comments.kite.plus`，提供无需框架依赖的嵌入组件。
- Explore 与评论服务分别注册 OIDC 客户端。
- 身份服务和评论服务都部署在境外，首选香港。在境内运营评论服务，要做评论者实名、公安交互式备案和安全评估，对这个项目不现实，理由见 [comments architecture.md §5](https://github.com/kite-plus/comments/blob/main/docs/design/architecture.md#5-部署地与合规)。

- **身份服务是基础设施，先于评论服务**：自托管开源实现，不自己写；首选 ZITADEL，与 Logto 实测对比后定案。选型、部署、运维都在 [identity 仓库](https://github.com/kite-plus/identity/blob/main/docs/design/architecture.md)（`kite-plus/identity`），各应用对它的要求也列在那里（[§1](https://github.com/kite-plus/identity/blob/main/docs/design/architecture.md#1-各服务对身份服务的要求)）。

一个人的多种登录方式（邮箱、GitHub、Passkey）在身份服务里关联到同一个账号，Explore 与评论服务不保存上游社交令牌。已经分成两个账号的，不提供合并：两个 `(iss, sub)` 在各应用里就是两个用户。

## 3. 协议与数据

1. 各服务通过 OIDC Discovery 获取端点和 JWKS。使用 Authorization Code + PKCE，随机 `state` 和 `nonce`，精确注册回调地址。
2. 回调校验 `state`、`nonce`、ID Token 签名及 `iss`、`aud`、有效期；用户身份以 `(iss, sub)` 标识，不能用邮箱、显示名或 GitHub ID。ZITADEL 的 `sub` 是整个实例统一的用户 ID，各应用拿到的一致（2026-09 核实）；换用其他实现时也必须满足这一点。
3. 每个应用建立自己的本地会话，Cookie 只发给自己的域名，名称用 `__Host-` 前缀：`comments.kite.plus` 承载用户生成的内容，前缀能防止兄弟子域覆盖别的应用的 Cookie。应用退出只结束本地会话。读者在身份服务退出时，身份服务用 back-channel logout 通知各应用，各应用按 `sid` 删除这次身份服务会话期间签发的本地会话，所以本地会话要记下签发时 ID Token 里的 `sid`。更早的身份服务会话期间签发、仍然有效的本地会话不受影响，各应用自己提供「退出所有设备」一类的入口。
4. Explore 只持有身份键和订阅；评论服务持有身份键、公开昵称、评论、站点归属与审核信息。隐私导出、删除和身份服务停用事件需跨服务协调：身份服务推送用户删除和停用事件，各应用删除或暂停本地数据；推送可能丢失，各应用另外定时对账（[comments login.md §7](https://github.com/kite-plus/comments/blob/main/docs/design/login.md#7-账号删除与停用)）。
5. 面向跨域博客的评论登录用弹窗进入身份服务，再返回评论服务；弹窗不可用时整页跳转。组件运行在评论服务自己的 iframe 里，宿主页拿不到任何凭据；不依赖第三方 Cookie。协议和威胁分析见 [comments login.md §4](https://github.com/kite-plus/comments/blob/main/docs/design/login.md#4-组件登录协议)。

## 4. 通用评论接入

博主注册站点，逐个验证 origin 的控制权（DNS TXT、HTML meta 或文件），然后嵌入 `<kite-comments>` 组件。组件只能出现在已验证的 origin 上，由浏览器按 `frame-ancestors` 保证。讨论页优先用页面的稳定 ID 标识，没有时用规范化后的路径；评论服务不抓取或存储文章正文。细节见 comments 仓库的 [widget.md](https://github.com/kite-plus/comments/blob/main/docs/design/widget.md) 和 [sites.md](https://github.com/kite-plus/comments/blob/main/docs/design/sites.md)。

纯静态博客只需嵌入代码；Kite 默认主题内置；WordPress、Halo、Typecho 先用主题模板或代码注入，C4 提供插件。站点若继续使用 Giscus、Waline 等原有评论后端，登录状态不会自动互通；可以把旧评论导入评论服务，读者再认领自己的旧评论。评论接入与否不影响 Explore 收录。

## 5. 交付顺序与验收

先做身份服务，再做评论服务：

1. **身份服务**按 [identity 路线图](https://github.com/kite-plus/identity/blob/main/docs/design/roadmap.md)：I0 选型 → I1 预发布 → I2 生产 → I3 Explore 上线。Explore 是第一个接入的应用，它的 OIDC 改造在 I1 的预发布环境里完成、I3 上线。
2. **评论服务**按 [comments 路线图](https://github.com/kite-plus/comments/blob/main/docs/design/roadmap.md)：C0 技术验证可以在 I1 之后开始，C2 内测需要 I2 完成，之后是 C3 开放接入、C4 生态。

身份相关的安全检查，identity 的 I2 和评论的 C2 退出前各完成一次：CSRF、开放重定向、ID Token 校验、回调重放、站点来源、脚本 CSP、跨域 Cookie 限制、速率限制、数据删除与备份恢复。

生产上线还需要真实域名、TLS、邮件渠道、管理员初始化及密钥保管；这些不是代码仓库可以代替的。

## 6. 依据

- [OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html)：授权码、ID Token 验证及 `iss`/`sub` 语义。
- [OpenID Connect Back-Channel Logout 1.0](https://openid.net/specs/openid-connect-backchannel-1_0.html)：按 `sid` 结束各应用的会话。
- [ZITADEL OIDC Authorization Code 与 PKCE 指南](https://zitadel.com/docs/guides/integrate/login/oidc/login-users)：客户端和回调流程。
- [ZITADEL Docker Compose 部署](https://zitadel.com/docs/self-hosting/deploy/compose)：自托管部署与主密钥约束。
- [ZITADEL back-channel logout](https://zitadel.com/docs/guides/integrate/back-channel-logout)、[Actions v2](https://zitadel.com/docs/guides/integrate/actions/usage)：会话结束通知和用户事件推送。

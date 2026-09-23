# Kite Plus 统一身份与跨博客评论

> 状态：架构决定；部署与接入待实现 · 最近更新：2026-09-23

## 1. 目标与范围

读者在 Kite Plus 身份服务登录一次后，可以在 Explore 订阅博客，也可以在主动接入 Kite Plus 评论的多个博客发表评论。博客作者保留原文、页面和接入选择权。独立博客的 RSS/Atom/JSON Feed 收录规则完全不依赖评论接入。

这里的「登录一次」指各应用经同一 OIDC 身份服务执行顶层跳转时复用身份服务会话。浏览器仍为每个应用维护独立会话；OIDC 不会自动登录未接入的 WordPress、Halo、Giscus 或其他评论系统，也不保证浏览器在第三方 iframe 中发送 Cookie。

## 2. 组件与责任

```text
                         ┌───────────────────────────────┐
                         │ 统一身份服务（OIDC Provider）  │
                         │ 注册、登录、账号合并、退出      │
                         └───────┬───────────────┬───────┘
                                 │ OIDC          │ OIDC
                        ┌────────▼────────┐ ┌────▼──────────────┐
                        │ Explore          │ │ 统一评论服务      │
                        │ 博客发现、订阅    │ │ 评论、审核、站点  │
                        └─────────────────┘ └────┬──────────────┘
                                                   │ 嵌入组件／API
                                          ┌────────▼──────────────┐
                                          │ Kite、Hugo、Hexo、     │
                                          │ WordPress、Halo 等博客 │
                                          └───────────────────────┘
```

身份服务单独部署，例如 `auth.kite.plus`；Explore 与评论服务分别注册 OIDC 客户端。评论服务单独部署，例如 `comments.kite.plus`，提供无需框架依赖的嵌入组件。域名在部署前确认，issuer 一经公开不能随意更换。

| 方案 | 成本与风险 | 结论 |
|---|---|---|
| 自写 OIDC Provider | 必须维护认证协议、签名密钥、账号恢复、密码/Passkey、漏洞响应 | 不采用 |
| 自托管 ZITADEL | 官方支持 OIDC、外部身份源、PostgreSQL 和 Compose；需要维护备份、升级和邮件 | 首选 |
| 托管身份服务 | 上线快；持续费用、数据位置及服务迁移风险由供应商决定 | 可作为运维备选 |

身份服务仓库只保存部署配置、初始化脚本和运维文档；不重写 ZITADEL。任何社交登录和账号合并都在身份服务完成，Explore 与评论服务不保存上游社交令牌。

## 3. 协议与数据

1. 各服务通过 OIDC Discovery 获取端点和 JWKS。使用 Authorization Code + PKCE，随机 `state` 和 `nonce`，精确注册回调地址。
2. 回调校验 `state`、`nonce`、ID Token 签名及 `iss`、`aud`、有效期；用户身份以 `(iss, sub)` 标识，不能用邮箱、显示名或 GitHub ID。多个应用需要同一逻辑用户键时，身份服务必须为它们提供一致的 subject 策略。
3. 每个应用建立自己的短期本地会话，Cookie 只发给自己的域名。应用退出只结束本地会话；「退出所有应用」还需要身份服务会话结束及后端通知机制。
4. Explore 只持有身份键和订阅；评论服务持有身份键、公开昵称、评论、站点归属与审核信息。隐私导出、删除和身份服务停用事件需跨服务协调。
5. 面向跨域博客的评论登录使用顶层导航或弹窗进入身份服务，再返回评论服务。嵌入脚本不能依赖第三方 Cookie；评论写入必须校验 CSRF、站点来源、文章归属及用户权限。

## 4. 通用评论接入

博客作者注册站点并验证控制权后，配置允许的 origin。每篇文章用规范化的绝对 URL 作为标识，并绑定站点 ID；评论服务不抓取或存储文章正文。站点控制权变更要有重新验证与迁移流程。

```html
<div id="kite-comments"></div>
<script
  src="https://comments.kite.plus/embed.js"
  data-site="已验证的站点标识"
  data-page="https://blog.example.com/posts/example"
  defer
></script>
```

组件先提供只读评论和登录入口。发表、回复、编辑、删除、举报、审核、反垃圾和邮件通知由评论服务统一处理；公开展示只用用户选定的昵称和头像，不泄露原始 `sub` 或邮箱。作者可关闭单篇评论，导出本站评论，并在移除组件后继续管理历史数据。

纯静态博客（Kite 静态模式、Hugo、Hexo、Jekyll）只需嵌入脚本；WordPress、Halo 可先用自定义代码区，后续提供便利插件。站点若继续使用 Giscus、Waline 等原有评论后端，登录状态不会自动互通，需迁移数据或专门适配。

## 5. 交付顺序与验收

1. 身份服务：固定版本的 ZITADEL Compose、示例配置、备份和升级说明；本地创建 Explore 与 Comments 两个客户端。
2. Explore：OIDC 登录、本站会话、订阅、订阅流、数据导出和删除；匿名阅读与当前公开缓存行为保持。
3. 评论服务：站点验证、评论 CRUD、审核与嵌入组件；用两个不同博客域名验证同一身份登录。
4. Kite 及其他博客：提供接入示例，Kite 主题提供配置项；WordPress/Halo 等只做安装说明或轻量适配，不影响 Explore 收录。
5. 完成安全检查：CSRF、开放重定向、ID Token 校验、回调重放、站点来源、脚本 CSP、跨域 Cookie 限制、速率限制、数据删除与备份恢复。

生产上线还需要真实域名、TLS、邮件渠道、管理员初始化及密钥保管；这些不是代码仓库可以代替的。

## 6. 依据

- [OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html)：授权码、ID Token 验证及 `iss`/`sub` 语义。
- [ZITADEL OIDC Authorization Code 与 PKCE 指南](https://zitadel.com/docs/guides/integrate/login/oidc/login-users)：客户端和回调流程。
- [ZITADEL Docker Compose 部署](https://zitadel.com/docs/self-hosting/deploy/compose)：自托管部署与主密钥约束。

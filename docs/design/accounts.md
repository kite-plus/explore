# 统一身份、订阅与标签

> 状态：本站账号、订阅流及其公开地址、OPML 导入导出、推荐评分和域名认领已实现；Kite Plus 账号登录（OIDC）、退出与删号同步和应用登录（§1.1）已实现，v0.1.20 起在生产启用；配置了 Kite Plus 时不再有本站密码（§1），下一版发布 · 最近更新：2026-10-11
> 跨站身份和评论的架构见 [identity-and-comments.md](identity-and-comments.md)。

## 0. 边界

配置了统一身份服务（§7）时，Explore 只用 Kite Plus 账号登录，没有本站密码；通过 `user_identities` 将 `(issuer, subject)` 关联到 `users.id`，订阅和博客归属不需要迁移。以前用密码注册的账号，用邮箱相同、经身份服务验证的 Kite Plus 账号登录时自动关联（§1）。没有配置时只有邮箱和密码登录，密码使用 bcrypt 哈希保存，行为和接入前完全一样，自托管和本地开发照旧。30 天服务端会话只存令牌哈希。评论服务仍独立保存评论及审核状态。

读者不登录也能看全部公开内容，匿名访问零 Cookie。Explore 不记录浏览、点击和阅读行为，不根据个人行为推荐。

## 1. 登录与会话

- **开关**：`EXPLORE_OIDC_ISSUER` 为空时，本节的 Kite Plus 部分都不存在：`/auth/*`、`/hooks/idp`、`/app/auth/done` 和应用兑换接口返回 `404`，`GET /api/v1/auth/options` 为 `{"password": true, "oidc": null}`，注册、密码登录、安装向导和 Cookie 名都和接入前一样。
- **只用 Kite Plus**：设置了 issuer，Explore 就只用 Kite Plus 账号登录，反过来，密码相关的接口都不存在、返回 `404`：密码登录（网页，以及应用带 `client` 的密码登录）`POST /api/v1/auth/login`、注册 `POST /api/v1/auth/register`、修改密码 `PUT /api/v1/me/password`、后台重置密码 `PUT /api/v1/admin/users/{id}/password`、安装向导的 `POST /api/v1/setup`；账号页的「关联 Kite Plus 账号」（v0.1.20 的 `POST /api/v1/auth/link`）也随之去掉，升级时还没走完的关联交易回来时按 `login_expired` 处理。`GET /api/v1/auth/options` 的 `password` 为 `false`，`/api/v1/me` 的 `password_login` 和 `temporary_password` 一律为 `false`，升级前留下的会话也不会提示去改临时密码。网页 `/login`、`/en/login` 直接 302 到 `/auth/login?return=<原路径>`，页头的「登录」链接也直接指向它，读者看不到密码表单；只有登录带着错误回来（`?error=`）时，登录页才显示原因和重试按钮，身份服务连不上（`unavailable`）也一样，不会退回密码登录；网页取不到登录方式时显示「暂时无法登录」和重试，不显示表单。后台的登录页只有「使用 Kite Plus 账号登录」，已登录的账号没有后台权限时说明这一点、提供退出；个人资料页没有修改密码，用户管理没有重置密码。没有管理员时，安装向导不建账号，改为说明：第一位管理员先用 Kite Plus 账号登录一次，服务器操作者再运行 `explore users promote-admin EMAIL`。库里剩下的密码哈希不需要另外清理：没有任何地方再用它们，账号按下一条关联时也会清空。
- **网页登录**：`GET /auth/login?return=<站内相对路径>` 使用 Authorization Code + PKCE（S256），`state`、`nonce` 都是 32 字节随机数。登录交易存在 `login_flows` 里，30 分钟有效（在身份服务注册可能要等验证邮件），`state` 只存哈希；交易用一个只在登录流程里用的 Cookie 绑在发起它的浏览器上：`__Host-explore_login`（http 开发环境为 `explore_login`），`HttpOnly`、`SameSite=Lax`、30 分钟，库里同样只存哈希。浏览器已有绑定 Cookie 时，同时发起的几次登录共用这个值，都能完成；从没登录过的浏览器在两个标签页同时发起时，后发起的会换掉 Cookie，先发起的那次回来时按 `login_expired` 处理，重试即可（接受的边角情况）。每次存交易时顺带删除过期的交易。`/auth/callback` 先按 `state` 和绑定值取出交易（只能用一次，取出即删除；这个浏览器没有别的登录在等时 Cookie 随之清除），再兑换授权码，校验 ID Token 的签名（discovery 给出的 JWKS）、`iss`、`aud`、`exp` 和 `nonce`；`aud` 里还有身份服务的项目 ID，所以 `azp` 存在时必须是 Explore 的客户端 ID。实现用 `coreos/go-oidc` 和 `golang.org/x/oauth2`（`internal/idp`）。
- **找账号**：按 `(iss, sub)` 找 `user_identities`，找到就是这个账号。`email_verified` 为 `false` 时拒绝（`email_unverified`）。
  - **按已验证的邮箱关联**：没找到、ID Token 里 `email_verified` 为 `true`、而且恰好有一个本站账号的邮箱与它相同（不分大小写）且还没有关联任何身份时，把这个身份关联到那个账号，记下 Kite Plus 账号的邮箱和关联时间（与原先手动关联时相同），清空它的密码哈希和临时密码标记，结束它的全部会话，然后照常登录，并写一条日志。这条规则取代了 Explore 原先「不按邮箱合并」的规定，只在只用 Kite Plus 的服务器上生效：本站密码已经没有了，用密码注册的账号没有别的办法回来；身份服务验证过这个邮箱，等于拥有这个邮箱的人通过邮件找回了账号，与按邮箱重置密码同样可信。`email_verified` 没有给出时不按邮箱关联。
  - 邮箱已被一个关联了别的身份的账号使用，或被两个只差大小写的账号使用时，拒绝登录（`email_in_use`），登录页请读者联系站点；目标账号已被停用时返回 `account_disabled`。
  - 都不是时用声明建一个没有密码的账号：名称依次取 `name`、`preferred_username`、邮箱 `@` 前的部分，截到 80 个字符；之后由读者在 Explore 自己改。后台关闭「开放注册」时不为新身份建账号，按邮箱关联已有账号和已关联的账号照常登录。同一读者两次首次登录同时进行时，后完成的一次找到先完成的那次建好或关联好的账号，不会报邮箱被占用。
- **回到哪里**：只跳回站内相对路径；不以 `/` 开头、以 `//` 或 `/\` 开头、含空白或控制字符的，以及 `/auth/`、`/app/` 和登录页本身，一律退回 `/`。失败时跳到登录页 `/login?error=<原因>&next=<原路径>`（原路径是英文页面时用 `/en/login`），原因有 `login_expired`（交易过期、不存在或不是这个浏览器发起的）、`login_failed`（身份服务拒绝或令牌校验失败）、`access_denied`（读者在身份服务取消）、`unavailable`（身份服务连不上）、`account_disabled`、`email_in_use`、`email_unverified`、`registration_closed`，登录页按页面语言显示对应的说明和重试按钮。
- **关联与解除**：Explore 记下关联的 Kite Plus 账号的邮箱（每次登录时更新），账号页据此显示关联的是哪个账号（`GET /api/v1/me/identity`）；它可以和本站账号的邮箱不同。关联错了或读者丢了 Kite Plus 账号时，管理员在后台用户详情里「解除 Kite Plus 关联」（`DELETE /api/v1/admin/users/{id}/identities`，不能对自己的账号操作）：身份解除，账号的会话全部退出。只用 Kite Plus 时，这个账号之后只能按上一条，用邮箱相同、已验证的 Kite Plus 账号登录时重新关联；没有配置 Kite Plus 时，管理员给它重置一个临时密码。没有关联的账号在账号页看到提示：退出后用这个邮箱的 Kite Plus 账号登录即可关联。v0.1.20 的过渡做法（密码账号用密码登录后在账号页输入密码关联，关联后密码停用）已经去掉。
- **没有 Kite Plus 时的密码**：最少 12 字符，bcrypt 哈希存储。没有邮箱验证或自助找回密码；忘记密码的读者联系站点，由管理员在后台重置为临时密码，读者登录后在账号页改掉（[admin-operations.md](admin-operations.md#数据与权限边界)）。
- **会话**：登录成功后 Explore 建立自己的服务端会话，记下 ID Token 里的 `sid` 和来源 `client`（`web`、`ios`、`android`）。配置了 OIDC 且 `EXPLORE_PUBLIC_URL` 是 https 时，会话 Cookie 名为 `__Host-explore_session`，旧名 `explore_session` 不再接受（登录和退出时顺带清掉旧 Cookie），防止 `kite.plus` 下的兄弟子域写一个 Cookie 把读者登录成别人（[identity-and-comments.md §3](identity-and-comments.md#3-协议与数据)）；本地 http 开发环境设不了这个前缀，没有配置 OIDC 时也不改名，都沿用旧名。Cookie 设 `HttpOnly`、`SameSite=Lax`，https 时 `Secure`。数据库只存会话令牌哈希。退出 Explore 只删除本站会话，不退出身份服务。
- **back-channel logout**：读者在身份服务退出（或身份服务管理员结束会话）时，身份服务向 `POST /auth/backchannel-logout` 发送 `logout_token`。Explore 校验签名、`iss`、`aud`、`exp`、`iat`（不早于 10 分钟前、不晚于 5 分钟后）、`events` 里的 back-channel logout 事件、没有 `nonce`、有 `sub` 或 `sid`；`jti` 记在 `idp_receipts` 里，重放的令牌和任何一步不对的一样返回 `400`。签名密钥取不到时返回 `503`，不算令牌无效。这个地址按来源地址限流（每分钟 120 次）；令牌里的密钥 ID 不认识时才重新取一次身份服务的密钥，而且至少隔 30 秒，伪造的令牌不会每次都让 Explore 去敲身份服务的门。有 `sid` 时删除这次身份服务会话签发的会话（同时有 `sub` 时只限这个身份的账号），网页和应用的都在内；只有 `sub` 时删除这个账号的全部会话。会话自然过期时身份服务不发通知，早先签发的本站会话照常有效，直到自己过期或读者退出。
- 登录回调只建立会话，不直接做订阅等数据变更。登录后跳回本站相对路径，再由读者确认订阅。
- 管理员是具有 `is_admin` 的本站账号。没有 Kite Plus 时，第一个管理员由安装向导创建（[project-layout.md §10](project-layout.md#10-部署)）；只用 Kite Plus 时，第一位管理员先用 Kite Plus 账号登录一次，再由服务器操作者运行 `explore users promote-admin EMAIL`。之后由已有管理员在后台授予，或由服务器操作者运行同一条命令。普通账号不能访问管理接口。
- **删除与停用**：删除 Explore 资料会删除本站订阅和会话（`DELETE /api/v1/me`）。身份服务的用户事件（`user.removed`、`user.deactivated`、`user.reactivated`）推送到 `POST /hooks/idp`：Explore 用 `EXPLORE_IDP_WEBHOOK_SECRET` 校验 `ZITADEL-Signature`（时间相差 5 分钟以上的拒绝），按 `(aggregateID, sequence)` 去重 7 天。推送只当作提示：每个服务的同步令牌都读得到所有推送的签名密钥，一个服务被攻破就能给别的服务伪造推送，所以 Explore 先用 `EXPLORE_IDP_API_TOKEN` 向身份服务读这个用户现在的状态，再按状态处理：
  - **已删除**：用户接口回答 404，而且是 ZITADEL 找不到用户的那个错误（错误 ID `QUERY-Dfbg2`，按本地 v4 实测；实例不存在、路径不对等其他 404 一律当出错，`/hooks/idp` 返回 `503`），身份服务的事件记录里也查得到这个用户的 `user.removed`（推送里的事件类型可以伪造，不算数），两者都对上才删除本站账号，像 `DELETE /api/v1/me` 一样：会话、订阅和认领随之删除，举报保留但去掉举报人。唯一可用的管理员例外：删了它，库里就没有管理员，安装向导会对任何人打开，所以先停用（原因 `removed`）并撤销会话，等有了另一位可用的管理员（后台授权，或 `explore users promote-admin`），下一次对账时再删除。查不到删除记录时什么也不做。
  - **停用**（ZITADEL 的 `INACTIVE`）：停用本站账号，`disabled_by` 记为 `idp`，`disabled_reason` 记为 `deactivated`，撤销它的全部会话，登录返回 `account_disabled`。管理员停用的账号保持原样。
  - **正常**：只恢复身份服务停用的账号，管理员的停用不会被撤销。
  - **锁定不算停用**：知道读者邮箱的人输错几次密码就能让身份服务锁定这个账号，身份服务也正在关掉永久锁定，所以锁定和解锁什么也不改：`user.locked`、`user.unlocked` 的推送收下就忽略，读到的 `LOCKED` 状态按正常处理（ZITADEL 只锁定正常的账号，锁定的账号不会同时是停用的）。早先因锁定停用、原因记为 `locked` 的账号，下一次读到状态时恢复。

  推送只发一次、不重试，身份服务读不到时 `/hooks/idp` 返回 `503`。伪造的推送可以抢先占用一个将来的 `(aggregateID, sequence)`，让真的那次推送被当成重复，这种情况由对账补上。worker 每小时对账一次：用同一令牌按游标读用户事件（`idp_cursors`；查询的 `from` 不含边界，每次往前多读 5 分钟，防止身份服务提交顺序略有先后；翻页时每页也从上一页最新事件往前 5 分钟开始，按 `(aggregate, sequence)` 跳过读过的，同一时刻的事件不会在页与页之间漏掉），对其中出现、在本站有账号的用户逐个读状态，照上面处理；删除要求这批事件里有它的 `user.removed`。某个用户读不到或处理失败时记日志、跳过，游标停在它最早那条事件之前，下一次重读；等待中的删除无论如何都会处理。没有配置令牌时 webhook 和对账都关闭（[identity-and-comments.md §3](identity-and-comments.md#3-协议与数据)）。

<a id="11-应用登录-设计中"></a>

### 1.1 应用登录

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
2. **回调用应用认领的 https 地址**：`https://explore.kite.plus/app/auth/done`（`EXPLORE_PUBLIC_URL` 加 `/app/auth/done`）。iOS 用 Associated Domains 加 `ASWebAuthenticationSession.Callback.https`（iOS 17.4 起，应用最低 iOS 26），Android 用 App Links（`autoVerify`）；Explore 为此发布 `/.well-known/apple-app-site-association` 和 `/.well-known/assetlinks.json`，前者要 Apple 开发者账号的 Team ID（`EXPLORE_APPLE_APP_IDS`，如 `TEAMID.plus.kite.explore`），后者要 Android 发布证书的 SHA-256 指纹（`EXPLORE_ANDROID_CERT_SHA256`，包名 `EXPLORE_ANDROID_PACKAGE` 默认 `plus.kite.explore`），没配置的那个文件不提供。`apple-app-site-association` 里 `applinks` 认领 `/app/auth/done`，`webcredentials` 列同样的应用：`Callback.https` 要求回调域名以 `webcredentials` 登记在应用的 Associated Domains 里（iOS 实现时核对），应用两种都登记。不用 `explore://`：任何应用都能注册同一个 scheme，再自己发起一次登录、用自己的 verifier 兑换，读者以为在登录 Kite，会话却给了它（[RFC 8252](https://www.rfc-editor.org/rfc/rfc8252) §7.2、§8.6）。应用没装或认领没生效时，浏览器会真的打开这个地址，页面只提示「请在 Kite 应用中完成登录」，不显示也不使用授权码，授权码到期作废。
3. **建交易**：`GET /auth/app/start?challenge=…&client=ios|android`，challenge 是 43 个字符的 base64url，`client` 必填，记作会话的来源。Explore 建交易（模式 `app`，30 分钟有效，注册可能要等验证邮件），和网页登录一样用一个只在登录流程里用的 Cookie 把交易绑在发起它的浏览器上，然后跳到身份服务。读者在应用里选「换一个账号」时带上 `switch=1`，Explore 向身份服务请求 `prompt=select_account`，ZITADEL 的登录页会列出账号让读者选（本地实测）。要用自定义 scheme 回调时带上 `callback=scheme`（第 10 条）。参数不对返回 `400 invalid_request`。
4. **回调**：和网页登录共用 `/auth/callback`，按交易模式分开。`app` 模式校验通过后不设 Cookie 会话，而是生成授权码（32 字节随机数，base64url 43 个字符，只存哈希，2 分钟有效，只能用一次），跳到第 2 条的回调地址 `…/app/auth/done?code=…`。自定义 scheme 回调（第 10 条）多一步确认，见第 10 条。找账号的规则和网页登录相同（§1）；不成功时跳到同一个回调地址，带 `?error=<原因>`，原因与网页登录相同（§1）。交易本身已过期或不是这个浏览器发起的，就不知道是应用登录，浏览器落在网页登录页上。
5. **兑换**：`POST /api/v1/auth/app/exchange {code, verifier}`，verifier 按 RFC 7636 是 43 到 128 个字符。`base64url(SHA-256(verifier))` 必须等于交易的 challenge，授权码未过期、未用过，账号没有停用；在一个事务里把授权码标为已用并签发会话，返回 `{token, expires_at, user}`，`user` 与 `GET /api/v1/me` 相同。任何一项不对都返回同一个 `400 invalid_grant`，与登录共用限流。授权码试过一次就作废，verifier 不对也一样；用过的授权码再拿来兑换，还会删除它第一次换出的会话（RFC 6749 §4.1.2）。授权码出现在地址里，可能留在系统日志里，没有 verifier 就换不出会话；攻击者把自己账号的授权码塞给读者的应用（登录 CSRF）也不成，因为应用手里的 verifier 对不上。应用只在自己发起了登录、手里有 verifier 时才处理回调。
6. **会话**：和网页同一张 `sessions` 表，时长相同（30 天），记下 `sid` 和来源（`web`、`ios`、`android`）。应用请求改用 `Authorization: Bearer <令牌>`，不再手写 `explore_session` Cookie：接入 OIDC 时网页的 Cookie 改名为 `__Host-explore_session`，旧名不能再接受，否则兄弟子域写一个旧名的 Cookie 就能把读者登录成别人。所有要登录的接口（包括管理接口）都接受 Bearer；请求带了 `Authorization` 头就只看它，不再看 Cookie。浏览器不会自动带 `Authorization` 头，所以 Bearer 请求不需要 CSRF 头。没有 Kite Plus 的服务器上，应用的密码登录用 `POST /api/v1/auth/login {email, password, client}`，`client` 为 `ios` 或 `android` 时不设 Cookie，返回和兑换相同的 `{token, expires_at, user}`；只用 Kite Plus 的服务器上这个接口不存在（§1）。Bearer 不依赖 OIDC 配置。
7. **退出**：应用里「退出」用 Bearer 调 `POST /api/v1/auth/logout` 删除 Explore 会话，再删掉钥匙串里的令牌，不退出 `id.kite.plus`，所以再点「登录」会直接回到同一个账号，要换账号用第 3 条的「换一个账号」。读者在浏览器里退出 `id.kite.plus` 时，身份服务的 back-channel logout 按 `sid` 删除会话，应用里这次登录也一并结束，规则和网页一致（§1）。
8. **删除账号**：App Store 要求支持注册的应用能在应用内删除账号，必须删掉整个账号，不能只是停用；需要到网站完成时，给出直达删除页面的链接（审核指南 5.1.1(v) 和 Apple 的删号说明，2026-10 核实）。所以「我」页面保留现有的删除 Explore 资料（§1），另加「删除 Kite Plus 账号」，直接打开 `id.kite.plus` 的删号页面；删号事件再回到 Explore，删除本站账号。页面地址由第 10 条的 `account_url` 给出，默认是身份服务的 `/ui/console/users/me`，普通用户在那里能看到 Delete Account（本地实测），`EXPLORE_OIDC_ACCOUNT_URL` 可以改。网页账号页已经关联的账号也有同样的入口。
9. **Apple 登录**：身份服务的登录页一旦提供 GitHub 这类第三方登录，按审核指南 4.8，应用必须同时提供一种只收集姓名和邮箱、允许隐藏邮箱、不做广告追踪的登录方式，那时应用切换之前身份服务要先上线 Apple 登录；只有邮箱加密码和 Passkey 时不需要。GitHub 登录目前延后（2026-10-11），见 [identity architecture.md §2](https://github.com/kite-plus/identity/blob/main/docs/design/architecture.md#2-登录方式)。
10. **按服务器提供的方式登录**：应用可以连自己部署的 Explore。`GET /api/v1/auth/options` 返回 `{"password": false, "oidc": {"name": "Kite Plus 账号", "account_url": "https://id.kite.plus/ui/console/users/me"}}`；没有配置 OIDC 时是 `{"password": true, "oidc": null}`。`name` 按 `Accept-Language`，英文是 `Kite Plus`，应用拼成「Sign In with Kite Plus」。应用据此只显示统一登录或只显示密码登录（含注册），两者不会同时出现；当前的 iOS 和 Android 版本在 `password` 为 `false` 时都不显示密码表单和注册（2026-10-11 核对）。应用的 Associated Domains 只登记了 `explore.kite.plus`，所以连其他服务器时回调退回 `explore://app/auth/done`：发起时带 `callback=scheme`。发布了应用认领文件（`EXPLORE_APPLE_APP_IDS` 或 `EXPLORE_ANDROID_CERT_SHA256` 有值）的服务器拒绝 `callback=scheme`。没有发布认领文件的服务器上，第 2 条说的冒充风险对它的每一位读者都存在，不只是手动填了服务器的人：任何网页都能把读者带到 `/auth/app/start?callback=scheme`，读者在身份服务已登录时立即回跳，授权码就交给了注册 `explore://` 的随便哪个应用。所以 scheme 回调在身份服务回跳之后先停在 Explore 的确认页：「以 <名称>（<邮箱>）的身份登录 Kite 应用？」/「Continue to the Kite app as <name> (<email>)?」，写明要登录的服务器，按页面语言显示。页面不能被嵌入（`frame-ancestors 'none'`、`X-Frame-Options: DENY`），「继续」是同源的 `POST /auth/app/confirm`，带页面里的一次性值，并且要发起登录的那个浏览器的绑定 Cookie（`SameSite=Lax`，别的网站提交不上来），10 分钟有效、只能用一次；通过后才生成授权码，`303` 跳到 `explore://app/auth/done?code=…`。「取消」跳到 `explore://app/auth/done?error=access_denied`；确认过期、用过或不是这个浏览器的，跳到 `explore://app/auth/done?error=login_expired`。https 回调（应用认领的地址）不变，不经过确认页。
11. **过渡**：Cookie 改名之后，还在手写 `explore_session` 的老版本应用会掉线，所以改用 Bearer 的那一版要在 identity 的 I3 之前发出。改名只在配置了 OIDC 的 https 服务器上发生，没有配置之前老版本照常工作。以前用密码注册的读者，在应用或网页里用邮箱相同的 Kite Plus 账号登录，就按 §1 自动关联回原来的账号，应用里不需要别的步骤。

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
user_identities (issuer, subject, user_id, linked_at, email,
                 primary key (issuer, subject))
sessions (token_hash primary key, user_id -> users on delete cascade,
          created_at, expires_at, sid, client)
subscriptions (user_id -> users on delete cascade,
               blog_id -> blogs on delete cascade, created_at,
               primary key (user_id, blog_id))
blog_owners (blog_id primary key, user_id -> users on delete cascade, verified_at)
blog_claim_challenges (blog_id, user_id, token_hash, expires_at)
login_flows (state_hash primary key, browser_hash, mode, nonce, verifier,
             return_path, user_id -> users on delete cascade, challenge,
             client, app_redirect, expires_at)
app_codes (code_hash primary key, user_id -> users on delete cascade, sid,
           challenge, client, expires_at, used_at, session_hash)
idp_receipts (kind, id, expires_at, primary key (kind, id))
idp_cursors (issuer primary key, read_until, updated_at)
app_confirmations (token_hash primary key, browser_hash,
                   user_id -> users on delete cascade, sid, challenge,
                   client, expires_at)
```

用户看到的 ID 是 `number`：按注册顺序递增的数字，账号页显示为「ID 12 · 2026年9月5日加入」，后台列表和详情也叫 ID，`/api/v1/me` 返回 `number` 和 `created_at`。`id` 列是 UUID，只在内部使用：其他表和统一身份服务都按它关联账号，接口路径里的账号也用它，后台详情里标为 UUID。`number` 由数据库序列生成（`GENERATED ALWAYS AS IDENTITY`），不能修改，可用于以后按注册先后安排的活动。注册时先确认邮箱没被占用再插入，重复注册不会消耗 ID；空号只来自删除的账号，以及两个人同时用同一邮箱注册这种极少见的情况。迁移 `00018_user_number.sql` 按注册时间给已有账号回填 1、2、3…，新账号从最大值之后继续。

`feed_token` 是订阅流公开地址里的随机值（32 字节，base64url），没有发布时为空，由迁移 `00022_feed_token.sql` 加入。它按原样保存而不是只存哈希：读者要在账号页再次看到地址，而它能透露的订阅本来就存在同一个库里。

迁移 `00023_oidc.sql` 加入 Kite Plus 登录用到的部分（§1、§1.1）。`sessions.sid` 是签发会话时 ID Token 里的身份服务会话，back-channel logout 按它删除，密码登录的会话为空；`client` 是 `web`、`ios` 或 `android`。`login_flows` 是等身份服务回跳的登录交易：`state` 和浏览器绑定值只存哈希，`nonce` 和给身份服务的 PKCE verifier 按原样保存（回调时要用，交易 30 分钟后作废）；`mode` 为 `web`、`app`（`challenge`、`client`、`app_redirect` 描述应用登录）或 `link`（v0.1.20 账号页关联用，`user_id` 是要关联的账号；现在只会遇到升级前留下的，按过期处理）。`app_codes` 是给应用的一次性授权码，只存哈希，`session_hash` 是它换出的会话，授权码被重复使用时据此删除。`idp_receipts` 记下最近收到的 logout token 的 `jti` 和推送事件的 ID，用来拒绝重放和重复投递。`idp_cursors` 是对账读到的事件位置。迁移 `00024_sign_in_review.sql` 给 `login_flows` 按浏览器绑定值建索引，给 `user_identities` 加 `email`（关联的 Kite Plus 账号的邮箱，账号页显示），并加入 `app_confirmations`：等读者在浏览器里确认的自定义 scheme 应用登录（§1.1 第 10 条），确认值和浏览器绑定值只存哈希，10 分钟有效。这些都不是新的读者信息：过期的交易、确认、授权码和回执由 worker 的每日维护删除，存新交易和确认时也顺带删除过期的。身份服务停用的账号，`users.disabled_by` 为 `idp`，`disabled_reason` 为 `deactivated` 或 `removed`；早先版本记的 `locked` 会在下一次读到状态时恢复（§1）。只用 Kite Plus 时 `users.password_hash` 不再使用，也不需要清理：账号按已验证的邮箱关联时清空它，表结构不变，这一版没有迁移。

不建 GitHub 凭据表，Explore 不持有身份服务的任何令牌。清空 `entries` 不丢订阅；订阅流用 `subscriptions` 过滤文章并复用游标分页。博客认领要求用户在 `_explore-claim.<host>` 发布随机值对应的 DNS TXT 记录，验证值有效期 30 分钟；一个博客只能有一个已验证归属。

## 6. 接口与前端

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/auth/register` | 创建本站账号并建立会话；只用 Kite Plus 时不存在（§1） |
| POST | `/api/v1/auth/login` | 邮箱密码登录；只用 Kite Plus 时不存在 |
| POST | `/api/v1/auth/logout` | 删除 Explore 会话 |
| GET | `/api/v1/auth/options` | 服务器提供哪些登录方式（§1.1） |
| GET | `/auth/login?return=…` | 用 Kite Plus 账号登录：建交易并跳到身份服务（§1） |
| GET | `/auth/callback` | 身份服务回跳：网页登录、关联和应用登录共用（§1、§1.1） |
| GET | `/api/v1/me/identity` | 关联的 Kite Plus 账号：`{email, linked_at}`，没有关联时 `404`（§1） |
| DELETE | `/api/v1/admin/users/{id}/identities` | 管理员解除账号的 Kite Plus 关联并让它全部下线（§1） |
| POST | `/auth/app/confirm` | 自定义 scheme 应用登录的确认（§1.1 第 10 条） |
| POST | `/auth/backchannel-logout` | 身份服务的 back-channel logout（§1） |
| POST | `/hooks/idp` | 身份服务推送的用户事件（§1） |
| GET | `/auth/app/start` | 应用登录：建交易并跳到身份服务（§1.1） |
| GET | `/app/auth/done` | 应用登录的回调地址；在浏览器里打开时提示回到应用（§1.1） |
| POST | `/api/v1/auth/app/exchange` | 应用登录：用授权码和 verifier 换会话令牌（§1.1） |
| GET | `/.well-known/apple-app-site-association`、`/.well-known/assetlinks.json` | 让应用认领回调地址，配置了才提供（§1.1） |
| GET、PATCH、DELETE | `/api/v1/me` | 本站资料、昵称、删除账号（唯一可用的管理员除外） |
| PUT | `/api/v1/me/password` | 验证当前密码后修改密码，撤销其他会话，清除管理员重置留下的临时标记；账号页「修改密码」使用；只用 Kite Plus 时不存在 |
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

`/login` 是登录页：配置了 Kite Plus 时直接跳到 `/auth/login`，只在登录带着错误回来时显示原因和重试（§1）；没有配置时是本站账号登录与注册页。账号页有「Kite Plus 账号」一节：已关联的显示关联的邮箱、时间和去身份服务管理账号的链接，没关联的提示用同一邮箱的 Kite Plus 账号登录；没有密码可用时不显示「修改密码」；已关联的账号在「删除账号」旁边另有「删除 Kite Plus 账号」。后台用户详情显示关联的身份、各会话的来源和身份服务停用的原因。`/following`、`/account` 均有英文页面。页头的登录状态由服务端按请求带的会话画出，画了读者名称的页面 `private, no-store`，匿名页面照常公开缓存（[frontend.md §5](frontend.md#5-数据获取)）；个人页面一律 `private, no-store`；已登录的读者打开 `/login` 直接跳到原本要去的页面。Cookie 会话的所有已登录写操作要求会话对应的 CSRF 请求头；`Authorization: Bearer` 请求不需要（§1.1 第 6 条）。

## 7. 部署与待定

统一身份服务单独部署，提供稳定的 HTTPS issuer。Explore 的配置都可选，issuer 为空就不接入（[project-layout.md §4](project-layout.md#4-配置)）：

| 变量 | 用途 |
|---|---|
| `EXPLORE_OIDC_ISSUER`、`EXPLORE_OIDC_CLIENT_ID`、`EXPLORE_OIDC_CLIENT_SECRET` | 身份服务和 Explore 的机密客户端（`client_secret_basic`）；有 issuer 时后两个必填，密钥只放服务端；`EXPLORE_PUBLIC_URL` 是 https 时 issuer 也必须是 https，除非打开了 `EXPLORE_ALLOW_PRIVATE_NETWORKS`（本地开发） |
| `EXPLORE_OIDC_ACCOUNT_URL` | 读者管理和删除 Kite Plus 账号的页面，默认 issuer 下的 `/ui/console/users/me` |
| `EXPLORE_IDP_WEBHOOK_SECRET` | 推送用户事件的签名密钥；需要同时配置下一项 |
| `EXPLORE_IDP_API_TOKEN` | 身份服务同步用机器用户的令牌，读用户状态和事件，用于确认推送和每小时对账 |
| `EXPLORE_APPLE_APP_IDS`、`EXPLORE_ANDROID_PACKAGE`、`EXPLORE_ANDROID_CERT_SHA256` | 应用认领回调地址用的两个 `.well-known` 文件（§1.1） |

身份服务里给每个环境登记独立的客户端，地址都由 `EXPLORE_PUBLIC_URL` 加路径组成：回调 `/auth/callback`（本地开发两个都登记：`http://localhost:4321/auth/callback` 和 `http://localhost:8080/auth/callback`），back-channel logout `/auth/backchannel-logout`，用户事件 `/hooks/idp`（本地开发时身份服务在 Docker 里，经 `http://host.docker.internal:8080` 直接找到 serve）。`/auth/*`、`/hooks/*`、`/app/auth/done` 和两个 `.well-known` 文件都由 serve 提供：Caddyfile 把它们和 `/api/*` 一起交给 serve；web 也把这些路径原样转给 serve，供 `astro dev` 和前面没有 Caddy 的部署使用。生产身份服务需要数据库备份、邮件渠道、TLS 和管理员初始化。

第一条端到端链路是身份服务 → Explore 登录与订阅；第二条是身份服务 → 统一评论服务 → 通用博客嵌入组件。二者使用同一身份服务，数据与 Cookie 分开。外部博客只有主动嵌入组件或接入兼容适配器，才有统一评论登录体验。

已定：身份服务 `id.kite.plus`、评论服务 `comments.kite.plus`，都部署在境外（[identity-and-comments.md §2](identity-and-comments.md#2-组件与责任)）；账号删除后历史评论的处理由评论服务负责（[comments login.md §7](https://github.com/kite-plus/comments/blob/main/docs/design/login.md#7-账号删除与停用)）。

待定：订阅上限与是否展示人数；标签落地页与隐私政策文字。

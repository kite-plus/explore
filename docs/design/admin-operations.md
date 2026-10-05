# 内容运营后台

后台入口为 `/admin`，只接受具有后台权限（`is_admin`）的本站账号：登录后使用 HttpOnly 会话 Cookie，写操作校验 CSRF 令牌。新安装还没有管理员时，`/admin` 显示安装向导，第一个管理员在那里创建（[project-layout.md §10](project-layout.md#10-部署)）。后续接入 OIDC 时，可替换账号登录入口，用户、订阅、认领及后台权限数据不需要重建。

## 页面与真实数据

| 页面 | 数据与操作 |
| --- | --- |
| 工作台 | 博客、文章、用户、待审核、抓取异常与待执行数；展示抓取进程心跳 |
| 收录审核 | 查看并处理投稿；已收录域名由创建博客事务同步归档 |
| 博客管理 | 检索、直接收录、编辑、暂停、立即抓取及查看抓取记录；列出每个博客当前缓存的文章数（最多 20 篇，见 `policy.EntriesPerBlog`） |
| 文章管理 | 检索缓存文章，记录原因后隐藏或恢复 |
| 下架审批 | 创建内部申请、接收已登录用户举报，审批博客或文章下架 |
| 用户管理 | 按邮箱、名称、ID（`12` 或 `#12`）或 UUID 检索，按状态和角色筛选，按 ID、注册时间、最近活跃、订阅数或认领数排序；详情抽屉列出会话、订阅、认领、待验证认领和提交的举报；停用（必须填写原因）或恢复、强制下线、编辑资料（名称和邮箱）、重置密码、授予或取消后台权限、解除认领、删除账号；停用、恢复和强制下线可以批量操作 |
| 抓取任务 | 查看队列状态、进程心跳、错误和抓取尝试，手动排队 |
| 公告与推广 | 发布、撤下、编辑和删除 Explore 自己的公告与广告，设置位置、受众和起止时间（[notices.md](notices.md)） |
| 系统设置 | 控制注册、投稿、任务领取，以及站点公告 |
| 个人资料 | 修改自己的名称和密码；改密码要输入当前密码，完成后其他设备上的登录失效，当前设备保持登录 |

所有后台数据都通过 `/api/v1/admin/*` 同源代理调用 Gin API。页面不内置模拟记录。文章缓存仍只存标题、摘要和链接，不保存正文。

## 界面

后台移植自 [satnaing/shadcn-admin](https://github.com/satnaing/shadcn-admin)（MIT，许可文本见 `web/src/admin/LICENSE-shadcn-admin.txt`），代码在 `web/src/admin/`，目录结构与原项目一致：`components/ui` 是原项目的 Radix 版 shadcn/ui 组件，`components/layout`、`components/data-table` 是侧边栏布局和数据表格，`features/` 按页面划分。前台的 `web/src/components/ui` 不受影响。

- **路由**：所有 `/admin/*` 路径由 `pages/admin/[...path].astro` 返回同一个只在浏览器渲染的 React 应用，`src/admin/router.tsx` 用 History API 切换页面，代替原项目的 TanStack Router。查询参数和原项目一样以 JSON 表示，列表的筛选、分页和搜索都写进地址。
- **数据**：TanStack Query 读取 `/api/v1/admin/*`，写操作完成后刷新全部后台查询，侧边栏的待处理数随之更新。博客、投稿、下架申请、抓取任务和排除名单一次取全量，在浏览器里筛选；文章和用户由接口分页和检索，用户的筛选和排序也由接口完成，筛选项旁的数量来自接口返回的 `counts`。用户详情抽屉的账号 ID 写在地址的 `user` 参数里，可以直接链接；下架审批里的举报人链接到用户管理并按邮箱检索。
- **样式**：后台使用单独的 `styles/admin.css`，只扫描 `src/admin`，读者页面不加载后台样式。配色变量与前台共用（`styles/theme.css`），字体仍是系统字体；深色模式与前台共用 `localStorage` 里的 `theme` 选择。
- **CSP**：Radix 的滚动锁定、滚动区域和 sonner 会在运行时插入 `<style>` 元素，所以后台页面的 `style-src-elem` 允许 `'unsafe-inline'`；内联脚本和内联 `style` 属性仍被禁止，读者页面的策略不变。
- **账号入口**：只在侧边栏底部，页头不再放头像。头像是名称的首字母，底色由邮箱算出，与前台博客头像同一套配色，改名后颜色不变；菜单里是个人资料、查看前台和退出登录。
- **与原项目的差异**：登录页用 `sign-in-2` 的版式但去掉右侧截图和第三方登录按钮，没有找回密码，安装向导沿用同一版式；前台的登录与注册页（`/login`）也用这套版式和表单样式，只是保留站点的页头和页脚；界面偏好（侧边栏、布局）存在 `localStorage` 而不是 Cookie；没有移植配置抽屉、字体切换和演示页面。

## 数据与权限边界

迁移 `00009_admin_operations.sql` 增加账号停用时间、站点设置、文章屏蔽记录和下架申请。文章屏蔽以 `(blog_id, identity)` 为键，抓取器更新或短暂移除文章后，屏蔽决定仍然生效。公开文章流、博客详情、订阅流、图片和链接检查都会排除这些记录。恢复展示会删除屏蔽记录。

安装向导不要求安装码：还没有管理员时，第一个完成安装的人就成为管理员。迁移 `00011_setup_code.sql` 曾增加保存安装码的 `setup_code` 表，`00014_drop_setup_code.sql` 已将它删除。完成安装和调整后台权限用同一把事务锁判断现有管理员，两个安装请求不会同时成功。

迁移 `00017_user_admin.sql` 给账号增加停用原因（`disabled_reason`，最多 500 字）、操作人（`disabled_by`，记为管理员邮箱）和最近活跃时间（`last_seen_at`）。最近活跃在登录和每次凭会话访问时更新，同一账号 5 分钟内只写一次库；迁移时用账号现存最新会话的创建时间回填。只记时间，不记地址和设备。

Explore 没有邮件服务，读者忘了密码不能自助找回，只能联系站点，由管理员在后台重置。重置时浏览器生成 16 位随机临时密码（四段，去掉 0/O、1/l/I 这类易混字符），管理员可以换一个或改成自己想给的，提交后该账号所有会话失效；重置成功后弹窗再显示一次密码供复制，关闭后不再显示，服务端只存哈希。迁移 `00019_password_reset.sql` 增加 `password_reset_at` 和 `password_reset_by`，记下重置时间和操作的管理员；读者自己改密码后两者清空。`/api/v1/me` 和登录响应的 `temporary_password` 表示密码还是管理员给的：读者用临时密码登录后直接进入账号页的「修改密码」，页面顶部提醒尽快修改；后台详情抽屉显示「临时密码，用户还没改」。

编辑资料可以同时改名称和邮箱。邮箱就是登录名，改后用新邮箱登录，密码不变；邮箱被其他账号占用时返回 `409 email_taken`，显示在邮箱输入框下。

博客下架审批通过后将博客设为 `paused` 并保留原因；文章下架审批通过后写入屏蔽记录。审批和状态变更在同一数据库事务中提交。被停用的用户无法登录，现有会话会被撤销。撤销后台权限时会清除该账号的会话。停用或降级最后一名可用管理员返回 `409 keep_admin`，给已停用的账号授予后台权限返回 `409 account_disabled`。管理员不能在后台停用、降级、强制下线、重置密码或删除自己的账号（`403 own_account`），可以编辑自己的资料。管理员删除账号与读者自己删除账号相同：会话、订阅和认领一起删除，举报保留但去掉举报人。解除认领只删除归属记录，博客不受影响，之后可以重新认领。

用户可在博客详情页点击“报告问题”。举报需要登录并提供至少 5 个字符的原因，提交到 `POST /api/v1/reports`；管理员也可在后台新建下架申请。重复的待处理申请会返回冲突错误。

## 接口速查

| 接口 | 用途 |
| --- | --- |
| `GET /api/v1/admin/overview` | 工作台统计和抓取进程状态 |
| `GET /api/v1/admin/users` | 用户分页检索、筛选和排序，返回 `data`、`total` 和 `counts`；每个账号带用户看到的 ID `number`（[accounts.md §5](accounts.md#5-数据模型)） |
| `GET /api/v1/admin/users/:id` | 用户详情：有效会话、订阅（最近 200 个）、认领、待验证认领、提交的举报（最近 200 条） |
| `PATCH /api/v1/admin/users/:id` | 改资料（`display_name`、`email`，可以一起改），或者只改 `disabled`（停用时必须带 `reason`）、`is_admin` 中的一项 |
| `PUT /api/v1/admin/users/:id/password` | 重置密码，`password` 为 12–72 字节；撤销该账号所有会话并标为临时密码 |
| `DELETE /api/v1/admin/users/:id` | 删除账号 |
| `DELETE /api/v1/admin/users/:id/sessions` | 强制下线，返回结束的有效会话数 `revoked` |
| `DELETE /api/v1/admin/users/:id/blogs/:host` | 解除账号对博客的认领 |
| `GET /api/v1/admin/entries`、`PATCH /api/v1/admin/entries/:id` | 文章分页检索、隐藏或恢复 |
| `GET/POST /api/v1/admin/takedowns`、`POST /api/v1/admin/takedowns/:id/review` | 下架申请及审批 |
| `GET /api/v1/admin/settings`、`PATCH /api/v1/admin/settings/:key` | 持久化站点设置 |
| `GET /api/v1/site-config` | 公开的站点公告与注册状态 |
| `GET/POST /api/v1/admin/notices`、`PATCH/DELETE /api/v1/admin/notices/:id` | 公告与广告的增删改 |
| `GET /api/v1/setup`、`POST /api/v1/setup` | 安装状态、创建第一个管理员 |
| `PATCH /api/v1/me`、`PUT /api/v1/me/password` | 个人资料页修改名称和密码，与前台共用账号接口；前台账号页也用后者修改密码 |

`users` 与 `entries` 使用 `limit`、`offset` 和 `q` 查询参数，每页最多 100 条。`users` 另接受 `status`（`active`、`disabled`）、`role`（`admin`、`reader`）、`sort`（`number`、`created`、`seen`、`subscriptions`、`blogs`，默认 `created`）和 `order`（`asc`、`desc`，默认 `desc`）；从未活跃的账号在按最近活跃排序时视为最早。`counts` 是各筛选项的数量，每组只套用检索词和另一组筛选，与筛选弹层的计数一致。下架申请按 `pending`、`approved`、`rejected` 查询。

设置项只允许 `registration_enabled`、`submissions_enabled`、`crawler_paused` 和 `site_notice`。前两项分别在注册与投稿接口校验；抓取暂停在任务领取查询中生效，队列不会被清空；公告展示在前台导航下方。设置页不会修改抓取进程本身，进程离线时仍需按运维方式启动。

## 本地验证

先运行数据库迁移，再启动 `explore serve` 与 Astro 开发服务。抓取进程可保持停止，工作台会明确显示离线状态。执行 `go test ./internal/store ./internal/api` 时设置 `EXPLORE_TEST_DATABASE_URL`，测试会在独立 schema 中迁移并自动清理。前端使用 `npm run check`、`npm run build` 和 `npm test` 验证。

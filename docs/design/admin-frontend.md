# 管理后台前端可视化实现设计文档

> 本文档是 [admin.md](./admin.md) 的**前端专项实现设计文档**，侧重于页面路由、组件树、数据流、鉴权实现、目录结构等工程细节。
>
> 阅读本文档的读者应已了解 admin.md 中的产品定位与 API 契约。

---

## 目录

1. [目录结构规划](#1-目录结构规划)
2. [页面路由设计](#2-页面路由设计)
3. [鉴权机制实现](#3-鉴权机制实现)
4. [前端同源代理路由](#4-前端同源代理路由)
5. [数据类型定义](#5-数据类型定义)
6. [数据获取与状态管理策略](#6-数据获取与状态管理策略)
7. [各页面组件树详细设计](#7-各页面组件树详细设计)
   - 7.1 登录页
   - 7.2 审核工作台
   - 7.3 博客治理
   - 7.4 排除名单
   - 7.5 诊断工具
8. [共享组件库（Admin UI Kit）](#8-共享组件库admin-ui-kit)
9. [键盘快捷键与审核效率设计](#9-键盘快捷键与审核效率设计)
10. [错误边界与 Loading 状态规范](#10-错误边界与-loading-状态规范)
11. [响应式与可访问性基线](#11-响应式与可访问性基线)

---

## 1. 目录结构规划

```
web/src/
├── pages/
│   └── admin/
│       ├── index.astro                  # 重定向 → /admin/submissions
│       ├── submissions.astro            # 审核工作台（Astro SSR 壳 + React 岛）
│       ├── blogs.astro                  # 博客治理列表
│       ├── excluded-hosts.astro         # 排除名单
│       └── tools.astro                  # 在线诊断工具
│
├── pages/api/v1/admin/
│   └── [...path].ts                     # 通配符代理：同源转发所有 admin API 请求
│
└── components/admin/
    ├── AdminShell.tsx                   # 顶部导航 + 认证 Context Provider
    ├── AdminLogin.tsx                   # Token 输入弹窗
    │
    ├── submissions/
    │   ├── SubmissionsQueue.tsx         # 主体：状态 Tab + 列表
    │   ├── SubmissionCard.tsx           # 单条提交卡片（高信息密度展示）
    │   ├── CheckReportPanel.tsx         # 折叠式检测报告面板
    │   ├── ApproveDialog.tsx            # 审批通过弹窗（含字段编辑）
    │   └── RejectDialog.tsx             # 驳回弹窗（含快捷理由 Chip）
    │
    ├── blogs/
    │   ├── BlogsTable.tsx               # 博客列表（带搜索/筛选栏）
    │   ├── BlogRow.tsx                  # 表格行组件
    │   ├── BlogDetailDrawer.tsx         # 博客详情侧边抽屉（编辑 + 运维操作）
    │   ├── EditBlogForm.tsx             # 博客元数据编辑表单
    │   └── DeleteBlogDialog.tsx         # 确认下架对话框（含排除原因选择）
    │
    ├── excluded/
    │   ├── ExcludedTable.tsx            # 排除名单表格
    │   └── RestoreDialog.tsx            # 确认解除排除对话框
    │
    ├── tools/
    │   ├── InspectForm.tsx              # URL 输入表单
    │   └── CheckReportView.tsx          # 完整检测报告展示（可复用 CheckReportPanel）
    │
    └── ui/
        ├── AdminBadge.tsx               # 状态徽章（统一状态色彩映射）
        ├── CopyButton.tsx               # 一键复制 URL/Host 按钮
        ├── TimeAgo.tsx                  # 相对时间显示（"10 分钟前"）
        └── KeyHint.tsx                  # 键盘快捷键提示标签
```

---

## 2. 页面路由设计

### 路由表

| 路径 | 对应文件 | 访问权限 | 主要功能 |
|---|---|---|---|
| `/admin` | `index.astro` | 需鉴权（客户端重定向） | 重定向至 `/admin/submissions` |
| `/admin/submissions` | `submissions.astro` | 需鉴权 | 审核工作台 |
| `/admin/blogs` | `blogs.astro` | 需鉴权 | 博客治理 |
| `/admin/excluded-hosts` | `excluded-hosts.astro` | 需鉴权 | 排除名单管理 |
| `/admin/tools` | `tools.astro` | 需鉴权 | 在线诊断工具 |

### Astro 页面模板结构

每个 Astro 页面（`submissions.astro`）采用同一种极简壳结构：

```astro
---
// 服务端完全不做鉴权，所有鉴权在客户端 React 层处理
// Astro 页面只负责输出极简 HTML 壳，加快首字节响应
---
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <title>Explore Admin</title>
    <!-- 不引用读者侧 theme.js；管理端用独立 minimal-theme.js -->
  </head>
  <body>
    <!-- AdminShell 是整页 React 岛，client:load 立即激活 -->
    <AdminShell client:load activeTab="submissions">
      <SubmissionsQueue />
    </AdminShell>
  </body>
</html>
```

**为什么不在 Astro 服务端做 Token 验证？**

- Astro 服务端验证需要从 `Cookie` 或请求头读取 Token，而 Token 存储在浏览器的 `sessionStorage` 中，服务端无法直接访问；
- 所有敏感数据来自 API，API 已有 `requireAdmin()` 中间件严格把关；
- 未登录时，前端 React 只渲染登录弹窗，不渲染任何数据；
- Astro 页面自身不含任何敏感信息，暴露 HTML 壳无安全风险。

---

## 3. 鉴权机制实现

### 3.1 Auth Context

```typescript
// web/src/components/admin/AdminShell.tsx

/** 鉴权上下文：Token 状态与操作方法 */
interface AdminAuthContext {
  /** 当前 Token（原始字符串），null 表示未登录 */
  token: string | null;
  /** 登录：将 Token 写入 sessionStorage */
  login: (token: string, remember?: boolean) => void;
  /** 登出：清除 Token 并重新展示登录弹窗 */
  logout: () => void;
  /** 判断是否已登录 */
  isAuthenticated: boolean;
}

const AuthContext = createContext<AdminAuthContext>(null!);
export const useAdminAuth = () => useContext(AuthContext);
```

### 3.2 Token 持久化策略

| 存储位置 | 触发条件 | 生命周期 |
|---|---|---|
| `sessionStorage["admin_token"]` | 默认（不勾选"记住我"） | 标签页关闭即清除 |
| `localStorage["admin_token"]` | 勾选"记住登录状态" | 手动登出或清除浏览器数据时清除 |

**读取优先级**：`sessionStorage` → `localStorage`

### 3.3 请求拦截与 401 自动登出

所有管理端 API 请求通过统一的 `adminFetch` 封装发出：

```typescript
// web/src/lib/admin-api.ts

/**
 * 管理端 API 请求封装
 * 自动附加 Authorization 头；遇到 401 时触发登出回调
 */
async function adminFetch(
  path: string,
  options: RequestInit = {},
  token: string,
  onUnauthorized: () => void,
): Promise<Response> {
  const res = await fetch(`/api/v1/admin${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
      ...options.headers,
    },
  });

  if (res.status === 401) {
    onUnauthorized(); // 触发登出，弹出登录弹窗
    throw new AdminUnauthorizedError();
  }
  return res;
}
```

### 3.4 登录弹窗行为

- **初始化**：`AdminShell` 挂载后立即检查 sessionStorage/localStorage；
- **未登录**：渲染 `AdminLogin` 全屏居中弹窗，主体内容不渲染；
- **已登录**：渲染主体内容，`AdminLogin` 不渲染；
- **Token 错误（401）**：`onUnauthorized` 回调清除 Token，自动弹出 `AdminLogin`，同时在弹窗内展示 "Token 无效，请重新输入" 提示。

---

## 4. 前端同源代理路由

### 4.1 通配符代理文件

**路径**：`web/src/pages/api/v1/admin/[...path].ts`

该文件将所有 `/api/v1/admin/**` 请求透明代理至后端 Gin 服务，并**原样透传客户端发出的 `Authorization` 头**：

```typescript
// web/src/pages/api/v1/admin/[...path].ts
import type { APIRoute } from "astro";

export const ALL: APIRoute = async ({ request, params }) => {
  const path = params.path ?? "";
  const url = new URL(request.url);
  // 将完整的 query string 也透传给后端
  const target = `${import.meta.env.EXPLORE_API_URL}/api/v1/admin/${path}${url.search}`;

  const res = await fetch(target, {
    method: request.method,
    headers: request.headers, // Authorization 头原样透传
    body: ["GET", "HEAD"].includes(request.method) ? undefined : request.body,
    // @ts-expect-error Node fetch duplex 选项
    duplex: "half",
  });

  return new Response(res.body, {
    status: res.status,
    headers: {
      "Content-Type": res.headers.get("Content-Type") ?? "application/json",
    },
  });
};
```

> **注意**：`export const ALL` 捕获所有 HTTP 方法（GET / POST / PATCH / DELETE），无需为每个方法单独导出。

### 4.2 代理路由覆盖的后端 API

| 客户端请求路径 | 转发至后端 | 说明 |
|---|---|---|
| `GET /api/v1/admin/submissions?status=pending` | 同 | 待审列表 |
| `POST /api/v1/admin/submissions/{id}/approve` | 同 | 审批通过 |
| `POST /api/v1/admin/submissions/{id}/reject` | 同 | 驳回 |
| `GET /api/v1/admin/blogs?health=unhealthy` | 同 | 博客列表 |
| `POST /api/v1/admin/blogs` | 同 | 直接添加博客 |
| `PATCH /api/v1/admin/blogs/{host}` | 同 | 更新博客元数据 |
| `DELETE /api/v1/admin/blogs/{host}?exclude=opt_out` | 同 | 移除博客（含排除） |
| `POST /api/v1/admin/blogs/{host}/fetch` | 同 | 立即抓取 |
| `GET /api/v1/admin/excluded-hosts` | 同 | 排除名单 |
| `DELETE /api/v1/admin/excluded-hosts/{host}` | 同 | 解除排除 |
| `POST /api/v1/admin/check` | 同 | 在线诊断 |

---

## 5. 数据类型定义

位于 `web/src/lib/admin-types.ts`，与后端 Go 结构体严格对应。

```typescript
// web/src/lib/admin-types.ts

/** 对应后端 adminSubmissionJSON（嵌套 submissionJSON） */
export interface AdminSubmission {
  id: string;
  status: "pending" | "approved" | "rejected";
  host: string;
  site_url: string;
  feed_url: string;
  /** 探测报告：含检查结果、博客元数据与诊断问题 */
  check_report: CheckReport | null;
  /** 审核驳回原因（approved/rejected 时有值） */
  review_note: string;
  /** 提交者的备注说明 */
  note: string;
  /** 审核者标识 */
  reviewed_by: string;
  created_at: string; // ISO 8601
  reviewed_at: string | null;
}

/** 对应后端 adminBlogJSON */
export interface AdminBlog {
  host: string;
  name: string;
  description: string;
  site_url: string;
  feed_url: string;
  language: string;
  generator: string;
  status: "active" | "paused";
  status_note: string;
  show_excerpt: boolean;
  extra_domains: string[];
  default_tags: string[];
  /** 是否被前端判定为健康可见（active + 30 天内成功抓取） */
  visible: boolean;
  fetch_interval_seconds: number;
  next_fetch_at: string;
  last_fetched_at: string | null;
  last_succeeded_at: string | null;
  consecutive_failures: number;
  last_error: string;
  gone_since: string | null;
  created_at: string;
}

/** 对应后端 excludedJSON */
export interface ExcludedHost {
  host: string;
  reason: "opt_out" | "blocked";
  note: string;
  created_at: string;
}

/** 对应后端 model.CheckReport（公共类型，复用读者侧的定义） */
export interface CheckReport {
  passed: boolean;
  title: string;
  description: string;
  latest_entry_title: string;
  feed_url: string;
  generator: string;
  language: string;
  items_total: number;
  items_valid: number;
  latest_published_at: string | null;
  problems: Problem[];
}

export interface Problem {
  code: string;
  message: string;
  level: "error" | "warning" | "info";
}

/** 审批通过请求体 */
export interface ApprovePayload {
  name?: string;
  language?: string;
  feed_url?: string;
  extra_domains?: string[];
  show_excerpt?: boolean;
}

/** 博客元数据更新请求体 */
export interface UpdateBlogPayload {
  name?: string;
  language?: string;
  feed_url?: string;
  extra_domains?: string[];
  default_tags?: string[];
  show_excerpt?: boolean;
  status?: "active" | "paused";
  status_note?: string;
}
```

---

## 6. 数据获取与状态管理策略

### 6.1 整体策略

管理后台**不引入 React Query / SWR 等三方库**，采用与读者侧一致的手写 `fetch` 策略：

- 数据加载：`useEffect` + `useState` 手写 fetch，简洁无依赖；
- 乐观更新：操作后**立即本地更新列表状态**（删除/修改对应项），无需等待服务端重新请求；
- 轮询刷新：审核工作台（`/admin/submissions`）支持手动刷新按钮，**不自动轮询**（管理后台流量极低，无需实时推送）；
- 错误重试：网络错误时展示 `<ErrorAlert>` 提示组件，提供"点击重试"按钮，**不自动重试**（避免因 Token 失效导致的无限重试循环）。

### 6.2 自定义 Hook 设计

```typescript
// web/src/components/admin/hooks/useAdminData.ts

/**
 * 通用管理数据获取 Hook
 * 封装加载状态、错误处理、手动刷新
 */
function useAdminData<T>(
  path: string,              // 例如 "/submissions?status=pending"
  token: string | null,
  onUnauthorized: () => void,
): {
  data: T | null;
  loading: boolean;
  error: string | null;
  refresh: () => void;
}
```

```typescript
// 各模块专用 Hook（在 useAdminData 基础上封装业务逻辑）

// 提交审核
function useSubmissions(status: SubmissionStatus): { ... }

// 博客列表（含健康过滤）
function useBlogs(health?: "unhealthy"): { ... }

// 排除名单
function useExcludedHosts(): { ... }
```

### 6.3 乐观更新示例（驳回操作）

```typescript
const handleReject = async (id: string, note: string) => {
  // 1. 立即从本地列表移除该提交（乐观更新）
  setSubmissions(prev => prev.filter(s => s.id !== id));

  try {
    await adminFetch(`/submissions/${id}/reject`, {
      method: "POST",
      body: JSON.stringify({ review_note: note }),
    }, token!, logout);
  } catch (e) {
    if (e instanceof AdminUnauthorizedError) return; // 已触发登出
    // 服务端失败：回滚列表，展示错误提示
    setSubmissions(prev => [...prev, targetSubmission]);
    setError("驳回操作失败，请重试");
  }
};
```

---

## 7. 各页面组件树详细设计

### 7.1 登录页（`AdminLogin.tsx`）

**触发场景**：未登录 / Token 失效（401）

**组件结构**：

```
AdminLogin
├── Dialog（shadcn/ui，modal，不可通过 Esc/外点关闭）
│   ├── DialogTitle: "Explore 管理后台"
│   ├── DialogDescription: "输入维护者 Token 以继续"
│   ├── Input（type="password"，ref autoFocus）
│   ├── Checkbox（"记住登录状态"）
│   └── Button（"确认登录"，Enter 触发）
└── （错误提示：Input 下方红色文字 "Token 无效，请检查"）
```

**交互细节**：
- `Dialog` 的 `preventClose` 属性设为 `true`，禁止用户绕过登录；
- 按下 Enter 键等同于点击"确认登录"；
- 提交时发送一次 `GET /api/v1/admin/submissions?status=pending&limit=1` 来验证 Token 有效性，成功则登录，401 则展示错误文案。

---

### 7.2 审核工作台（`SubmissionsQueue.tsx`）

**完整组件树**：

```
SubmissionsQueue
├── Tabs（shadcn/ui Tabs，状态: pending / approved / rejected）
│   ├── TabsList
│   │   ├── TabsTrigger: "待审核 (N)"  ← N = pending 数量徽标
│   │   ├── TabsTrigger: "已通过"
│   │   └── TabsTrigger: "已驳回"
│   └── TabsContent
│       └── SubmissionList（根据当前 Tab 获取不同 status 数据）
│           ├── 加载中: 3 个骨架 SubmissionCard
│           ├── 空状态: "暂无待审提交 🎉"
│           └── SubmissionCard（×N，逐条渲染）
│               ├── [左] 博客信息区（高信息密度）
│               │   ├── Badge（status 状态徽章）
│               │   ├── [域名 Host]（加粗大字）
│               │   ├── [博客名称]（副标题）
│               │   ├── [Feed URL]（可点击链接 + CopyButton）
│               │   ├── [博客简介 description]
│               │   ├── [最新文章: 标题 · 日期]
│               │   ├── [文章统计: N 篇有效 / M 篇总计]
│               │   ├── [生成器: Hugo / Hexo / ...]（可选显示）
│               │   └── [提交者备注 note]
│               │
│               ├── [右上] 提交时间（TimeAgo）+ 审核者（approved/rejected 时显示）
│               │
│               ├── CheckReportPanel（折叠式，默认关闭）
│               │   ├── 诊断汇总行: "0 错误 · 2 警告 · 通过" + 展开图标
│               │   └── [展开后] ProblemList（每条 Problem 带 level 颜色图标）
│               │
│               └── [底部操作栏]（pending 时显示）
│                   ├── Button variant="ghost": "预览主页 ↗"（新标签打开）
│                   ├── Button variant="ghost": "检查报告"（展开 CheckReportPanel）
│                   ├── Button variant="destructive" outline: "驳回..."
│                   │   └── → 触发 RejectDialog
│                   └── Button variant="default": "审核通过 ✓"
│                       └── → 触发 ApproveDialog
```

#### ApproveDialog 组件

```
ApproveDialog（shadcn/ui Dialog）
├── DialogHeader: "确认通过收录"
├── 预览摘要区（只读，展示检测到的博客信息）
│   ├── 域名 / SiteURL 链接
│   └── 最新文章标题 + 发布日期
│
├── 可选编辑字段（均为 optional，仅在需要修改时填写）
│   ├── Input: 博客名称 name（placeholder = 检测到的标题）
│   ├── Input: 语言 language（placeholder = 检测到的语言，如 "zh-CN"）
│   ├── Input: Feed URL（placeholder = 当前值）
│   ├── TagInput: 额外域名 extra_domains（逗号/空格分隔，badge 形式展示）
│   └── Switch: 展示文章摘要 show_excerpt（默认 true）
│
├── DialogFooter
│   ├── Button variant="outline": "取消"（⌘K / Esc）
│   └── Button variant="default": "确认通过"（⌘Enter，提交中 disabled + 加载动画）
└── （错误提示区：提交失败时展示 API 错误信息）
```

#### RejectDialog 组件

```
RejectDialog（shadcn/ui Dialog）
├── DialogHeader: "驳回提交"
├── 快捷理由 Chips（单选，点击填入 Textarea）
│   ├── "非个人原创独立博客"
│   ├── "缺少 RSS/Atom 订阅源"
│   ├── "长期未更新（停更超过 1 年）"
│   ├── "内容涉及商业营销推广"
│   └── "内容质量不符合收录标准"
├── Textarea: 驳回说明 review_note（必填，最少 10 字，字数计数显示）
└── DialogFooter
    ├── Button: "取消"
    └── Button variant="destructive": "确认驳回"（review_note 为空时 disabled）
```

---

### 7.3 博客治理（`BlogsTable.tsx`）

**完整组件树**：

```
BlogsTable
├── 工具栏
│   ├── Input: 搜索框（域名 / 名称实时过滤，前端 filter，无需 API）
│   ├── Select: 状态筛选（全部 / 健康 / 异常告警 / 暂停中）
│   └── Button: "直接添加博客"（→ CreateBlogDialog）
│
├── Table（shadcn/ui Table）
│   ├── TableHeader
│   │   ├── [域名]（可排序，默认按创建时间）
│   │   ├── [名称]
│   │   ├── [状态]（Badge）
│   │   ├── [文章数]
│   │   ├── [上次成功抓取]（可排序）
│   │   ├── [连续失败]
│   │   └── [操作]
│   └── TableBody
│       └── BlogRow（×N）
│           ├── 域名（加粗，可点击展开详情）
│           ├── 名称（单行省略 truncate）
│           ├── AdminBadge（active / paused / unhealthy）
│           ├── 最近成功抓取时间（TimeAgo 或 "—"）
│           ├── 连续失败次数（> 0 时红色高亮）
│           └── 操作按钮组（DropdownMenu）
│               ├── "查看详情"（→ 展开 BlogDetailDrawer）
│               ├── "立即抓取"（→ POST fetch，Toast 确认）
│               ├── "暂停/恢复"（→ PATCH status）
│               └── "移除..."（→ DeleteBlogDialog）
│
└── BlogDetailDrawer（Sheet，右侧滑出，宽 480px）
    ├── SheetHeader: 域名 + 状态 Badge
    ├── Tab 切换: "基本信息" / "健康状态" / "编辑"
    │
    ├── [基本信息 Tab]
    │   ├── 名称、描述、语言、生成器
    │   ├── SiteURL / FeedURL（可点击链接 + CopyButton）
    │   ├── 额外域名列表（Badge 组）
    │   ├── 默认标签（Tag Badge 组）
    │   └── 创建时间
    │
    ├── [健康状态 Tab]
    │   ├── 抓取间隔（N 小时/分钟，人类可读转换）
    │   ├── 下次抓取时间（TimeAgo + 绝对时间 tooltip）
    │   ├── 上次抓取时间 / 上次成功时间
    │   ├── 连续失败次数（带进度条或颜色强调）
    │   ├── 失联时间（gone_since，不为 null 时显示警告卡片）
    │   └── 最后错误信息（code block 样式展示 last_error）
    │
    └── [编辑 Tab]
        └── EditBlogForm
            ├── Input: 博客名称
            ├── Textarea: 博客简介 description
            ├── Input: 语言
            ├── Input: Feed URL
            ├── TagInput: 额外域名
            ├── TagSelect: 默认标签（下拉多选，最多 3 个，选项来自 /api/v1/tags）
            ├── Switch: 展示文章摘要
            ├── Select: 状态（active / paused）
            ├── Input: 状态说明 status_note（status=paused 时显示）
            └── Button: "保存修改"
```

#### DeleteBlogDialog 组件

```
DeleteBlogDialog（shadcn/ui AlertDialog）
├── AlertDialogTitle: "确认移除 {host}？"
├── AlertDialogDescription: 说明此操作将移除博客及其所有文章
├── RadioGroup: 操作类型
│   ├── "仅移除（无排除，可重新提交）"
│   ├── "移除并退出收录（opt_out，写入排除名单）"
│   └── "移除并永久封禁（blocked）"
├── Input: 封禁/退出说明（reason 不为空时显示）
└── AlertDialogFooter
    ├── Button: "取消"
    └── Button variant="destructive": "确认移除"
```

---

### 7.4 排除名单（`ExcludedTable.tsx`）

```
ExcludedTable
├── 工具栏
│   ├── Input: 搜索（域名过滤）
│   └── Select: 原因筛选（全部 / opt_out / blocked）
│
└── Table
    ├── TableHeader: [域名] [排除原因] [备注] [加入时间] [操作]
    └── TableBody
        └── ExcludedRow（×N）
            ├── host（加粗）
            ├── AdminBadge（opt_out: 灰色 "申请退出" / blocked: 红色 "永久封禁"）
            ├── note（单行省略）
            ├── 加入时间（TimeAgo）
            └── Button: "解除排除"（→ RestoreDialog）

RestoreDialog（AlertDialog）
├── "确认解除 {host} 的排除名单？"
├── "解除后该域名可重新申请收录"
└── 确认 / 取消按钮
```

---

### 7.5 在线诊断工具（`InspectForm.tsx` + `CheckReportView.tsx`）

```
InspectTool
├── InspectForm
│   ├── Input: 博客网址 url（必填，placeholder="https://example.com"）
│   ├── Input: 可选指定 Feed URL（选填，展开式可选字段）
│   └── Button: "开始检测"（加载中: 禁用 + 旋转图标，最长 30 秒）
│
└── CheckReportView（检测完成后出现，带动画过渡）
    ├── 综合判定横幅（passed: 绿色"通过" / failed: 红色"未通过"）
    ├── 基本信息区
    │   ├── 博客标题、描述
    │   ├── Feed URL（最终确认的）、语言、生成器
    │   └── 文章统计（N 篇有效 / M 篇总计 / 最新发布日期）
    ├── 问题清单（Problem 列表，按 level 分组展示）
    │   ├── [Error] 红色图标 + 问题文案
    │   ├── [Warning] 黄色图标
    │   └── [Info] 蓝色图标
    └── [快捷操作栏]（仅 passed=true 时显示）
        └── Button: "直接添加此博客"（→ CreateBlogDialog，预填 URL）
```

---

## 8. 共享组件库（Admin UI Kit）

### 8.1 AdminBadge（状态徽章）

```typescript
// 状态 → 色彩映射表
const STATUS_STYLES = {
  // 提交审核状态
  pending:  "bg-amber-500/10 text-amber-600 border-amber-200",
  approved: "bg-emerald-500/10 text-emerald-600 border-emerald-200",
  rejected: "bg-red-500/10 text-destructive border-red-200",
  // 博客运行状态
  active:     "bg-emerald-500/10 text-emerald-600 border-emerald-200",
  paused:     "bg-muted text-muted-foreground border-border",
  unhealthy:  "bg-red-500/10 text-destructive border-red-200",
  // 排除名单原因
  opt_out: "bg-muted text-muted-foreground border-border",
  blocked: "bg-red-500/10 text-destructive border-red-200",
} as const;
```

### 8.2 TimeAgo（相对时间）

```typescript
/**
 * 将 ISO 时间字符串渲染为相对时间
 * 附带 title tooltip 显示绝对时间（用户可悬停查看）
 * 例："3 分钟前"  "2 天前"  "刚刚"
 */
function TimeAgo({ iso }: { iso: string | null }): JSX.Element
```

### 8.3 TagInput（标签输入）

用于 `extra_domains`、`extra_domains` 等多值字段输入：
- 用户输入后按 `Enter` 或 `空格` 确认，生成 Badge；
- Badge 右侧有 `×` 删除按钮；
- 超过最大数量限制时 Input 禁用。

### 8.4 CopyButton（一键复制）

```typescript
/**
 * 点击后复制 value 到剪贴板
 * 成功后图标变为 ✓（1.5 秒后恢复）
 */
function CopyButton({ value }: { value: string }): JSX.Element
```

---

## 9. 键盘快捷键与审核效率设计

审核工作台是高频操作区域，键盘快捷键能大幅提升审核效率：

### 9.1 全局快捷键

| 快捷键 | 功能 |
|---|---|
| `G S` | 跳转至审核工作台（Go Submissions）|
| `G B` | 跳转至博客治理（Go Blogs）|
| `G E` | 跳转至排除名单（Go Excluded）|
| `G T` | 跳转至诊断工具（Go Tools）|
| `R` | 刷新当前页面数据 |
| `?` | 展示快捷键帮助面板 |

### 9.2 审核工作台快捷键

| 快捷键 | 功能 |
|---|---|
| `↑` / `↓` | 切换焦点至上/下一条提交卡片 |
| `A` | 对焦点卡片执行"审核通过"（打开 ApproveDialog）|
| `R` | 对焦点卡片执行"驳回"（打开 RejectDialog）|
| `O` | 在新标签打开焦点博客主页 |
| `⌘ Enter` / `Ctrl Enter` | 在任意弹窗中确认提交 |
| `Esc` | 关闭当前弹窗 |

### 9.3 实现方式

```typescript
// 使用 useEffect + keydown 监听，挂载在 document 上
// 当焦点在 Input / Textarea 内时，关闭全局快捷键（避免冲突）

useEffect(() => {
  const handler = (e: KeyboardEvent) => {
    if (isInputFocused()) return; // 输入框内不触发全局快捷键
    switch (e.key) {
      case "a": openApproveDialog(focusedId); break;
      case "r": openRejectDialog(focusedId); break;
      // ...
    }
  };
  document.addEventListener("keydown", handler);
  return () => document.removeEventListener("keydown", handler);
}, [focusedId]);
```

---

## 10. 错误边界与 Loading 状态规范

### 10.1 Loading 状态

| 场景 | 展示方式 |
|---|---|
| 列表初次加载 | 骨架屏（Skeleton），与真实卡片等高，避免布局跳动 |
| 操作按钮提交中 | 按钮内旋转 Spinner + 禁用（`disabled`），防重复点击 |
| 诊断检测中（最长 30s）| 按钮禁用 + 进度文案 "检测中…（约 5~30 秒）" |

### 10.2 错误状态

| 场景 | 展示方式 |
|---|---|
| 列表加载失败 | 居中 `ErrorAlert`（shadcn Alert variant=destructive）+ "重试" 按钮 |
| 操作失败（网络/服务端 500）| 弹窗内行内红色错误文案 + 弹窗不关闭（允许用户重试）|
| Token 失效（401）| 清除 Token，弹出 `AdminLogin`，展示"Token 已失效，请重新登录" |
| 数据不存在（404）| Toast 提示 "操作目标不存在，列表将刷新" + 自动刷新列表 |

### 10.3 空状态

| 页面/场景 | 空状态文案 |
|---|---|
| 待审队列为空 | 🎉 "目前没有待审提交，继续保持！" |
| 博客列表为空 | "尚未收录任何博客" + "直接添加博客" 快捷入口 |
| 排除名单为空 | "排除名单为空，所有域名均可正常提交" |
| 搜索无结果 | "未找到匹配 '{keyword}' 的结果，请尝试其他关键词" |

---

## 11. 响应式与可访问性基线

### 11.1 响应式断点策略

管理后台**以桌面端为主**，最小支持宽度为 1024px（iPad 横屏）：

- `≥ 1280px`：完整布局，侧边抽屉宽 480px；
- `1024px ~ 1279px`：博客列表隐藏"描述"和"额外域名"列，Table 列精简；
- `< 1024px`：展示友好提示 "推荐在桌面浏览器使用管理后台"，功能仍可用但不做额外优化。

### 11.2 可访问性（a11y）基线

- 所有 shadcn/ui 组件原生支持键盘导航和 ARIA 属性，无需额外配置；
- Dialog / Sheet 弹窗打开时自动 `focus trap`（焦点锁定在弹窗内），关闭后焦点返回触发元素；
- 操作按钮设置有意义的 `aria-label`（如 "通过 amigoer.com 的提交"）；
- 颜色状态（Badge）辅以图标（✓ / ✗ / ⚠）确保色觉障碍用户可辨识。

### 11.3 深色模式

跟随 Explore 读者侧已有的 `dark` class 切换机制，管理端页面在 `<html>` 元素加载时读取 `localStorage["theme"]`，无需额外实现。

---

## 附录：关键状态机

### 提交审核流程状态机

```
           提交 POST /submissions
                    │
                    ▼
               [pending]
              ╱         ╲
     adminApprove     adminReject
            │                │
            ▼                ▼
       [approved]        [rejected]
```

### 博客生命周期状态机

```
  adminCreateBlog / adminApprove
              │
              ▼
           [active]  ──────────── PATCH status=paused ──────► [paused]
              │                                                    │
              │  连续失败 > 阈值                                   │ PATCH status=active
              │                                                    │
              ▼                                                    │
        [unhealthy]（前端计算，visible=false）◄──────────────────┘
              │
              │ adminDeleteBlog
              ▼
           [已移除]
         ╱         ╲
    exclude=        exclude=
    opt_out         blocked
       │               │
       ▼               ▼
  [excluded:       [excluded:
   opt_out]         blocked]
```

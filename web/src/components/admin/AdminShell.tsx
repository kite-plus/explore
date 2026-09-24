import { useState } from "react";
import { Activity, BookOpen, FileText, FolderKanban, LayoutDashboard, ListChecks, LogOut, Menu, Settings, ShieldCheck, SlidersHorizontal, Users, X } from "lucide-react";

import { AdminLogin } from "@/components/admin/AdminLogin";
import { BlogsTable } from "@/components/admin/blogs/BlogsTable";
import { ExcludedTable } from "@/components/admin/excluded/ExcludedTable";
import { OverviewPage, UsersPage, EntriesPage, TakedownsPage, SettingsPage } from "@/components/admin/OperationsPages";
import { FetchQueue } from "@/components/admin/queue/FetchQueue";
import { SubmissionsQueue } from "@/components/admin/submissions/SubmissionsQueue";
import { InspectTool } from "@/components/admin/tools/InspectTool";
import { AuthContext, useAuthState } from "@/components/admin/useAdminAuth";
import "@/styles/admin.css";

const GROUPS = [
  { title: "总览", links: [{ href: "/admin", label: "工作台", key: "overview", icon: LayoutDashboard }] },
  { title: "内容运营", links: [
    { href: "/admin/submissions", label: "收录审核", key: "submissions", icon: ListChecks },
    { href: "/admin/blogs", label: "博客管理", key: "blogs", icon: BookOpen },
    { href: "/admin/entries", label: "文章管理", key: "entries", icon: FileText },
    { href: "/admin/takedowns", label: "下架审批", key: "takedowns", icon: ShieldCheck },
  ] },
  { title: "用户与任务", links: [
    { href: "/admin/users", label: "用户管理", key: "users", icon: Users },
    { href: "/admin/queue", label: "抓取任务", key: "queue", icon: Activity },
  ] },
  { title: "系统", links: [
    { href: "/admin/settings", label: "系统设置", key: "settings", icon: Settings },
    { href: "/admin/excluded-hosts", label: "排除名单", key: "excluded-hosts", icon: FolderKanban },
    { href: "/admin/tools", label: "诊断工具", key: "tools", icon: SlidersHorizontal },
  ] },
] as const;

type Tab = "overview" | "submissions" | "blogs" | "entries" | "takedowns" | "users" | "queue" | "settings" | "excluded-hosts" | "tools";
const TITLES: Record<Tab, string> = { overview: "工作台", submissions: "收录审核", blogs: "博客管理", entries: "文章管理", takedowns: "下架审批", users: "用户管理", queue: "抓取任务", settings: "系统设置", "excluded-hosts": "排除名单", tools: "诊断工具" };
const PAGES: Record<Tab, () => React.JSX.Element> = {
  overview: OverviewPage, submissions: SubmissionsQueue, blogs: BlogsTable,
  entries: EntriesPage, takedowns: TakedownsPage, users: UsersPage,
  queue: FetchQueue, settings: SettingsPage, "excluded-hosts": ExcludedTable, tools: InspectTool,
};

export function AdminShell({ activeTab }: { activeTab: Tab }) {
  const { token, authError, login, logout, onUnauthorized, checking } = useAuthState();
  const [mobileOpen, setMobileOpen] = useState(false);
  const Page = PAGES[activeTab];
  const title = TITLES[activeTab];

  return <AuthContext.Provider value={{ token, isAuthenticated: token !== null, authError, login, logout, onUnauthorized }}>
    {checking ? <div className="admin-boot">正在连接管理后台…</div> : token === null ? <AdminLogin error={authError} /> :
      <div className="admin-layout">
        <aside className={`admin-sidebar ${mobileOpen ? "is-open" : ""}`}>
          <div className="admin-brand"><a href="/admin"><span className="admin-brand-mark">E<span>.</span></span><span>Explore<small>CONTROL ROOM</small></span></a><button className="admin-mobile-close" aria-label="关闭导航" onClick={() => setMobileOpen(false)}><X size={20} /></button></div>
          <nav className="admin-navigation" aria-label="管理后台导航">{GROUPS.map(group => <div className="admin-nav-group" key={group.title}><span className="admin-nav-label">{group.title}</span>{group.links.map(link => { const Icon = link.icon; return <a key={link.key} href={link.href} aria-current={activeTab === link.key ? "page" : undefined} className={activeTab === link.key ? "active" : ""}><Icon size={18} strokeWidth={1.8} />{link.label}</a>; })}</div>)}</nav>
          <div className="admin-sidebar-foot"><a href="/" target="_blank" rel="noopener noreferrer">查看前台 <span>↗</span></a><button onClick={logout}><LogOut size={17} />退出登录</button></div>
        </aside>
        {mobileOpen && <button className="admin-backdrop" aria-label="关闭导航" onClick={() => setMobileOpen(false)} />}
        <div className="admin-workspace"><header className="admin-topbar"><button className="admin-menu-button" aria-label="打开导航" onClick={() => setMobileOpen(true)}><Menu size={21} /></button><div className="admin-breadcrumb">Explore <span>/</span> 管理后台 <span>/</span> <strong>{title}</strong></div><span className="admin-topbar-status"><i />已连接</span></header><main className="admin-content"><div className="admin-page-intro"><div><span className="admin-page-overline">MANAGEMENT / {activeTab.toUpperCase()}</span><h1>{title}</h1></div><span className="admin-page-date">{new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "long", day: "numeric" }).format(new Date())}</span></div><Page /></main></div>
      </div>}
  </AuthContext.Provider>;
}

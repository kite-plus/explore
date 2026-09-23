import { AdminLogin } from "@/components/admin/AdminLogin";
import { BlogsTable } from "@/components/admin/blogs/BlogsTable";
import { ExcludedTable } from "@/components/admin/excluded/ExcludedTable";
import { FetchQueue } from "@/components/admin/queue/FetchQueue";
import { SubmissionsQueue } from "@/components/admin/submissions/SubmissionsQueue";
import { InspectTool } from "@/components/admin/tools/InspectTool";
import { AuthContext, useAuthState } from "@/components/admin/useAdminAuth";

const NAV_LINKS = [
  { href: "/admin/submissions", label: "审核工作台", key: "submissions" },
  { href: "/admin/blogs", label: "收录博客", key: "blogs" },
  { href: "/admin/queue", label: "抓取队列", key: "queue" },
  { href: "/admin/excluded-hosts", label: "排除名单", key: "excluded-hosts" },
  { href: "/admin/tools", label: "诊断工具", key: "tools" },
] as const;

interface Props {
  activeTab: "submissions" | "blogs" | "queue" | "excluded-hosts" | "tools";
}

const PAGES = {
  submissions: SubmissionsQueue,
  blogs: BlogsTable,
  queue: FetchQueue,
  "excluded-hosts": ExcludedTable,
  tools: InspectTool,
};

export function AdminShell({ activeTab }: Props) {
  const { token, authError, login, logout, onUnauthorized } = useAuthState();
  const isAuthenticated = token !== null;
  const Page = PAGES[activeTab];

  return (
    <AuthContext.Provider value={{ token, isAuthenticated, authError, login, logout, onUnauthorized }}>
      {/* 未登录时渲染登录弹窗 */}
      {!isAuthenticated && <AdminLogin error={authError} />}

      <div className="flex min-h-screen flex-col bg-background">
        {/* 顶部导航栏 */}
        <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur">
          <div className="mx-auto flex max-w-7xl flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3">
            {/* Logo / 标题 */}
            <a href="/admin" className="text-sm font-semibold text-foreground">
              Explore Admin
            </a>

            {/* 导航链接 */}
            <nav aria-label="后台导航" className="order-3 flex w-full gap-1 overflow-x-auto sm:order-none sm:w-auto">
              {NAV_LINKS.map((link) => (
                <a
                  key={link.key}
                  href={link.href}
                  aria-current={activeTab === link.key ? "page" : undefined}
                  className={[
                    "whitespace-nowrap rounded-md px-3 py-1.5 text-sm transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
                    activeTab === link.key
                      ? "bg-accent text-accent-foreground font-medium"
                      : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                  ].join(" ")}
                >
                  {link.label}
                </a>
              ))}
            </nav>

            {/* 右侧：登出按钮（已登录时才显示） */}
            {isAuthenticated && (
              <button
                onClick={logout}
                className="ml-auto rounded-md px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
              >
                登出
              </button>
            )}
          </div>
        </header>

        {/* 主体内容区：未登录时渲染空白（数据从弹窗中获取后才渲染） */}
        {isAuthenticated && (
          <main className="mx-auto w-full max-w-7xl flex-1 px-4 py-6">
            <Page />
          </main>
        )}
      </div>
    </AuthContext.Provider>
  );
}

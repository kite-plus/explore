import * as React from "react";
import { RefreshCwIcon, PlusIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { AdminBadge } from "@/components/admin/ui/AdminBadge";
import { TimeAgo } from "@/components/admin/ui/TimeAgo";
import { BlogDetailDrawer } from "@/components/admin/blogs/BlogDetailDrawer";
import { CreateBlogDialog } from "@/components/admin/blogs/CreateBlogDialog";
import { DeleteBlogDialog } from "@/components/admin/blogs/DeleteBlogDialog";
import { useAdminAuth } from "@/components/admin/useAdminAuth";
import { createBlog, deleteBlog, fetchBlogNow, fetchBlogs, fetchQueue, updateBlog } from "@/lib/admin-api";
import type { AdminBlog, CreateBlogPayload, UpdateBlogPayload } from "@/lib/admin-types";

type HealthFilter = "all" | "unhealthy" | "paused";

export function BlogsTable() {
  const { token, onUnauthorized } = useAdminAuth();
  const [blogs, setBlogs] = React.useState<AdminBlog[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [search, setSearch] = React.useState("");
  const [healthFilter, setHealthFilter] = React.useState<HealthFilter>("all");
  const [detailBlog, setDetailBlog] = React.useState<AdminBlog | null>(null);
  const [deleteBlogHost, setDeleteBlogHost] = React.useState<string | null>(null);
  const [createOpen, setCreateOpen] = React.useState(false);
  const [actionMessage, setActionMessage] = React.useState<string | null>(null);
  const [actionHost, setActionHost] = React.useState<string | null>(null);

  async function load() {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      // 异常告警时传 unhealthy，其余不传（全量获取后前端过滤）
      const data = await fetchBlogs(healthFilter === "unhealthy" ? "unhealthy" : undefined, token, onUnauthorized);
      setBlogs(data);
      setDetailBlog((current) => current ? data.find((blog) => blog.host === current.host) ?? current : null);
    } catch (e) {
      if (e instanceof Error && e.message !== "Unauthorized") setError(e.message);
    } finally {
      setLoading(false);
    }
  }

  React.useEffect(() => { load(); }, [token, healthFilter]);

  // 前端过滤：搜索词 + paused 状态
  const filtered = blogs.filter((b) => {
    const matchSearch = !search || b.host.includes(search) || b.name.toLowerCase().includes(search.toLowerCase());
    const matchPaused = healthFilter !== "paused" || b.status === "paused";
    return matchSearch && matchPaused;
  });

  async function handleUpdate(host: string, payload: UpdateBlogPayload) {
    if (!token) return;
    const updated = await updateBlog(host, payload, token, onUnauthorized);
    setBlogs((prev) => prev.map((b) => (b.host === host ? updated : b)));
    if (detailBlog?.host === host) setDetailBlog(updated);
  }

  async function handleFetchNow(host: string) {
    if (!token) return;
    await fetchBlogNow(host, token, onUnauthorized);
    let workerOnline: boolean | null = null;
    try {
      workerOnline = (await fetchQueue(token, onUnauthorized)).worker_online;
    } catch {
      workerOnline = null;
    }
    setActionMessage(workerOnline === false
      ? `${host} 已设为立即抓取，但 worker 离线，任务暂不会执行。`
      : `${host} 已设为立即抓取，请到抓取队列查看执行状态。`);
    setActionHost(host);
    await load();
    return workerOnline;
  }

  async function handleFetchFromRow(host: string) {
    try {
      await handleFetchNow(host);
    } catch (cause) {
      setActionMessage(cause instanceof Error ? cause.message : "抓取排队失败");
      setActionHost(null);
    }
  }

  async function handleCreate(payload: CreateBlogPayload) {
    if (!token) return;
    const created = await createBlog(payload, token, onUnauthorized);
    setBlogs((previous) => [created, ...previous]);
    setHealthFilter("all");
    setActionMessage(`${created.host} 已收录`);
    setActionHost(created.host);
  }

  async function handleDelete(exclude: "" | "opt_out" | "blocked", note: string) {
    if (!token || !deleteBlogHost) return;
    await deleteBlog(deleteBlogHost, exclude, note, token, onUnauthorized);
    setBlogs((prev) => prev.filter((b) => b.host !== deleteBlogHost));
    if (detailBlog?.host === deleteBlogHost) setDetailBlog(null);
  }

  return (
    <div className="admin-legacy-panel space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-lg font-semibold">收录博客</h2>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={load} disabled={loading}>
            <RefreshCwIcon className={["size-3.5", loading ? "animate-spin" : ""].join(" ")} />
            刷新
          </Button>
          <Button size="sm" onClick={() => setCreateOpen(true)}><PlusIcon className="size-4" />直接添加博客</Button>
        </div>
      </div>

      {actionMessage && <p role="status" className="text-sm text-muted-foreground">
        {actionMessage} {actionHost && <a href={`/admin/queue?host=${encodeURIComponent(actionHost)}`} className="text-primary underline underline-offset-2">查看抓取队列</a>}
      </p>}

      {/* 工具栏 */}
      <div className="flex flex-wrap items-center gap-3">
        <Input
          placeholder="搜索域名或名称…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-64"
        />
        <div className="flex items-center gap-1 rounded-md border border-input p-0.5">
          {[
            { value: "all", label: "全部" },
            { value: "unhealthy", label: "异常告警" },
            { value: "paused", label: "已暂停" },
          ].map((opt) => (
            <button
              key={opt.value}
              type="button"
              onClick={() => setHealthFilter(opt.value as HealthFilter)}
              className={[
                "rounded px-2.5 py-1 text-xs transition-colors",
                healthFilter === opt.value
                  ? "bg-background shadow-sm font-medium text-foreground"
                  : "text-muted-foreground hover:text-foreground",
              ].join(" ")}
            >
              {opt.label}
            </button>
          ))}
        </div>
      </div>

      {/* 内容区 */}
      {loading ? (
        <div className="space-y-2">
          {[1, 2, 3, 4, 5].map((i) => <Skeleton key={i} className="h-12 w-full rounded-md" />)}
        </div>
      ) : error ? (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4">
          <p className="text-destructive text-sm">{error}</p>
          <Button variant="outline" size="sm" className="mt-2" onClick={load}>重试</Button>
        </div>
      ) : filtered.length === 0 ? (
        <div className="py-12 text-center text-muted-foreground">
          {search ? `未找到匹配 "${search}" 的博客` : "暂无博客"}
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="pb-2 pr-4 font-medium">域名</th>
                <th className="pb-2 pr-4 font-medium">名称</th>
                <th className="pb-2 pr-4 font-medium">状态</th>
                <th className="pb-2 pr-4 font-medium">上次成功</th>
                <th className="pb-2 pr-4 font-medium text-center">失败次数</th>
                <th className="pb-2 font-medium">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {filtered.map((blog) => {
                const isUnhealthy = blog.status === "active" && (!blog.visible || blog.consecutive_failures > 0);
                return (
                  <tr key={blog.host} className="group hover:bg-muted/30">
                    <td className="py-2.5 pr-4">
                      <button
                        type="button"
                        onClick={() => setDetailBlog(blog)}
                        className="font-mono font-medium text-primary hover:underline"
                      >
                        {blog.host}
                      </button>
                    </td>
                    <td className="py-2.5 pr-4 max-w-48 truncate text-muted-foreground" title={blog.name}>
                      {blog.name}
                    </td>
                    <td className="py-2.5 pr-4">
                      <AdminBadge status={isUnhealthy ? "unhealthy" : blog.status} />
                    </td>
                    <td className="py-2.5 pr-4 text-muted-foreground">
                      <TimeAgo iso={blog.last_succeeded_at} />
                    </td>
                    <td className="py-2.5 pr-4 text-center">
                      <span className={blog.consecutive_failures > 0 ? "font-medium text-destructive" : "text-muted-foreground"}>
                        {blog.consecutive_failures}
                      </span>
                    </td>
                    <td className="py-2.5">
                      <div className="flex items-center gap-1">
                        <Button size="sm" variant="ghost" onClick={() => setDetailBlog(blog)}>详情</Button>
                        <Button size="sm" variant="ghost" onClick={() => handleFetchFromRow(blog.host)}>立即抓取</Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          className="text-destructive hover:text-destructive"
                          onClick={() => setDeleteBlogHost(blog.host)}
                        >
                          移除
                        </Button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
          <p className="mt-2 text-right text-xs text-muted-foreground">
            共 {filtered.length} 个博客{search || healthFilter !== "all" ? `（总 ${blogs.length} 个）` : ""}
          </p>
        </div>
      )}

      <BlogDetailDrawer
        blog={detailBlog}
        open={detailBlog !== null}
        onClose={() => setDetailBlog(null)}
        onUpdate={handleUpdate}
        onFetchNow={handleFetchNow}
      />

      <DeleteBlogDialog
        host={deleteBlogHost}
        open={deleteBlogHost !== null}
        onClose={() => setDeleteBlogHost(null)}
        onConfirm={handleDelete}
      />

      <CreateBlogDialog open={createOpen} onClose={() => setCreateOpen(false)} onConfirm={handleCreate} />
    </div>
  );
}

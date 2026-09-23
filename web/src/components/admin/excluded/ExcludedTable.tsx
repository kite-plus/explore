import * as React from "react";
import { RefreshCwIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { AdminBadge } from "@/components/admin/ui/AdminBadge";
import { TimeAgo } from "@/components/admin/ui/TimeAgo";
import { useAdminAuth } from "@/components/admin/useAdminAuth";
import { deleteExcludedHost, fetchExcludedHosts } from "@/lib/admin-api";
import type { ExcludedHost } from "@/lib/admin-types";

export function ExcludedTable() {
  const { token, onUnauthorized } = useAdminAuth();
  const [hosts, setHosts] = React.useState<ExcludedHost[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [search, setSearch] = React.useState("");
  const [reasonFilter, setReasonFilter] = React.useState<"all" | "opt_out" | "blocked">("all");
  const [restoreHost, setRestoreHost] = React.useState<string | null>(null);
  const [restoreLoading, setRestoreLoading] = React.useState(false);
  const [restoreError, setRestoreError] = React.useState<string | null>(null);

  async function load() {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      const data = await fetchExcludedHosts(token, onUnauthorized);
      setHosts(data);
    } catch (e) {
      if (e instanceof Error && e.message !== "Unauthorized") setError(e.message);
    } finally {
      setLoading(false);
    }
  }

  React.useEffect(() => { load(); }, [token]);

  const filtered = hosts.filter((h) => {
    const matchSearch = !search || h.host.includes(search);
    const matchReason = reasonFilter === "all" || h.reason === reasonFilter;
    return matchSearch && matchReason;
  });

  async function handleRestore() {
    if (!token || !restoreHost) return;
    setRestoreLoading(true);
    setRestoreError(null);
    try {
      await deleteExcludedHost(restoreHost, token, onUnauthorized);
      setHosts((prev) => prev.filter((h) => h.host !== restoreHost));
      setRestoreHost(null);
    } catch (e) {
      setRestoreError(e instanceof Error ? e.message : "操作失败，请重试");
    } finally {
      setRestoreLoading(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">排除名单</h1>
        <Button variant="outline" size="sm" onClick={load} disabled={loading}>
          <RefreshCwIcon className={["size-3.5", loading ? "animate-spin" : ""].join(" ")} />
          刷新
        </Button>
      </div>

      {/* 工具栏 */}
      <div className="flex flex-wrap items-center gap-3">
        <Input
          placeholder="搜索域名…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-64"
        />
        <div className="flex items-center gap-1 rounded-md border border-input p-0.5">
          {[
            { value: "all", label: "全部" },
            { value: "opt_out", label: "申请退出" },
            { value: "blocked", label: "永久封禁" },
          ].map((opt) => (
            <button
              key={opt.value}
              type="button"
              onClick={() => setReasonFilter(opt.value as "all" | "opt_out" | "blocked")}
              className={[
                "rounded px-2.5 py-1 text-xs transition-colors",
                reasonFilter === opt.value
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
          {[1, 2, 3].map((i) => <Skeleton key={i} className="h-12 w-full rounded-md" />)}
        </div>
      ) : error ? (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4">
          <p className="text-destructive text-sm">{error}</p>
          <Button variant="outline" size="sm" className="mt-2" onClick={load}>重试</Button>
        </div>
      ) : filtered.length === 0 ? (
        <div className="py-12 text-center text-muted-foreground">
          {hosts.length === 0
            ? "排除名单为空，所有域名均可正常提交"
            : `未找到匹配 "${search}" 的域名`}
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="pb-2 pr-4 font-medium">域名</th>
                <th className="pb-2 pr-4 font-medium">排除原因</th>
                <th className="pb-2 pr-4 font-medium">备注</th>
                <th className="pb-2 pr-4 font-medium">加入时间</th>
                <th className="pb-2 font-medium">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {filtered.map((h) => (
                <tr key={h.host} className="group hover:bg-muted/30">
                  <td className="py-2.5 pr-4 font-mono font-medium">{h.host}</td>
                  <td className="py-2.5 pr-4">
                    <AdminBadge status={h.reason} />
                  </td>
                  <td className="py-2.5 pr-4 max-w-64 truncate text-muted-foreground" title={h.note}>
                    {h.note || "—"}
                  </td>
                  <td className="py-2.5 pr-4 text-muted-foreground">
                    <TimeAgo iso={h.created_at} />
                  </td>
                  <td className="py-2.5">
                    <Button
                      size="sm"
                      variant="outline"
                      className="opacity-0 group-hover:opacity-100 transition-opacity"
                      onClick={() => setRestoreHost(h.host)}
                    >
                      解除排除
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <p className="mt-2 text-right text-xs text-muted-foreground">共 {filtered.length} 条记录</p>
        </div>
      )}

      {/* 解除排除确认弹窗 */}
      <AlertDialog open={restoreHost !== null} onOpenChange={(o) => !o && setRestoreHost(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认解除 {restoreHost} 的排除？</AlertDialogTitle>
            <AlertDialogDescription>
              解除后该域名可重新提交申请收录。
            </AlertDialogDescription>
          </AlertDialogHeader>
          {restoreError && <p className="text-destructive text-sm">{restoreError}</p>}
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setRestoreHost(null)} disabled={restoreLoading}>
              取消
            </AlertDialogCancel>
            <AlertDialogAction onClick={handleRestore} disabled={restoreLoading}>
              {restoreLoading ? "处理中…" : "确认解除"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

import * as React from "react";
import { RefreshCwIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ApproveDialog } from "@/components/admin/submissions/ApproveDialog";
import { RejectDialog } from "@/components/admin/submissions/RejectDialog";
import { SubmissionCard } from "@/components/admin/submissions/SubmissionCard";
import { useAdminAuth } from "@/components/admin/useAdminAuth";
import { approveSubmission, fetchSubmissions, rejectSubmission } from "@/lib/admin-api";
import type { AdminSubmission, ApprovePayload } from "@/lib/admin-types";

type Status = "pending" | "approved" | "rejected";

export function SubmissionsQueue() {
  const { token, onUnauthorized } = useAdminAuth();
  const [status, setStatus] = React.useState<Status>("pending");
  const [submissions, setSubmissions] = React.useState<AdminSubmission[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  // 弹窗状态
  const [approveTarget, setApproveTarget] = React.useState<AdminSubmission | null>(null);
  const [rejectTarget, setRejectTarget] = React.useState<AdminSubmission | null>(null);

  // 键盘导航焦点
  const [focusedIdx, setFocusedIdx] = React.useState(0);

  async function load(s: Status = status) {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      const data = await fetchSubmissions(s, token, onUnauthorized);
      setSubmissions(data);
      setFocusedIdx(0);
    } catch (e) {
      if (e instanceof Error && e.message !== "Unauthorized") {
        setError(e.message);
      }
    } finally {
      setLoading(false);
    }
  }

  React.useEffect(() => {
    load(status);
  }, [token, status]);

  // 键盘快捷键：↑↓ 导航，A 通过，R 驳回
  React.useEffect(() => {
    function handler(e: KeyboardEvent) {
      const tag = (e.target as HTMLElement)?.tagName;
      if (["INPUT", "TEXTAREA", "SELECT"].includes(tag)) return;
      // 弹窗打开时不触发导航快捷键
      if (approveTarget || rejectTarget) return;

      switch (e.key) {
        case "ArrowDown":
          e.preventDefault();
          setFocusedIdx((i) => Math.min(i + 1, Math.max(0, submissions.length - 1)));
          break;
        case "ArrowUp":
          e.preventDefault();
          setFocusedIdx((i) => Math.max(i - 1, 0));
          break;
        case "a":
        case "A": {
          const sub = submissions[focusedIdx];
          if (sub?.status === "pending") setApproveTarget(sub);
          break;
        }
        case "r":
        case "R": {
          const sub = submissions[focusedIdx];
          if (sub?.status === "pending") setRejectTarget(sub);
          break;
        }
      }
    }
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [submissions, focusedIdx, approveTarget, rejectTarget]);

  async function handleApprove(id: string, payload: ApprovePayload): Promise<void> {
    if (!token) return;
    // 乐观更新：立即从列表移除
    const removed = submissions.find((s) => s.id === id);
    setSubmissions((prev) => prev.filter((s) => s.id !== id));
    try {
      await approveSubmission(id, payload, token, onUnauthorized);
    } catch (e) {
      // 失败时回滚
      if (removed) setSubmissions((prev) => [removed, ...prev]);
      throw e;
    }
  }

  async function handleReject(id: string, reviewNote: string): Promise<void> {
    if (!token) return;
    const removed = submissions.find((s) => s.id === id);
    setSubmissions((prev) => prev.filter((s) => s.id !== id));
    try {
      await rejectSubmission(id, reviewNote, token, onUnauthorized);
    } catch (e) {
      if (removed) setSubmissions((prev) => [removed, ...prev]);
      throw e;
    }
  }

  const pendingCount = status === "pending" ? submissions.length : undefined;

  return (
    <div className="admin-legacy-panel space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">提交审核</h2>
        <Button variant="outline" size="sm" onClick={() => load()} disabled={loading}>
          <RefreshCwIcon className={["size-3.5", loading ? "animate-spin" : ""].join(" ")} />
          刷新
        </Button>
      </div>

      <Tabs value={status} onValueChange={(v) => setStatus(v as Status)}>
        <TabsList>
          <TabsTrigger value="pending">
            待审核{pendingCount !== undefined && submissions.length > 0 ? ` (${submissions.length})` : ""}
          </TabsTrigger>
          <TabsTrigger value="approved">已通过</TabsTrigger>
          <TabsTrigger value="rejected">已驳回</TabsTrigger>
        </TabsList>

        {(["pending", "approved", "rejected"] as const).map((tab) => (
          <TabsContent key={tab} value={tab} className="mt-4">
            {loading ? (
              <div className="space-y-3">
                {[1, 2, 3].map((i) => (
                  <Skeleton key={i} className="h-40 w-full rounded-lg" />
                ))}
              </div>
            ) : error ? (
              <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4">
                <p className="text-destructive text-sm">{error}</p>
                <Button variant="outline" size="sm" className="mt-2" onClick={() => load()}>
                  重试
                </Button>
              </div>
            ) : submissions.length === 0 ? (
              <div className="py-16 text-center text-muted-foreground">
                {tab === "pending" ? "🎉 目前没有待审提交，继续保持！" :
                  tab === "approved" ? "暂无已通过的记录" : "暂无已驳回的记录"}
              </div>
            ) : (
              <div className="space-y-3">
                {submissions.map((sub, idx) => (
                  <SubmissionCard
                    key={sub.id}
                    submission={sub}
                    focused={idx === focusedIdx}
                    onApprove={setApproveTarget}
                    onReject={setRejectTarget}
                  />
                ))}
              </div>
            )}
          </TabsContent>
        ))}
      </Tabs>

      {/* 快捷键提示（pending 时显示） */}
      {status === "pending" && submissions.length > 0 && !loading && (
        <p className="text-xs text-muted-foreground text-center">
          ↑↓ 切换焦点 · A 通过 · R 驳回
        </p>
      )}

      {/* 弹窗 */}
      <ApproveDialog
        submission={approveTarget}
        open={approveTarget !== null}
        onClose={() => setApproveTarget(null)}
        onConfirm={handleApprove}
      />
      <RejectDialog
        submission={rejectTarget}
        open={rejectTarget !== null}
        onClose={() => setRejectTarget(null)}
        onConfirm={handleReject}
      />
    </div>
  );
}

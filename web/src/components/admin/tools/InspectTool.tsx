import * as React from "react";
import { AlertCircleIcon, AlertTriangleIcon, InfoIcon, SearchIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAdminAuth } from "@/components/admin/useAdminAuth";
import { CreateBlogDialog } from "@/components/admin/blogs/CreateBlogDialog";
import { createBlog, runCheck } from "@/lib/admin-api";
import type { CreateBlogPayload } from "@/lib/admin-types";
import type { CheckReport, Problem } from "@/lib/types";
import { cn } from "@/lib/utils";

const LEVEL_ICON: Record<string, React.ReactNode> = {
  error: <AlertCircleIcon className="size-4 shrink-0 text-destructive" />,
  warning: <AlertTriangleIcon className="size-4 shrink-0 text-amber-500" />,
  info: <InfoIcon className="size-4 shrink-0 text-blue-500" />,
};

export function InspectTool() {
  const { token, onUnauthorized } = useAdminAuth();
  const [url, setUrl] = React.useState("");
  const [feedURL, setFeedURL] = React.useState("");
  const [showFeedInput, setShowFeedInput] = React.useState(false);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [report, setReport] = React.useState<CheckReport | null>(null);
  const [createOpen, setCreateOpen] = React.useState(false);
  const [createdHost, setCreatedHost] = React.useState<string | null>(null);

  async function handleCreate(payload: CreateBlogPayload) {
    if (!token) return;
    const blog = await createBlog(payload, token, onUnauthorized);
    setCreatedHost(blog.host);
  }

  async function handleCheck(e: React.FormEvent) {
    e.preventDefault();
    const u = url.trim();
    if (!u || !token) return;
    setLoading(true);
    setError(null);
    setReport(null);
    setCreatedHost(null);
    try {
      const result = await runCheck(u, feedURL.trim() || undefined, token, onUnauthorized);
      setReport(result);
    } catch (e) {
      if (e instanceof Error && e.message !== "Unauthorized") {
        setError(e.message || "检测失败，请检查 URL 后重试");
      }
    } finally {
      setLoading(false);
    }
  }

  const errors = report?.problems?.filter((p) => p.severity === "error") ?? [];
  const warnings = report?.problems?.filter((p) => p.severity === "warning") ?? [];
  const infos = report?.problems?.filter((p) => p.severity === "info") ?? [];

  return (
    <div className="admin-legacy-panel space-y-6">
      <h2 className="text-lg font-semibold">在线诊断工具</h2>

      {/* 输入表单 */}
      <form onSubmit={handleCheck} className="space-y-3 rounded-lg border bg-card p-4">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="inspect-url">博客网址</Label>
          <Input
            id="inspect-url"
            type="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://example.com"
            required
          />
        </div>

        {!showFeedInput ? (
          <button
            type="button"
            onClick={() => setShowFeedInput(true)}
            className="text-xs text-muted-foreground hover:text-foreground underline-offset-2 hover:underline"
          >
            + 指定 Feed URL（可选）
          </button>
        ) : (
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="inspect-feed">Feed URL（可选）</Label>
            <Input
              id="inspect-feed"
              type="url"
              value={feedURL}
              onChange={(e) => setFeedURL(e.target.value)}
              placeholder="https://example.com/feed.xml"
            />
          </div>
        )}

        <Button type="submit" disabled={loading || !url.trim()} className="w-full sm:w-auto">
          <SearchIcon className={["size-4", loading ? "animate-pulse" : ""].join(" ")} />
          {loading ? "检测中…（约 5~30 秒）" : "开始检测"}
        </Button>
      </form>

      {/* 错误提示 */}
      {error && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4">
          <p className="text-destructive text-sm">{error}</p>
        </div>
      )}

      {/* 检测结果 */}
      {report && (
        <div className="space-y-4 rounded-lg border bg-card p-4">
          {/* 综合判定横幅 */}
          <div className={cn(
            "flex items-center gap-2 rounded-md p-3 font-medium",
            report.passed
              ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400"
              : "bg-red-500/10 text-destructive",
          )}>
            {report.passed ? "✅ 检测通过，可以收录" : "❌ 检测未通过，存在问题需修复"}
          </div>

          {/* 基本信息 */}
          <div className="space-y-2 text-sm">
            <h3 className="font-medium text-muted-foreground text-xs uppercase tracking-wide">博客信息</h3>
            <div className="grid gap-1.5">
              {report.title && <InfoRow label="标题" value={report.title} />}
              {report.description && <InfoRow label="简介" value={report.description} />}
              {report.feed_url && <InfoRow label="Feed URL" value={report.feed_url} />}
              {report.language && <InfoRow label="语言" value={report.language} />}
              {report.generator && <InfoRow label="生成器" value={report.generator} />}
              {report.discovered_by && (
                <InfoRow label="发现方式" value={report.discovered_by} />
              )}
              {report.items && (
                <InfoRow label="文章数" value={`${report.items.valid} 篇有效 / ${report.items.total} 篇总计`} />
              )}
              {report.latest_entry_title && (
                <InfoRow label="最新文章" value={report.latest_entry_title} />
              )}
            </div>
          </div>

          {/* 问题清单 */}
          {report.problems?.length > 0 && (
            <div className="space-y-2">
              <h3 className="font-medium text-muted-foreground text-xs uppercase tracking-wide">诊断结果</h3>
              <div className="space-y-1">
                {[...errors, ...warnings, ...infos].map((p, i) => (
                  <ProblemRow key={i} problem={p} />
                ))}
              </div>
            </div>
          )}
          {report.passed && !createdHost && (
            <Button onClick={() => setCreateOpen(true)}>直接添加此博客</Button>
          )}
          {createdHost && <p role="status" className="text-sm text-emerald-700">{createdHost} 已收录</p>}
        </div>
      )}
      <CreateBlogDialog open={createOpen} initialURL={url.trim()} onClose={() => setCreateOpen(false)} onConfirm={handleCreate} />
    </div>
  );
}

function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 text-sm">
      <span className="shrink-0 text-muted-foreground">{label}</span>
      <span className="text-right break-all">{value}</span>
    </div>
  );
}

function ProblemRow({ problem }: { problem: Problem }) {
  const icon = LEVEL_ICON[problem.severity] ?? LEVEL_ICON.info;
  return (
    <div className="flex items-start gap-2 rounded-md bg-muted/30 p-2 text-sm">
      {icon}
      <span className="flex-1">
        {problem.detail || problem.hint || problem.code}
        {problem.count && problem.count > 1 && (
          <span className="ml-1 text-muted-foreground text-xs">×{problem.count}</span>
        )}
      </span>
    </div>
  );
}

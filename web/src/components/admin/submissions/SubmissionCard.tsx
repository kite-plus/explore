import { ExternalLinkIcon } from "lucide-react";

import { Button, ButtonLink } from "@/components/ui/button";
import { AdminBadge } from "@/components/admin/ui/AdminBadge";
import { CopyButton } from "@/components/admin/ui/CopyButton";
import { TimeAgo } from "@/components/admin/ui/TimeAgo";
import { CheckReportPanel } from "@/components/admin/submissions/CheckReportPanel";
import type { AdminSubmission } from "@/lib/admin-types";

interface Props {
  submission: AdminSubmission;
  /** 是否为焦点卡片（键盘导航高亮） */
  focused?: boolean;
  onApprove: (sub: AdminSubmission) => void;
  onReject: (sub: AdminSubmission) => void;
}

export function SubmissionCard({ submission: sub, focused, onApprove, onReject }: Props) {
  const report = sub.check_report;

  return (
    <article
      className={[
        "rounded-lg border bg-card p-4 transition-shadow",
        focused ? "ring-2 ring-ring shadow-sm" : "",
      ].join(" ")}
      tabIndex={0}
      aria-label={`提交：${sub.host}`}
    >
      {/* ── 顶部：主信息行 ── */}
      <div className="flex items-start justify-between gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <AdminBadge status={sub.status} />
          <span className="font-mono text-sm font-semibold">{sub.host}</span>
          {report?.title && (
            <span className="text-sm text-muted-foreground">· {report.title}</span>
          )}
        </div>
        <div className="shrink-0 text-right text-xs text-muted-foreground">
          <TimeAgo iso={sub.created_at} />
          {sub.reviewed_by && (
            <div className="mt-0.5">
              {sub.reviewed_by === "system" ? "已收录，系统自动归档" : `审核人：${sub.reviewed_by}`}
            </div>
          )}
        </div>
      </div>

      {/* ── 博客元数据 ── */}
      <div className="mt-3 space-y-1 text-sm">
        {/* Feed URL */}
        <div className="flex items-center gap-1.5 text-muted-foreground">
          <span className="shrink-0">Feed:</span>
          <a
            href={sub.feed_url}
            target="_blank"
            rel="noopener noreferrer"
            className="truncate text-primary hover:underline"
            title={sub.feed_url}
          >
            {sub.feed_url}
          </a>
          <CopyButton value={sub.feed_url} />
          {report?.discovered_by && (
            <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-xs">
              {report.discovered_by}
            </span>
          )}
        </div>

        {/* 描述 */}
        {report?.description && (
          <p className="text-muted-foreground line-clamp-2">{report.description}</p>
        )}

        {/* 文章统计 */}
        {report && (
          <div className="flex flex-wrap gap-3 text-xs text-muted-foreground">
            <span>{report.passed ? "✅" : "❌"} 检测{report.passed ? "通过" : "未通过"}</span>
            {report.items && (
              <span>文章: {report.items.valid} 篇有效 / {report.items.total} 篇总计</span>
            )}
            {report.items?.latest_published_at && (
              <span>最新: <TimeAgo iso={report.items.latest_published_at} /></span>
            )}
            {report.generator && (
              <span>生成器: {report.generator}</span>
            )}
            {report.language && (
              <span>语言: {report.language}</span>
            )}
          </div>
        )}

        {/* 提交者备注 */}
        {sub.note && (
          <div className="flex items-start gap-1.5">
            <span className="shrink-0 text-muted-foreground">备注:</span>
            <p className="italic text-muted-foreground">{sub.note}</p>
          </div>
        )}

        {/* 驳回原因（已驳回时显示） */}
        {sub.status === "rejected" && sub.review_note && (
          <div className="flex items-start gap-1.5 rounded-md bg-red-50 p-2 dark:bg-red-950/30">
            <span className="shrink-0 text-destructive text-xs font-medium">驳回原因:</span>
            <p className="text-destructive text-xs">{sub.review_note}</p>
          </div>
        )}
      </div>

      {/* ── 检测报告面板 ── */}
      {report && (
        <div className="mt-3">
          <CheckReportPanel report={report} />
        </div>
      )}

      {/* ── 操作栏（仅 pending 状态） ── */}
      {sub.status === "pending" && (
        <div className="mt-3 flex flex-wrap items-center gap-2 border-t pt-3">
          <ButtonLink
            variant="ghost"
            size="sm"
            href={sub.site_url}
            target="_blank"
            rel="noopener noreferrer"
          >
            <ExternalLinkIcon className="size-3.5" />
            预览主页
          </ButtonLink>

          <div className="ml-auto flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              className="text-destructive hover:text-destructive"
              onClick={() => onReject(sub)}
            >
              驳回…
            </Button>
            <Button
              size="sm"
              onClick={() => onApprove(sub)}
            >
              审核通过 ✓
            </Button>
          </div>
        </div>
      )}
    </article>
  );
}

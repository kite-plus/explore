import * as React from "react";
import { AlertCircleIcon, AlertTriangleIcon, ChevronDownIcon, ChevronUpIcon, InfoIcon } from "lucide-react";

import type { CheckReport, Problem } from "@/lib/types";
import { cn } from "@/lib/utils";

interface Props {
  report: CheckReport | null;
  /** 是否默认展开 */
  defaultOpen?: boolean;
}

const LEVEL_ICON: Record<string, React.ReactNode> = {
  error: <AlertCircleIcon className="size-3.5 shrink-0 text-destructive" />,
  warning: <AlertTriangleIcon className="size-3.5 shrink-0 text-amber-500" />,
  info: <InfoIcon className="size-3.5 shrink-0 text-blue-500" />,
};

const LEVEL_CLASS: Record<string, string> = {
  error: "text-destructive",
  warning: "text-amber-600",
  info: "text-blue-600",
};

export function CheckReportPanel({ report, defaultOpen = false }: Props) {
  const [open, setOpen] = React.useState(defaultOpen);

  if (!report) return null;

  const errors = report.problems?.filter((p) => p.severity === "error") ?? [];
  const warnings = report.problems?.filter((p) => p.severity === "warning") ?? [];
  const hasProblems = errors.length > 0 || warnings.length > 0;

  // 摘要行文案
  let summaryText: string;
  if (errors.length > 0) {
    summaryText = `${errors.length} 个错误`;
    if (warnings.length > 0) summaryText += `，${warnings.length} 个警告`;
  } else if (warnings.length > 0) {
    summaryText = `${warnings.length} 个警告`;
  } else {
    summaryText = "无问题";
  }

  return (
    <div className="rounded-md border bg-muted/30">
      {/* 摘要行（可折叠） */}
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center justify-between px-3 py-2 text-sm"
        aria-expanded={open}
      >
        <span className="flex items-center gap-2">
          {errors.length > 0 ? (
            <AlertCircleIcon className="size-3.5 text-destructive" />
          ) : warnings.length > 0 ? (
            <AlertTriangleIcon className="size-3.5 text-amber-500" />
          ) : (
            <InfoIcon className="size-3.5 text-emerald-500" />
          )}
          <span className={cn("font-medium", hasProblems ? (errors.length > 0 ? "text-destructive" : "text-amber-600") : "text-emerald-600")}>
            {summaryText}
          </span>
          {report.passed ? (
            <span className="text-xs text-emerald-600">· 通过</span>
          ) : (
            <span className="text-xs text-destructive">· 未通过</span>
          )}
        </span>
        {(report.problems?.length ?? 0) > 0 && (
          open ? <ChevronUpIcon className="size-4 text-muted-foreground" /> : <ChevronDownIcon className="size-4 text-muted-foreground" />
        )}
      </button>

      {/* 展开：问题列表 */}
      {open && (report.problems?.length ?? 0) > 0 && (
        <ul className="border-t px-3 pb-2 pt-1 space-y-1">
          {report.problems.map((p, i) => (
            <ProblemItem key={i} problem={p} />
          ))}
        </ul>
      )}
    </div>
  );
}

function ProblemItem({ problem }: { problem: Problem }) {
  const icon = LEVEL_ICON[problem.severity] ?? LEVEL_ICON.info;
  const cls = LEVEL_CLASS[problem.severity] ?? "";

  return (
    <li className="flex items-start gap-2 py-0.5 text-xs">
      <span className="mt-0.5">{icon}</span>
      <span className={cn("flex-1", cls)}>
        {problem.detail || problem.hint || problem.code}
        {problem.count && problem.count > 1 ? <span className="text-muted-foreground ml-1">×{problem.count}</span> : null}
      </span>
    </li>
  );
}

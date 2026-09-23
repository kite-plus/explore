import { cn } from "@/lib/utils";

/** 状态 → 样式映射 */
const STATUS_STYLES: Record<string, string> = {
  // 提交审核状态
  pending: "bg-amber-500/10 text-amber-600 border-amber-200 dark:border-amber-800",
  approved: "bg-emerald-500/10 text-emerald-600 border-emerald-200 dark:border-emerald-800",
  rejected: "bg-red-500/10 text-destructive border-red-200 dark:border-red-900",
  // 博客运行状态
  active: "bg-emerald-500/10 text-emerald-600 border-emerald-200 dark:border-emerald-800",
  paused: "bg-muted text-muted-foreground border-border",
  unhealthy: "bg-red-500/10 text-destructive border-red-200 dark:border-red-900",
  // 排除名单原因
  opt_out: "bg-muted text-muted-foreground border-border",
  blocked: "bg-red-500/10 text-destructive border-red-200 dark:border-red-900",
};

/** 状态 → 中文标签映射 */
const STATUS_LABELS: Record<string, string> = {
  pending: "待审核",
  approved: "已通过",
  rejected: "已驳回",
  active: "运行中",
  paused: "已暂停",
  unhealthy: "异常",
  opt_out: "申请退出",
  blocked: "永久封禁",
};

interface Props {
  status: string;
  className?: string;
}

export function AdminBadge({ status, className }: Props) {
  const style = STATUS_STYLES[status] ?? "bg-muted text-muted-foreground border-border";
  const label = STATUS_LABELS[status] ?? status;

  return (
    <span
      className={cn(
        "inline-flex w-fit shrink-0 items-center justify-center rounded-md border px-2 py-0.5 text-xs font-medium whitespace-nowrap",
        style,
        className,
      )}
    >
      {label}
    </span>
  );
}

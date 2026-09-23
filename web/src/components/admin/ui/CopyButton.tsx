import * as React from "react";
import { CheckIcon, CopyIcon } from "lucide-react";

import { cn } from "@/lib/utils";

/** 一键复制按钮，成功后图标变为 ✓（1.5 秒后恢复） */
export function CopyButton({ value, className }: { value: string; className?: string }) {
  const [copied, setCopied] = React.useState(false);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // 剪贴板 API 不可用时静默失败
    }
  }

  return (
    <button
      type="button"
      onClick={handleCopy}
      title={copied ? "已复制" : "复制"}
      aria-label={copied ? "已复制" : "复制到剪贴板"}
      className={cn(
        "inline-flex size-5 items-center justify-center rounded text-muted-foreground transition-colors hover:text-foreground",
        className,
      )}
    >
      {copied ? <CheckIcon className="size-3.5 text-emerald-500" /> : <CopyIcon className="size-3.5" />}
    </button>
  );
}

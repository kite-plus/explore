import * as React from "react";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import type { AdminSubmission } from "@/lib/admin-types";

/** 常用驳回理由快捷 Chip */
const QUICK_REASONS = [
  "非个人原创独立博客",
  "缺少 RSS/Atom 订阅源",
  "长期未更新（停更超过 1 年）",
  "内容涉及商业营销推广",
  "内容质量不符合收录标准",
  "非中英文博客（暂不收录）",
];

interface Props {
  submission: AdminSubmission | null;
  open: boolean;
  onClose: () => void;
  onConfirm: (id: string, reviewNote: string) => Promise<void>;
}

export function RejectDialog({ submission, open, onClose, onConfirm }: Props) {
  const [reviewNote, setReviewNote] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const textareaRef = React.useRef<HTMLTextAreaElement>(null);

  React.useEffect(() => {
    if (open) {
      setReviewNote("");
      setError(null);
      setTimeout(() => textareaRef.current?.focus(), 100);
    }
  }, [open]);

  async function handleConfirm() {
    if (!submission) return;
    const note = reviewNote.trim();
    if (!note) {
      setError("请填写驳回原因");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      await onConfirm(submission.id, note);
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "操作失败，请重试");
    } finally {
      setLoading(false);
    }
  }

  const canSubmit = reviewNote.trim().length > 0;

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            驳回提交
            {submission?.host && (
              <span className="ml-2 font-mono text-sm text-muted-foreground">{submission.host}</span>
            )}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-3">
          {/* 快捷理由 Chips */}
          <div>
            <p className="mb-2 text-xs text-muted-foreground">快捷理由（点击填入）</p>
            <div className="flex flex-wrap gap-1.5">
              {QUICK_REASONS.map((reason) => (
                <button
                  key={reason}
                  type="button"
                  onClick={() => setReviewNote(reason)}
                  className="rounded-full border border-border bg-muted/50 px-2.5 py-0.5 text-xs text-muted-foreground transition-colors hover:border-border hover:bg-muted hover:text-foreground"
                >
                  {reason}
                </button>
              ))}
            </div>
          </div>

          {/* 驳回原因文本框 */}
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="reject-note">
              驳回说明
              <span className="ml-1 text-destructive">*</span>
            </Label>
            <Textarea
              id="reject-note"
              ref={textareaRef}
              value={reviewNote}
              onChange={(e) => setReviewNote(e.target.value)}
              placeholder="请说明驳回原因，将对提交者可见"
              rows={3}
            />
            <p className="text-right text-xs text-muted-foreground">{reviewNote.length} 字</p>
          </div>
        </div>

        {error && <p className="text-destructive text-sm">{error}</p>}

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={loading}>
            取消
          </Button>
          <Button
            className="bg-destructive text-white hover:bg-destructive/90"
            onClick={handleConfirm}
            disabled={!canSubmit || loading}
          >
            {loading ? "提交中…" : "确认驳回"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

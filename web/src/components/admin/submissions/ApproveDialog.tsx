import * as React from "react";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { TagInput } from "@/components/admin/ui/TagInput";
import type { AdminSubmission, ApprovePayload } from "@/lib/admin-types";

interface Props {
  submission: AdminSubmission | null;
  open: boolean;
  onClose: () => void;
  onConfirm: (id: string, payload: ApprovePayload) => Promise<void>;
}

export function ApproveDialog({ submission, open, onClose, onConfirm }: Props) {
  const report = submission?.check_report;
  const [name, setName] = React.useState("");
  const [language, setLanguage] = React.useState("");
  const [feedURL, setFeedURL] = React.useState("");
  const [extraDomains, setExtraDomains] = React.useState<string[]>([]);
  const [showExcerpt, setShowExcerpt] = React.useState(true);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  // 每次弹窗打开时重置表单（使用检测到的值作为 placeholder）
  React.useEffect(() => {
    if (open) {
      setName("");
      setLanguage("");
      setFeedURL("");
      setExtraDomains([]);
      setShowExcerpt(true);
      setError(null);
    }
  }, [open]);

  async function handleConfirm() {
    if (!submission) return;
    setLoading(true);
    setError(null);
    try {
      const payload: ApprovePayload = {};
      if (name.trim()) payload.name = name.trim();
      if (language.trim()) payload.language = language.trim();
      if (feedURL.trim()) payload.feed_url = feedURL.trim();
      if (extraDomains.length > 0) payload.extra_domains = extraDomains;
      payload.show_excerpt = showExcerpt;
      await onConfirm(submission.id, payload);
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "操作失败，请重试");
    } finally {
      setLoading(false);
    }
  }

  // ⌘Enter / Ctrl+Enter 确认
  React.useEffect(() => {
    if (!open) return;
    function handler(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
        e.preventDefault();
        handleConfirm();
      }
    }
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [open, submission, name, language, feedURL, extraDomains, showExcerpt]);

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>确认通过收录</DialogTitle>
        </DialogHeader>

        {/* 预览摘要（只读） */}
        {submission && (
          <div className="rounded-md bg-muted/40 p-3 text-sm space-y-1">
            <div>
              <span className="text-muted-foreground">域名：</span>
              <span className="font-mono font-medium">{submission.host}</span>
            </div>
            {report?.title && (
              <div>
                <span className="text-muted-foreground">检测标题：</span>
                <span>{report.title}</span>
              </div>
            )}
            {submission.feed_url && (
              <div>
                <span className="text-muted-foreground">Feed：</span>
                <a href={submission.feed_url} target="_blank" rel="noopener" className="text-primary break-all hover:underline">
                  {submission.feed_url}
                </a>
              </div>
            )}
          </div>
        )}

        {/* 可选编辑字段（留空则使用检测到的值） */}
        <div className="space-y-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="approve-name">博客名称（可选，留空使用检测值）</Label>
            <Input
              id="approve-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={report?.title ?? submission?.host ?? ""}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="approve-language">语言（可选，如 zh-CN、en）</Label>
            <Input
              id="approve-language"
              value={language}
              onChange={(e) => setLanguage(e.target.value)}
              placeholder={report?.language ?? "zh-CN"}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="approve-feed">Feed URL（可选，修正地址）</Label>
            <Input
              id="approve-feed"
              value={feedURL}
              onChange={(e) => setFeedURL(e.target.value)}
              placeholder={submission?.feed_url ?? ""}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label>额外允许域名（CDN / 图床，可选）</Label>
            <TagInput value={extraDomains} onChange={setExtraDomains} placeholder="输入域名按 Enter 添加" />
          </div>

          <div className="flex items-center justify-between">
            <Label htmlFor="approve-excerpt">展示文章摘要</Label>
            <Switch id="approve-excerpt" checked={showExcerpt} onCheckedChange={setShowExcerpt} />
          </div>
        </div>

        {error && <p className="text-destructive text-sm">{error}</p>}

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={loading}>
            取消
          </Button>
          <Button onClick={handleConfirm} disabled={loading}>
            {loading ? "提交中…" : "确认通过 ✓"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

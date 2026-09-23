import * as React from "react";

import { TagInput } from "@/components/admin/ui/TagInput";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { CreateBlogPayload } from "@/lib/admin-types";

interface Props {
  open: boolean;
  initialURL?: string;
  onClose: () => void;
  onConfirm: (payload: CreateBlogPayload) => Promise<void>;
}

export function CreateBlogDialog({ open, initialURL = "", onClose, onConfirm }: Props) {
  const [siteURL, setSiteURL] = React.useState(initialURL);
  const [feedURL, setFeedURL] = React.useState("");
  const [name, setName] = React.useState("");
  const [language, setLanguage] = React.useState("");
  const [extraDomains, setExtraDomains] = React.useState<string[]>([]);
  const [showExcerpt, setShowExcerpt] = React.useState(true);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!open) return;
    setSiteURL(initialURL);
    setFeedURL("");
    setName("");
    setLanguage("");
    setExtraDomains([]);
    setShowExcerpt(true);
    setError(null);
  }, [open, initialURL]);

  async function handleSubmit(event: React.SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!siteURL.trim() || loading) return;
    setLoading(true);
    setError(null);
    try {
      await onConfirm({
        site_url: siteURL.trim(),
        ...(feedURL.trim() && { feed_url: feedURL.trim() }),
        ...(name.trim() && { name: name.trim() }),
        ...(language.trim() && { language: language.trim() }),
        extra_domains: extraDomains,
        show_excerpt: showExcerpt,
      });
      onClose();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "添加失败，请重试");
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && !loading && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>直接添加博客</DialogTitle>
          <p className="text-sm text-muted-foreground">保存前会检查博客和订阅源，检查通过后立即收录。</p>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="create-site-url">博客网址</Label>
            <Input id="create-site-url" type="url" required placeholder="https://example.com" value={siteURL} onChange={(event) => setSiteURL(event.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="create-feed-url">Feed URL（可选）</Label>
            <Input id="create-feed-url" type="url" placeholder="自动发现" value={feedURL} onChange={(event) => setFeedURL(event.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="create-name">博客名称（可选）</Label>
            <Input id="create-name" placeholder="使用检测到的名称" value={name} onChange={(event) => setName(event.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="create-language">语言（可选）</Label>
            <Input id="create-language" placeholder="使用检测到的语言" value={language} onChange={(event) => setLanguage(event.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label>额外允许域名</Label>
            <TagInput value={extraDomains} onChange={setExtraDomains} placeholder="输入域名后按 Enter 添加" />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="create-excerpt">展示文章摘要</Label>
            <Switch id="create-excerpt" checked={showExcerpt} onCheckedChange={setShowExcerpt} />
          </div>
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
          <DialogFooter>
            <Button type="button" variant="outline" disabled={loading} onClick={onClose}>取消</Button>
            <Button type="submit" disabled={loading || !siteURL.trim()}>{loading ? "检查并添加中…" : "检查并添加"}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

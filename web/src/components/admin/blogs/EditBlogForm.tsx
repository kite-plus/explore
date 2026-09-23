import * as React from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { TagInput } from "@/components/admin/ui/TagInput";
import type { AdminBlog, UpdateBlogPayload } from "@/lib/admin-types";
import type { Tag } from "@/lib/types";

interface Props {
  blog: AdminBlog;
  onSave: (payload: UpdateBlogPayload) => Promise<void>;
}

export function EditBlogForm({ blog, onSave }: Props) {
  const [name, setName] = React.useState(blog.name);
  const [language, setLanguage] = React.useState(blog.language);
  const [feedURL, setFeedURL] = React.useState(blog.feed_url);
  const [extraDomains, setExtraDomains] = React.useState(blog.extra_domains ?? []);
  const [defaultTags, setDefaultTags] = React.useState(blog.default_tags ?? []);
  const [showExcerpt, setShowExcerpt] = React.useState(blog.show_excerpt);
  const [blogStatus, setBlogStatus] = React.useState<"active" | "paused">(blog.status);
  const [statusNote, setStatusNote] = React.useState(blog.status_note ?? "");
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [saved, setSaved] = React.useState(false);
  const [availableTags, setAvailableTags] = React.useState<Tag[]>([]);
  const [tagError, setTagError] = React.useState<string | null>(null);

  React.useEffect(() => {
    let active = true;
    fetch("/api/v1/tags")
      .then((response) => {
        if (!response.ok) throw new Error("标签列表加载失败");
        return response.json() as Promise<{ data: Tag[] }>;
      })
      .then((result) => { if (active) setAvailableTags(result.data); })
      .catch(() => { if (active) setTagError("标签列表加载失败，暂时无法修改默认标签"); });
    return () => { active = false; };
  }, []);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setSaved(false);
    try {
      const payload: UpdateBlogPayload = {
        name: name.trim() || undefined,
        language: language.trim() || undefined,
        feed_url: feedURL.trim() || undefined,
        extra_domains: extraDomains,
        default_tags: tagError ? undefined : defaultTags,
        show_excerpt: showExcerpt,
        status: blogStatus,
        status_note: statusNote.trim() || undefined,
      };
      await onSave(payload);
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch (e) {
      setError(e instanceof Error ? e.message : "保存失败，请重试");
    } finally {
      setLoading(false);
    }
  }

  return (
    <form onSubmit={handleSave} className="space-y-4 p-4">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="edit-name">博客名称</Label>
        <Input id="edit-name" value={name} onChange={(e) => setName(e.target.value)} />
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="edit-language">语言</Label>
        <Input id="edit-language" value={language} onChange={(e) => setLanguage(e.target.value)} placeholder="zh-CN" />
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="edit-feed">Feed URL</Label>
        <Input id="edit-feed" value={feedURL} onChange={(e) => setFeedURL(e.target.value)} />
      </div>

      <div className="flex flex-col gap-1.5">
        <Label>额外允许域名</Label>
        <TagInput value={extraDomains} onChange={setExtraDomains} placeholder="CDN / 图床域名" />
      </div>

      <div className="flex flex-col gap-1.5">
        <Label>默认标签（最多 3 个）</Label>
        <div className="flex flex-wrap gap-1.5">
          {availableTags.map((tag) => {
            const selected = defaultTags.includes(tag.slug);
            return (
              <button
                key={tag.slug}
                type="button"
                onClick={() => {
                  if (selected) {
                    setDefaultTags(defaultTags.filter((slug) => slug !== tag.slug));
                  } else if (defaultTags.length < 3) {
                    setDefaultTags([...defaultTags, tag.slug]);
                  }
                }}
                aria-pressed={selected}
                className={[
                  "rounded-full border px-2.5 py-0.5 text-xs transition-colors",
                  selected
                    ? "border-primary bg-primary text-primary-foreground"
                    : "border-border bg-muted text-muted-foreground hover:border-border hover:text-foreground",
                ].join(" ")}
              >
                {tag.name.zh}
              </button>
            );
          })}
        </div>
        {tagError && <p role="alert" className="text-xs text-destructive">{tagError}</p>}
      </div>

      <div className="flex items-center justify-between">
        <Label htmlFor="edit-excerpt">展示文章摘要</Label>
        <Switch id="edit-excerpt" checked={showExcerpt} onCheckedChange={setShowExcerpt} />
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="edit-status">运行状态</Label>
        <select
          id="edit-status"
          value={blogStatus}
          onChange={(e) => setBlogStatus(e.target.value as "active" | "paused")}
          className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
        >
          <option value="active">运行中 (active)</option>
          <option value="paused">已暂停 (paused)</option>
        </select>
      </div>

      {blogStatus === "paused" && (
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="edit-status-note">暂停原因</Label>
          <Textarea
            id="edit-status-note"
            value={statusNote}
            onChange={(e) => setStatusNote(e.target.value)}
            placeholder="说明暂停原因，仅内部可见"
            rows={2}
          />
        </div>
      )}

      {error && <p className="text-destructive text-sm">{error}</p>}

      <Button type="submit" className="w-full" disabled={loading}>
        {loading ? "保存中…" : saved ? "✓ 已保存" : "保存修改"}
      </Button>
    </form>
  );
}

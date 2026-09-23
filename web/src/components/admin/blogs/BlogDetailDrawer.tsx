import * as React from "react";
import { ExternalLinkIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { AdminBadge } from "@/components/admin/ui/AdminBadge";
import { CopyButton } from "@/components/admin/ui/CopyButton";
import { TimeAgo } from "@/components/admin/ui/TimeAgo";
import { EditBlogForm } from "@/components/admin/blogs/EditBlogForm";
import type { AdminBlog, UpdateBlogPayload } from "@/lib/admin-types";

interface Props {
  blog: AdminBlog | null;
  open: boolean;
  onClose: () => void;
  onUpdate: (host: string, payload: UpdateBlogPayload) => Promise<void>;
  onFetchNow: (host: string) => Promise<boolean | null | undefined>;
}

/** 将秒数转为人类可读的时间间隔 */
function formatInterval(seconds: number): string {
  if (seconds < 3600) return `${Math.round(seconds / 60)} 分钟`;
  if (seconds < 86400) return `${Math.round(seconds / 3600)} 小时`;
  return `${Math.round(seconds / 86400)} 天`;
}

export function BlogDetailDrawer({ blog, open, onClose, onUpdate, onFetchNow }: Props) {
  const [fetchLoading, setFetchLoading] = React.useState(false);
  const [fetchMsg, setFetchMsg] = React.useState<string | null>(null);

  async function handleFetchNow() {
    if (!blog) return;
    setFetchLoading(true);
    setFetchMsg(null);
    try {
      const workerOnline = await onFetchNow(blog.host);
      setFetchMsg(workerOnline === false ? "worker 离线，暂不会执行" : "已设为立即抓取");
    } catch {
      setFetchMsg("操作失败，请重试");
    } finally {
      setFetchLoading(false);
    }
  }

  // 判断健康状态
  const isUnhealthy = blog ? blog.status === "active" && (!blog.visible || blog.consecutive_failures > 0) : false;

  return (
    <Sheet open={open} onOpenChange={(o) => !o && onClose()}>
      <SheetContent side="right" className="w-full max-w-md overflow-y-auto p-0">
        {blog && (
          <>
            <SheetHeader className="p-4 pb-3">
              <div className="flex items-start justify-between gap-2">
                <div>
                  <SheetTitle className="font-mono text-base">{blog.host}</SheetTitle>
                  <p className="mt-0.5 text-sm text-muted-foreground">{blog.name}</p>
                </div>
                <AdminBadge status={isUnhealthy ? "unhealthy" : blog.status} />
              </div>
            </SheetHeader>

            <Separator />

            <Tabs defaultValue="info" className="flex flex-col">
              <TabsList className="mx-4 mt-3">
                <TabsTrigger value="info">基本信息</TabsTrigger>
                <TabsTrigger value="health">健康状态</TabsTrigger>
                <TabsTrigger value="edit">编辑</TabsTrigger>
              </TabsList>

              {/* ── 基本信息 ── */}
              <TabsContent value="info" className="p-4 space-y-3">
                <InfoRow label="名称" value={blog.name} />
                <InfoRow label="语言" value={blog.language || "—"} />
                <InfoRow label="生成器" value={blog.generator || "—"} />
                <InfoRow label="摘要" value={blog.show_excerpt ? "显示" : "隐藏"} />

                <div>
                  <p className="text-xs text-muted-foreground mb-1">Site URL</p>
                  <div className="flex items-center gap-1.5">
                    <a href={blog.site_url} target="_blank" rel="noopener" className="break-all text-sm text-primary hover:underline">
                      {blog.site_url}
                    </a>
                    <CopyButton value={blog.site_url} />
                    <a href={blog.site_url} target="_blank" rel="noopener" className="text-muted-foreground hover:text-foreground">
                      <ExternalLinkIcon className="size-3.5" />
                    </a>
                  </div>
                </div>

                <div>
                  <p className="text-xs text-muted-foreground mb-1">Feed URL</p>
                  <div className="flex items-center gap-1.5">
                    <a href={blog.feed_url} target="_blank" rel="noopener" className="break-all text-sm text-primary hover:underline">
                      {blog.feed_url}
                    </a>
                    <CopyButton value={blog.feed_url} />
                  </div>
                </div>

                {blog.extra_domains?.length > 0 && (
                  <div>
                    <p className="text-xs text-muted-foreground mb-1">额外允许域名</p>
                    <div className="flex flex-wrap gap-1">
                      {blog.extra_domains.map((d) => (
                        <span key={d} className="rounded bg-muted px-2 py-0.5 text-xs font-mono">{d}</span>
                      ))}
                    </div>
                  </div>
                )}

                {blog.default_tags?.length > 0 && (
                  <div>
                    <p className="text-xs text-muted-foreground mb-1">默认标签</p>
                    <div className="flex flex-wrap gap-1">
                      {blog.default_tags.map((t) => (
                        <span key={t} className="rounded-full border border-primary/30 bg-primary/5 px-2 py-0.5 text-xs text-primary">{t}</span>
                      ))}
                    </div>
                  </div>
                )}

                <InfoRow label="收录于" value={<TimeAgo iso={blog.created_at} />} />
              </TabsContent>

              {/* ── 健康状态 ── */}
              <TabsContent value="health" className="p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <h3 className="text-sm font-medium">抓取调度</h3>
                  <div className="flex items-center gap-2">
                    {fetchMsg && <span className="text-xs text-muted-foreground">{fetchMsg} · <a href={`/admin/queue?host=${encodeURIComponent(blog.host)}`} className="underline underline-offset-2">查看队列</a></span>}
                    <Button size="sm" variant="outline" onClick={handleFetchNow} disabled={fetchLoading}>
                      {fetchLoading ? "排队中…" : "立即抓取"}
                    </Button>
                  </div>
                </div>

                <div className="space-y-2 rounded-lg border bg-muted/30 p-3 text-sm">
                  <InfoRow label="抓取间隔" value={formatInterval(blog.fetch_interval_seconds)} />
                  <InfoRow label="下次抓取" value={<TimeAgo iso={blog.next_fetch_at} />} />
                  <InfoRow label="上次抓取" value={<TimeAgo iso={blog.last_fetched_at} />} />
                  <InfoRow label="上次成功" value={<TimeAgo iso={blog.last_succeeded_at} />} />
                </div>

                <div className="space-y-2 rounded-lg border bg-muted/30 p-3 text-sm">
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">连续失败</span>
                    <span className={blog.consecutive_failures > 0 ? "font-medium text-destructive" : "text-emerald-600"}>
                      {blog.consecutive_failures} 次
                    </span>
                  </div>
                  {blog.gone_since && (
                    <div className="rounded-md bg-destructive/10 p-2">
                      <p className="text-xs text-destructive font-medium">⚠ 博客已失联</p>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        失联时间：<TimeAgo iso={blog.gone_since} />
                      </p>
                    </div>
                  )}
                  {blog.last_error && (
                    <div>
                      <p className="text-xs text-muted-foreground mb-1">最近错误</p>
                      <pre className="rounded bg-muted p-2 text-xs text-destructive whitespace-pre-wrap break-all">
                        {blog.last_error}
                      </pre>
                    </div>
                  )}
                </div>
              </TabsContent>

              {/* ── 编辑 ── */}
              <TabsContent value="edit" className="p-0">
                <EditBlogForm
                  key={blog.host}
                  blog={blog}
                  onSave={(payload) => onUpdate(blog.host, payload)}
                />
              </TabsContent>
            </Tabs>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}

function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium">{value}</span>
    </div>
  );
}

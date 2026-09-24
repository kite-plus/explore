import * as React from "react";
import { RefreshCwIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { TimeAgo } from "@/components/admin/ui/TimeAgo";
import { useAdminAuth } from "@/components/admin/useAdminAuth";
import { fetchAttempts, fetchBlogNow, fetchQueue } from "@/lib/admin-api";
import type { FetchAttempt, FetchQueueItem, FetchQueueSnapshot } from "@/lib/admin-types";

const STATUS: Record<FetchQueueItem["queue_status"], { label: string; color: string }> = {
  queued: { label: "待执行", color: "text-amber-700 bg-amber-50 border-amber-200" },
  running: { label: "抓取中", color: "text-blue-700 bg-blue-50 border-blue-200" },
  retry: { label: "等待重试", color: "text-red-700 bg-red-50 border-red-200" },
  scheduled: { label: "定时等待", color: "text-muted-foreground bg-muted border-border" },
  paused: { label: "已暂停", color: "text-muted-foreground bg-muted border-border" },
  stalled: { label: "执行中断", color: "text-red-700 bg-red-50 border-red-200" },
};

const OUTCOME: Record<FetchAttempt["outcome"], string> = {
  running: "抓取中",
  changed: "成功，内容已更新",
  unchanged: "成功，内容未变化",
  failed: "失败",
};

export function FetchQueue() {
  const { token, onUnauthorized } = useAdminAuth();
  const [snapshot, setSnapshot] = React.useState<FetchQueueSnapshot | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [search, setSearch] = React.useState("");
  const [filter, setFilter] = React.useState("all");
  const [expandedHost, setExpandedHost] = React.useState<string | null>(null);
  const [attempts, setAttempts] = React.useState<FetchAttempt[]>([]);
  const [attemptsError, setAttemptsError] = React.useState<string | null>(null);
  const [attemptsLoading, setAttemptsLoading] = React.useState(false);
  const [message, setMessage] = React.useState<string | null>(null);

  const load = React.useCallback(async (silent = false) => {
    if (!token) return;
    if (!silent) setLoading(true);
    try {
      const data = await fetchQueue(token, onUnauthorized);
      setSnapshot(data);
      setError(null);
      return data;
    } catch (cause) {
      if (cause instanceof Error && cause.message !== "Unauthorized") setError(cause.message);
    } finally {
      if (!silent) setLoading(false);
    }
  }, [token, onUnauthorized]);

  React.useEffect(() => {
    setSearch(new URLSearchParams(window.location.search).get("host") ?? "");
  }, []);

  React.useEffect(() => {
    void load();
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") void load(true);
    }, 15_000);
    return () => window.clearInterval(timer);
  }, [load]);

  async function showAttempts(host: string) {
    if (expandedHost === host) {
      setExpandedHost(null);
      return;
    }
    setExpandedHost(host);
    if (!token) return;
    setAttemptsLoading(true);
    setAttemptsError(null);
    setAttempts([]);
    try {
      setAttempts(await fetchAttempts(host, token, onUnauthorized));
    } catch (cause) {
      setAttemptsError(cause instanceof Error ? cause.message : "读取抓取记录失败");
    } finally {
      setAttemptsLoading(false);
    }
  }

  async function runNow(host: string) {
    if (!token) return;
    setMessage(null);
    try {
      await fetchBlogNow(host, token, onUnauthorized);
      const updated = await load(true);
      setMessage(updated?.crawler_paused
        ? `${host} 已设为立即抓取，但系统已暂停领取任务。请先到系统设置恢复抓取。`
        : updated?.worker_online
        ? `${host} 已设为立即抓取，请等待 worker 领取。`
        : `${host} 已设为立即抓取，但 worker 当前离线，任务暂不会执行。`);
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "设置立即抓取失败");
    }
  }

  const items = snapshot?.data ?? [];
  const due = items.filter((item) => item.queue_status === "queued" || item.queue_status === "stalled").length;
  const running = items.filter((item) => item.queue_status === "running").length;
  const failures = items.filter((item) => item.consecutive_failures > 0).length;
  const retrying = items.filter((item) => item.queue_status === "retry").length;
  const visible = items.filter((item) => {
    const matchesSearch = !search || item.host.includes(search.toLowerCase()) || item.name.toLowerCase().includes(search.toLowerCase());
    const matchesFilter = filter === "all" ||
      (filter === "due" && ["queued", "running", "stalled"].includes(item.queue_status)) ||
      (filter === "problems" && (item.consecutive_failures > 0 || item.queue_status === "stalled" || item.last_attempt?.outcome === "failed"));
    return matchesSearch && matchesFilter;
  });

  return (
    <div className="admin-legacy-panel space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold">抓取队列</h2>
          <p className="text-sm text-muted-foreground">每 15 秒更新一次，展示真实调度与最近执行记录。</p>
        </div>
        <Button variant="outline" size="sm" onClick={() => void load()} disabled={loading}>
          <RefreshCwIcon className={loading ? "size-3.5 animate-spin" : "size-3.5"} />刷新
        </Button>
      </div>

      {error && <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">{error}</div>}
      {snapshot && (
        <>
          {snapshot.crawler_paused && <div role="status" className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">系统已暂停领取抓取任务。队列保留，<a className="font-medium underline" href="/admin/settings">前往系统设置恢复</a>。</div>}
          <div role="status" className={snapshot.worker_online
            ? "rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-800"
            : "rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive"}>
            {snapshot.worker_online
              ? `抓取 worker 在线（${snapshot.worker_count} 个），待执行 ${due} 项，抓取中 ${running} 项，连续失败 ${failures} 项。`
              : `抓取 worker 离线，待执行 ${due} 项、等待重试 ${retrying} 项暂不会运行。请检查 worker 进程和日志。`}
            {snapshot.worker_last_seen_at && <span className="ml-2">最近心跳：<TimeAgo iso={snapshot.worker_last_seen_at} /></span>}
          </div>

          {message && <p role="status" className="text-sm text-foreground">{message}</p>}

          <div className="flex flex-wrap items-center gap-3">
            <Input aria-label="搜索队列" placeholder="搜索域名或名称…" value={search} onChange={(e) => setSearch(e.target.value)} className="max-w-64" />
            <div className="flex gap-1 rounded-md border border-input p-0.5">
              {[
                { value: "all", label: "全部" },
                { value: "due", label: "待执行 / 抓取中" },
                { value: "problems", label: "失败 / 中断" },
              ].map((option) => (
                <button key={option.value} type="button" onClick={() => setFilter(option.value)}
                  className={filter === option.value
                    ? "rounded bg-background px-2.5 py-1 text-xs font-medium shadow-sm"
                    : "rounded px-2.5 py-1 text-xs text-muted-foreground hover:text-foreground"}>
                  {option.label}
                </button>
              ))}
            </div>
          </div>

          {visible.length === 0 ? (
            <p className="py-12 text-center text-sm text-muted-foreground">没有匹配的抓取任务。</p>
          ) : (
            <div className="space-y-2">
              {visible.map((item) => {
                const status = STATUS[item.queue_status];
                const attemptError = item.last_attempt?.error;
                return (
                  <section key={item.host} className="rounded-lg border bg-card p-4">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="font-mono text-sm font-semibold">{item.host}</span>
                          <span className={`rounded border px-2 py-0.5 text-xs ${status.color}`}>{status.label}</span>
                          {item.consecutive_failures > 0 && <span className="text-xs text-destructive">连续失败 {item.consecutive_failures} 次</span>}
                        </div>
                        <p className="mt-0.5 text-xs text-muted-foreground">{item.name}</p>
                      </div>
                      <div className="flex items-center gap-1">
                        <Button variant="ghost" size="sm" onClick={() => void showAttempts(item.host)} aria-expanded={expandedHost === item.host}>
                          {expandedHost === item.host ? "收起记录" : "查看记录"}
                        </Button>
                        <Button variant="outline" size="sm" onClick={() => void runNow(item.host)} disabled={item.queue_status === "paused"}>
                          立即抓取
                        </Button>
                      </div>
                    </div>
                    <div className="mt-3 grid gap-x-5 gap-y-1 text-xs text-muted-foreground sm:grid-cols-3">
                      <span>下次调度：<TimeAgo iso={item.next_fetch_at} /></span>
                      <span>上次执行：<TimeAgo iso={item.last_fetched_at} /></span>
                      <span>上次成功：<TimeAgo iso={item.last_succeeded_at} /></span>
                    </div>
                    {attemptError && <div className="mt-3 rounded-md bg-destructive/5 p-2 text-xs text-destructive">
                      <p className="mb-1 font-medium">最近执行错误</p>
                      <pre className="whitespace-pre-wrap break-all">{attemptError}</pre>
                    </div>}
                    {item.last_error && item.last_error !== attemptError && <div className="mt-2 rounded-md bg-amber-50 p-2 text-xs text-amber-900">
                      <p className="mb-1 font-medium">站点最近错误</p>
                      <pre className="whitespace-pre-wrap break-all">{item.last_error}</pre>
                    </div>}

                    {expandedHost === item.host && (
                      <div className="mt-4 border-t pt-3">
                        <h2 className="text-sm font-medium">最近抓取记录</h2>
                        {attemptsLoading ? <p className="mt-2 text-xs text-muted-foreground">加载中…</p> :
                          attemptsError ? <p role="alert" className="mt-2 text-xs text-destructive">{attemptsError}</p> :
                          attempts.length === 0 ? <p className="mt-2 text-xs text-muted-foreground">暂无执行记录，等待 worker 领取。</p> :
                          <ol className="mt-2 space-y-2">
                            {attempts.map((attempt) => (
                              <li key={attempt.id} className="rounded-md bg-muted/40 p-2 text-xs">
                                <div className="flex flex-wrap items-center gap-2">
                                  <span className="font-medium">{OUTCOME[attempt.outcome]}</span>
                                  <TimeAgo iso={attempt.started_at} />
                                  {attempt.http_status !== null && <span>HTTP {attempt.http_status}</span>}
                                  {attempt.entry_count !== null && <span>{attempt.entry_count} 篇文章</span>}
                                </div>
                                {attempt.error && <pre className="mt-1 whitespace-pre-wrap break-all text-destructive">{attempt.error}</pre>}
                              </li>
                            ))}
                          </ol>}
                      </div>
                    )}
                  </section>
                );
              })}
            </div>
          )}
        </>
      )}
      {loading && !snapshot && !error && <p className="py-12 text-center text-sm text-muted-foreground">正在读取抓取队列…</p>}
    </div>
  );
}

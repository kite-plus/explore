import { useState, type FormEvent } from "react";
import { Flag } from "lucide-react";
import { currentReader, readerRequest } from "@/lib/reader-api";

export function ReportButton({ host, lang }: { host: string; lang: "zh" | "en" }) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);
  const zh = lang === "zh";

  async function start() {
    const user = await currentReader();
    if (!user) {
      const path = window.location.pathname;
      window.location.assign(`${lang === "en" ? "/en" : ""}/login?next=${encodeURIComponent(path)}`);
      return;
    }
    setOpen(true);
  }

  async function submit(event: FormEvent) {
    event.preventDefault(); setBusy(true); setError("");
    try {
      const user = await currentReader();
      if (!user) throw new Error(zh ? "登录已失效，请重新登录" : "Please sign in again");
      await readerRequest<void>("reports", { method: "POST", body: JSON.stringify({ target_type: "blog", blog_host: host, reason: reason.trim() }) }, user.csrf_token);
      setSent(true); setOpen(false);
    } catch (cause) { setError(cause instanceof Error ? cause.message : zh ? "提交失败" : "Could not submit"); }
    finally { setBusy(false); }
  }

  return <>{sent ? <span className="text-xs text-muted-foreground">{zh ? "报告已提交，等待审核" : "Report sent for review"}</span> : <button type="button" className="inline-flex items-center gap-1 rounded-md border px-3 py-1.5 text-xs text-muted-foreground hover:bg-accent" onClick={() => void start()}><Flag size={13} />{zh ? "报告问题" : "Report issue"}</button>}{open && <div className="fixed inset-0 z-50 grid place-items-center bg-black/55 p-4" onMouseDown={event => { if (event.target === event.currentTarget) setOpen(false); }}><form onSubmit={event => void submit(event)} className="grid w-full max-w-md gap-4 rounded-xl bg-background p-6 shadow-xl" role="dialog" aria-modal="true" aria-label={zh ? "报告博客问题" : "Report a blog issue"}><div><h2 className="text-lg font-semibold">{zh ? "报告博客问题" : "Report a blog issue"}</h2><p className="mt-1 text-sm text-muted-foreground">{host}</p></div><label className="grid gap-2 text-sm">{zh ? "问题说明" : "What happened?"}<textarea required minLength={5} maxLength={1000} className="min-h-28 rounded-md border bg-background p-3" value={reason} onChange={event => setReason(event.target.value)} placeholder={zh ? "请说明需要审核的原因" : "Explain why this needs review"} /></label>{error && <p role="alert" className="text-sm text-destructive">{error}</p>}<div className="flex justify-end gap-2"><button type="button" className="rounded-md border px-4 py-2 text-sm" onClick={() => setOpen(false)}>{zh ? "取消" : "Cancel"}</button><button disabled={busy} className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground">{busy ? zh ? "提交中…" : "Sending…" : zh ? "提交审核" : "Send report"}</button></div></form></div>}</>;
}

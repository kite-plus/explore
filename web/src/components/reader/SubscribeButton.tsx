import { useEffect, useState } from "react";

import { currentReader, readerRequest, type ReaderBlog, type ReaderUser } from "@/lib/reader-api";

export function SubscribeButton({ host, lang }: { host: string; lang: "zh" | "en" }) {
  const [user, setUser] = useState<ReaderUser | null>(null);
  const [following, setFollowing] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const zh = lang === "zh";
  useEffect(() => {
    void currentReader().then(async person => {
      setUser(person);
      if (!person) return;
      try {
        const list = await readerRequest<{ data: ReaderBlog[] }>("me/subscriptions");
        setFollowing(list.data.some(blog => blog.host === host));
      } catch { /* 页面仍可展示订阅入口。 */ }
    });
  }, [host]);
  async function toggle() {
    if (!user) {
      const next = `${lang === "en" ? "/en" : ""}/blogs/${encodeURIComponent(host)}`;
      window.location.assign(`${lang === "en" ? "/en" : ""}/login?next=${encodeURIComponent(next)}`);
      return;
    }
    setBusy(true);
    setError("");
    try {
      await readerRequest(`me/subscriptions/${encodeURIComponent(host)}`, { method: following ? "DELETE" : "PUT" }, user.csrf_token);
      setFollowing(!following);
    } catch (reason) { setError(reason instanceof Error ? reason.message : zh ? "操作失败" : "Action failed"); }
    finally { setBusy(false); }
  }
  return <><button type="button" disabled={busy} onClick={toggle} className="rounded-md border px-3 py-2 text-sm font-medium hover:bg-accent disabled:opacity-60">{following ? zh ? "已订阅 · 取消" : "Following · Unfollow" : zh ? "订阅博客" : "Follow blog"}</button>{error && <span role="alert" className="text-sm text-destructive">{error}</span>}</>;
}

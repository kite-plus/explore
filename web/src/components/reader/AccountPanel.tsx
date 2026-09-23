import { useEffect, useState } from "react";

import { currentReader, readerRequest, type ReaderBlog, type ReaderUser } from "@/lib/reader-api";

interface Challenge { record: string; value: string; expires_in_seconds: number }

export function AccountPanel({ lang }: { lang: "zh" | "en" }) {
  const [user, setUser] = useState<ReaderUser | null>(null);
  const [subscriptions, setSubscriptions] = useState<ReaderBlog[]>([]);
  const [owned, setOwned] = useState<ReaderBlog[]>([]);
  const [host, setHost] = useState("");
  const [challenge, setChallenge] = useState<Challenge | null>(null);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const zh = lang === "zh";
  const prefix = lang === "en" ? "/en" : "";

  async function load() {
    const person = await currentReader();
    if (!person) { window.location.assign(`${prefix}/login?next=${encodeURIComponent(`${prefix}/account`)}`); return; }
    setUser(person);
    const [followed, blogs] = await Promise.all([
      readerRequest<{ data: ReaderBlog[] }>("me/subscriptions"),
      readerRequest<{ data: ReaderBlog[] }>("me/blogs"),
    ]);
    setSubscriptions(followed.data);
    setOwned(blogs.data);
  }
  useEffect(() => { void load().catch(reason => setError(String(reason))); }, []);

  async function remove(blogHost: string) {
    if (!user) return;
    try {
      await readerRequest(`me/subscriptions/${encodeURIComponent(blogHost)}`, { method: "DELETE" }, user.csrf_token);
      setSubscriptions(items => items.filter(item => item.host !== blogHost));
    } catch (reason) { setError(String(reason)); }
  }
  async function startClaim() {
    if (!user) return;
    setError(""); setMessage("");
    try {
      const result = await readerRequest<Challenge>(`me/blog-claims/${encodeURIComponent(host.trim().toLowerCase())}`, { method: "POST" }, user.csrf_token);
      setChallenge(result);
    } catch (reason) { setError(String(reason)); }
  }
  async function verifyClaim() {
    if (!user) return;
    setError("");
    try {
      await readerRequest(`me/blog-claims/${encodeURIComponent(host.trim().toLowerCase())}/verify`, { method: "POST" }, user.csrf_token);
      setMessage(zh ? "认领成功。" : "Blog claimed.");
      setChallenge(null);
      await load();
    } catch (reason) { setError(String(reason)); }
  }
  async function logout() {
    if (!user) return;
    await readerRequest("auth/logout", { method: "POST" }, user.csrf_token);
    window.location.assign(`${prefix}/`);
  }

  return <div className="space-y-10">
    <header className="flex flex-wrap items-start justify-between gap-4">
      <div><h1 className="text-2xl font-semibold">{zh ? "我的账号" : "My account"}</h1><p className="mt-1 text-sm text-muted-foreground">{user ? `${user.display_name} · ${user.email}` : zh ? "正在读取账号…" : "Loading account…"}</p></div>
      <button type="button" onClick={logout} className="rounded-md border px-3 py-2 text-sm hover:bg-accent">{zh ? "退出登录" : "Sign out"}</button>
    </header>
    {error && <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">{error}</p>}
    {message && <p role="status" className="text-sm text-emerald-700">{message}</p>}
    <section><div className="flex items-center justify-between"><h2 className="text-lg font-semibold">{zh ? "订阅的博客" : "Following"}</h2><a className="text-sm underline" href={`${prefix}/following`}>{zh ? "查看订阅流" : "Open feed"}</a></div>
      {subscriptions.length ? <ul className="mt-4 divide-y rounded-lg border">{subscriptions.map(blog => <li className="flex items-center justify-between gap-3 px-4 py-3" key={blog.host}><a className="min-w-0 truncate text-sm font-medium hover:underline" href={`${prefix}/blogs/${blog.host}`}>{blog.name} <span className="font-normal text-muted-foreground">· {blog.host}</span></a><button className="shrink-0 text-sm text-muted-foreground hover:text-foreground" onClick={() => remove(blog.host)}>{zh ? "取消订阅" : "Unfollow"}</button></li>)}</ul> : <p className="mt-3 text-sm text-muted-foreground">{zh ? "还没有订阅博客。可从博客详情页订阅。" : "No blogs followed yet. Open a blog to follow it."}</p>}
    </section>
    <section><h2 className="text-lg font-semibold">{zh ? "我认领的博客" : "My blogs"}</h2>
      {owned.length ? <ul className="mt-3 space-y-2">{owned.map(blog => <li key={blog.host}><a className="text-sm underline" href={`${prefix}/blogs/${blog.host}`}>{blog.name} · {blog.host}</a></li>)}</ul> : <p className="mt-3 text-sm text-muted-foreground">{zh ? "尚未认领博客。" : "No claimed blogs yet."}</p>}
      <div className="mt-5 rounded-lg border p-4"><h3 className="font-medium">{zh ? "认领已收录博客" : "Claim a listed blog"}</h3><p className="mt-1 text-sm text-muted-foreground">{zh ? "输入域名，添加 DNS TXT 验证记录，然后点击验证。记录有效期 30 分钟。" : "Enter its domain, add the DNS TXT record, then verify within 30 minutes."}</p>
        <div className="mt-4 flex flex-wrap gap-2"><input aria-label={zh ? "博客域名" : "Blog domain"} className="min-w-0 flex-1 rounded-md border bg-background px-3 py-2 text-sm" placeholder="blog.example.com" value={host} onChange={e => { setHost(e.target.value); setChallenge(null); }} /><button className="rounded-md bg-primary px-3 py-2 text-sm text-primary-foreground" onClick={startClaim} disabled={!host.trim() || !user}>{zh ? "生成验证记录" : "Create verification"}</button></div>
        {challenge && <div className="mt-4 space-y-2 rounded-md bg-muted p-3 text-sm"><p>{zh ? "TXT 主机名" : "TXT name"}: <code className="break-all">{challenge.record}</code></p><p>{zh ? "TXT 内容" : "TXT value"}: <code className="break-all">{challenge.value}</code></p><button className="mt-2 rounded-md border bg-background px-3 py-2 text-sm" onClick={verifyClaim}>{zh ? "验证并认领" : "Verify and claim"}</button></div>}
      </div>
    </section>
  </div>;
}

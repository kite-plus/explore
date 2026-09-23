import { useState, type FormEvent } from "react";

import { readerRequest, type ReaderUser } from "@/lib/reader-api";

export function AuthForm({ lang, next }: { lang: "zh" | "en"; next: string }) {
  const [register, setRegister] = useState(false);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const zh = lang === "zh";

  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      await readerRequest<ReaderUser>(`auth/${register ? "register" : "login"}`, {
        method: "POST",
        body: JSON.stringify({ email, password, ...(register ? { display_name: name } : {}) }),
      });
      window.location.assign(next.startsWith("/") && !next.startsWith("//") ? next : lang === "en" ? "/en/following" : "/following");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : zh ? "登录失败" : "Sign in failed");
    } finally { setBusy(false); }
  }

  return <div className="mx-auto max-w-md rounded-xl border bg-card p-6 shadow-sm">
    <h1 className="text-2xl font-semibold">{register ? zh ? "创建账号" : "Create account" : zh ? "登录 Explore" : "Sign in to Explore"}</h1>
    <p className="mt-2 text-sm text-muted-foreground">{zh ? "登录后可订阅博客、查看个人文章流并认领自己的博客。" : "Follow blogs, read your feed, and claim your own blog."}</p>
    <form className="mt-6 grid gap-4" onSubmit={submit}>
      {register && <label className="grid gap-1.5 text-sm">{zh ? "显示名称" : "Display name"}<input className="h-10 rounded-md border bg-background px-3" value={name} onChange={e => setName(e.target.value)} required maxLength={80} /></label>}
      <label className="grid gap-1.5 text-sm">{zh ? "邮箱" : "Email"}<input className="h-10 rounded-md border bg-background px-3" type="email" value={email} onChange={e => setEmail(e.target.value)} required autoComplete="email" /></label>
      <label className="grid gap-1.5 text-sm">{zh ? "密码" : "Password"}<input className="h-10 rounded-md border bg-background px-3" type="password" value={password} onChange={e => setPassword(e.target.value)} required minLength={register ? 12 : undefined} autoComplete={register ? "new-password" : "current-password"} /></label>
      {register && <p className="text-xs text-muted-foreground">{zh ? "密码至少 12 个字符。" : "Use at least 12 characters."}</p>}
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      <button className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-60" disabled={busy}>{busy ? zh ? "请稍候…" : "Please wait…" : register ? zh ? "创建并登录" : "Create and sign in" : zh ? "登录" : "Sign in"}</button>
    </form>
    <button className="mt-5 text-sm text-muted-foreground underline" type="button" onClick={() => { setRegister(!register); setError(""); }}>{register ? zh ? "已有账号？登录" : "Already have an account? Sign in" : zh ? "没有账号？创建账号" : "New here? Create account"}</button>
  </div>;
}

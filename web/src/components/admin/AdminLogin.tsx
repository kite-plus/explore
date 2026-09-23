import * as React from "react";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAdminAuth } from "@/components/admin/useAdminAuth";
import { ACCOUNT_ADMIN } from "@/lib/admin-api";
import { readerRequest, type ReaderUser } from "@/lib/reader-api";

interface Props {
  /** 来自父组件的错误提示（如 Token 失效） */
  error?: string | null;
}

export function AdminLogin({ error }: Props) {
  const { login } = useAdminAuth();
  const [token, setToken] = React.useState("");
  const [email, setEmail] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [useToken, setUseToken] = React.useState(false);
  const [remember, setRemember] = React.useState(false);
  const [localError, setLocalError] = React.useState<string | null>(null);
  const [loading, setLoading] = React.useState(false);
  const inputRef = React.useRef<HTMLInputElement>(null);

  // 弹窗打开时自动聚焦到输入框
  React.useEffect(() => {
    const timer = setTimeout(() => inputRef.current?.focus(), 100);
    return () => clearTimeout(timer);
  }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!useToken) {
      setLoading(true); setLocalError(null);
      try {
        const user = await readerRequest<ReaderUser>("auth/login", { method: "POST", body: JSON.stringify({ email, password }) });
        if (!user.is_admin) {
          await readerRequest("auth/logout", { method: "POST" }, user.csrf_token);
          setLocalError("此账号没有后台权限");
          return;
        }
        login(ACCOUNT_ADMIN, false);
      } catch (reason) { setLocalError(reason instanceof Error ? reason.message : "登录失败"); }
      finally { setLoading(false); }
      return;
    }
    const t = token.trim();
    if (!t) {
      setLocalError("请输入 Token");
      return;
    }
    setLoading(true);
    setLocalError(null);
    try {
      // 用一个轻量请求验证 Token 是否有效
      const res = await fetch("/api/v1/admin/submissions?status=pending", {
        headers: { Authorization: `Bearer ${t}` },
      });
      if (res.status === 401) {
        setLocalError("Token 无效，请检查后重试");
        return;
      }
      if (!res.ok) {
        setLocalError(res.status === 404 ? "管理接口未启用，请检查服务端配置" : `验证失败（HTTP ${res.status}），请稍后重试`);
        return;
      }
      login(t, remember);
    } catch {
      setLocalError("网络错误，请检查连接后重试");
    } finally {
      setLoading(false);
    }
  }

  const displayError = error ?? localError;

  return (
    <Dialog open>
      {/* showCloseButton=false：管理员必须登录才能继续，不允许直接关闭弹窗 */}
      <DialogContent showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>Explore 管理后台</DialogTitle>
          <DialogDescription>使用管理员账号登录</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="mt-2 flex flex-col gap-4">
          {!useToken && <>
            <div className="flex flex-col gap-1.5"><Label htmlFor="admin-email">邮箱</Label><Input id="admin-email" type="email" value={email} onChange={e => setEmail(e.target.value)} autoComplete="username" required /></div>
            <div className="flex flex-col gap-1.5"><Label htmlFor="admin-password">密码</Label><Input id="admin-password" type="password" value={password} onChange={e => setPassword(e.target.value)} autoComplete="current-password" required /></div>
          </>}
          {useToken && <>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="admin-token">Token</Label>
            <Input
              id="admin-token"
              ref={inputRef}
              type="password"
              placeholder="请输入维护者 Token"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              autoComplete="current-password"
            />
            {displayError && (
              <p className="text-destructive text-sm">{displayError}</p>
            )}
          </div>

          <label className="flex cursor-pointer items-center gap-2 text-sm text-muted-foreground">
            <input
              type="checkbox"
              checked={remember}
              onChange={(e) => setRemember(e.target.checked)}
              className="size-4 rounded border"
            />
            记住登录状态
          </label>
          </>}

          <Button type="submit" disabled={loading}>
            {loading ? "验证中…" : "登录"}
          </Button>
        </form>
        <button type="button" className="mt-3 text-sm text-muted-foreground underline" onClick={() => { setUseToken(!useToken); setLocalError(null); }}>{useToken ? "使用账号登录" : "使用维护者 Token"}</button>
      </DialogContent>
    </Dialog>
  );
}

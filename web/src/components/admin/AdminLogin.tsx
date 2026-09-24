import { useState, type FormEvent } from "react";
import { ArrowLeft, ArrowRight, LockKeyhole } from "lucide-react";
import { useAdminAuth } from "@/components/admin/useAdminAuth";
import { ACCOUNT_ADMIN } from "@/lib/admin-api";
import { readerRequest, type ReaderUser } from "@/lib/reader-api";

export function AdminLogin({ error }: { error?: string | null }) {
  const { login } = useAdminAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [token, setToken] = useState("");
  const [useToken, setUseToken] = useState(false);
  const [remember, setRemember] = useState(false);
  const [localError, setLocalError] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault(); setLoading(true); setLocalError("");
    try {
      if (useToken) {
        const value = token.trim();
        const response = await fetch("/api/v1/admin/session", { headers: { Authorization: `Bearer ${value}` } });
        if (!response.ok) throw new Error(response.status === 401 ? "维护者令牌无效" : `后台连接失败（HTTP ${response.status}）`);
        login(value, remember);
      } else {
        const user = await readerRequest<ReaderUser>("auth/login", { method: "POST", body: JSON.stringify({ email, password }) });
        if (!user.is_admin) {
          await readerRequest("auth/logout", { method: "POST" }, user.csrf_token);
          throw new Error("当前账号没有管理权限");
        }
        login(ACCOUNT_ADMIN, false);
      }
    } catch (cause) { setLocalError(cause instanceof Error ? cause.message : "登录失败，请稍后再试"); }
    finally { setLoading(false); }
  }

  return <div className="admin-login"><div className="admin-login-side"><a href="/" className="admin-login-home"><ArrowLeft size={16} /> 返回 Explore</a><div className="admin-login-side-copy"><span className="admin-login-kicker">EXPLORE / MANAGEMENT</span><h1>让好的独立博客<br />被更多人看见<span>。</span></h1><p>从投稿审核、内容治理到抓取运行，所有工作都在这里发生。</p></div><div className="admin-login-side-footer"><span>CONTENT OPERATIONS</span><span>EXPLORE</span></div></div><div className="admin-login-form-side"><div className="admin-login-card"><div className="admin-login-icon"><LockKeyhole size={23} /></div><span className="admin-login-kicker">ADMIN ACCESS</span><h2>登录管理后台</h2><p>使用已授权的管理员账号继续。</p><form onSubmit={submit}>{!useToken ? <><label>邮箱地址<input autoFocus type="email" autoComplete="username" required value={email} onChange={event => setEmail(event.target.value)} placeholder="name@example.com" /></label><label>密码<input type="password" autoComplete="current-password" required value={password} onChange={event => setPassword(event.target.value)} placeholder="输入账号密码" /></label></> : <><label>维护者令牌<input autoFocus type="password" autoComplete="current-password" required value={token} onChange={event => setToken(event.target.value)} placeholder="输入管理令牌" /></label><label className="admin-login-remember"><input type="checkbox" checked={remember} onChange={event => setRemember(event.target.checked)} />在这台设备上记住令牌</label></>}{(error || localError) && <div className="admin-login-error" role="alert">{localError || error}</div>}<button disabled={loading} className="admin-login-submit">{loading ? "正在验证…" : "进入工作台"}<ArrowRight size={18} /></button></form><button type="button" className="admin-login-switch" onClick={() => { setUseToken(!useToken); setLocalError(""); }}>{useToken ? "使用管理员账号登录" : "使用维护者令牌登录"}</button></div><div className="admin-login-note">仅限经过授权的运营人员访问</div></div></div>;
}

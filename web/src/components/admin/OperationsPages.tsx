import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react";
import { AlertCircle, ArrowRight, Check, ChevronLeft, ChevronRight, RefreshCw } from "lucide-react";
import { adminRequest } from "@/lib/admin-api";
import { useAdminAuth } from "@/components/admin/useAdminAuth";

interface OverviewData {
  stats: { blogs: number; entries: number; users: number; pending_submissions: number; pending_takedowns: number; failing_blogs: number; due_fetches: number };
  worker_online: boolean;
  worker_count: number;
  worker_last_seen_at: string | null;
  crawler_paused: boolean;
}

interface Paged<T> { data: T[]; total: number }
interface AdminUser { id: string; email: string; display_name: string; is_admin: boolean; disabled_at: string | null; created_at: string; subscription_count: number; owned_blog_count: number }
interface AdminEntry { id: number; blog_host: string; blog_name: string; title: string; url: string; published_at: string | null; synced_at: string; tags: string[]; hidden: boolean; hide_reason: string }
interface Takedown { id: string; target_type: "blog" | "entry"; blog_host: string; entry_identity: string | null; entry_title: string; requester: string; reason: string; status: "pending" | "approved" | "rejected"; review_note: string; reviewed_by: string; created_at: string; reviewed_at: string | null }
interface Setting { key: string; value: string; updated_by: string; updated_at: string }

function date(value: string | null) { return value ? new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value)) : "—"; }
function ErrorBox({ error }: { error: string }) { return <div className="ops-error" role="alert"><AlertCircle size={17} />{error}</div>; }
function Status({ children, tone = "neutral" }: { children: ReactNode; tone?: "good" | "warn" | "bad" | "neutral" }) { return <span className={`ops-status ops-status--${tone}`}>{children}</span>; }

function useAdminLoad<T>(path: string) {
  const { token, onUnauthorized } = useAdminAuth();
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true); setError("");
    try { setData(await adminRequest<T>(path, token, onUnauthorized)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "读取数据失败"); }
    finally { setLoading(false); }
  }, [path, token, onUnauthorized]);
  useEffect(() => { void load(); }, [load]);
  const mutate = useCallback(async (endpoint: string, body: unknown, method = "PATCH") => {
    if (!token) return;
    setError("");
    try {
      await adminRequest<void>(endpoint, token, onUnauthorized, { method, body: JSON.stringify(body) });
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "操作失败");
      throw cause;
    }
  }, [token, onUnauthorized, load]);
  return { data, loading, error, load, mutate };
}

function SectionHeading({ title, description, refresh }: { title: string; description: string; refresh?: () => void }) {
  return <div className="ops-section-head"><div><h2>{title}</h2><p>{description}</p></div>{refresh && <button className="ops-button ops-button--subtle" onClick={refresh}><RefreshCw size={16} />刷新</button>}</div>;
}

export function OverviewPage() {
  const { data, loading, error, load } = useAdminLoad<OverviewData>("/overview");
  const stats = data?.stats;
  const cards = [
    { label: "待审核收录", value: stats?.pending_submissions, href: "/admin/submissions", tone: "attention" },
    { label: "待处理下架", value: stats?.pending_takedowns, href: "/admin/takedowns", tone: "attention" },
    { label: "抓取异常", value: stats?.failing_blogs, href: "/admin/queue", tone: "critical" },
    { label: "待抓取任务", value: stats?.due_fetches, href: "/admin/queue", tone: "normal" },
  ];
  return <div className="ops-stack">
    <div className="ops-hero"><div className="ops-eyebrow">EXPLORE / OPERATIONS</div><h2>内容与系统，一眼掌握。</h2><p>从真实数据出发，先处理待办，再检查抓取与内容状态。</p></div>
    {error && <ErrorBox error={error} />}
    <div className="ops-kpis">{cards.map(card => <a className={`ops-kpi ops-kpi--${card.tone}`} href={card.href} key={card.label}><span>{card.label}</span><strong>{loading ? "…" : card.value ?? 0}</strong><ArrowRight size={17} /></a>)}</div>
    <div className="ops-panel"><SectionHeading title="系统状态" description="工作进程和内容规模由服务器实时提供。" refresh={() => void load()} />
      <div className="ops-health-grid"><div><span>抓取进程</span><strong>{data?.worker_online ? <Status tone="good">在线 · {data.worker_count} 个</Status> : <Status tone="bad">离线</Status>}</strong><small>{data?.crawler_paused ? "系统已暂停领取任务" : data?.worker_online ? "正在领取抓取任务" : "任务保留在队列，进程启动后继续执行"}</small></div><div><span>上次心跳</span><strong>{date(data?.worker_last_seen_at ?? null)}</strong></div><div><span>已收录博客</span><strong>{stats?.blogs ?? "…"}</strong></div><div><span>缓存文章</span><strong>{stats?.entries ?? "…"}</strong></div><div><span>用户账号</span><strong>{stats?.users ?? "…"}</strong></div></div>
    </div>
    <div className="ops-shortcuts"><a href="/admin/blogs">管理博客 <ArrowRight size={15} /></a><a href="/admin/entries">检查文章 <ArrowRight size={15} /></a><a href="/admin/settings">系统设置 <ArrowRight size={15} /></a></div>
  </div>;
}

function Pager({ offset, total, limit, change }: { offset: number; total: number; limit: number; change: (offset: number) => void }) {
  return <div className="ops-pager"><span>{total === 0 ? "暂无记录" : `${offset + 1}–${Math.min(offset + limit, total)} / ${total}`}</span><div><button aria-label="上一页" disabled={offset === 0} onClick={() => change(Math.max(0, offset - limit))}><ChevronLeft size={17} /></button><button aria-label="下一页" disabled={offset + limit >= total} onClick={() => change(offset + limit)}><ChevronRight size={17} /></button></div></div>;
}

export function UsersPage() {
  const [query, setQuery] = useState("");
  const [search, setSearch] = useState("");
  const [offset, setOffset] = useState(0);
  const [changing, setChanging] = useState("");
  const [roleTarget, setRoleTarget] = useState<AdminUser | null>(null);
  const limit = 30;
  const { data, loading, error, load, mutate } = useAdminLoad<Paged<AdminUser>>(`/users?limit=${limit}&offset=${offset}&q=${encodeURIComponent(search)}`);
  async function changeUser(user: AdminUser, body: { disabled?: boolean; is_admin?: boolean }) {
    setChanging(user.id);
    try { await mutate(`/users/${encodeURIComponent(user.id)}`, body); setRoleTarget(null); }
    catch { /* 错误由页面提示展示。 */ }
    finally { setChanging(""); }
  }
  return <div className="ops-panel"><SectionHeading title="用户管理" description="查看账号、订阅与博客认领，管理账号状态及后台权限。" refresh={() => void load()} />
    <form className="ops-toolbar" onSubmit={event => { event.preventDefault(); setOffset(0); setSearch(query.trim()); }}><input aria-label="搜索用户" placeholder="搜索邮箱或显示名称" value={query} onChange={event => setQuery(event.target.value)} /><button className="ops-button" type="submit">搜索</button></form>
    {error && <ErrorBox error={error} />}
    <div className="ops-table-scroll"><table className="ops-table"><thead><tr><th>用户</th><th>角色 / 状态</th><th>订阅</th><th>认领博客</th><th>加入时间</th><th>操作</th></tr></thead><tbody>{loading ? <tr><td colSpan={6}>正在读取用户…</td></tr> : data?.data.length ? data.data.map(user => <tr key={user.id}><td><strong>{user.display_name}</strong><small>{user.email}</small></td><td><Status tone={user.disabled_at ? "bad" : "good"}>{user.disabled_at ? "已停用" : "正常"}</Status> {user.is_admin && <Status>管理员</Status>}</td><td>{user.subscription_count}</td><td>{user.owned_blog_count}</td><td>{date(user.created_at)}</td><td><div className="ops-row-actions"><button disabled={changing === user.id} className="ops-text-button" onClick={() => void changeUser(user, { disabled: !user.disabled_at })}>{user.disabled_at ? "恢复账号" : "停用账号"}</button><button disabled={changing === user.id || Boolean(user.disabled_at)} className="ops-text-button" onClick={() => setRoleTarget(user)}>{user.is_admin ? "取消管理权限" : "授予管理权限"}</button></div></td></tr>) : <tr><td colSpan={6}>没有符合条件的用户。</td></tr>}</tbody></table></div>
    <Pager offset={offset} total={data?.total ?? 0} limit={limit} change={setOffset} />
    {roleTarget && <div className="ops-modal-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) setRoleTarget(null); }}><div className="ops-modal" role="dialog" aria-modal="true" aria-label="调整管理权限"><h3>{roleTarget.is_admin ? "取消管理权限" : "授予管理权限"}</h3><p>{roleTarget.email}</p><p>{roleTarget.is_admin ? "确认后，该账号的后台会话会被撤销。" : "确认后，该账号可以访问后台并处理内容与用户。"}</p><div className="ops-modal-actions"><button className="ops-button ops-button--subtle" onClick={() => setRoleTarget(null)}>取消</button><button className="ops-button" disabled={changing === roleTarget.id} onClick={() => void changeUser(roleTarget, { is_admin: !roleTarget.is_admin })}>确认修改</button></div></div></div>}
  </div>;
}

export function EntriesPage() {
  const [query, setQuery] = useState("");
  const [search, setSearch] = useState("");
  const [offset, setOffset] = useState(0);
  const [target, setTarget] = useState<AdminEntry | null>(null);
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const limit = 30;
  const { data, loading, error, load, mutate } = useAdminLoad<Paged<AdminEntry>>(`/entries?limit=${limit}&offset=${offset}&q=${encodeURIComponent(search)}`);
  async function hide(event: FormEvent) {
    event.preventDefault(); if (!target) return;
    setSaving(true);
    try { await mutate(`/entries/${target.id}`, { hidden: true, reason: reason.trim() }); setTarget(null); setReason(""); }
    catch { /* 错误由页面提示展示。 */ }
    finally { setSaving(false); }
  }
  async function restore(entry: AdminEntry) {
    setSaving(true);
    try { await mutate(`/entries/${entry.id}`, { hidden: false }); }
    catch { /* 错误由页面提示展示。 */ }
    finally { setSaving(false); }
  }
  return <div className="ops-panel"><SectionHeading title="文章管理" description="检索抓取缓存中的文章；隐藏记录会跨抓取保留。" refresh={() => void load()} />
    <form className="ops-toolbar" onSubmit={event => { event.preventDefault(); setOffset(0); setSearch(query.trim()); }}><input aria-label="搜索文章" placeholder="搜索文章标题或博客域名" value={query} onChange={event => setQuery(event.target.value)} /><button className="ops-button" type="submit">搜索</button></form>
    {error && <ErrorBox error={error} />}
    <div className="ops-table-scroll"><table className="ops-table"><thead><tr><th>文章</th><th>来源博客</th><th>发布时间</th><th>状态</th><th>操作</th></tr></thead><tbody>{loading ? <tr><td colSpan={5}>正在读取文章…</td></tr> : data?.data.length ? data.data.map(entry => <tr key={entry.id}><td className="ops-main-cell"><a href={entry.url} target="_blank" rel="noopener noreferrer">{entry.title}</a><small>{entry.tags?.join(" · ") || "未分类"}</small></td><td><a href={`/admin/blogs?host=${encodeURIComponent(entry.blog_host)}`}>{entry.blog_name}</a><small>{entry.blog_host}</small></td><td>{date(entry.published_at)}</td><td>{entry.hidden ? <><Status tone="bad">已隐藏</Status><small>{entry.hide_reason}</small></> : <Status tone="good">公开</Status>}</td><td><button disabled={saving} className="ops-text-button" onClick={() => entry.hidden ? void restore(entry) : (setTarget(entry), setReason(""))}>{entry.hidden ? "恢复展示" : "隐藏文章"}</button></td></tr>) : <tr><td colSpan={5}>没有符合条件的文章。</td></tr>}</tbody></table></div>
    <Pager offset={offset} total={data?.total ?? 0} limit={limit} change={setOffset} />
    {target && <div className="ops-modal-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) setTarget(null); }}><form className="ops-modal" onSubmit={event => void hide(event)} aria-label="隐藏文章"><h3>隐藏文章</h3><p>{target.title}</p><label>处理原因<textarea required maxLength={500} value={reason} onChange={event => setReason(event.target.value)} placeholder="记录下架依据，供后续排查" /></label><div className="ops-modal-actions"><button type="button" className="ops-button ops-button--subtle" onClick={() => setTarget(null)}>取消</button><button disabled={saving || !reason.trim()} className="ops-button ops-button--danger">确认隐藏</button></div></form></div>}
  </div>;
}

export function TakedownsPage() {
  const [status, setStatus] = useState<"pending" | "approved" | "rejected">("pending");
  const [newOpen, setNewOpen] = useState(false);
  const [targetType, setTargetType] = useState<"blog" | "entry">("blog");
  const [host, setHost] = useState("");
  const [entryID, setEntryID] = useState("");
  const [reason, setReason] = useState("");
  const [review, setReview] = useState<Takedown | null>(null);
  const [reviewNote, setReviewNote] = useState("");
  const [busy, setBusy] = useState(false);
  const { data, loading, error, load, mutate } = useAdminLoad<{ data: Takedown[] }>(`/takedowns?status=${status}`);
  async function create(event: FormEvent) {
    event.preventDefault(); setBusy(true);
    try { await mutate("/takedowns", { target_type: targetType, blog_host: host.trim(), entry_id: Number(entryID), reason: reason.trim() }, "POST"); setNewOpen(false); setHost(""); setEntryID(""); setReason(""); setStatus("pending"); }
    catch { /* 错误由页面提示展示。 */ }
    finally { setBusy(false); }
  }
  async function decide(decision: "approved" | "rejected") {
    if (!review) return; setBusy(true);
    try { await mutate(`/takedowns/${review.id}/review`, { decision, review_note: reviewNote.trim() }, "POST"); setReview(null); setReviewNote(""); }
    catch { /* 错误由页面提示展示。 */ }
    finally { setBusy(false); }
  }
  return <div className="ops-panel"><div className="ops-section-head"><div><h2>下架审批</h2><p>接收用户举报或内部申请，审批后自动暂停博客或隐藏文章。</p></div><div className="ops-heading-actions"><button className="ops-button ops-button--subtle" onClick={() => void load()}><RefreshCw size={16} />刷新</button><button className="ops-button" onClick={() => setNewOpen(true)}>新建申请</button></div></div>
    <div className="ops-tabs" role="tablist">{(["pending", "approved", "rejected"] as const).map(item => <button role="tab" aria-selected={status === item} className={status === item ? "active" : ""} key={item} onClick={() => setStatus(item)}>{({ pending: "待处理", approved: "已通过", rejected: "已驳回" })[item]}</button>)}</div>
    {error && <ErrorBox error={error} />}
    <div className="ops-table-scroll"><table className="ops-table"><thead><tr><th>对象</th><th>申请原因</th><th>发起人</th><th>时间</th><th>操作</th></tr></thead><tbody>{loading ? <tr><td colSpan={5}>正在读取申请…</td></tr> : data?.data.length ? data.data.map(item => <tr key={item.id}><td><strong>{item.target_type === "blog" ? item.blog_host : item.entry_title || item.entry_identity}</strong><small>{item.target_type === "blog" ? "博客" : `文章 · ${item.blog_host}`}</small></td><td className="ops-main-cell">{item.reason}{item.review_note && <small>处理备注：{item.review_note}</small>}</td><td>{item.requester || "管理员"}</td><td>{date(item.created_at)}</td><td>{status === "pending" ? <button className="ops-text-button" onClick={() => { setReview(item); setReviewNote(""); }}>处理申请</button> : <Status tone={status === "approved" ? "good" : "neutral"}>{status === "approved" ? "已下架" : "已驳回"}</Status>}</td></tr>) : <tr><td colSpan={5}>当前没有{status === "pending" ? "待处理" : "符合条件"}的下架申请。</td></tr>}</tbody></table></div>
    {newOpen && <div className="ops-modal-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) setNewOpen(false); }}><form className="ops-modal" onSubmit={event => void create(event)}><h3>新建下架申请</h3><p>申请进入待处理列表，审批通过后才会下架。</p><label>对象类型<select value={targetType} onChange={event => setTargetType(event.target.value as "blog" | "entry")}><option value="blog">博客</option><option value="entry">文章</option></select></label><label>博客域名<input required value={host} onChange={event => setHost(event.target.value)} placeholder="example.com" /></label>{targetType === "entry" && <label>文章 ID<input type="number" required min="1" value={entryID} onChange={event => setEntryID(event.target.value)} placeholder="从文章管理中查看" /></label>}<label>申请原因<textarea required minLength={5} maxLength={1000} value={reason} onChange={event => setReason(event.target.value)} /></label><div className="ops-modal-actions"><button type="button" className="ops-button ops-button--subtle" onClick={() => setNewOpen(false)}>取消</button><button disabled={busy} className="ops-button">创建申请</button></div></form></div>}
    {review && <div className="ops-modal-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) setReview(null); }}><div className="ops-modal" role="dialog" aria-modal="true" aria-label="处理下架申请"><h3>处理下架申请</h3><p><strong>{review.target_type === "blog" ? review.blog_host : review.entry_title}</strong><br />{review.reason}</p><label>处理备注<textarea value={reviewNote} maxLength={500} onChange={event => setReviewNote(event.target.value)} placeholder="驳回时必填" /></label><div className="ops-modal-actions"><button className="ops-button ops-button--subtle" onClick={() => setReview(null)}>取消</button><button className="ops-button ops-button--subtle" disabled={busy || !reviewNote.trim()} onClick={() => void decide("rejected")}>驳回</button><button className="ops-button ops-button--danger" disabled={busy} onClick={() => void decide("approved")}><Check size={15} />通过并下架</button></div></div></div>}
  </div>;
}

const SETTINGS: Record<string, { label: string; description: string }> = {
  registration_enabled: { label: "开放用户注册", description: "关闭后，新用户无法创建账号；已有用户仍可登录。" },
  submissions_enabled: { label: "开放博客投稿", description: "关闭后，前台不再接受新的博客收录申请。" },
  crawler_paused: { label: "暂停抓取任务", description: "暂停后，抓取进程不再领取新任务，队列内容保留。" },
  site_notice: { label: "站点公告", description: "展示在前台导航下方，最多 280 字。" },
};

export function SettingsPage() {
  const { data, loading, error, mutate } = useAdminLoad<{ data: Setting[] }>("/settings");
  const [notice, setNotice] = useState("");
  const [saving, setSaving] = useState("");
  useEffect(() => { if (data) setNotice(data.data.find(item => item.key === "site_notice")?.value ?? ""); }, [data]);
  async function change(key: string, value: string) {
    setSaving(key);
    try { await mutate(`/settings/${key}`, { value }); }
    catch { /* 错误由页面提示展示。 */ }
    finally { setSaving(""); }
  }
  return <div className="ops-panel"><SectionHeading title="系统设置" description="设置保存在数据库中，并直接影响注册、投稿与抓取流程。" />
    {error && <ErrorBox error={error} />}
    {loading ? <p className="ops-empty">正在读取设置…</p> : <div className="ops-settings">{data?.data.filter(item => item.key !== "site_notice").map(item => <div className="ops-setting" key={item.key}><div><strong>{SETTINGS[item.key]?.label ?? item.key}</strong><p>{SETTINGS[item.key]?.description}</p><small>最近更新：{date(item.updated_at)} · {item.updated_by}</small></div><button className={`ops-switch ${item.value === "true" ? "is-on" : ""}`} role="switch" aria-checked={item.value === "true"} aria-label={SETTINGS[item.key]?.label ?? item.key} disabled={saving === item.key} onClick={() => void change(item.key, item.value === "true" ? "false" : "true")}><span /></button></div>)}<form className="ops-notice" onSubmit={event => { event.preventDefault(); void change("site_notice", notice); }}><strong>站点公告</strong><p>{SETTINGS.site_notice.description}</p><textarea maxLength={280} value={notice} onChange={event => setNotice(event.target.value)} placeholder="留空则不显示公告" /><div><small>{notice.length} / 280</small><button className="ops-button" disabled={saving === "site_notice" || notice === data?.data.find(item => item.key === "site_notice")?.value}>保存公告</button></div></form></div>}
  </div>;
}

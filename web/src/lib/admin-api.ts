/**
 * 管理后台 API 请求封装
 * 所有请求经过 /api/v1/admin/* 同源代理，自动附加 Authorization 头
 */

import type {
  AdminBlog,
  AdminSubmission,
  ApprovePayload,
  CreateBlogPayload,
  ExcludedHost,
  FetchAttempt,
  FetchQueueSnapshot,
  UpdateBlogPayload,
} from "@/lib/admin-types";
import type { CheckReport } from "@/lib/types";
import { readerRequest, type ReaderUser } from "@/lib/reader-api";

export const ACCOUNT_ADMIN = "__account_session__";

/** Token 无效或过期时抛出此错误（401 响应） */
export class AdminUnauthorizedError extends Error {
  constructor() {
    super("Unauthorized");
    this.name = "AdminUnauthorizedError";
  }
}

/**
 * 基础 fetch 封装
 * - 自动附加 Authorization: Bearer <token>
 * - 遇到 401 时抛出 AdminUnauthorizedError
 */
async function adminFetch(
  path: string,
  options: RequestInit,
  token: string,
  onUnauthorized: () => void,
): Promise<Response> {
  const account = token === ACCOUNT_ADMIN;
  const csrf = account && !["GET", "HEAD"].includes(options.method ?? "GET")
    ? (await readerRequest<ReaderUser>("me")).csrf_token
    : undefined;
  const res = await fetch(`/api/v1/admin${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      "Accept-Language": "zh-CN",
      ...(account ? {} : { Authorization: `Bearer ${token}` }),
      ...(csrf ? { "X-CSRF-Token": csrf } : {}),
      ...(options.headers as Record<string, string> | undefined),
    },
  });

  if (res.status === 401) {
    onUnauthorized();
    throw new AdminUnauthorizedError();
  }
  return res;
}

export async function adminRequest<T>(
  path: string,
  token: string,
  onUnauthorized: () => void,
  options: RequestInit = {},
): Promise<T> {
  const response = await adminFetch(path, options, token, onUnauthorized);
  if (!response.ok) throw new Error(await extractError(response));
  const body = await response.text();
  return body ? JSON.parse(body) as T : undefined as T;
}

/** 从响应体中提取错误消息 */
async function extractError(res: Response): Promise<string> {
  try {
    const data = await res.json();
    if (data?.error?.code === "check_failed") {
      const problem = data.check_report?.problems?.find((item: { severity: string }) => item.severity === "error");
      if (problem?.hint) return `检查未通过：${problem.hint}`;
    }
    return data?.error?.message ?? data?.error?.code ?? `HTTP ${res.status}`;
  } catch {
    return `HTTP ${res.status}`;
  }
}

// ─────────────────────────────────────────
// 提交审核 API
// ─────────────────────────────────────────

export async function fetchSubmissions(
  status: "pending" | "approved" | "rejected",
  token: string,
  onUnauthorized: () => void,
): Promise<AdminSubmission[]> {
  const res = await adminFetch(`/submissions?status=${status}`, {}, token, onUnauthorized);
  if (!res.ok) throw new Error(await extractError(res));
  const data = await res.json();
  return data.data ?? [];
}

export async function approveSubmission(
  id: string,
  payload: ApprovePayload,
  token: string,
  onUnauthorized: () => void,
): Promise<AdminBlog> {
  const res = await adminFetch(`/submissions/${id}/approve`, {
    method: "POST",
    body: JSON.stringify(payload),
  }, token, onUnauthorized);
  if (!res.ok) throw new Error(await extractError(res));
  return res.json();
}

export async function rejectSubmission(
  id: string,
  reviewNote: string,
  token: string,
  onUnauthorized: () => void,
): Promise<void> {
  const res = await adminFetch(`/submissions/${id}/reject`, {
    method: "POST",
    body: JSON.stringify({ review_note: reviewNote }),
  }, token, onUnauthorized);
  if (!res.ok) throw new Error(await extractError(res));
}

// ─────────────────────────────────────────
// 博客治理 API
// ─────────────────────────────────────────

export async function fetchBlogs(
  health: "unhealthy" | undefined,
  token: string,
  onUnauthorized: () => void,
): Promise<AdminBlog[]> {
  const qs = health ? `?health=${health}` : "";
  const res = await adminFetch(`/blogs${qs}`, {}, token, onUnauthorized);
  if (!res.ok) throw new Error(await extractError(res));
  const data = await res.json();
  return data.data ?? [];
}

export async function fetchQueue(token: string, onUnauthorized: () => void): Promise<FetchQueueSnapshot> {
  const res = await adminFetch("/fetch-queue", {}, token, onUnauthorized);
  if (!res.ok) throw new Error(await extractError(res));
  return res.json();
}

export async function fetchAttempts(
  host: string,
  token: string,
  onUnauthorized: () => void,
): Promise<FetchAttempt[]> {
  const res = await adminFetch(`/blogs/${encodeURIComponent(host)}/fetch-attempts`, {}, token, onUnauthorized);
  if (!res.ok) throw new Error(await extractError(res));
  const data = await res.json();
  return data.data ?? [];
}

export async function createBlog(
  payload: CreateBlogPayload,
  token: string,
  onUnauthorized: () => void,
): Promise<AdminBlog> {
  const res = await adminFetch("/blogs", {
    method: "POST",
    body: JSON.stringify(payload),
  }, token, onUnauthorized);
  if (!res.ok) throw new Error(await extractError(res));
  return res.json();
}

export async function updateBlog(
  host: string,
  payload: UpdateBlogPayload,
  token: string,
  onUnauthorized: () => void,
): Promise<AdminBlog> {
  const res = await adminFetch(`/blogs/${encodeURIComponent(host)}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  }, token, onUnauthorized);
  if (!res.ok) throw new Error(await extractError(res));
  return res.json();
}

export async function deleteBlog(
  host: string,
  exclude: "" | "opt_out" | "blocked",
  note: string,
  token: string,
  onUnauthorized: () => void,
): Promise<void> {
  const qs = exclude ? `?exclude=${exclude}&note=${encodeURIComponent(note)}` : "";
  const res = await adminFetch(`/blogs/${encodeURIComponent(host)}${qs}`, {
    method: "DELETE",
  }, token, onUnauthorized);
  if (!res.ok) throw new Error(await extractError(res));
}

export async function fetchBlogNow(
  host: string,
  token: string,
  onUnauthorized: () => void,
): Promise<void> {
  const res = await adminFetch(`/blogs/${encodeURIComponent(host)}/fetch`, {
    method: "POST",
  }, token, onUnauthorized);
  if (!res.ok) throw new Error(await extractError(res));
}

// ─────────────────────────────────────────
// 排除名单 API
// ─────────────────────────────────────────

export async function fetchExcludedHosts(
  token: string,
  onUnauthorized: () => void,
): Promise<ExcludedHost[]> {
  const res = await adminFetch("/excluded-hosts", {}, token, onUnauthorized);
  if (!res.ok) throw new Error(await extractError(res));
  const data = await res.json();
  return data.data ?? [];
}

export async function deleteExcludedHost(
  host: string,
  token: string,
  onUnauthorized: () => void,
): Promise<void> {
  const res = await adminFetch(`/excluded-hosts/${encodeURIComponent(host)}`, {
    method: "DELETE",
  }, token, onUnauthorized);
  if (!res.ok) throw new Error(await extractError(res));
}

// ─────────────────────────────────────────
// 在线检测 API
// ─────────────────────────────────────────

export async function runCheck(
  url: string,
  feedURL: string | undefined,
  token: string,
  onUnauthorized: () => void,
): Promise<CheckReport> {
  const body: Record<string, string> = { url };
  if (feedURL) body.feed_url = feedURL;
  const res = await adminFetch("/check", {
    method: "POST",
    body: JSON.stringify(body),
  }, token, onUnauthorized);
  if (!res.ok) throw new Error(await extractError(res));
  return res.json();
}

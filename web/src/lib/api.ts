import { apiURL } from "@/lib/config";
import type { ApiError, Blog, BlogPage, Entry, Notice, Page, Submission, Tag } from "@/lib/types";
import type { Lang } from "@/i18n";

// Page loaders call the API from the server. The on-demand article check is
// the one same-origin browser request; see docs/design/frontend.md section 5.

const TIMEOUT_MS = 3000;
const SUBMIT_TIMEOUT_MS = 35_000;

export type Result<T> =
  | { kind: "ok"; data: T }
  | { kind: "notFound" }
  | { kind: "error"; status: number; body: ApiError }
  | { kind: "unavailable" };

/** Whom a call is for: the interface language, and the reader's address (lib/forwarded.ts). */
export interface Caller {
  lang: Lang;
  forwardedFor?: string;
}

const acceptLanguage = (lang: Lang) => (lang === "zh" ? "zh-CN" : "en");

async function call<T>(path: string, init: RequestInit & { caller: Caller; timeout?: number }): Promise<Result<T>> {
  const { caller, timeout, ...rest } = init;
  let res: Response;
  try {
    res = await fetch(apiURL() + path, {
      ...rest,
      headers: {
        "Accept-Language": acceptLanguage(caller.lang),
        ...(caller.forwardedFor ? { "X-Forwarded-For": caller.forwardedFor } : {}),
        ...(rest.headers ?? {}),
      },
      signal: AbortSignal.timeout(timeout ?? TIMEOUT_MS),
    });
  } catch {
    return { kind: "unavailable" };
  }
  if (res.status === 404) {
    return { kind: "notFound" };
  }
  if (res.status >= 500) {
    return { kind: "unavailable" };
  }
  let body: unknown;
  try {
    body = await res.json();
  } catch {
    return { kind: "unavailable" };
  }
  if (!res.ok) {
    return { kind: "error", status: res.status, body: body as ApiError };
  }
  return { kind: "ok", data: body as T };
}

function query(params: Record<string, string | undefined>): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v) q.set(k, v);
  }
  const s = q.toString();
  return s ? `?${s}` : "";
}

export const api = {
  entries: (caller: Caller, p: { order?: "recommended"; cursor?: string; lang?: string; tag?: string; limit?: number; tz?: string }) =>
    call<Page<Entry>>(
      `/api/v1/entries${query({ order: p.order, cursor: p.cursor, lang: p.lang, tag: p.tag, limit: p.limit?.toString(), tz: p.tz })}`,
      { caller },
    ),

  me: (caller: Caller, cookie: string) =>
    call<{ id: string; number: number; created_at: string; temporary_password: boolean; email: string; display_name: string; is_admin: boolean }>("/api/v1/me", { caller, headers: { Cookie: cookie } }),

  following: (caller: Caller, p: { cursor?: string; lang?: string; tag?: string; limit?: number }, cookie: string) =>
    call<Page<Entry>>(
      `/api/v1/me/entries${query({ cursor: p.cursor, lang: p.lang, tag: p.tag, limit: p.limit?.toString() })}`,
      { caller, headers: { Cookie: cookie } },
    ),

  entry: (caller: Caller, id: string) => call<Entry>(`/api/v1/entries/${encodeURIComponent(id)}`, { caller }),

  tags: (caller: Caller) => call<{ data: Tag[] }>("/api/v1/tags", { caller }),

  notices: (caller: Caller, audience: "zh" | "en") => call<{ data: Notice[] }>(`/api/v1/notices${query({ audience })}`, { caller }),

  notice: (caller: Caller, id: string) => call<Notice>(`/api/v1/notices/${encodeURIComponent(id)}`, { caller }),

  blogs: (caller: Caller, p: { order?: "newest"; cursor?: string; lang?: string; limit?: number }) =>
    call<Page<Blog>>(`/api/v1/blogs${query({ order: p.order, cursor: p.cursor, lang: p.lang, limit: p.limit?.toString() })}`, { caller }),

  blog: (caller: Caller, host: string, p: { cursor?: string; limit?: number } = {}) =>
    call<BlogPage>(`/api/v1/blogs/${encodeURIComponent(host)}${query({ cursor: p.cursor, limit: p.limit?.toString() })}`, { caller }),

  submission: (caller: Caller, id: string) => call<Submission>(`/api/v1/submissions/${encodeURIComponent(id)}`, { caller }),

  submit: (caller: Caller, body: { site_url: string; feed_url: string; note: string; title?: string; description?: string }) =>
    call<Submission>("/api/v1/submissions", {
      caller,
      method: "POST",
      timeout: SUBMIT_TIMEOUT_MS,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }),
};

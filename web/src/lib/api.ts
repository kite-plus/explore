import { apiURL } from "@/lib/config";
import type { ApiError, Blog, BlogPage, Entry, Page, Submission, Tag } from "@/lib/types";
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

const acceptLanguage = (lang: Lang) => (lang === "zh" ? "zh-CN" : "en");

async function call<T>(path: string, init: RequestInit & { lang: Lang; timeout?: number }): Promise<Result<T>> {
  let res: Response;
  try {
    res = await fetch(apiURL() + path, {
      ...init,
      headers: { "Accept-Language": acceptLanguage(init.lang), ...(init.headers ?? {}) },
      signal: AbortSignal.timeout(init.timeout ?? TIMEOUT_MS),
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
  entries: (lang: Lang, p: { cursor?: string; lang?: string; tag?: string; limit?: number }) =>
    call<Page<Entry>>(
      `/api/v1/entries${query({ cursor: p.cursor, lang: p.lang, tag: p.tag, limit: p.limit?.toString() })}`,
      { lang },
    ),

  following: (lang: Lang, p: { cursor?: string; lang?: string; tag?: string; limit?: number }, cookie: string) =>
    call<Page<Entry>>(
      `/api/v1/me/entries${query({ cursor: p.cursor, lang: p.lang, tag: p.tag, limit: p.limit?.toString() })}`,
      { lang, headers: { Cookie: cookie } },
    ),

  tags: (lang: Lang) => call<{ data: Tag[] }>("/api/v1/tags", { lang }),

  blogs: (lang: Lang, p: { cursor?: string; lang?: string; limit?: number }) =>
    call<Page<Blog>>(`/api/v1/blogs${query({ cursor: p.cursor, lang: p.lang, limit: p.limit?.toString() })}`, { lang }),

  blog: (lang: Lang, host: string, p: { cursor?: string; limit?: number } = {}) =>
    call<BlogPage>(`/api/v1/blogs/${encodeURIComponent(host)}${query({ cursor: p.cursor, limit: p.limit?.toString() })}`, { lang }),

  submission: (lang: Lang, id: string) => call<Submission>(`/api/v1/submissions/${encodeURIComponent(id)}`, { lang }),

  // The reader's address is passed on so the API's rate limit counts
  // readers rather than this server.
  submit: (
    lang: Lang,
    body: { site_url: string; feed_url: string; note: string; title?: string; description?: string },
    clientAddress?: string,
  ) =>
    call<Submission>("/api/v1/submissions", {
      lang,
      method: "POST",
      timeout: SUBMIT_TIMEOUT_MS,
      headers: {
        "Content-Type": "application/json",
        ...(clientAddress ? { "X-Forwarded-For": clientAddress } : {}),
      },
      body: JSON.stringify(body),
    }),
};

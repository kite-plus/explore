// Admin requests go through the same-origin /api/v1/admin/* proxy with the
// admin account's session cookie, plus its CSRF token for writes.

import { readerRequest, type ReaderUser } from "@/lib/reader-api";

/** Thrown for a 401, after onUnauthorized has signed the admin out. */
export class AdminUnauthorizedError extends Error {
  constructor() {
    super("Unauthorized");
    this.name = "AdminUnauthorizedError";
  }
}

async function adminFetch(path: string, options: RequestInit, onUnauthorized: () => void): Promise<Response> {
  const csrf = !["GET", "HEAD"].includes(options.method ?? "GET") ? (await readerRequest<ReaderUser>("me")).csrf_token : undefined;
  const response = await fetch(`/api/v1/admin${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      "Accept-Language": "zh-CN",
      ...(csrf ? { "X-CSRF-Token": csrf } : {}),
      ...(options.headers as Record<string, string> | undefined),
    },
  });
  if (response.status === 401) {
    onUnauthorized();
    throw new AdminUnauthorizedError();
  }
  return response;
}

async function extractError(response: Response): Promise<string> {
  try {
    const data = await response.json();
    if (data?.error?.code === "check_failed") {
      const problem = data.check_report?.problems?.find((item: { severity: string }) => item.severity === "error");
      if (problem?.hint) return `检查未通过：${problem.hint}`;
    }
    return data?.error?.message ?? data?.error?.code ?? `HTTP ${response.status}`;
  } catch {
    return `HTTP ${response.status}`;
  }
}

export async function adminRequest<T>(path: string, onUnauthorized: () => void, options: RequestInit = {}): Promise<T> {
  const response = await adminFetch(path, options, onUnauthorized);
  if (!response.ok) throw new Error(await extractError(response));
  const body = await response.text();
  return body ? (JSON.parse(body) as T) : (undefined as T);
}

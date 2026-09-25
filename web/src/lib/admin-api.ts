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

/** A failed request, with the API's error code when it gave one. */
export class AdminRequestError extends Error {
  constructor(
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "AdminRequestError";
  }
}

async function apiFetch(url: string, options: RequestInit, onUnauthorized: () => void): Promise<Response> {
  const csrf = !["GET", "HEAD"].includes(options.method ?? "GET") ? (await readerRequest<ReaderUser>("me")).csrf_token : undefined;
  const response = await fetch(url, {
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

async function extractError(response: Response): Promise<AdminRequestError> {
  try {
    const data = await response.json();
    const code: string = data?.error?.code ?? "";
    if (code === "check_failed") {
      const problem = data.check_report?.problems?.find((item: { severity: string }) => item.severity === "error");
      if (problem?.hint) return new AdminRequestError(code, `检查未通过：${problem.hint}`);
    }
    return new AdminRequestError(code, data?.error?.message ?? (code || `HTTP ${response.status}`));
  } catch {
    return new AdminRequestError("", `HTTP ${response.status}`);
  }
}

async function request<T>(url: string, onUnauthorized: () => void, options: RequestInit): Promise<T> {
  const response = await apiFetch(url, options, onUnauthorized);
  if (!response.ok) throw await extractError(response);
  const body = await response.text();
  return body ? (JSON.parse(body) as T) : (undefined as T);
}

export function adminRequest<T>(path: string, onUnauthorized: () => void, options: RequestInit = {}): Promise<T> {
  return request<T>(`/api/v1/admin${path}`, onUnauthorized, options);
}

/** A request about the signed-in account itself, such as /me/password. */
export function accountRequest<T>(path: string, onUnauthorized: () => void, options: RequestInit = {}): Promise<T> {
  return request<T>(`/api/v1${path}`, onUnauthorized, options);
}

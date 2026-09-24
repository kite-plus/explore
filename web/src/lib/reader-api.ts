export interface ReaderUser {
  id: string;
  email: string;
  display_name: string;
  is_admin: boolean;
  csrf_token: string;
}

export interface ReaderBlog {
  host: string;
  name: string;
  description: string;
  site_url: string;
}

export async function readerRequest<T>(path: string, init: RequestInit = {}, csrfToken?: string): Promise<T> {
  const response = await fetch(`/api/v1/${path}`, {
    ...init,
    credentials: "same-origin",
    headers: {
      ...(init.body ? { "Content-Type": "application/json" } : {}),
      ...(csrfToken ? { "X-CSRF-Token": csrfToken } : {}),
      ...init.headers,
    },
  });
  if (!response.ok) {
    let message = `HTTP ${response.status}`;
    try { message = (await response.json()).error?.message ?? message; } catch { /* 响应可能没有正文。 */ }
    throw new Error(message);
  }
  const body = await response.text();
  return body ? JSON.parse(body) as T : undefined as T;
}

export async function currentReader(): Promise<ReaderUser | null> {
  try { return await readerRequest<ReaderUser>("me"); } catch { return null; }
}

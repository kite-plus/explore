/** Who is signed in, as the header shows it. */
export interface Reader {
  id: string;
  /** The ID people see: sign-up order, never changes. `id` is internal. */
  number: number;
  created_at: string;
  email: string;
  display_name: string;
  is_admin: boolean;
}

export interface ReaderUser extends Reader {
  csrf_token: string;
}

export interface ReaderBlog {
  host: string;
  name: string;
  description: string;
  site_url: string;
}

export interface ImportResult {
  outlines: number;
  added: number;
  already_following: number;
  ignored: number;
  not_listed: { title: string; site_url: string; feed_url: string }[];
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

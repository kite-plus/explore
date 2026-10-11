import { api, type AuthOptions, type Caller } from "@/lib/api";

// How readers sign in is a setting of the server, so it is kept a minute.
const TTL_MS = 60_000;

let cached: { at: number; options: AuthOptions } | undefined;

/** The server's sign-in options, or undefined when the API cannot be asked and nothing is cached. */
export async function signInOptions(from: Caller): Promise<AuthOptions | undefined> {
  if (cached && Date.now() - cached.at < TTL_MS) return cached.options;
  const r = await api.authOptions(from);
  if (r.kind !== "ok") return cached?.options;
  cached = { at: Date.now(), options: r.data };
  return cached.options;
}

/** Whether readers sign in with Kite Plus alone: no passwords, no sign-up form. */
export function kitePlusOnly(options: AuthOptions | undefined): boolean {
  return Boolean(options?.oidc) && options?.password === false;
}

/**
 * Where a "Sign in" link goes from a page at returnTo: straight to Kite Plus
 * when it is the only way in, otherwise to the sign-in page. See
 * docs/design/accounts.md section 1.
 */
export function signInHref(options: AuthOptions | undefined, loginPath: string, returnTo: string): string {
  if (kitePlusOnly(options)) return `/auth/login?return=${encodeURIComponent(returnTo)}`;
  return `${loginPath}?next=${encodeURIComponent(returnTo)}`;
}

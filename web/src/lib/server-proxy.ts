import type { APIRoute } from "astro";

import { apiURL } from "@/lib/config";
import { forwardedFor } from "@/lib/forwarded";

// Headers the API reads on these routes: the session and sign-in cookies,
// the reader's language, and the signature on the identity provider's
// pushed events.
const requestHeaders = ["cookie", "content-type", "accept-language", "zitadel-signature"];
const responseHeaders = ["content-type", "cache-control", "location", "content-security-policy", "referrer-policy", "x-robots-tag", "vary"];

/**
 * Hands a request to the API as it is, for the paths the API itself serves
 * outside /api: sign-in (/auth), the identity provider's hooks (/hooks), the
 * apps' sign-in callback page and the app link files. The reverse proxy
 * sends them straight to the API; this covers astro dev and deployments
 * without it. See docs/design/accounts.md section 7.
 */
export const serverProxy: APIRoute = async ({ request, clientAddress }) => {
  const url = new URL(request.url);
  const headers = new Headers();
  for (const name of requestHeaders) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }
  const forwarded = forwardedFor(request, clientAddress);
  if (forwarded) headers.set("X-Forwarded-For", forwarded);
  try {
    const response = await fetch(`${apiURL()}${url.pathname}${url.search}`, {
      method: request.method,
      headers,
      body: ["GET", "HEAD"].includes(request.method) ? undefined : await request.arrayBuffer(),
      redirect: "manual",
      signal: AbortSignal.timeout(15_000),
    });
    const outgoing = new Headers();
    for (const name of responseHeaders) {
      const value = response.headers.get(name);
      if (value) outgoing.set(name, value);
    }
    for (const cookie of response.headers.getSetCookie()) outgoing.append("Set-Cookie", cookie);
    return new Response(response.body, { status: response.status, headers: outgoing });
  } catch {
    return new Response(JSON.stringify({ error: { code: "unavailable" } }), {
      status: 502,
      headers: { "Content-Type": "application/json", "Cache-Control": "no-store" },
    });
  }
};

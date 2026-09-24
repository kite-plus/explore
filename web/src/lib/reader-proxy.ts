import type { APIRoute } from "astro";

import { apiURL } from "@/lib/config";

export const readerProxy: APIRoute = async ({ request, params, clientAddress }) => {
  const url = new URL(request.url);
  const path = params.path ?? "";
  const group = url.pathname.startsWith("/api/v1/auth/") ? "auth" : url.pathname === "/api/v1/reports" ? "reports" : "me";
  const target = `${apiURL()}/api/v1/${group}${path ? `/${path}` : ""}${url.search}`;
  const headers = new Headers();
  for (const name of ["cookie", "content-type", "accept-language", "x-csrf-token"]) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }
  if (clientAddress) headers.set("X-Forwarded-For", clientAddress);
  try {
    const response = await fetch(target, {
      method: request.method,
      headers,
      body: ["GET", "HEAD"].includes(request.method) ? undefined : request.body,
      // @ts-expect-error Node 支持流式请求体。
      duplex: "half",
      redirect: "manual",
      signal: AbortSignal.timeout(15_000),
    });
    const outgoing = new Headers({
      "Content-Type": response.headers.get("Content-Type") ?? "application/json",
      "Cache-Control": "private, no-store",
    });
    for (const cookie of response.headers.getSetCookie()) outgoing.append("Set-Cookie", cookie);
    const location = response.headers.get("Location");
    if (location) outgoing.set("Location", location);
    return new Response(response.body, { status: response.status, headers: outgoing });
  } catch {
    return new Response(JSON.stringify({ error: { code: "unavailable" } }), {
      status: 502,
      headers: { "Content-Type": "application/json", "Cache-Control": "no-store" },
    });
  }
};

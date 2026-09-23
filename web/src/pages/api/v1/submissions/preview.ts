import type { APIRoute } from "astro";

import { apiURL } from "@/lib/config";

const noStore = { "Cache-Control": "no-store", "Content-Type": "application/json; charset=utf-8" };

export const POST: APIRoute = async ({ clientAddress, request }) => {
  let body: string;
  try {
    body = await request.text();
  } catch {
    return new Response(JSON.stringify({ error: { code: "invalid_request" } }), { status: 400, headers: noStore });
  }

  let response: Response;
  try {
    response = await fetch(`${apiURL()}/api/v1/submissions/preview`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Accept-Language": request.headers.get("Accept-Language") ?? "zh-CN,zh;q=0.9,en;q=0.8",
        "X-Forwarded-For": clientAddress,
      },
      body,
      signal: AbortSignal.timeout(35_000),
    });
  } catch {
    return new Response(JSON.stringify({ error: { code: "unavailable" } }), { status: 502, headers: noStore });
  }

  const headers = new Headers(noStore);
  const retryAfter = response.headers.get("Retry-After");
  if (retryAfter) headers.set("Retry-After", retryAfter);
  return new Response(response.body, { status: response.status, headers });
};

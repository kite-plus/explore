import type { APIRoute } from "astro";

import { apiURL } from "@/lib/config";

const noStore = { "Cache-Control": "no-store", "Content-Type": "application/json; charset=utf-8" };

const forward: APIRoute = async ({ clientAddress, params, request }) => {
  const id = params.id ?? "";
  if (!/^[1-9]\d*$/.test(id)) return new Response(null, { status: 404, headers: noStore });

  let response: Response;
  try {
    response = await fetch(`${apiURL()}/api/v1/entries/${id}/check`, {
      method: request.method,
      headers: {
        "Accept-Language": request.headers.get("Accept-Language") ?? "en",
        ...(request.method === "POST" ? { "Content-Type": "application/json", "X-Forwarded-For": clientAddress } : {}),
      },
      signal: AbortSignal.timeout(15_000),
    });
  } catch {
    return new Response(JSON.stringify({ error: { code: "unavailable" } }), { status: 502, headers: noStore });
  }
  const headers = new Headers(noStore);
  const retryAfter = response.headers.get("Retry-After");
  if (retryAfter) headers.set("Retry-After", retryAfter);
  return new Response(response.body, { status: response.status, headers });
};

export const GET = forward;
export const POST = forward;

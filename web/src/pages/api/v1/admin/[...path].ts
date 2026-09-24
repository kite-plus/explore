import type { APIRoute } from "astro";

import { apiURL } from "@/lib/config";

/**
 * Forwards /api/v1/admin/* to the Gin API with the query string, the admin's
 * session cookie and its CSRF token, for every method.
 */
export const ALL: APIRoute = async ({ request, params }) => {
  const path = params.path ?? "";
  const url = new URL(request.url);
  const target = `${apiURL()}/api/v1/admin/${path}${url.search}`;
  const headers = new Headers();
  for (const name of ["content-type", "accept-language", "cookie", "x-csrf-token"]) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }

  let response: Response;
  try {
    response = await fetch(target, {
      method: request.method,
      headers,
      body: ["GET", "HEAD"].includes(request.method) ? undefined : request.body,
      // @ts-expect-error Node's fetch needs duplex to stream a request body.
      duplex: "half",
      signal: AbortSignal.timeout(35_000),
    });
  } catch {
    return new Response(JSON.stringify({ error: { code: "unavailable" } }), {
      status: 502,
      headers: { "Content-Type": "application/json" },
    });
  }

  // Only the content type passes through; the API's other headers stay behind.
  const responseHeaders = new Headers({
    "Content-Type": response.headers.get("Content-Type") ?? "application/json",
    "Cache-Control": "no-store",
  });

  return new Response(response.body, {
    status: response.status,
    headers: responseHeaders,
  });
};

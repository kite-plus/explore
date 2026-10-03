import type { APIRoute } from "astro";

import { apiURL } from "@/lib/config";
import { forwardedHeaders } from "@/lib/forwarded";

export const GET: APIRoute = async ({ request, clientAddress }) => {
  try {
    const response = await fetch(`${apiURL()}/api/v1/tags`, {
      headers: forwardedHeaders(request, clientAddress),
      signal: AbortSignal.timeout(10_000),
    });
    return new Response(response.body, {
      status: response.status,
      headers: {
        "Content-Type": response.headers.get("Content-Type") ?? "application/json",
        "Cache-Control": "public, max-age=3600",
      },
    });
  } catch {
    return new Response(JSON.stringify({ error: { code: "unavailable" } }), {
      status: 502,
      headers: { "Content-Type": "application/json", "Cache-Control": "no-store" },
    });
  }
};

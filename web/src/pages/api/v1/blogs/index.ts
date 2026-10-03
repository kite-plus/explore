import type { APIRoute } from "astro";
import { apiURL } from "@/lib/config";
import { forwardedHeaders } from "@/lib/forwarded";

export const GET: APIRoute = async ({ url, request, clientAddress }) => {
  try {
    const response = await fetch(`${apiURL()}/api/v1/blogs${url.search}`, {
      headers: forwardedHeaders(request, clientAddress),
      signal: AbortSignal.timeout(10_000),
    });
    return new Response(response.body, { status: response.status, headers: {
      "Content-Type": response.headers.get("Content-Type") ?? "application/json",
      "Cache-Control": response.ok ? "public, max-age=60" : "no-store",
    } });
  } catch {
    return new Response(JSON.stringify({ error: { code: "unavailable" } }), { status: 502,
      headers: { "Content-Type": "application/json", "Cache-Control": "no-store" } });
  }
};

import type { APIRoute } from "astro";
import { apiURL } from "@/lib/config";

export const GET: APIRoute = async () => {
  try {
    const response = await fetch(`${apiURL()}/api/v1/site-config`, { signal: AbortSignal.timeout(10_000) });
    return new Response(response.body, { status: response.status, headers: {
      "Content-Type": response.headers.get("Content-Type") ?? "application/json",
      "Cache-Control": "no-store",
    } });
  } catch {
    return new Response(JSON.stringify({ error: { code: "unavailable" } }), { status: 502,
      headers: { "Content-Type": "application/json", "Cache-Control": "no-store" } });
  }
};

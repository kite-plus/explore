import type { APIRoute } from "astro";

import { apiURL } from "@/lib/config";

export const GET: APIRoute = async ({ params }) => {
  const id = params.id ?? "";
  if (!/^\d+$/.test(id)) return new Response(null, { status: 404 });

  let response: Response;
  try {
    response = await fetch(`${apiURL()}/api/v1/entries/${id}/image`, { signal: AbortSignal.timeout(10_000) });
  } catch {
    return new Response(null, { status: 502, headers: { "Cache-Control": "no-store" } });
  }
  if (!response.ok) return new Response(null, { status: response.status, headers: { "Cache-Control": "no-store" } });

  const contentType = response.headers.get("Content-Type") ?? "";
  if (!/^image\/(jpeg|png|gif|webp)$/.test(contentType)) {
    return new Response(null, { status: 502, headers: { "Cache-Control": "no-store" } });
  }
  return new Response(response.body, {
    headers: {
      "Content-Type": contentType,
      "Cache-Control": response.headers.get("Cache-Control") ?? "public, max-age=60",
      "X-Content-Type-Options": "nosniff",
    },
  });
};

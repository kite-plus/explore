import type { APIRoute } from "astro";

import { apiURL } from "@/lib/config";

export const GET: APIRoute = async ({ params }) => {
  const host = params.host ?? "";
  if (!/^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$/i.test(host)) return new Response(null, { status: 404 });

  let response: Response;
  try {
    response = await fetch(`${apiURL()}/api/v1/blogs/${encodeURIComponent(host)}/favicon`, { signal: AbortSignal.timeout(30_000) });
  } catch {
    return new Response(null, { status: 502, headers: { "Cache-Control": "no-store" } });
  }
  if (!response.ok) return new Response(null, { status: response.status, headers: { "Cache-Control": "no-store" } });

  const contentType = response.headers.get("Content-Type") ?? "";
  if (!/^image\/(png|gif|jpeg|webp|x-icon)$/.test(contentType)) {
    return new Response(null, { status: 502, headers: { "Cache-Control": "no-store" } });
  }
  return new Response(response.body, {
    headers: {
      "Content-Type": contentType,
      "Cache-Control": response.headers.get("Cache-Control") ?? "public, max-age=3600",
      "X-Content-Type-Options": "nosniff",
    },
  });
};

import type { APIRoute } from "astro";

import { publicURL } from "@/lib/config";

// An endpoint rather than a static file, so the sitemap address follows
// EXPLORE_PUBLIC_URL.
export const GET: APIRoute = () => {
  const body = [
    "User-agent: *",
    "Allow: /",
    "Disallow: /submit",
    "Disallow: /submissions/",
    "Disallow: /en/submit",
    "Disallow: /en/submissions/",
    "",
    `Sitemap: ${publicURL()}/sitemap.xml`,
    "",
  ].join("\n");
  return new Response(body, {
    headers: { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "public, max-age=86400" },
  });
};

import type { APIRoute } from "astro";

import { localePath, type Lang } from "@/i18n";
import { api } from "@/lib/api";
import { publicURL } from "@/lib/config";

// Written by hand: Astro's sitemap integration cannot list pages rendered
// on demand. See docs/design/frontend.md section 7.
export const GET: APIRoute = async () => {
  const base = publicURL();
  const langs: Lang[] = ["zh", "en"];
  const urls: { loc: string; lastmod?: string }[] = [];

  for (const path of ["/", "/blogs", "/about"]) {
    for (const lang of langs) urls.push({ loc: base + localePath(lang, path) });
  }
  urls.push({ loc: `${base}/bot` });

  let cursor: string | undefined;
  for (;;) {
    const r = await api.blogs("en", { cursor, limit: 100 });
    if (r.kind !== "ok") {
      return new Response("sitemap unavailable\n", { status: 503, headers: { "Retry-After": "60", "Cache-Control": "no-store" } });
    }
    for (const b of r.data.data) {
      for (const lang of langs) {
        urls.push({ loc: base + localePath(lang, `/blogs/${b.host}`), lastmod: b.last_published_at ?? undefined });
      }
    }
    if (!r.data.next_cursor) break;
    cursor = r.data.next_cursor;
  }

  const body = [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    ...urls.map(
      (u) => `  <url><loc>${escape(u.loc)}</loc>${u.lastmod ? `<lastmod>${escape(u.lastmod)}</lastmod>` : ""}</url>`,
    ),
    "</urlset>",
    "",
  ].join("\n");
  return new Response(body, {
    headers: { "Content-Type": "application/xml; charset=utf-8", "Cache-Control": "public, max-age=3600" },
  });
};

function escape(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

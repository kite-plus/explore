import { defineMiddleware } from "astro:middleware";

import { api } from "@/lib/api";
import { publicURL } from "@/lib/config";
import { forwardedFor } from "@/lib/forwarded";

const formTypes = ["application/x-www-form-urlencoded", "multipart/form-data", "text/plain"];

// Astro's own checkOrigin compares Origin with the URL this server sees,
// which behind the proxy is http://, so it refused every form a browser
// posted. This is the same check against EXPLORE_PUBLIC_URL, still taking
// the server's own address for astro dev and the tests.
function crossSite(request: Request, url: URL): boolean {
  if (["GET", "HEAD", "OPTIONS"].includes(request.method)) return false;
  const origin = request.headers.get("origin");
  if (origin === new URL(publicURL()).origin || origin === url.origin) return false;
  const type = request.headers.get("content-type")?.toLowerCase();
  return type === undefined || formTypes.some((t) => type.includes(t));
}

// Pages draw the header from the session the request carries, so a signed-in
// reader sees their name from the first paint; see docs/design/frontend.md
// section 5. A page drawn for one reader goes out private, and every page
// varies on the cookie, so caches keep the public copy for anonymous readers.
export const onRequest = defineMiddleware(async (ctx, next) => {
  if (!ctx.isPrerendered && crossSite(ctx.request, ctx.url)) {
    return new Response(`Cross-site ${ctx.request.method} form submissions are forbidden`, { status: 403 });
  }
  ctx.locals.reader = null;
  ctx.locals.readerChecked = false;
  const path = ctx.url.pathname;
  if (path.startsWith("/api/") || path.startsWith("/admin") || /^(\/en)?\/go$/.test(path) || /\.[a-z0-9]+$/i.test(path)) return next();

  const cookie = ctx.request.headers.get("cookie");
  if (cookie) {
    const me = await api.me({ lang: path.startsWith("/en") ? "en" : "zh", forwardedFor: forwardedFor(ctx.request, ctx.clientAddress) }, cookie);
    if (me.kind === "ok") {
      const { id, number, created_at, temporary_password, email, display_name, is_admin } = me.data;
      ctx.locals.reader = { id, number, created_at, temporary_password, email, display_name, is_admin };
      ctx.locals.readerChecked = true;
    } else if (me.kind === "error" && me.status === 401) {
      ctx.locals.readerChecked = true;
    }
  }

  const response = await next();
  if (!response.headers.get("content-type")?.startsWith("text/html")) return response;
  response.headers.append("Vary", "Cookie");
  if (ctx.locals.reader) response.headers.set("Cache-Control", "private, no-store");
  return response;
});

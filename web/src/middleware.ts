import { defineMiddleware } from "astro:middleware";

import { api } from "@/lib/api";
import { forwardedFor } from "@/lib/forwarded";

// Pages draw the header from the session the request carries, so a signed-in
// reader sees their name from the first paint; see docs/design/frontend.md
// section 5. A page drawn for one reader goes out private, and every page
// varies on the cookie, so caches keep the public copy for anonymous readers.
export const onRequest = defineMiddleware(async (ctx, next) => {
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

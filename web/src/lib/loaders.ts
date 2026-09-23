import type { AstroGlobal } from "astro";

import { dict, localePath, type Lang } from "@/i18n";
import { api } from "@/lib/api";
import { tagList } from "@/lib/tags";
import type { BlogPage, Blog, CheckReport, Entry, Page, Submission, Tag } from "@/lib/types";

// Loaders run in a page's frontmatter, the only place where the status
// and headers can still change: Astro streams the body after that.

export type Loaded<T> = { kind: "ok"; data: T } | { kind: "notFound" } | { kind: "unavailable" };

const cacheControl = (ctx: AstroGlobal, value: string) => ctx.response.headers.set("Cache-Control", value);

function notFound<T>(ctx: AstroGlobal): Loaded<T> {
  ctx.response.status = 404;
  cacheControl(ctx, "no-store");
  return { kind: "notFound" };
}

// A failing API gets 503 with Retry-After, so search engines come back
// later instead of dropping pages that only broke for a while.
function unavailable<T>(ctx: AstroGlobal): Loaded<T> {
  ctx.response.status = 503;
  ctx.response.headers.set("Retry-After", "60");
  cacheControl(ctx, "no-store");
  return { kind: "unavailable" };
}

const param = (ctx: AstroGlobal, name: string) => ctx.url.searchParams.get(name) ?? undefined;

export interface StreamPage {
  page: Page<Entry>;
  filter?: string;
  tag?: string;
  /** Empty when the list could not be had; pages then show no tags. */
  tags: Tag[];
  paged: boolean;
}

export async function loadHome(ctx: AstroGlobal, lang: Lang): Promise<Loaded<StreamPage>> {
  const cursor = param(ctx, "cursor");
  const filter = param(ctx, "lang");
  const tag = param(ctx, "tag");
  const [r, tags] = await Promise.all([api.entries(lang, { cursor, lang: filter, tag, limit: 30 }), tagList(lang)]);
  if (r.kind === "unavailable") return unavailable(ctx);
  if (r.kind !== "ok") return notFound(ctx);
  cacheControl(ctx, "public, max-age=60");
  return { kind: "ok", data: { page: r.data, filter, tag, tags: tags ?? [], paged: Boolean(cursor) } };
}

export interface DirectoryPage {
  page: Page<Blog>;
  filter?: string;
  paged: boolean;
}

export async function loadBlogs(ctx: AstroGlobal, lang: Lang): Promise<Loaded<DirectoryPage>> {
  const cursor = param(ctx, "cursor");
  const filter = param(ctx, "lang");
  const r = await api.blogs(lang, { cursor, lang: filter, limit: 50 });
  if (r.kind === "unavailable") return unavailable(ctx);
  if (r.kind !== "ok") return notFound(ctx);
  cacheControl(ctx, "public, max-age=300");
  return { kind: "ok", data: { page: r.data, filter, paged: Boolean(cursor) } };
}

export type BlogView = BlogPage & { tags: Tag[] };

export async function loadBlog(ctx: AstroGlobal, lang: Lang): Promise<Loaded<BlogView>> {
  const host = ctx.params.host ?? "";
  const [r, tags] = await Promise.all([api.blog(lang, host), tagList(lang)]);
  if (r.kind === "unavailable") return unavailable(ctx);
  if (r.kind !== "ok") return notFound(ctx);
  cacheControl(ctx, "public, max-age=300");
  return { kind: "ok", data: { ...r.data, tags: tags ?? [] } };
}

export async function loadSubmission(ctx: AstroGlobal, lang: Lang): Promise<Loaded<Submission>> {
  const r = await api.submission(lang, ctx.params.id ?? "");
  if (r.kind === "unavailable") return unavailable(ctx);
  if (r.kind !== "ok") return notFound(ctx);
  cacheControl(ctx, "no-store");
  return { kind: "ok", data: r.data };
}

export interface SubmitState {
  values: { site_url: string; feed_url: string; note: string; title?: string; description?: string };
  outcome?:
    | { kind: "failed"; report: CheckReport }
    | { kind: "listed"; host: string }
    | { kind: "message"; text: string }
    | { kind: "unavailable" };
}

/**
 * loadSubmit shows the form, or handles a posted one. Success redirects
 * to the submission's status page, so a reload does not submit again.
 */
export async function loadSubmit(ctx: AstroGlobal, lang: Lang): Promise<SubmitState | Response> {
  cacheControl(ctx, "no-store");
  const state: SubmitState = { values: { site_url: "", feed_url: "", note: "" } };
  if (ctx.request.method !== "POST") return state;

  const form = await ctx.request.formData();
  const field = (name: string) => String(form.get(name) ?? "").trim();
  state.values = {
    site_url: field("site_url"),
    feed_url: field("feed_url"),
    note: field("note"),
    title: field("title"),
    description: field("description"),
  };
  const t = dict(lang).submit;

  const r = await api.submit(lang, state.values, ctx.clientAddress);
  if (r.kind === "ok") {
    return ctx.redirect(localePath(lang, `/submissions/${r.data.id}`), 303);
  }
  if (r.kind === "unavailable" || r.kind === "notFound") {
    ctx.response.status = 503;
    state.outcome = { kind: "unavailable" };
    return state;
  }

  ctx.response.status = r.status;
  switch (r.body.error.code) {
    case "already_pending":
      if (r.body.submission_id) return ctx.redirect(localePath(lang, `/submissions/${r.body.submission_id}`), 303);
      break;
    case "already_listed":
      state.outcome = { kind: "listed", host: hostOf(state.values.site_url) };
      return state;
    case "check_failed":
      if (r.body.check_report) {
        state.outcome = { kind: "failed", report: r.body.check_report };
        return state;
      }
      break;
    case "excluded":
      state.outcome = { kind: "message", text: t.excluded };
      return state;
    case "invalid_url":
    case "invalid_request":
      state.outcome = { kind: "message", text: t.invalid };
      return state;
    case "rate_limited":
      state.outcome = { kind: "message", text: t.tooMany };
      return state;
  }
  state.outcome = { kind: "message", text: r.body.error.message };
  return state;
}

function hostOf(site: string): string {
  try {
    return new URL(site.includes("://") ? site : `https://${site}`).hostname.toLowerCase();
  } catch {
    return "";
  }
}

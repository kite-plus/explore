// Read at request time rather than build time, so one image serves any
// deployment. See docs/design/frontend.md section 10.
export function apiURL(): string {
  return (process.env.EXPLORE_API_URL ?? "http://127.0.0.1:8080").replace(/\/+$/, "");
}

export function publicURL(): string {
  return (process.env.EXPLORE_PUBLIC_URL ?? "https://explore.kite.plus").replace(/\/+$/, "");
}

/**
 * A link to a blog with utm_source naming this site, so the blog's analytics
 * can tell a visit came from Explore even when the browser sends no referrer.
 * The rest of the address stays as it was, and a utm_source the author set
 * is left alone.
 */
export function withSource(link: string): string {
  try {
    const url = new URL(link);
    if ((url.protocol !== "http:" && url.protocol !== "https:") || url.searchParams.has("utm_source")) return link;
    const param = `utm_source=${encodeURIComponent(new URL(publicURL()).host)}`;
    url.search = url.search ? `${url.search}&${param}` : `?${param}`;
    return url.href;
  } catch {
    return link;
  }
}

// What the transition page reads from its fragment, which never reaches the
// server; see docs/design/architecture.md section 6.3.

/** destination is where the page leads, or null for an address it will not open. */
export function destination(hash) {
  const params = new URLSearchParams(hash.replace(/^#/, ""));
  let to;
  try {
    to = new URL(params.get("to") ?? "");
  } catch {
    return null;
  }
  if (to.protocol !== "http:" && to.protocol !== "https:") return null;
  // The blog's own host, for its avatar; it may differ from the post's.
  const site = (params.get("site") ?? "").trim().toLowerCase();
  return {
    url: to.href,
    host: to.hostname,
    site: /^[a-z0-9.-]+$/.test(site) ? site : "",
    blog: (params.get("blog") ?? "").trim(),
    title: (params.get("title") ?? "").trim(),
  };
}

/**
 * openedByExplore reports whether an Explore page opened this one, the only
 * case in which it goes on by itself; anyone else's link waits for a click,
 * so the page cannot be used to bounce readers to a site of someone's choice.
 */
export function openedByExplore(win) {
  try {
    return Boolean(win.opener) && win.opener.location.origin === win.location.origin;
  } catch {
    return false;
  }
}

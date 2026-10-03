// The filters and tabs over a list change only the list's part of the page:
// the part marked data-swap-root is fetched anew and swapped in place, and
// the rest of the page stays as it was. Whatever cannot be swapped loads the
// usual way. See docs/design/frontend.md section 4.

const ROOT = "[data-swap-root]";

let listening = false;
let pending: AbortController | null = null;

interface Options {
  /** False for back and forward, whose history entry already exists. */
  push?: boolean;
  /** Something to finish before the swap, such as a switch's slide. */
  after?: Promise<unknown>;
}

/** swap replaces the page's swap root with the one the page at href has. */
export async function swap(href: string, { push = true, after }: Options = {}): Promise<void> {
  const url = new URL(href, location.href);
  const root = document.querySelector<HTMLElement>(ROOT);
  if (!root || url.origin !== location.origin) {
    location.assign(url.href);
    return;
  }
  pending?.abort();
  const controller = new AbortController();
  pending = controller;
  root.setAttribute("aria-busy", "true");

  let doc: Document;
  try {
    const response = await fetch(url, { signal: controller.signal, headers: { Accept: "text/html" } });
    // A redirect leaves the page that was asked for, such as to sign in.
    if (!response.ok || new URL(response.url).pathname !== url.pathname) throw new Error("not swappable");
    doc = new DOMParser().parseFromString(await response.text(), "text/html");
    await after;
  } catch {
    if (!controller.signal.aborted) location.assign(url.href);
    return;
  }
  const next = doc.querySelector<HTMLElement>(ROOT);
  if (controller.signal.aborted) return;
  if (!next) {
    location.assign(url.href);
    return;
  }

  const hadFocus = root.contains(document.activeElement);
  root.replaceWith(next);
  pending = null;
  document.title = doc.title;
  // Links outside the root that follow the page, such as the footer's
  // language switch, take the new page's address.
  for (const mirror of document.querySelectorAll<HTMLAnchorElement>("a[data-swap-mirror]")) {
    const fresh = doc.querySelector<HTMLAnchorElement>(`a[data-swap-mirror="${mirror.dataset.swapMirror}"]`);
    if (fresh) mirror.href = fresh.getAttribute("href") ?? mirror.href;
  }
  if (push) history.pushState({ swap: true }, "", url.href);
  if (hadFocus) {
    const here = url.pathname + url.search;
    next.querySelector<HTMLElement>(`a[aria-current="page"][href="${CSS.escape(here)}"]`)?.focus({ preventScroll: true });
  }
  // A reader who had scrolled down the old list starts the new one at its top.
  const top = next.getBoundingClientRect().top;
  if (top < 0) window.scrollBy({ top: top - 16 });
  document.dispatchEvent(new CustomEvent("explore:swap"));
}

/** listen swaps on plain clicks on a[data-swap], and on back and forward. */
export function listen(): void {
  if (listening) return;
  listening = true;
  history.replaceState({ ...history.state, swap: true }, "");
  // The page swaps its content itself, so the browser keeps the scroll as it is.
  history.scrollRestoration = "manual";
  document.addEventListener("click", (event) => {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const link = event.target instanceof Element ? event.target.closest<HTMLAnchorElement>("a[data-swap]") : null;
    if (!link) return;
    event.preventDefault();
    link.closest("details")?.removeAttribute("open");
    void swap(link.href);
  });
  addEventListener("popstate", (event) => {
    if (event.state?.swap) void swap(location.href, { push: false });
  });
}

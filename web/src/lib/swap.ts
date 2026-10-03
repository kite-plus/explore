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

  const focused = root.contains(document.activeElement) ? (document.activeElement as HTMLElement) : null;
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
  if (focused) refocus(next, focused, url);
  // A reader who had scrolled down the old list starts the new one at its
  // top, below the header that stays at the top of the window.
  const header = document.querySelector("body > header")?.getBoundingClientRect().bottom ?? 0;
  const top = next.getBoundingClientRect().top - header - 16;
  if (top < 0) window.scrollBy({ top });
  document.dispatchEvent(new CustomEvent("explore:swap"));
}

// Focus returns to the control that had it: the same element by id, else
// the chosen link in the same navigation, else in any. Copies hidden at
// this width cannot take it, and a choice inside a menu, which the new
// content brings closed, hands it to the menu's button.
function refocus(next: HTMLElement, focused: HTMLElement, url: URL): void {
  const chosen = `a[aria-current="page"][href="${CSS.escape(url.pathname + url.search)}"]`;
  const nav = focused.closest("nav[aria-label]")?.getAttribute("aria-label");
  const candidates = [
    ...(focused.id ? next.querySelectorAll<HTMLElement>(`#${CSS.escape(focused.id)}`) : []),
    ...(nav ? next.querySelectorAll<HTMLElement>(`nav[aria-label="${CSS.escape(nav)}"] ${chosen}`) : []),
    ...next.querySelectorAll<HTMLElement>(chosen),
  ];
  for (const candidate of candidates) {
    const target = candidate.closest("details:not([open])")?.querySelector("summary") ?? candidate;
    target.focus({ preventScroll: true });
    if (document.activeElement === target) return;
  }
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
    // A choice in a menu closes it at once, and the focus it had goes back
    // to the menu's button rather than to nowhere.
    const menu = link.closest("details");
    if (menu?.open) {
      const focused = menu.contains(document.activeElement);
      menu.open = false;
      if (focused) menu.querySelector("summary")?.focus();
    }
    void swap(link.href);
  });
  addEventListener("popstate", (event) => {
    if (event.state?.swap) void swap(location.href, { push: false });
  });
}

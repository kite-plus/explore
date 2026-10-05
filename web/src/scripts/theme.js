// Inlined into every page's head, before the first paint. It applies the
// reader's theme and runs the theme buttons: the header's flips between
// light and dark and remembers the choice, the footer's forgets it. Without
// a stored choice the page follows the system setting. The choice stays in
// this browser's localStorage and never reaches the server. See
// docs/design/frontend.md.
(() => {
  const root = document.documentElement;
  const read = () => {
    try {
      const saved = localStorage.getItem("theme");
      return saved === "light" || saved === "dark" ? saved : null;
    } catch {
      return null;
    }
  };
  const store = () => {
    try {
      if (theme) localStorage.setItem("theme", theme);
      else localStorage.removeItem("theme");
    } catch {
      // Storage may be blocked; the choice then holds only on this page.
    }
  };
  const media = typeof matchMedia === "function" ? matchMedia("(prefers-color-scheme: dark)") : null;
  const prefers = (query) => typeof matchMedia === "function" && matchMedia(query).matches;
  let theme = read();
  // The theme on screen: the choice, else the system's.
  const shown = () => theme ?? (prefers("(prefers-color-scheme: dark)") ? "dark" : "light");
  const apply = () => {
    if (theme) root.dataset.theme = theme;
    else delete root.dataset.theme;
    root.dataset.themeMode = theme ?? "auto";
    root.dataset.themeShown = shown();
    // The header button names the theme a click goes to.
    for (const button of document.querySelectorAll("[data-theme-toggle]")) {
      const label = shown() === "dark" ? button.dataset.toLight : button.dataset.toDark;
      button.setAttribute("aria-label", label);
      button.setAttribute("title", label);
    }
  };
  const reload = () => {
    theme = read();
    apply();
  };

  root.dataset.js = "";
  apply();
  document.addEventListener("DOMContentLoaded", apply);
  // Another tab, or a page restored from the back-forward cache, may be
  // behind the latest choice.
  addEventListener("storage", (event) => {
    if (event.key === "theme" || event.key === null) reload();
  });
  addEventListener("pageshow", (event) => {
    if (event.persisted) reload();
  });
  // Following the system, the page and its icon change when the system does.
  media?.addEventListener?.("change", () => {
    if (!theme) apply();
  });

  document.addEventListener("click", (event) => {
    const target = event.target instanceof Element ? event.target : null;
    const toggle = target?.closest("[data-theme-toggle]");
    const auto = target?.closest("[data-theme-auto]");
    const button = toggle ?? auto;
    if (!button) return;
    const before = shown();
    theme = toggle ? (before === "dark" ? "light" : "dark") : null;
    store();
    // Lets the new icon turn in (global.css) without animating the first paint.
    root.dataset.themeSwitched = "";
    if (
      shown() === before ||
      prefers("(prefers-reduced-motion: reduce)") ||
      typeof document.startViewTransition !== "function"
    ) {
      apply();
      return;
    }
    // The new theme spreads from the button as a growing circle.
    const box = button.getBoundingClientRect();
    const x = box.left + box.width / 2;
    const y = box.top + box.height / 2;
    const radius = Math.hypot(Math.max(x, innerWidth - x), Math.max(y, innerHeight - y));
    document.startViewTransition(apply).ready.then(
      () =>
        root.animate(
          { clipPath: [`circle(0 at ${x}px ${y}px)`, `circle(${radius}px at ${x}px ${y}px)`] },
          { duration: 450, easing: "cubic-bezier(0.4, 0, 0.2, 1)", pseudoElement: "::view-transition-new(root)" },
        ),
      // A newer click skipped this transition; its theme is already applied.
      () => {},
    );
  });

  const updateAvatar = (img) => {
    if (img && img.naturalWidth > 1) {
      img.parentElement?.classList.add("has-favicon");
    }
  };
  // A cover that fails to load goes with its frame, so the post reads like
  // one without a cover instead of showing a broken image.
  const dropCover = (img) => {
    if (img && img.complete && img.naturalWidth === 0) img.parentElement?.remove();
  };
  const initImages = () => {
    if (typeof document === "undefined" || !document.querySelectorAll) return;
    for (const img of document.querySelectorAll("[data-blog-favicon]")) {
      if (img.complete) updateAvatar(img);
    }
    for (const img of document.querySelectorAll("[data-entry-cover]")) dropCover(img);
  };
  if (typeof document !== "undefined") {
    document.addEventListener?.("DOMContentLoaded", initImages);
    document.addEventListener?.(
      "load",
      (event) => {
        const target = event.target;
        if (target && target.nodeType === 1 && target.hasAttribute?.("data-blog-favicon")) {
          updateAvatar(target);
        }
      },
      true,
    );
    document.addEventListener?.(
      "error",
      (event) => {
        const target = event.target;
        if (target && target.nodeType === 1 && target.hasAttribute?.("data-entry-cover")) dropCover(target);
      },
      true,
    );
    initImages();
  }
})();

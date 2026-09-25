// Inlined into every page's head, before the first paint. It applies the
// reader's theme and runs the theme toggle; without a stored choice the page
// follows the system setting. The choice stays in this browser's
// localStorage and never reaches the server. See docs/design/frontend.md.
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
  let theme = read();
  const apply = () => {
    if (theme) root.dataset.theme = theme;
    else delete root.dataset.theme;
    const mode = theme ?? "auto";
    root.dataset.themeMode = mode;
    const next = mode === "auto" ? "dark" : mode === "dark" ? "light" : "auto";
    for (const button of document.querySelectorAll("[data-theme-toggle]")) {
      const label = `${button.dataset[mode]} · ${button.dataset.switchTo}${button.dataset[next]}`;
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

  const prefers = (query) => typeof matchMedia === "function" && matchMedia(query).matches;
  // Automatic mode shows the system's theme.
  const shown = () => theme ?? (prefers("(prefers-color-scheme: dark)") ? "dark" : "light");

  document.addEventListener("click", (event) => {
    const button = event.target instanceof Element ? event.target.closest("[data-theme-toggle]") : null;
    if (!button) return;
    const before = shown();
    theme = theme === null ? "dark" : theme === "dark" ? "light" : null;
    try {
      if (theme) localStorage.setItem("theme", theme);
      else localStorage.removeItem("theme");
    } catch {
      // Storage may be blocked; the choice then holds only on this page.
    }
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
    // The new theme spreads from the toggle as a growing circle.
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

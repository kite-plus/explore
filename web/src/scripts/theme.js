// Inlined into every page's head, before the first paint. It applies the
// reader's theme and runs the theme toggle; without a stored choice the page
// follows the system setting. The choice stays in this browser's
// localStorage and never reaches the server. See docs/design/frontend.md.
(() => {
  const root = document.documentElement;
  const system = matchMedia("(prefers-color-scheme: dark)");
  const read = () => {
    try {
      const saved = localStorage.getItem("theme");
      return saved === "light" || saved === "dark" ? saved : null;
    } catch {
      return null;
    }
  };
  let theme = read();
  const shown = () => theme ?? (system.matches ? "dark" : "light");
  const apply = () => {
    if (theme) root.dataset.theme = theme;
    else delete root.dataset.theme;
    for (const button of document.querySelectorAll("[data-theme-toggle]")) {
      button.setAttribute("aria-pressed", String(shown() === "dark"));
    }
  };
  const reload = () => {
    theme = read();
    apply();
  };

  root.dataset.js = "";
  apply();
  document.addEventListener("DOMContentLoaded", apply);
  system.addEventListener("change", apply);
  // Another tab, or a page restored from the back-forward cache, may be
  // behind the latest choice.
  addEventListener("storage", (event) => {
    if (event.key === "theme" || event.key === null) reload();
  });
  addEventListener("pageshow", (event) => {
    if (event.persisted) reload();
  });

  document.addEventListener("click", (event) => {
    if (!(event.target instanceof Element) || !event.target.closest("[data-theme-toggle]")) return;
    const next = shown() === "dark" ? "light" : "dark";
    // Picking what the system shows anyway means following the system again.
    theme = next === (system.matches ? "dark" : "light") ? null : next;
    try {
      if (theme) localStorage.setItem("theme", theme);
      else localStorage.removeItem("theme");
    } catch {
      // Storage may be blocked; the choice then holds only on this page.
    }
    apply();
  });
})();

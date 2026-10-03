(() => {
  // Sets up for the list on the page. Swapping the page's content in place
  // (lib/swap.ts) brings a new list, so it sets up again for that one.
  let teardown = () => {};
  document.addEventListener("explore:swap", () => setup());
  setup();

  function setup() {
    teardown();
    teardown = () => {};
    const stream = document.querySelector("[data-entry-stream]");
    const pagination = document.querySelector("[data-entry-pagination]");
    if (!stream || !pagination) return;

    const MAX_AUTO_PAGES = 2;
    let autoPagesLoaded = 0;
    let loading = false;
    let observer = null;

    // Appended pages leave the address bar alone: a cursor is not for people
    // to read, and a reload should start from the newest posts.
    async function loadNextPage() {
      const button = pagination.querySelector("[data-load-more-btn]");
      if (!button || loading) return;
      const nextHref = button.getAttribute("href");
      if (!nextHref) return;

      loading = true;
      button.disabled = true;
      button.setAttribute("aria-busy", "true");
      const textEl = button.querySelector("[data-load-more-text]");
      const originalText = button.dataset.labelOlder || (textEl ? textEl.textContent : "");
      if (textEl) textEl.textContent = button.dataset.labelLoading || "…";

      try {
        const res = await fetch(nextHref, {
          headers: { "X-Requested-With": "Explore-Stream" },
        });
        if (!res.ok) throw new Error("stream fetch failed");
        const html = await res.text();
        const doc = new DOMParser().parseFromString(html, "text/html");
        const newStream = doc.querySelector("[data-entry-stream]");
        if (!newStream) throw new Error("missing entry stream in response");

        // A page that starts on the day the last one ended with repeats that
        // day's heading; the group simply goes on.
        const headings = stream.querySelectorAll("[data-day]");
        const lastDay = headings.length ? headings[headings.length - 1].getAttribute("data-day") : null;
        for (const child of Array.from(newStream.children)) {
          if (lastDay && child.getAttribute("data-day") === lastDay) continue;
          stream.appendChild(child);
        }

        const newButton = doc.querySelector("[data-load-more-btn]");
        const newerHref = newButton ? newButton.getAttribute("href") : null;

        if (newerHref) {
          button.setAttribute("href", newerHref);
          button.disabled = false;
          button.removeAttribute("aria-busy");
          if (textEl) textEl.textContent = originalText;

          autoPagesLoaded++;
          if (autoPagesLoaded >= MAX_AUTO_PAGES && observer) {
            observer.disconnect();
            observer = null;
          }
        } else {
          if (observer) {
            observer.disconnect();
            observer = null;
          }
          button.classList.add("hidden");
          const endEl = pagination.querySelector("[data-load-more-end]");
          if (endEl) endEl.classList.remove("hidden");
        }
      } catch (_) {
        button.disabled = false;
        button.removeAttribute("aria-busy");
        if (textEl) textEl.textContent = button.dataset.labelRetry || "Retry";
      } finally {
        loading = false;
      }
    }

    const onClick = (event) => {
      const button = event.target instanceof Element && event.target.closest("[data-load-more-btn]");
      if (button) {
        event.preventDefault();
        loadNextPage();
      }
    };
    pagination.addEventListener("click", onClick);

    if ("IntersectionObserver" in window) {
      observer = new IntersectionObserver(
        (entries) => {
          for (const entry of entries) {
            if (entry.isIntersecting && autoPagesLoaded < MAX_AUTO_PAGES && !loading) {
              loadNextPage();
            }
          }
        },
        { rootMargin: "250px" },
      );
      const initialBtn = pagination.querySelector("[data-load-more-btn]");
      if (initialBtn) observer.observe(initialBtn);
    }

    teardown = () => {
      if (observer) observer.disconnect();
      pagination.removeEventListener("click", onClick);
    };
  }
})();

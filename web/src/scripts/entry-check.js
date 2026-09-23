(() => {
  const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

  async function request(button) {
    const response = await fetch(`/api/v1/entries/${button.dataset.entryCheck}/check`, {
      method: "POST",
      headers: { "Accept-Language": button.dataset.lang, "Content-Type": "application/json" },
      cache: "no-store",
    });
    if (!response.ok && response.status !== 202) throw new Error("link check failed");
    return response.json();
  }

  function showResult(button, state) {
    const parent = button.parentElement;
    const result = parent.querySelector("[data-entry-check-result]");
    const time = parent.querySelector("[data-entry-check-time]");
    const status = ["available", "unavailable"].includes(state.link_status) ? state.link_status : "unknown";
    const label = button.dataset[status];
    const description = [label, button.dataset.justChecked, button.dataset.hint].join(" · ");

    button.hidden = true;
    result.classList.remove("hidden");
    result.classList.add("inline-flex", `entry-status-${status}`);
    if (status !== "available") result.classList.add("px-2", "py-0.5");
    result.title = description;
    result.querySelector(`[data-link-icon="${status}"]`).classList.remove("hidden");
    const text = result.querySelector("[data-link-label]");
    text.textContent = status === "available" ? description : label;
    if (status === "available") text.classList.add("sr-only");
    if (status !== "available" && state.link_checked_at) {
      time.classList.remove("hidden");
      time.dateTime = state.link_checked_at;
      time.textContent = button.dataset.justChecked;
    }
  }

  async function check(button) {
    const label = button.querySelector("[data-check-label]");
    button.disabled = true;
    button.setAttribute("aria-busy", "true");
    label.textContent = button.dataset.checking;
    try {
      let state = await request(button);
      for (let attempt = 0; state.checking && attempt < 20; attempt++) {
        await pause(3000);
        const response = await fetch(`/api/v1/entries/${button.dataset.entryCheck}/check`, {
          headers: { "Accept-Language": button.dataset.lang },
          cache: "no-store",
        });
        if (!response.ok && response.status !== 202) throw new Error("link check status failed");
        state = await response.json();
      }
      if (state.checking || !state.link_checked_at) throw new Error("link check still pending");
      showResult(button, state);
    } catch {
      button.disabled = false;
      button.removeAttribute("aria-busy");
      label.textContent = button.dataset.retry;
      button.title = button.dataset.failed;
      button.setAttribute("aria-label", button.dataset.failed);
    }
  }

  document.addEventListener("click", (event) => {
    const button = event.target instanceof Element && event.target.closest("[data-entry-check]");
    if (button && !button.disabled) check(button);
  });
})();

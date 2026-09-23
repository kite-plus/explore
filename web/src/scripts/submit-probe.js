(() => {
  const form = document.querySelector("[data-submit-form]");
  if (!form) return;

  const siteInput = form.querySelector('[name="site_url"]');
  const feedInput = form.querySelector('[name="feed_url"]');
  const mainBtn = form.querySelector("[data-main-btn]");
  const waitHint = form.querySelector("[data-wait-hint]");
  const previewSection = form.querySelector("[data-preview-section]");
  const errorSection = form.querySelector("[data-probe-error]");

  // Preview elements
  const avatarEl = form.querySelector("[data-preview-avatar]");
  const initialEl = form.querySelector("[data-preview-initial]");
  const titleInput = form.querySelector('[name="title"]');
  const descInput = form.querySelector('[name="description"]');
  const previewHost = form.querySelector("[data-preview-host]");
  const previewFeed = form.querySelector("[data-preview-feed]");
  const previewArticles = form.querySelector("[data-preview-articles]");
  const previewLatest = form.querySelector("[data-preview-latest]");

  let hasPreviewed = false;
  let probing = false;

  const avatarColors = [
    "bg-red-600",
    "bg-orange-600",
    "bg-amber-600",
    "bg-yellow-600",
    "bg-lime-600",
    "bg-green-600",
    "bg-emerald-600",
    "bg-teal-600",
    "bg-cyan-600",
    "bg-sky-600",
    "bg-blue-600",
    "bg-indigo-600",
    "bg-violet-600",
    "bg-purple-600",
    "bg-fuchsia-600",
    "bg-pink-600",
    "bg-rose-600",
  ];

  function getAvatarColor(host) {
    let hash = 0;
    for (let i = 0; i < host.length; i++) {
      hash = (hash * 31 + host.charCodeAt(i)) >>> 0;
    }
    return avatarColors[hash % avatarColors.length];
  }

  function clearError() {
    if (!errorSection) return;
    errorSection.innerHTML = "";
    errorSection.classList.add("hidden");
  }

  function showError(html) {
    if (!errorSection) return;
    errorSection.innerHTML = html;
    errorSection.classList.remove("hidden");
    errorSection.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }

  async function probeBlog() {
    const siteUrl = (siteInput?.value || "").trim();
    if (!siteUrl) {
      siteInput?.focus();
      return;
    }

    probing = true;
    clearError();
    if (mainBtn) {
      mainBtn.disabled = true;
      mainBtn.setAttribute("aria-busy", "true");
      mainBtn.textContent = mainBtn.dataset.labelProbing || "…";
    }

    try {
      const feedUrl = (feedInput?.value || "").trim();
      const res = await fetch("/api/v1/submissions/preview", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ site_url: siteUrl, feed_url: feedUrl }),
      });

      const data = await res.json();

      if (!res.ok) {
        if (previewSection) previewSection.classList.add("hidden");
        hasPreviewed = false;

        const code = data.error?.code;
        if (code === "already_listed") {
          const host = (siteUrl.replace(/^https?:\/\//i, "").split("/")[0] || "").toLowerCase();
          showError(`
            <div class="rounded-lg border p-4 text-sm">
              <p>${errorSection.dataset.textListed || "This blog is already listed."}</p>
              ${host ? `<a href="/blogs/${encodeURIComponent(host)}" class="mt-2 inline-block font-medium text-primary underline underline-offset-4">${errorSection.dataset.textViewBlog || "View blog"}</a>` : ""}
            </div>
          `);
        } else if (code === "already_pending") {
          const subId = data.submission_id || data.error?.submission_id;
          showError(`
            <div class="rounded-lg border p-4 text-sm">
              <p>${errorSection.dataset.textPending || "This blog is already awaiting review."}</p>
              ${subId ? `<a href="/submissions/${encodeURIComponent(subId)}" class="mt-2 inline-block font-medium text-primary underline underline-offset-4">${errorSection.dataset.textViewStatus || "View status"}</a>` : ""}
            </div>
          `);
        } else if (code === "check_failed" && data.check_report?.problems?.length) {
          const list = data.check_report.problems
            .map((p) => `<li class="text-xs text-destructive">• ${p.message || p.code}</li>`)
            .join("");
          showError(`
            <div class="rounded-lg border border-destructive/30 p-4 text-sm" role="alert">
              <p class="font-medium text-destructive mb-2">${errorSection.dataset.textFailed || "Check failed:"}</p>
              <ul class="flex flex-col gap-1">${list}</ul>
            </div>
          `);
        } else {
          showError(`
            <p role="alert" class="rounded-lg border border-destructive/30 p-4 text-sm text-destructive">
              ${data.error?.message || errorSection.dataset.textError || "Failed to inspect blog."}
            </p>
          `);
        }
        return;
      }

      // Success: populate preview
      if (titleInput) titleInput.value = data.title || "";
      if (descInput) descInput.value = data.description || "";
      if (feedInput && data.feed_url) feedInput.value = data.feed_url;

      if (previewHost) previewHost.textContent = data.host;
      if (previewFeed) previewFeed.textContent = data.feed_url;
      if (previewArticles) {
        previewArticles.textContent = `${data.items_valid || 0}`;
      }
      if (previewLatest) {
        if (data.latest_entry_title) {
          previewLatest.textContent = data.latest_entry_title;
          previewLatest.parentElement?.classList.remove("hidden");
        } else {
          previewLatest.parentElement?.classList.add("hidden");
        }
      }

      if (avatarEl && initialEl) {
        for (const cls of avatarColors) avatarEl.classList.remove(cls);
        avatarEl.classList.add(getAvatarColor(data.host));
        initialEl.textContent = (data.title || data.host || "?").trim().charAt(0).toUpperCase();
      }

      if (previewSection) {
        previewSection.classList.remove("hidden");
        previewSection.scrollIntoView({ behavior: "smooth", block: "nearest" });
      }

      hasPreviewed = true;
      if (mainBtn) {
        mainBtn.textContent = mainBtn.dataset.labelConfirm || "Confirm & Submit";
      }
      if (waitHint) waitHint.classList.add("hidden");
    } catch (_) {
      showError(`
        <p role="alert" class="rounded-lg border border-destructive/30 p-4 text-sm text-destructive">
          ${errorSection.dataset.textUnavailable || "Service temporarily unavailable. Please try again later."}
        </p>
      `);
    } finally {
      probing = false;
      if (mainBtn) {
        mainBtn.disabled = false;
        mainBtn.removeAttribute("aria-busy");
        if (!hasPreviewed) {
          mainBtn.textContent = mainBtn.dataset.labelProbe || "Fetch blog info";
        }
      }
    }
  }

  form.addEventListener("submit", (e) => {
    if (!hasPreviewed) {
      e.preventDefault();
      probeBlog();
    }
  });

  siteInput?.addEventListener("input", () => {
    if (hasPreviewed) {
      hasPreviewed = false;
      if (previewSection) previewSection.classList.add("hidden");
      if (mainBtn) mainBtn.textContent = mainBtn.dataset.labelProbe || "Fetch blog info";
      if (waitHint) waitHint.classList.remove("hidden");
    }
  });
})();

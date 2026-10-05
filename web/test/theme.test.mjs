import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { describe, test } from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("../src/scripts/theme.js", import.meta.url), "utf8");

function load({ saved, storage = true, systemDark = false, reducedMotion = false, transitions = false, covers = [] } = {}) {
  const store = new Map(saved ? [["theme", saved]] : []);
  const listeners = { document: {}, window: {}, media: [] };
  const on = (bucket) => (type, fn) => {
    (bucket[type] ??= []).push(fn);
  };
  const fire = (bucket, type, event = {}) => {
    for (const fn of bucket[type] ?? []) fn(event);
  };
  const blocked = () => {
    if (!storage) throw new Error("storage is blocked");
  };
  let dark = systemDark;

  class Element {
    constructor(kind = null) {
      this.kind = kind;
      this.attrs = {};
      this.dataset = { toDark: "Switch to dark mode", toLight: "Switch to light mode" };
    }
    closest(selector) {
      return selector === `[data-theme-${this.kind}]` ? this : null;
    }
    setAttribute(name, value) {
      this.attrs[name] = value;
    }
    getBoundingClientRect() {
      return { left: 100, top: 10, width: 32, height: 32 };
    }
  }
  const toggle = new Element("toggle");
  const auto = new Element("auto");
  const animations = [];
  const root = { dataset: {}, animate: (keyframes, options) => animations.push({ keyframes, options }) };
  let started = 0;
  const startViewTransition = (update) => {
    started++;
    update();
    return { ready: Promise.resolve() };
  };

  vm.runInNewContext(source, {
    Element,
    addEventListener: on(listeners.window),
    localStorage: {
      getItem: (key) => (blocked(), store.get(key) ?? null),
      setItem: (key, value) => (blocked(), store.set(key, value)),
      removeItem: (key) => (blocked(), store.delete(key)),
    },
    document: {
      documentElement: root,
      querySelectorAll: (selector) => (selector === "[data-entry-cover]" ? covers : selector === "[data-theme-toggle]" ? [toggle] : []),
      addEventListener: on(listeners.document),
      ...(transitions ? { startViewTransition } : {}),
    },
    matchMedia: (query) => ({
      matches: (query.includes("color-scheme: dark") && dark) || (query.includes("reduced-motion") && reducedMotion),
      addEventListener: (type, fn) => type === "change" && listeners.media.push(fn),
    }),
    innerWidth: 1200,
    innerHeight: 800,
  });

  return {
    root,
    store,
    animations,
    transitions: () => started,
    label: () => toggle.attrs["aria-label"],
    title: () => toggle.attrs.title,
    ready: () => fire(listeners.document, "DOMContentLoaded"),
    failed: (target) => fire(listeners.document, "error", { target }),
    click: () => fire(listeners.document, "click", { target: toggle }),
    followSystem: () => fire(listeners.document, "click", { target: auto }),
    clickElsewhere: () => fire(listeners.document, "click", { target: new Element() }),
    systemTurns: (isDark) => {
      dark = isDark;
      for (const fn of listeners.media) fn({ matches: isDark });
    },
    otherTab: (theme) => {
      if (theme) store.set("theme", theme);
      else store.delete("theme");
      fire(listeners.window, "storage", { key: "theme" });
    },
  };
}

// An entry cover in its frame; the frame records whether it was removed.
function cover({ complete = false, naturalWidth = 0 } = {}) {
  const frame = { removed: false, remove() { this.removed = true; } };
  return { nodeType: 1, complete, naturalWidth, parentElement: frame, hasAttribute: (name) => name === "data-entry-cover" };
}

describe("theme script", () => {
  test("drops a cover that fails to load, with its frame", () => {
    const broken = cover({ complete: true });
    const shown = cover({ complete: true, naturalWidth: 640 });
    const lazy = cover();
    const page = load({ covers: [broken, shown, lazy] });
    page.ready();
    assert.equal(broken.parentElement.removed, true, "a cover that failed before the page was ready");
    assert.equal(shown.parentElement.removed, false);
    assert.equal(lazy.parentElement.removed, false, "a lazy cover that has not loaded yet");
    lazy.complete = true;
    page.failed(lazy);
    assert.equal(lazy.parentElement.removed, true, "a cover that fails later");
  });

  test("follows the system until the reader picks a theme", () => {
    const page = load();
    page.ready();
    assert.equal(page.root.dataset.theme, undefined);
    assert.equal(page.root.dataset.themeMode, "auto");
    assert.equal(page.root.dataset.themeShown, "light");
    assert.equal(page.root.dataset.js, "", "marks the page as scripted so the buttons show");
    assert.equal(page.label(), "Switch to dark mode", "the button names where a click goes");
    assert.equal(page.title(), page.label());

    const night = load({ systemDark: true });
    night.ready();
    assert.equal(night.root.dataset.themeShown, "dark");
    assert.equal(night.label(), "Switch to light mode");
  });

  test("applies a stored choice before the page renders", () => {
    const page = load({ saved: "dark" });
    assert.equal(page.root.dataset.theme, "dark");
    assert.equal(page.root.dataset.themeMode, "dark");
    assert.equal(page.root.dataset.themeShown, "dark");
    page.ready();
    assert.equal(page.label(), "Switch to light mode");
  });

  test("ignores a stored value it does not know", () => {
    const page = load({ saved: "sepia" });
    assert.equal(page.root.dataset.theme, undefined);
    assert.equal(page.root.dataset.themeMode, "auto");
  });

  test("the header button flips between light and dark and remembers it", () => {
    const page = load();
    page.ready();
    page.click();
    assert.equal(page.root.dataset.theme, "dark");
    assert.equal(page.root.dataset.themeMode, "dark");
    assert.equal(page.store.get("theme"), "dark");
    assert.equal(page.label(), "Switch to light mode");
    page.click();
    assert.equal(page.root.dataset.theme, "light");
    assert.equal(page.store.get("theme"), "light");
    assert.equal(page.label(), "Switch to dark mode");
    page.click();
    assert.equal(page.root.dataset.theme, "dark", "never back to automatic on its own");
  });

  test("from a dark system, the first click goes light", () => {
    const page = load({ systemDark: true });
    page.click();
    assert.equal(page.root.dataset.theme, "light");
  });

  test("the footer button forgets the choice and follows the system again", () => {
    const page = load({ saved: "dark" });
    page.ready();
    page.followSystem();
    assert.equal(page.root.dataset.theme, undefined);
    assert.equal(page.root.dataset.themeMode, "auto");
    assert.equal(page.store.has("theme"), false);
    assert.equal(page.label(), "Switch to dark mode");
  });

  test("following the system, the page changes when the system does", () => {
    const page = load();
    page.ready();
    page.systemTurns(true);
    assert.equal(page.root.dataset.themeShown, "dark");
    assert.equal(page.label(), "Switch to light mode");

    const chosen = load({ saved: "light" });
    chosen.systemTurns(true);
    assert.equal(chosen.root.dataset.themeShown, "light", "a choice holds");
  });

  test("clicks elsewhere change nothing", () => {
    const page = load();
    page.clickElsewhere();
    assert.equal(page.root.dataset.theme, undefined);
    assert.equal(page.store.size, 0);
  });

  test("keeps working when storage is blocked", () => {
    const page = load({ storage: false });
    page.click();
    assert.equal(page.root.dataset.theme, "dark");
    page.click();
    assert.equal(page.root.dataset.theme, "light");
    page.followSystem();
    assert.equal(page.root.dataset.theme, undefined);
  });

  test("reveals the new theme from the button when the page changes color", async () => {
    const page = load({ transitions: true });
    page.ready();
    page.click();
    assert.equal(page.transitions(), 1);
    assert.equal(page.root.dataset.theme, "dark", "the transition applies the theme");
    assert.equal(page.root.dataset.themeSwitched, "", "the new icon turns in");
    await Promise.resolve();
    const [{ keyframes, options }] = page.animations;
    assert.equal(options.pseudoElement, "::view-transition-new(root)");
    const radius = Math.hypot(1200 - 116, 800 - 26);
    assert.deepEqual([...keyframes.clipPath], ["circle(0 at 116px 26px)", `circle(${radius}px at 116px 26px)`]);
  });

  test("switches at once when the page keeps its colors or motion is reduced", () => {
    const page = load({ transitions: true, saved: "light" });
    page.followSystem();
    assert.equal(page.root.dataset.theme, undefined);
    assert.equal(page.transitions(), 0, "the system already shows light");

    const still = load({ transitions: true, reducedMotion: true });
    still.click();
    assert.equal(still.root.dataset.theme, "dark");
    assert.equal(still.transitions(), 0);
  });

  test("follows a choice made in another tab", () => {
    const page = load();
    page.otherTab("dark");
    assert.equal(page.root.dataset.theme, "dark");
    assert.equal(page.label(), "Switch to light mode");
    page.otherTab(null);
    assert.equal(page.root.dataset.themeMode, "auto");
  });
});

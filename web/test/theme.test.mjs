// Runs src/scripts/theme.js against a stand-in page: <html>, one toggle
// button, localStorage and the system color scheme.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { describe, test } from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("../src/scripts/theme.js", import.meta.url), "utf8");

function load({ saved, systemDark = false, storage = true } = {}) {
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

  class Element {
    constructor(toggle = false) {
      this.toggle = toggle;
      this.attrs = {};
    }
    closest(selector) {
      return selector === "[data-theme-toggle]" && this.toggle ? this : null;
    }
    setAttribute(name, value) {
      this.attrs[name] = value;
    }
  }
  const button = new Element(true);
  const root = { dataset: {} };
  const media = { matches: systemDark, addEventListener: (_, fn) => listeners.media.push(fn) };

  vm.runInNewContext(source, {
    Element,
    matchMedia: () => media,
    addEventListener: on(listeners.window),
    localStorage: {
      getItem: (key) => (blocked(), store.get(key) ?? null),
      setItem: (key, value) => (blocked(), store.set(key, value)),
      removeItem: (key) => (blocked(), store.delete(key)),
    },
    document: { documentElement: root, querySelectorAll: () => [button], addEventListener: on(listeners.document) },
  });

  return {
    root,
    store,
    pressed: () => button.attrs["aria-pressed"],
    ready: () => fire(listeners.document, "DOMContentLoaded"),
    click: (target = button) => fire(listeners.document, "click", { target }),
    clickElsewhere: () => fire(listeners.document, "click", { target: new Element() }),
    systemTurns: (dark) => {
      media.matches = dark;
      for (const fn of listeners.media) fn();
    },
    otherTab: (theme) => {
      store.set("theme", theme);
      fire(listeners.window, "storage", { key: "theme" });
    },
  };
}

describe("theme script", () => {
  test("follows the system until the reader picks a theme", () => {
    const page = load({ systemDark: true });
    page.ready();
    assert.equal(page.root.dataset.theme, undefined);
    assert.equal(page.root.dataset.js, "", "marks the page as scripted so the toggle shows");
    assert.equal(page.pressed(), "true");
  });

  test("applies a stored choice before the page renders", () => {
    const page = load({ saved: "dark" });
    assert.equal(page.root.dataset.theme, "dark");
  });

  test("ignores a stored value it does not know", () => {
    const page = load({ saved: "sepia" });
    assert.equal(page.root.dataset.theme, undefined);
  });

  test("a click switches theme, and switching back follows the system again", () => {
    const page = load({ systemDark: false });
    page.ready();
    page.click();
    assert.equal(page.root.dataset.theme, "dark");
    assert.equal(page.store.get("theme"), "dark");
    assert.equal(page.pressed(), "true");
    page.click();
    assert.equal(page.root.dataset.theme, undefined);
    assert.equal(page.store.has("theme"), false);
    assert.equal(page.pressed(), "false");
  });

  test("on a dark system the first click picks light", () => {
    const page = load({ systemDark: true });
    page.click();
    assert.equal(page.root.dataset.theme, "light");
    assert.equal(page.store.get("theme"), "light");
    assert.equal(page.pressed(), "false");
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
    assert.equal(page.root.dataset.theme, undefined);
  });

  test("tracks the system while no choice is stored", () => {
    const page = load({ systemDark: false });
    page.ready();
    page.systemTurns(true);
    assert.equal(page.pressed(), "true");
  });

  test("follows a choice made in another tab", () => {
    const page = load();
    page.otherTab("dark");
    assert.equal(page.root.dataset.theme, "dark");
  });
});

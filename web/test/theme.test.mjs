import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { describe, test } from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("../src/scripts/theme.js", import.meta.url), "utf8");

function load({ saved, storage = true } = {}) {
  const store = new Map(saved ? [["theme", saved]] : []);
  const listeners = { document: {}, window: {} };
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
      this.dataset = { auto: "Automatic mode", dark: "Dark mode", light: "Light mode", switchTo: "Switch to " };
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

  vm.runInNewContext(source, {
    Element,
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
    label: () => button.attrs["aria-label"],
    title: () => button.attrs.title,
    ready: () => fire(listeners.document, "DOMContentLoaded"),
    click: (target = button) => fire(listeners.document, "click", { target }),
    clickElsewhere: () => fire(listeners.document, "click", { target: new Element() }),
    otherTab: (theme) => {
      if (theme) store.set("theme", theme);
      else store.delete("theme");
      fire(listeners.window, "storage", { key: "theme" });
    },
  };
}

describe("theme script", () => {
  test("starts in automatic mode until the reader picks a theme", () => {
    const page = load();
    page.ready();
    assert.equal(page.root.dataset.theme, undefined);
    assert.equal(page.root.dataset.themeMode, "auto");
    assert.equal(page.root.dataset.js, "", "marks the page as scripted so the toggle shows");
    assert.equal(page.label(), "Automatic mode · Switch to Dark mode");
    assert.equal(page.title(), page.label());
  });

  test("applies a stored choice before the page renders", () => {
    const page = load({ saved: "dark" });
    assert.equal(page.root.dataset.theme, "dark");
    assert.equal(page.root.dataset.themeMode, "dark");
    page.ready();
    assert.equal(page.label(), "Dark mode · Switch to Light mode");
  });

  test("ignores a stored value it does not know", () => {
    const page = load({ saved: "sepia" });
    assert.equal(page.root.dataset.theme, undefined);
    assert.equal(page.root.dataset.themeMode, "auto");
  });

  test("cycles through automatic, dark, and light modes", () => {
    const page = load();
    page.ready();
    page.click();
    assert.equal(page.root.dataset.theme, "dark");
    assert.equal(page.root.dataset.themeMode, "dark");
    assert.equal(page.store.get("theme"), "dark");
    assert.equal(page.label(), "Dark mode · Switch to Light mode");
    page.click();
    assert.equal(page.root.dataset.theme, "light");
    assert.equal(page.root.dataset.themeMode, "light");
    assert.equal(page.store.get("theme"), "light");
    assert.equal(page.label(), "Light mode · Switch to Automatic mode");
    page.click();
    assert.equal(page.root.dataset.theme, undefined);
    assert.equal(page.root.dataset.themeMode, "auto");
    assert.equal(page.store.has("theme"), false);
    assert.equal(page.label(), "Automatic mode · Switch to Dark mode");
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
    page.click();
    assert.equal(page.root.dataset.theme, undefined);
  });

  test("follows a choice made in another tab", () => {
    const page = load();
    page.otherTab("dark");
    assert.equal(page.root.dataset.theme, "dark");
    assert.equal(page.label(), "Dark mode · Switch to Light mode");
    page.otherTab(null);
    assert.equal(page.root.dataset.themeMode, "auto");
  });
});

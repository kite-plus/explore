import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { describe, test } from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("../src/scripts/submit-probe.js", import.meta.url), "utf8");

// Runs the script on a page in lang, submits its form, has the preview
// answer with body, and returns what the reader is shown and the request.
async function probe(body, lang = "zh-CN") {
  const element = (props = {}) => ({
    value: "",
    innerHTML: "",
    textContent: "",
    dataset: {},
    classList: { add() {}, remove() {}, contains: () => false },
    setAttribute() {},
    removeAttribute() {},
    addEventListener() {},
    scrollIntoView() {},
    focus() {},
    ...props,
  });
  const site = element({ value: "https://blog.example.com/" });
  const errors = element({ dataset: { textFailed: "Did not pass:" } });
  const listeners = {};
  const form = {
    querySelector: (selector) => (selector === '[name="site_url"]' ? site : selector === "[data-probe-error]" ? errors : element()),
    addEventListener: (type, fn) => (listeners[type] = fn),
  };
  const requests = [];
  vm.runInNewContext(source, {
    document: { querySelector: (selector) => (selector === "[data-submit-form]" ? form : null), documentElement: { lang } },
    fetch: async (url, init) => {
      requests.push({ url, init });
      return { ok: false, status: 422, json: async () => body };
    },
    setTimeout,
  });
  listeners.submit({ preventDefault() {} });
  await new Promise((resolve) => setImmediate(resolve));
  return { shown: errors.innerHTML, request: requests[0] };
}

const failed = (problem) => ({ error: { code: "check_failed" }, check_report: { passed: false, problems: [problem] } });

describe("submit form check", () => {
  test("a failed check shows each problem's hint, escaped, with its code", async () => {
    const { shown, request } = await probe(failed({ code: "feed_not_found", severity: "error", hint: "No feed <found>", detail: "a&b" }));
    assert.equal(request.url, "/api/v1/submissions/preview");
    assert.match(shown, /• No feed &lt;found&gt;/, "the hint, not the bare code");
    assert.match(shown, /<code>feed_not_found<\/code> · a&amp;b/);
  });

  test("the hints come in the page's language", async () => {
    assert.equal((await probe(failed({ code: "feed_not_found", severity: "error" }), "zh-CN")).request.init.headers["Accept-Language"], "zh-CN");
    const english = await probe(failed({ code: "feed_not_found", severity: "error" }), "en");
    assert.equal(english.request.init.headers["Accept-Language"], "en");
    assert.match(english.shown, /• feed_not_found/, "a problem without a hint falls back to its code");
  });
});

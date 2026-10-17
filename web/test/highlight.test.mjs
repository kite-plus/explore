import assert from "node:assert/strict";
import { describe, test } from "node:test";

import { highlight, searchWords } from "../src/lib/highlight.ts";

const marked = (text, words) =>
  highlight(text, words)
    .map((p) => (p.mark ? `[${p.text}]` : p.text))
    .join("");

describe("search highlights", () => {
  test("splits a query as the API does", () => {
    assert.deepEqual(searchWords("  摄影  Rust "), ["摄影", "rust"]);
    assert.deepEqual(searchWords("你好，世界 node.js"), ["你好", "世界", "node.js"]);
    assert.deepEqual(searchWords("!!!"), []);
    assert.equal(searchWords("a b c d e f g h i j").length, 8);
  });

  test("marks Chinese as typed, anywhere", () => {
    assert.equal(marked("胶片摄影入门：摄影后期", ["摄影"]), "胶片[摄影]入门：[摄影]后期");
    assert.equal(marked("我用Rust写了一个博客", ["用rust写"]), "我[用Rust写]了一个博客");
  });

  test("marks other words whole, in any case", () => {
    assert.equal(marked("AI said: use ai, not email", ["ai"]), "[AI] said: use [ai], not email");
    assert.equal(marked("Rust's rustc", ["rust"]), "[Rust]'s rustc");
    // Next to Chinese a word still ends, as the API splits there.
    assert.equal(marked("用Rust写", ["rust"]), "用[Rust]写");
  });

  test("prefers the longer word and keeps the text whole", () => {
    assert.equal(marked("独立博客的博客", ["博客", "独立博客"]), "[独立博客]的[博客]");
    assert.equal(marked("a (b) c", ["(b)"]), "a [(b)] c");
    assert.equal(marked("nothing here", []), "nothing here");
    assert.equal(marked("", ["x"]), "");
  });
});

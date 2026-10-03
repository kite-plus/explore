import assert from "node:assert/strict";
import { describe, test } from "node:test";

import { destination, openedByExplore } from "../src/scripts/go.js";

describe("the transition page", () => {
  test("reads where it leads from its fragment", () => {
    const hash = "#" + new URLSearchParams({ to: "https://a.example/p/1?utm_source=explore.kite.plus", site: "A.Example", blog: " A blog ", title: "A post" });
    assert.deepEqual(destination(hash), {
      url: "https://a.example/p/1?utm_source=explore.kite.plus",
      host: "a.example",
      site: "a.example",
      blog: "A blog",
      title: "A post",
    });
  });

  test("opens only http and https addresses", () => {
    for (const to of ["javascript:alert(1)", "data:text/html,hi", "file:///etc/passwd", "//a.example/", "", "not a url"]) {
      assert.equal(destination("#" + new URLSearchParams({ to })), null, to);
    }
    assert.equal(destination(""), null);
  });

  test("drops a blog host that is not one", () => {
    const hash = "#" + new URLSearchParams({ to: "https://a.example/", site: "a.example/../api" });
    assert.equal(destination(hash).site, "");
  });

  test("goes on by itself only for a page Explore opened", () => {
    const at = (origin) => ({ origin });
    assert.equal(openedByExplore({ location: at("https://explore.kite.plus"), opener: { location: at("https://explore.kite.plus") } }), true);
    assert.equal(openedByExplore({ location: at("https://explore.kite.plus"), opener: null }), false);
    assert.equal(openedByExplore({ location: at("https://explore.kite.plus"), opener: { location: at("https://elsewhere.example") } }), false);
    const crossOrigin = {
      get location() {
        throw new Error("blocked");
      },
    };
    assert.equal(openedByExplore({ location: at("https://explore.kite.plus"), opener: crossOrigin }), false);
  });
});

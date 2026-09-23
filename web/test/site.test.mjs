// Tests the built site (pnpm build first) against the stub API, checking
// the rules of docs/design/frontend.md at the HTML level, without a browser.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import net from "node:net";
import { after, before, describe, test } from "node:test";

import { startStub } from "./stub-api.mjs";

const PUBLIC = "https://explore.example.org";
let stub;
let site;
let base;

function freePort() {
  return new Promise((resolve) => {
    const s = net.createServer().listen(0, "127.0.0.1", () => {
      const { port } = s.address();
      s.close(() => resolve(port));
    });
  });
}

before(async () => {
  stub = await startStub();
  const port = await freePort();
  base = `http://127.0.0.1:${port}`;
  site = spawn(process.execPath, ["dist/server/entry.mjs"], {
    env: { ...process.env, HOST: "127.0.0.1", PORT: String(port), EXPLORE_API_URL: stub.url, EXPLORE_PUBLIC_URL: PUBLIC },
    stdio: "ignore",
  });
  for (let i = 0; i < 100; i++) {
    try {
      await fetch(`${base}/robots.txt`);
      return;
    } catch {
      await new Promise((r) => setTimeout(r, 100));
    }
  }
  throw new Error("the site did not start");
});

after(() => {
  site?.kill();
  stub?.server.close();
});

const get = (path, init) => fetch(base + path, { redirect: "manual", ...init });

async function page(path) {
  const res = await get(path);
  return { res, html: await res.text() };
}

const publicPages = ["/", "/en/", "/blogs", "/en/blogs", "/blogs/zh.example.com", "/en/blogs/zh.example.com", "/about", "/en/about", "/bot"];

describe("zero JavaScript, zero third parties", () => {
  for (const path of publicPages) {
    test(path, async () => {
      const { res, html } = await page(path);
      assert.equal(res.status, 200);
      assert.equal(res.headers.get("set-cookie"), null, "no cookies");
      assert.doesNotMatch(html, /<script\b/i, "no scripts");
      assert.doesNotMatch(html, /\sstyle="/i, "no inline style attributes");
      const csp = res.headers.get("content-security-policy") ?? "";
      assert.match(csp, /default-src 'self'/);
      assert.match(csp, /frame-ancestors 'none'/);
      // Resources may only come from the site itself; links to posts are fine.
      for (const [, url] of html.matchAll(/<(?:script|img|iframe|source|link)\b[^>]*\s(?:src|href)="([^"]+)"/gi)) {
        if (/^https?:\/\//.test(url)) {
          assert.ok(url.startsWith(PUBLIC), `external resource ${url} on ${path}`);
        }
      }
    });
  }
});

describe("search engines", () => {
  test("the Chinese home page", async () => {
    const { res, html } = await page("/");
    assert.equal(res.headers.get("cache-control"), "public, max-age=60");
    assert.match(html, /<html lang="zh-CN">/);
    assert.match(html, new RegExp(`<link rel="canonical" href="${PUBLIC}/">`));
    assert.match(html, new RegExp(`hreflang="zh" href="${PUBLIC}/"`));
    assert.match(html, new RegExp(`hreflang="en" href="${PUBLIC}/en/"`));
    assert.match(html, new RegExp(`hreflang="x-default" href="${PUBLIC}/en/"`));
    assert.match(html, /<meta property="og:locale" content="zh_CN">/);
    assert.doesNotMatch(html, /name="robots"/);
  });

  test("the English blog page points back at the Chinese one", async () => {
    const { html } = await page("/en/blogs/zh.example.com");
    assert.match(html, /<html lang="en">/);
    assert.match(html, new RegExp(`<link rel="canonical" href="${PUBLIC}/en/blogs/zh.example.com">`));
    assert.match(html, new RegExp(`hreflang="zh" href="${PUBLIC}/blogs/zh.example.com"`));
    assert.match(html, /<title>中文博客 - Explore<\/title>/);
  });

  for (const path of ["/?cursor=page-two", "/?lang=zh", "/blogs?lang=en", "/submit", `/submissions/11111111-2222-3333-4444-555555555555`]) {
    test(`${path} stays out of the index`, async () => {
      const { html } = await page(path);
      assert.match(html, /<meta name="robots" content="noindex, follow">/);
    });
  }

  test("sitemap lists both languages of every blog", async () => {
    const res = await get("/sitemap.xml");
    const xml = await res.text();
    assert.equal(res.status, 200);
    for (const loc of [`${PUBLIC}/`, `${PUBLIC}/en/`, `${PUBLIC}/blogs/zh.example.com`, `${PUBLIC}/en/blogs/en.example.com`, `${PUBLIC}/bot`]) {
      assert.ok(xml.includes(`<loc>${loc}</loc>`), `sitemap lacks ${loc}`);
    }
    assert.match(xml, /<lastmod>/);
  });

  test("robots.txt names the sitemap", async () => {
    const txt = await (await get("/robots.txt")).text();
    assert.match(txt, new RegExp(`Sitemap: ${PUBLIC}/sitemap.xml`));
    assert.match(txt, /Disallow: \/submit/);
  });
});

describe("content", () => {
  test("entries keep their own language and link to the original", async () => {
    const { html } = await page("/en/");
    assert.match(html, /<article class="[^"]*" lang="zh-CN">/);
    assert.match(html, /<article class="[^"]*" lang="en">/);
    assert.match(html, /href="https:\/\/zh\.example\.com\/posts\/cache\/"/);
    assert.match(html, /缓存可以随时删掉/, "titles are never translated");
  });

  test("the older posts link carries the cursor", async () => {
    const { html } = await page("/?lang=");
    assert.match(html, /href="\/\?cursor=page-two"/);
  });

  test("an undated post says so", async () => {
    const { html } = await page("/blogs/zh.example.com");
    assert.match(html, /日期未知/);
  });
});

describe("status codes", () => {
  for (const [path, status] of [["/blogs/missing.example.com", 404], ["/no/such/page", 404], ["/?cursor=bad", 404], ["/submissions/unknown", 404]]) {
    test(`${path} is ${status}`, async () => {
      const res = await get(path);
      assert.equal(res.status, status);
      assert.equal(res.headers.get("cache-control"), "no-store");
    });
  }

  test("an API outage is a 503 with Retry-After", async () => {
    stub.state.down = true;
    try {
      const res = await get("/");
      assert.equal(res.status, 503);
      assert.equal(res.headers.get("retry-after"), "60");
      assert.match(await res.text(), /暂时无法访问/);
    } finally {
      stub.state.down = false;
    }
  });
});

describe("submissions", () => {
  const submit = (path, site_url, headers = {}) =>
    get(path, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded", Origin: base, ...headers },
      body: new URLSearchParams({ site_url, feed_url: "", note: "hello" }),
    });

  test("a passing check redirects to the status page", async () => {
    const res = await submit("/en/submit", "https://ok.example.com/");
    assert.equal(res.status, 303);
    assert.equal(res.headers.get("location"), "/en/submissions/11111111-2222-3333-4444-555555555555");
    const last = stub.state.submits.at(-1);
    assert.equal(last.lang, "en");
    assert.ok(last.forwardedFor, "the reader's address is passed on for rate limiting");
  });

  test("a failed check shows the problems in the page's language", async () => {
    const res = await submit("/submit", "https://fail.example.com/");
    assert.equal(res.status, 422);
    const html = await res.text();
    assert.match(html, /links_off_domain/);
    assert.match(html, /改 _config\.yml 的 url/);
    assert.match(html, /value="https:\/\/fail\.example\.com\/"/, "the form keeps what was typed");
  });

  test("a listed blog links to its page", async () => {
    const html = await (await submit("/submit", "https://listed.example.com/")).text();
    assert.match(html, /href="\/blogs\/listed\.example\.com"/);
  });

  test("a pending blog redirects to its submission", async () => {
    const res = await submit("/submit", "https://pending.example.com/");
    assert.equal(res.status, 303);
    assert.equal(res.headers.get("location"), "/submissions/11111111-2222-3333-4444-555555555555");
  });

  test("a post from another site is refused", async () => {
    const res = await submit("/submit", "https://ok.example.com/", { Origin: "https://evil.example" });
    assert.equal(res.status, 403);
  });

  test("the status page localizes the stored report", async () => {
    const zh = await (await get("/submissions/11111111-2222-3333-4444-555555555555")).text();
    const en = await (await get("/en/submissions/11111111-2222-3333-4444-555555555555")).text();
    assert.match(zh, /中文提示/);
    assert.match(zh, /等待审核/);
    assert.match(en, /English hint/);
    assert.match(en, /Waiting for review/);
  });
});

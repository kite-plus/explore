// Tests the built site (pnpm build first) against the stub API, checking
// the rules of docs/design/frontend.md at the HTML level, without a browser.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
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

const themeScript = readFileSync(new URL("../src/scripts/theme.js", import.meta.url), "utf8");
const themeHash = `'sha256-${createHash("sha256").update(themeScript).digest("base64")}'`;
const entryCheckScript = readFileSync(new URL("../src/scripts/entry-check.js", import.meta.url), "utf8");
const entryCheckHash = `'sha256-${createHash("sha256").update(entryCheckScript).digest("base64")}'`;
const entryStreamScript = readFileSync(new URL("../src/scripts/entry-stream.js", import.meta.url), "utf8");
const entryStreamHash = `'sha256-${createHash("sha256").update(entryStreamScript).digest("base64")}'`;
const entryPages = new Set(["/", "/en/", "/blogs/zh.example.com", "/en/blogs/zh.example.com"]);

const publicPages = ["/", "/en/", "/blogs", "/en/blogs", "/blogs/zh.example.com", "/en/blogs/zh.example.com", "/about", "/en/about", "/bot"];

describe("only local scripts, zero third parties", () => {
  for (const path of publicPages) {
    test(path, async () => {
      const { res, html } = await page(path);
      assert.equal(res.status, 200);
      assert.equal(res.headers.get("set-cookie"), null, "no cookies");
      const scripts = [...html.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script>/gi)];
      assert.equal(scripts.length, entryPages.has(path) ? 3 : 1);
      assert.equal(scripts[0][1], "", "inline, without src or type");
      assert.equal(scripts[0][2], themeScript, "the theme script as written");
      if (entryPages.has(path)) {
        assert.equal(scripts[1][1], "", "inline, without src or type");
        assert.equal(scripts[1][2], entryCheckScript, "the link check script as written");
        assert.equal(scripts[2][1], "", "inline, without src or type");
        assert.equal(scripts[2][2], entryStreamScript, "the stream script as written");
      }
      assert.doesNotMatch(html, /\sstyle="/i, "no inline style attributes");
      const csp = res.headers.get("content-security-policy") ?? "";
      assert.match(csp, /default-src 'self'/);
      assert.match(csp, /frame-ancestors 'none'/);
      assert.ok(csp.includes(themeHash), "the CSP allows the theme script by its hash");
      if (entryPages.has(path)) {
        assert.ok(csp.includes(entryCheckHash), "the CSP allows the link check script by its hash");
        assert.ok(csp.includes(entryStreamHash), "the CSP allows the stream script by its hash");
      }
      // Resources may only come from the site itself; links to posts are fine.
      for (const [, url] of html.matchAll(/<(?:script|img|iframe|source|link)\b[^>]*\s(?:src|href)="([^"]+)"/gi)) {
        if (/^https?:\/\//.test(url)) {
          assert.ok(url.startsWith(PUBLIC), `external resource ${url} on ${path}`);
        }
      }
      // Links that leave Explore open in a new tab; links within it do not.
      for (const [tag, url] of html.matchAll(/<a\b[^>]*\shref="([^"]+)"[^>]*>/gi)) {
        if (/^https?:\/\//.test(url) && !url.startsWith(PUBLIC)) {
          assert.match(tag, /\starget="_blank"/, `${url} on ${path} opens in this tab`);
          assert.match(tag, /\srel="noopener"/, `${url} on ${path} lacks rel=noopener`);
        } else {
          assert.doesNotMatch(tag, /\starget=/, `${url} on ${path} leaves this tab`);
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

  for (const path of ["/?cursor=page-two", "/?lang=zh", "/?tag=backend", "/blogs?lang=en", "/submit", `/submissions/11111111-2222-3333-4444-555555555555`]) {
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
    assert.match(html, /<a href="https:\/\/zh\.example\.com\/posts\/cache\/" target="_blank" rel="noopener"/);
    assert.match(html, /缓存可以随时删掉/, "titles are never translated");
  });

  test("the older posts link carries the cursor", async () => {
    const { html } = await page("/?lang=");
    assert.match(html, /href="\/\?cursor=page-two"/);
    assert.match(html, /data-entry-stream/);
    assert.match(html, /data-entry-pagination/);
    assert.match(html, /data-load-more-btn/);
  });

  test("links that open a new tab say so, in the page's language", async () => {
    const mark = '<span aria-hidden="true" class="external-mark">↗</span>';
    const zh = (await page("/")).html;
    const en = (await page("/en/")).html;
    assert.ok(zh.includes(`${mark}<span class="sr-only" lang="zh-CN">（在新标签页打开）</span></a>`));
    assert.ok(en.includes(`${mark}<span class="sr-only" lang="en"> (opens in a new tab)</span></a>`));
    assert.match(zh, /会在新标签页打开/, "the stream says so once, up front");
    // The crawler page has both languages; each link follows its own text.
    const bot = (await page("/bot")).html;
    assert.ok(bot.includes(`提交一个 Issue${mark}<span class="sr-only" lang="zh-CN">（在新标签页打开）</span></a>`));
    assert.ok(bot.includes(`open an issue${mark}<span class="sr-only" lang="en"> (opens in a new tab)</span></a>`));
  });

  test("entries show their tags, each a way into the stream", async () => {
    const { html } = await page("/");
    assert.match(html, /<a href="\/\?tag=backend" class="[^"]*">后端<\/a>/);
    assert.match(html, /<a href="\/\?tag=ops" class="[^"]*">运维与云<\/a>/);
    const en = (await page("/en/blogs/zh.example.com")).html;
    assert.match(en, /<a href="\/en\/\?tag=backend" class="[^"]*">Backend<\/a>/);
  });

  test("entry thumbnails use the local image endpoint", async () => {
    const { html } = await page("/");
    assert.match(html, /<img src="\/api\/v1\/entries\/2\/image" alt="" loading="lazy" decoding="async"/);
    const image = await get("/api/v1/entries/2/image");
    assert.equal(image.status, 200);
    assert.equal(image.headers.get("content-type"), "image/png");
    assert.equal(image.headers.get("cache-control"), "public, max-age=60");
    assert.ok((await image.arrayBuffer()).byteLength > 0);
  });

  test("the tag and language filters keep each other", async () => {
    const tagged = (await page("/?tag=backend")).html;
    assert.match(tagged, /缓存可以随时删掉/);
    assert.doesNotMatch(tagged, /Notes on feeds/, "only entries with the tag");
    assert.match(tagged, /<a href="\/\?lang=zh&amp;tag=backend"[^>]*>中文<\/a>/);
    assert.match(tagged, /<a href="\/\?tag=backend" aria-current="page"[^>]*>后端<\/a>/);
    const zh = (await page("/?lang=zh")).html;
    assert.match(zh, /<a href="\/\?lang=zh&amp;tag=life"[^>]*>生活随笔<\/a>/);
  });

  test("the tag list is asked for once, not on every page", async () => {
    await page("/");
    const before = stub.state.tagLists;
    await page("/en/");
    await page("/blogs/zh.example.com");
    assert.equal(stub.state.tagLists, before);
  });

  test("the theme toggle speaks the page's language", async () => {
    const zh = (await page("/")).html;
    const en = (await page("/en/")).html;
    assert.match(zh, /<button type="button" data-theme-toggle data-auto="自动模式" data-dark="深色模式" data-light="浅色模式" data-switch-to="点击切换至" aria-label="自动模式 · 点击切换至深色模式"/);
    assert.match(en, /<button type="button" data-theme-toggle data-auto="Automatic mode" data-dark="Dark mode" data-light="Light mode" data-switch-to="Switch to " aria-label="Automatic mode · Switch to Dark mode"/);
    for (const mode of ["auto", "dark", "light"]) {
      assert.match(zh, new RegExp(`data-theme-icon="${mode}"`));
    }
  });

  test("article links show their checked status in both languages", async () => {
    const zh = (await page("/")).html;
    const en = (await page("/en/")).html;
    assert.match(zh, /entry-status-available/);
    assert.match(zh, /class="sr-only">最近检测正常/);
    assert.match(zh, /疑似失效/);
    assert.match(en, /class="sr-only">Last check succeeded/);
    assert.match(en, /Possibly unavailable/);
    const pending = (await page("/?cursor=page-two")).html;
    assert.match(pending, /待检测/);
    assert.match(pending, /<button type="button"[^>]*data-entry-check="3"/);
    assert.match((await page("/en/?cursor=page-two")).html, /data-checking="Checking…"/);
  });

  test("a pending article can trigger its check through the site", async () => {
    const before = stub.state.linkChecks;
    const response = await get("/api/v1/entries/3/check", { method: "POST", headers: { "Content-Type": "application/json", Origin: base } });
    assert.equal(response.status, 200);
    assert.equal(response.headers.get("cache-control"), "no-store");
    const state = await response.json();
    assert.equal(state.link_status, "available");
    assert.ok(state.link_checked_at);
    assert.equal(stub.state.linkChecks, before + 1);
    const current = await get("/api/v1/entries/3/check");
    assert.equal(current.status, 200);
    assert.equal((await current.json()).link_status, "available");
  });

  test("an undated post says so", async () => {
    const { html } = await page("/blogs/zh.example.com");
    assert.match(html, /日期未知/);
  });

  test("the directory and blog page show the source description", async () => {
    assert.match((await page("/blogs")).html, /记录代码与生活中的新发现。/);
    assert.match((await page("/blogs/zh.example.com")).html, /记录代码与生活中的新发现。/);
  });
});

describe("status codes", () => {
  test("blog avatars use the site's favicon endpoint", async () => {
    const { html } = await page("/blogs");
    assert.match(html, /src="\/api\/v1\/blogs\/zh\.example\.com\/favicon"/);
    assert.match(html, /src="\/api\/v1\/blogs\/en\.example\.com\/favicon"/);
    const res = await get("/api/v1/blogs/zh.example.com/favicon");
    assert.equal(res.status, 200);
    assert.equal(res.headers.get("content-type"), "image/png");
    assert.equal(res.headers.get("cache-control"), "public, max-age=3600");
  });

  for (const [path, status] of [["/blogs/missing.example.com", 404], ["/no/such/page", 404], ["/?cursor=bad", 404], ["/?tag=nonsense", 404], ["/submissions/unknown", 404]]) {
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

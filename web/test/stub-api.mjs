// A stand-in for the Gin API with canned responses, so the built site can be
// tested without a database. Shapes follow docs/design/api.md.
import http from "node:http";

const now = Date.now();
const iso = (hoursAgo) => new Date(now - hoursAgo * 3600_000).toISOString();

const zhBlog = { host: "zh.example.com", name: "中文博客", site_url: "https://zh.example.com/", language: "zh-CN" };
const enBlog = { host: "en.example.com", name: "English Blog", site_url: "https://en.example.com/", language: "en" };

const entries = {
  first: {
    data: [
      { id: "2", title: "缓存可以随时删掉", url: "https://zh.example.com/posts/cache/", excerpt: "第一段摘要。", published_at: iso(1), blog: zhBlog },
      { id: "1", title: "Notes on feeds", url: "https://en.example.com/feeds/", excerpt: null, published_at: iso(30), blog: enBlog },
    ],
    next_cursor: "page-two",
  },
  second: {
    data: [{ id: "0", title: "Older post", url: "https://en.example.com/older/", excerpt: "Old.", published_at: iso(200), blog: enBlog }],
    next_cursor: null,
  },
};

const blogs = [
  { ...zhBlog, feed_url: "https://zh.example.com/rss.xml", generator: "halo", last_published_at: iso(1) },
  { ...enBlog, feed_url: "https://en.example.com/atom.xml", generator: "hexo", last_published_at: null },
];

const report = (passed, problems) => ({ input_url: "https://x.example/", feed_url: "https://x.example/feed", generator: "hexo", problems, passed });

const submission = {
  id: "11111111-2222-3333-4444-555555555555",
  status: "pending",
  host: "ok.example.com",
  site_url: "https://ok.example.com/",
  feed_url: "https://ok.example.com/atom.xml",
  check_report: report(true, [{ code: "no_conditional_get", severity: "info", hint: "HINT_FOR_LANG" }]),
  created_at: iso(0),
  reviewed_at: null,
};

export function startStub() {
  const state = { down: false, submits: [] };
  const server = http.createServer(async (req, res) => {
    const url = new URL(req.url, "http://stub");
    const lang = req.headers["accept-language"]?.startsWith("zh") ? "zh" : "en";
    const send = (status, body) => {
      res.writeHead(status, { "Content-Type": "application/json" });
      res.end(JSON.stringify(body));
    };
    const error = (status, code, extra = {}) => send(status, { error: { code, message: code }, ...extra });

    if (state.down) return error(500, "internal");

    if (req.method === "GET" && url.pathname === "/api/v1/entries") {
      const cursor = url.searchParams.get("cursor");
      if (cursor === "bad") return error(400, "invalid_cursor");
      if (url.searchParams.get("lang") === "zh") return send(200, { data: [entries.first.data[0]], next_cursor: null });
      return send(200, cursor === "page-two" ? entries.second : entries.first);
    }
    if (req.method === "GET" && url.pathname === "/api/v1/blogs") {
      return send(200, { data: blogs, next_cursor: null });
    }
    if (req.method === "GET" && url.pathname.startsWith("/api/v1/blogs/")) {
      const host = decodeURIComponent(url.pathname.slice("/api/v1/blogs/".length));
      const b = blogs.find((x) => x.host === host);
      if (!b) return error(404, "not_found");
      return send(200, { blog: b, entries: [{ ...entries.first.data[0], blog: undefined }, { id: "9", title: "无日期", url: "https://zh.example.com/undated/", excerpt: null, published_at: null }] });
    }
    if (req.method === "GET" && url.pathname === `/api/v1/submissions/${submission.id}`) {
      const localized = structuredClone(submission);
      localized.check_report.problems[0].hint = lang === "zh" ? "中文提示" : "English hint";
      return send(200, localized);
    }
    if (req.method === "GET" && url.pathname.startsWith("/api/v1/submissions/")) {
      return error(404, "not_found");
    }
    if (req.method === "POST" && url.pathname === "/api/v1/submissions") {
      let raw = "";
      for await (const chunk of req) raw += chunk;
      const body = JSON.parse(raw);
      state.submits.push({ body, forwardedFor: req.headers["x-forwarded-for"], lang });
      switch (body.site_url) {
        case "https://ok.example.com/":
          return send(201, submission);
        case "https://fail.example.com/":
          return error(422, "check_failed", {
            check_report: report(false, [{ code: "links_off_domain", severity: "error", count: 3, detail: "example.com", hint: lang === "zh" ? "改 _config.yml 的 url" : "Set url in _config.yml" }]),
          });
        case "https://listed.example.com/":
          return error(409, "already_listed");
        case "https://pending.example.com/":
          return error(409, "already_pending", { submission_id: submission.id });
        default:
          return error(400, "invalid_url");
      }
    }
    return error(404, "not_found");
  });
  return new Promise((resolve) => {
    server.listen(0, "127.0.0.1", () => resolve({ server, state, url: `http://127.0.0.1:${server.address().port}` }));
  });
}

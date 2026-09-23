package check

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kite-plus/explore/internal/fetch"
	"github.com/kite-plus/explore/internal/i18n"
	"github.com/kite-plus/explore/internal/model"
)

var now = time.Date(2026, time.September, 23, 12, 0, 0, 0, time.UTC)

// site is a fake blog. Routes map a path to a response body; fixture
// bodies have their example host replaced with the server's own address.
type site struct {
	t      *testing.T
	srv    *httptest.Server
	routes map[string]route
}

type route struct {
	status      int
	body        string
	contentType string
	location    string
}

func newSite(t *testing.T) *site {
	s := &site{t: t, routes: map[string]route{}}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rt, ok := s.routes[r.URL.RequestURI()]
		if !ok {
			rt, ok = s.routes[r.URL.Path]
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		if rt.location != "" {
			http.Redirect(w, r, rt.location, rt.status)
			return
		}
		if rt.contentType != "" {
			w.Header().Set("Content-Type", rt.contentType)
		}
		if rt.status != 0 {
			w.WriteHeader(rt.status)
		}
		_, _ = w.Write([]byte(rt.body))
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *site) serve(path, body string) { s.routes[path] = route{body: body} }

func (s *site) fixture(path, file, exampleOrigin string) {
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "feeds", file))
	if err != nil {
		s.t.Fatal(err)
	}
	body := string(b)
	if exampleOrigin != "" {
		body = strings.ReplaceAll(body, exampleOrigin, s.srv.URL)
	}
	s.serve(path, body)
}

func (s *site) page(generator string, feedLinks ...string) string {
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><title>Blog</title>")
	if generator != "" {
		b.WriteString(`<meta name="generator" content="` + generator + `">`)
	}
	for _, l := range feedLinks {
		b.WriteString(`<link rel="alternate" type="application/atom+xml" title="Feed" href="` + l + `">`)
	}
	b.WriteString("</head><body><h1>Blog</h1></body></html>")
	return b.String()
}

func (s *site) check(in Input) *model.CheckReport {
	s.t.Helper()
	c := &Checker{
		Fetch: fetch.New(fetch.Options{UserAgent: "test", AllowPrivate: true, Timeout: 2 * time.Second}),
		Now:   func() time.Time { return now },
	}
	r, err := c.Run(context.Background(), in)
	if err != nil {
		s.t.Fatal(err)
	}
	return r
}

func codes(r *model.CheckReport) map[model.ProblemCode]model.Problem {
	m := make(map[model.ProblemCode]model.Problem)
	for _, p := range r.Problems {
		m[p.Code] = p
	}
	return m
}

func TestHexoFoundByAutodiscovery(t *testing.T) {
	s := newSite(t)
	s.serve("/", s.page("Hexo 8.1.2", "/atom.xml"))
	s.fixture("/atom.xml", "hexo/atom.xml", "https://hexo.example.com")

	r := s.check(Input{SiteURL: s.srv.URL})
	if !r.Passed {
		t.Fatalf("report failed: %+v", r.Problems)
	}
	if r.DiscoveredBy != "autodiscovery" || r.Format != "atom" || r.Generator != model.GeneratorHexo {
		t.Errorf("report = %+v", r)
	}
	if r.FeedURL != s.srv.URL+"/atom.xml" {
		t.Errorf("feed url = %s", r.FeedURL)
	}
	if r.Items.Valid != 2 || r.Items.TrustedDates != 2 || r.Items.LatestPublishedAt == nil {
		t.Errorf("items = %+v", r.Items)
	}
	if _, ok := codes(r)[model.ProblemNoConditionalGet]; !ok {
		t.Errorf("want no_conditional_get info, got %+v", r.Problems)
	}
}

func TestHaloFoundByDefaultPath(t *testing.T) {
	// Halo's default theme advertises no feed; /rss.xml is found by trying.
	s := newSite(t)
	s.serve("/", s.page("Halo 2.26.1"))
	s.fixture("/rss.xml", "halo/rss.xml", "http://localhost:8090")

	r := s.check(Input{SiteURL: s.srv.URL + "/"})
	if !r.Passed || r.DiscoveredBy != "candidate" || r.Generator != model.GeneratorHalo {
		t.Fatalf("report = %+v", r)
	}
	if r.FeedURL != s.srv.URL+"/rss.xml" {
		t.Errorf("feed url = %s", r.FeedURL)
	}
}

func TestHaloExternalURLUnset(t *testing.T) {
	// Served from the server, but every link still says localhost:8090.
	s := newSite(t)
	s.serve("/", s.page("Halo 2.26.1"))
	s.fixture("/rss.xml", "halo/rss.xml", "")

	r := s.check(Input{SiteURL: s.srv.URL})
	p, ok := codes(r)[model.ProblemLinksOffDomain]
	if r.Passed || !ok || p.Count != 1 {
		t.Fatalf("want links_off_domain, got %+v", r.Problems)
	}
	Localize(r, i18n.Chinese)
	if !strings.Contains(codes(r)[model.ProblemLinksOffDomain].Hint, "halo.external-url") {
		t.Errorf("hint = %q", codes(r)[model.ProblemLinksOffDomain].Hint)
	}
}

func TestGivenFeedURL(t *testing.T) {
	s := newSite(t)
	s.fixture("/feed/", "wordpress/feed.xml", "https://wp.example.com")

	r := s.check(Input{SiteURL: s.srv.URL, FeedURL: s.srv.URL + "/feed/"})
	if !r.Passed || r.DiscoveredBy != "given" || r.Generator != model.GeneratorWordPress {
		t.Fatalf("report = %+v", r)
	}
}

func TestGivenFeedURLThatIsNotAFeed(t *testing.T) {
	s := newSite(t)
	s.serve("/feed/", "<html><body>Not a feed</body></html>")

	r := s.check(Input{SiteURL: s.srv.URL, FeedURL: s.srv.URL + "/feed/"})
	if _, ok := codes(r)[model.ProblemParseError]; r.Passed || !ok {
		t.Fatalf("want parse_error, got %+v", r.Problems)
	}
}

func TestHexoWithoutFeedPlugin(t *testing.T) {
	// The landscape theme advertises /atom.xml even without the plugin.
	s := newSite(t)
	s.serve("/", s.page("Hexo 8.1.2", "/atom.xml"))

	r := s.check(Input{SiteURL: s.srv.URL})
	if _, ok := codes(r)[model.ProblemFeedNotFound]; r.Passed || !ok {
		t.Fatalf("want feed_not_found, got %+v", r.Problems)
	}
	if r.Generator != model.GeneratorHexo {
		t.Fatalf("generator = %s, want hexo from the meta tag", r.Generator)
	}
	Localize(r, i18n.English)
	if !strings.Contains(r.Problems[0].Hint, "hexo-generator-feed") {
		t.Errorf("hint = %q", r.Problems[0].Hint)
	}
}

func TestHexoDefaultURL(t *testing.T) {
	s := newSite(t)
	s.serve("/", s.page("Hexo 8.1.2", "/atom.xml"))
	s.fixture("/atom.xml", "hexo/atom-default-url.xml", "")

	r := s.check(Input{SiteURL: s.srv.URL})
	m := codes(r)
	if p, ok := m[model.ProblemLinksOffDomain]; r.Passed || !ok || p.Detail != "example.com" {
		t.Fatalf("want links_off_domain to example.com, got %+v", r.Problems)
	}
	if _, ok := m[model.ProblemNoValidItems]; !ok {
		t.Errorf("want no_valid_items too, got %+v", r.Problems)
	}
	Localize(r, i18n.Chinese)
	if !strings.Contains(codes(r)[model.ProblemLinksOffDomain].Hint, "_config.yml") {
		t.Errorf("hint = %q", codes(r)[model.ProblemLinksOffDomain].Hint)
	}
}

func TestFeedThatLinksOut(t *testing.T) {
	// Like gohugo.io: the feed names this site as home, but every item is a
	// release page on github.com. Nothing is misconfigured; it is not a blog.
	s := newSite(t)
	var items strings.Builder
	for _, v := range []string{"v0.166.0", "v0.165.0", "v0.164.0"} {
		items.WriteString(`<item><title>Hugo ` + v + `</title><link>https://github.com/gohugoio/hugo/releases/tag/` + v +
			`</link><pubDate>Mon, 21 Sep 2026 10:00:00 +0000</pubDate></item>`)
	}
	s.serve("/index.xml", `<?xml version="1.0"?><rss version="2.0"><channel><title>Hugo</title><link>`+
		s.srv.URL+`/</link><generator>Hugo</generator>`+items.String()+`</channel></rss>`)

	r := s.check(Input{SiteURL: s.srv.URL + "/index.xml"})
	m := codes(r)
	p, ok := m[model.ProblemLinksElsewhere]
	if r.Passed || !ok || p.Detail != "github.com" || p.Count != 3 {
		t.Fatalf("want links_elsewhere to github.com, got %+v", r.Problems)
	}
	if _, ok := m[model.ProblemLinksOffDomain]; ok {
		t.Errorf("a feed that links out on purpose is not misconfigured: %+v", r.Problems)
	}
}

func TestDiscoveredFeedTooLarge(t *testing.T) {
	s := newSite(t)
	s.serve("/", s.page("Hugo 0.157.0", "/index.xml"))
	s.serve("/index.xml", `<?xml version="1.0"?><rss version="2.0"><channel><title>t</title>`+
		strings.Repeat("<item><title>x</title></item>", 200_000)+`</channel></rss>`)

	r := s.check(Input{SiteURL: s.srv.URL})
	m := codes(r)
	if _, ok := m[model.ProblemTooLarge]; !ok || r.Passed {
		t.Fatalf("want too_large, got %+v", r.Problems)
	}
	if _, ok := m[model.ProblemFeedNotFound]; ok {
		t.Errorf("the feed was found, only too large: %+v", r.Problems)
	}
}

func TestSiteMoved(t *testing.T) {
	// The address someone remembers redirects to the blog's new domain, and
	// the feed there links to the new domain: nothing needs fixing but the
	// address.
	moved := newSite(t)
	origin := "http://" + strings.Replace(moved.srv.Listener.Addr().String(), "127.0.0.1", "localhost", 1)
	moved.serve("/", moved.page("Hexo 8.1.2", "/atom.xml"))
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "feeds", "hexo", "atom.xml"))
	if err != nil {
		t.Fatal(err)
	}
	moved.serve("/atom.xml", strings.ReplaceAll(string(b), "https://hexo.example.com", origin))
	old := newSite(t)
	old.routes["/"] = route{status: http.StatusMovedPermanently, location: origin + "/"}

	r := old.check(Input{SiteURL: old.srv.URL})
	m := codes(r)
	p, ok := m[model.ProblemSiteMoved]
	if r.Passed || !ok || p.Detail != origin+"/" {
		t.Fatalf("want site_moved to %s, got %+v", origin, r.Problems)
	}
	if _, ok := m[model.ProblemLinksOffDomain]; ok {
		t.Errorf("the configuration is right; only the address is old: %+v", r.Problems)
	}
}

func TestMetaRefreshToALanguage(t *testing.T) {
	// A multilingual Hugo site: the root only refreshes to /zh/, whose page
	// links the feed.
	s := newSite(t)
	s.serve("/", `<!doctype html><html><head><meta http-equiv="refresh" content="0; url=/zh/"></head></html>`)
	s.serve("/zh/", s.page("Hugo 0.157.0", "/zh/index.xml"))
	s.fixture("/zh/index.xml", "hugo/posts-index.xml", "https://hugo.example.com")

	r := s.check(Input{SiteURL: s.srv.URL})
	if !r.Passed || r.FeedURL != s.srv.URL+"/zh/index.xml" || r.DiscoveredBy != "autodiscovery" || r.Generator != model.GeneratorHugo {
		t.Fatalf("report = %+v", r)
	}
}

func TestWordPressPlainPermalinks(t *testing.T) {
	// Without pretty permalinks /feed/ is missing, and a theme may not link
	// the feed from the page.
	s := newSite(t)
	s.serve("/", s.page("WordPress 7.1.2"))
	s.fixture("/?feed=rss2", "wordpress/feed.xml", "https://wp.example.com")

	r := s.check(Input{SiteURL: s.srv.URL})
	if r.FeedURL != s.srv.URL+"/?feed=rss2" || r.DiscoveredBy != "candidate" || r.Generator != model.GeneratorWordPress {
		t.Fatalf("report = %+v", r)
	}
}

func TestRefreshTarget(t *testing.T) {
	for content, want := range map[string]string{
		"0; url=/zh/":                "/zh/",
		"0;URL='https://a.example/'": "https://a.example/",
		`5; url="/en/"`:              "/en/",
		"0; https://b.example/":      "https://b.example/",
		"300":                        "",
		"0; url=":                    "",
	} {
		if got := refreshTarget(content); got != want {
			t.Errorf("refreshTarget(%q) = %q, want %q", content, got, want)
		}
	}
}

func TestHexoCIDates(t *testing.T) {
	s := newSite(t)
	s.fixture("/atom.xml", "hexo/ci-dates.xml", "https://ci.example.com")

	r := s.check(Input{SiteURL: s.srv.URL + "/atom.xml"})
	p, ok := codes(r)[model.ProblemDatesUntrusted]
	if !ok || p.Count != 7 {
		t.Fatalf("want dates_untrusted for 7 posts, got %+v", r.Problems)
	}
	if !r.Passed {
		t.Errorf("a warning must not fail the check: %+v", r.Problems)
	}
}

func TestHugoHomeFeedWithPages(t *testing.T) {
	s := newSite(t)
	s.serve("/", s.page("Hugo 0.157.0"))
	s.fixture("/index.xml", "hugo/index.xml", "https://hugo.example.com")
	s.fixture("/posts/index.xml", "hugo/posts-index.xml", "https://hugo.example.com")

	r := s.check(Input{SiteURL: s.srv.URL})
	m := codes(r)
	p, ok := m[model.ProblemIncludesNonPosts]
	if !ok || p.Detail != s.srv.URL+"/posts/index.xml" {
		t.Fatalf("want includes_non_posts pointing at /posts/index.xml, got %+v", r.Problems)
	}
	if u, ok := m[model.ProblemNoDates]; !ok || u.Count != 1 {
		t.Errorf("want no_dates for the undated post, got %+v", r.Problems)
	}
	if !r.Passed {
		t.Errorf("warnings must not fail the check: %+v", r.Problems)
	}
}

func TestRobotsDisallowed(t *testing.T) {
	s := newSite(t)
	s.serve("/robots.txt", "User-agent: KiteExplore\nDisallow: /\n")
	s.serve("/", s.page("", "/atom.xml"))

	r := s.check(Input{SiteURL: s.srv.URL})
	if _, ok := codes(r)[model.ProblemRobotsDisallowed]; r.Passed || !ok {
		t.Fatalf("want robots_disallowed, got %+v", r.Problems)
	}
}

func TestStale(t *testing.T) {
	s := newSite(t)
	s.fixture("/atom.xml", "hexo/atom.xml", "https://hexo.example.com")
	c := &Checker{
		Fetch: fetch.New(fetch.Options{UserAgent: "test", AllowPrivate: true}),
		Now:   func() time.Time { return time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC) },
	}
	r, err := c.Run(context.Background(), Input{SiteURL: s.srv.URL + "/atom.xml"})
	if err != nil {
		t.Fatal(err)
	}
	p, ok := codes(r)[model.ProblemStale]
	if r.Passed || !ok || p.Detail != "2026-09-19" {
		t.Fatalf("want stale with the latest date, got %+v", r.Problems)
	}
}

func TestFutureDatesDoNotKeepABlogAlive(t *testing.T) {
	s := newSite(t)
	s.serve("/feed.xml", `<?xml version="1.0"?><rss version="2.0"><channel><title>t</title><link>`+
		`https://x.invalid/</link><item><title>Later</title><link>/later</link><guid>later</guid>`+
		`<pubDate>Fri, 01 Jan 2099 00:00:00 +0000</pubDate></item></channel></rss>`)
	r := s.check(Input{SiteURL: s.srv.URL + "/feed.xml"})
	if _, ok := codes(r)[model.ProblemStale]; r.Passed || !ok {
		t.Fatalf("want stale, got %+v", r.Problems)
	}
}

func TestRedirectedFeed(t *testing.T) {
	s := newSite(t)
	s.routes["/old-feed"] = route{status: http.StatusMovedPermanently, location: "/atom.xml"}
	s.fixture("/atom.xml", "hexo/atom.xml", "https://hexo.example.com")

	r := s.check(Input{SiteURL: s.srv.URL, FeedURL: s.srv.URL + "/old-feed"})
	p, ok := codes(r)[model.ProblemRedirected]
	if !ok || p.Detail != s.srv.URL+"/atom.xml" || r.FeedURL != s.srv.URL+"/atom.xml" {
		t.Fatalf("want redirected to /atom.xml, got feed=%s problems=%+v", r.FeedURL, r.Problems)
	}
}

func TestServerError(t *testing.T) {
	s := newSite(t)
	s.routes["/"] = route{status: http.StatusInternalServerError, body: "oops"}

	r := s.check(Input{SiteURL: s.srv.URL})
	p, ok := codes(r)[model.ProblemHTTPError]
	if r.Passed || !ok || p.Detail != "HTTP 500" {
		t.Fatalf("want http_error HTTP 500, got %+v", r.Problems)
	}
}

func TestPrivateAddressRefused(t *testing.T) {
	c := &Checker{Fetch: fetch.New(fetch.Options{UserAgent: "test"}), Now: func() time.Time { return now }}
	r, err := c.Run(context.Background(), Input{SiteURL: "http://localhost/"})
	if err != nil {
		t.Fatal(err)
	}
	p, ok := codes(r)[model.ProblemHTTPError]
	if r.Passed || !ok || !strings.HasPrefix(p.Detail, "address is not public: ") {
		t.Fatalf("want the private address refused, got %+v", r.Problems)
	}
}

func TestParseURL(t *testing.T) {
	cases := map[string]string{
		"blog.example.com":             "https://blog.example.com/",
		"  https://Blog.Example.com  ": "https://blog.example.com/",
		"http://blog.example.com/a#x":  "http://blog.example.com/a",
	}
	for in, want := range cases {
		u, err := ParseURL(in)
		if err != nil || u.String() != want {
			t.Errorf("ParseURL(%q) = %v, %v; want %s", in, u, err, want)
		}
	}
	for _, bad := range []string{"", "ftp://example.com", "javascript:alert(1)", "https://user:pw@example.com", "https://"} {
		if _, err := ParseURL(bad); err == nil {
			t.Errorf("ParseURL(%q) accepted", bad)
		}
	}
}

func TestEveryProblemHasHintsInBothLanguages(t *testing.T) {
	all := []model.ProblemCode{
		model.ProblemFeedNotFound, model.ProblemRobotsDisallowed, model.ProblemHTTPError,
		model.ProblemTooLarge, model.ProblemParseError, model.ProblemLinksOffDomain, model.ProblemLinksElsewhere,
		model.ProblemNoValidItems, model.ProblemStale, model.ProblemSomeLinksOffDomain,
		model.ProblemNoDates, model.ProblemDatesUntrusted, model.ProblemIncludesNonPosts,
		model.ProblemNoConditionalGet, model.ProblemRedirected,
	}
	for _, code := range all {
		for gen, h := range hints[code] {
			if h.en == "" || h.zh == "" {
				t.Errorf("%s/%s lacks a translation", code, gen)
			}
		}
		if Hint(code, model.GeneratorOther, i18n.English) == "" || Hint(code, model.GeneratorOther, i18n.Chinese) == "" {
			t.Errorf("%s has no fallback hint", code)
		}
	}
}

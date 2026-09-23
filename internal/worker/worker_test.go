package worker

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kite-plus/explore/internal/fetch"
	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/policy"
	"github.com/kite-plus/explore/internal/store"
	"github.com/kite-plus/explore/internal/store/storetest"
)

func TestNextInterval(t *testing.T) {
	cases := []struct {
		prev    time.Duration
		changed bool
		want    time.Duration
	}{
		{0, false, time.Hour},
		{time.Hour, true, time.Hour},
		{time.Hour, false, 90 * time.Minute},
		{5 * time.Hour, false, 6 * time.Hour},
		{6 * time.Hour, false, 6 * time.Hour},
		{6 * time.Hour, true, time.Hour},
	}
	for _, c := range cases {
		if got := nextInterval(c.prev, c.changed); got != c.want {
			t.Errorf("nextInterval(%v, %v) = %v, want %v", c.prev, c.changed, got, c.want)
		}
	}
}

func TestBackoff(t *testing.T) {
	cases := []struct {
		failures   int
		retryAfter time.Duration
		want       time.Duration
	}{
		{1, 0, time.Hour},
		{2, 0, 2 * time.Hour},
		{3, 0, 4 * time.Hour},
		{6, 0, 24 * time.Hour},
		{40, 0, 24 * time.Hour},
		{1, 2 * time.Hour, 2 * time.Hour},
		{1, time.Minute, time.Hour},
		{1, 72 * time.Hour, 24 * time.Hour},
	}
	for _, c := range cases {
		if got := backoff(c.failures, c.retryAfter); got != c.want {
			t.Errorf("backoff(%d, %v) = %v, want %v", c.failures, c.retryAfter, got, c.want)
		}
	}
}

func TestJitter(t *testing.T) {
	if jitter(time.Hour, 0) != 54*time.Minute || jitter(time.Hour, 0.5) != time.Hour {
		t.Error("jitter must span 90% to 110%")
	}
	if got := jitter(time.Hour, 0.999999); got < 65*time.Minute || got > 66*time.Minute {
		t.Errorf("jitter near the top = %v", got)
	}
}

// site is a fake blog whose responses a test can change between fetches.
type site struct {
	t   *testing.T
	srv *httptest.Server
	// host is how the blog is listed: 127.0.0.1 or localhost, so two sites
	// can be listed side by side.
	host string

	mu          sync.Mutex
	routes      map[string]func(w http.ResponseWriter, r *http.Request)
	conditional int
}

func newSite(t *testing.T, host string) *site {
	s := &site{t: t, host: host, routes: map[string]func(http.ResponseWriter, *http.Request){}}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		h, ok := s.routes[r.URL.Path]
		if r.Header.Get("If-None-Match") != "" || r.Header.Get("If-Modified-Since") != "" {
			s.conditional++
		}
		s.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

// base is the server's address under the blog's host name.
func (s *site) base() string {
	u, _ := url.Parse(s.srv.URL)
	return "http://" + s.host + ":" + u.Port()
}

func (s *site) handle(path string, h func(w http.ResponseWriter, r *http.Request)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes[path] = h
}

// feed serves a fixture with its example origin rewritten to this site,
// answering 304 when the client already holds etag.
func (s *site) feed(path, fixture, exampleOrigin, etag string) {
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "feeds", fixture))
	if err != nil {
		s.t.Fatal(err)
	}
	body := strings.ReplaceAll(string(b), exampleOrigin, s.base())
	s.handle(path, func(w http.ResponseWriter, r *http.Request) {
		if etag != "" && r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if etag != "" {
			w.Header().Set("ETag", etag)
		}
		_, _ = io.WriteString(w, body)
	})
}

func (s *site) conditionalRequests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conditional
}

type env struct {
	t *testing.T
	s *store.Store
	w *Worker
}

func newEnv(t *testing.T) *env {
	st := storetest.New(t)
	return &env{t: t, s: st, w: &Worker{
		Store: st,
		Fetch: fetch.New(fetch.Options{UserAgent: "test", AllowPrivate: true, Timeout: 3 * time.Second}),
		Log:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Rand:  func() float64 { return 0.5 },
	}}
}

func (e *env) list(s *site, feedPath string) model.Blog {
	e.t.Helper()
	b, err := e.s.CreateBlog(context.Background(), store.NewBlog{
		Host: s.host, Name: s.host, SiteURL: s.base() + "/", FeedURL: s.base() + feedPath, Language: "en", ShowExcerpt: true,
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return b
}

func (e *env) runOnce() int {
	e.t.Helper()
	n, err := e.w.RunOnce(context.Background())
	if err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *env) due(hosts ...string) {
	e.t.Helper()
	for _, h := range hosts {
		if err := e.s.FetchNow(context.Background(), h); err != nil {
			e.t.Fatal(err)
		}
	}
}

func (e *env) blog(host string) model.Blog {
	e.t.Helper()
	b, err := e.s.Blog(context.Background(), host)
	if err != nil {
		e.t.Fatal(err)
	}
	return b
}

func near(t *testing.T, what string, got time.Time, want time.Time) {
	t.Helper()
	if d := got.Sub(want); d < -5*time.Second || d > 5*time.Second {
		t.Errorf("%s = %v, want about %v", what, got, want)
	}
}

func TestFetchCycle(t *testing.T) {
	e := newEnv(t)
	s := newSite(t, "127.0.0.1")
	s.feed("/atom.xml", "hexo/atom.xml", "https://hexo.example.com", `"v1"`)
	b := e.list(s, "/atom.xml")

	if n := e.runOnce(); n != 1 {
		t.Fatalf("claimed %d, want 1", n)
	}
	_, entries, err := e.s.VisibleBlog(context.Background(), b.Host)
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries = %v, %v", entries, err)
	}
	got := e.blog(b.Host)
	if got.ETag != `"v1"` || got.Generator != model.GeneratorHexo || got.FetchInterval != time.Hour {
		t.Fatalf("after first fetch = %+v", got)
	}
	near(t, "next fetch", got.NextFetchAt, time.Now().Add(time.Hour))

	// Unchanged: the server answers 304 and the interval stretches.
	e.due(b.Host)
	e.runOnce()
	if s.conditionalRequests() != 1 {
		t.Fatalf("conditional requests = %d, want 1", s.conditionalRequests())
	}
	if got := e.blog(b.Host); got.FetchInterval != 90*time.Minute {
		t.Errorf("interval after 304 = %v, want 1h30m", got.FetchInterval)
	}

	// Changed: new body, new ETag, interval back to the base.
	s.feed("/atom.xml", "hexo/rss2-no-guid.xml", "https://hexo.example.com", `"v2"`)
	e.due(b.Host)
	e.runOnce()
	if got := e.blog(b.Host); got.ETag != `"v2"` || got.FetchInterval != time.Hour {
		t.Errorf("after change = %+v", got)
	}
}

type snapshot struct {
	Stream    []string
	Directory []string
	Blogs     map[string][]string
}

// view renders what readers can see, without row ids, which change when the
// cache is rebuilt.
func (e *env) view(hosts ...string) snapshot {
	e.t.Helper()
	ctx := context.Background()
	v := snapshot{Blogs: map[string][]string{}}
	stream, err := e.s.Stream(ctx, store.StreamQuery{Limit: 100})
	if err != nil {
		e.t.Fatal(err)
	}
	for _, se := range stream {
		v.Stream = append(v.Stream, se.Blog.Host+" "+se.Identity+" "+se.Title+" "+se.Excerpt+" "+se.PublishedAt.String())
	}
	dir, err := e.s.Directory(ctx, store.DirectoryQuery{Limit: 100})
	if err != nil {
		e.t.Fatal(err)
	}
	for _, b := range dir {
		last := ""
		if b.LastPublishedAt != nil {
			last = b.LastPublishedAt.String()
		}
		v.Directory = append(v.Directory, b.Host+" "+b.Name+" "+last)
	}
	for _, h := range hosts {
		_, entries, err := e.s.VisibleBlog(ctx, h)
		if err != nil {
			e.t.Fatal(err)
		}
		for _, en := range entries {
			raw, _ := json.Marshal([]any{en.Identity, en.URL, en.Title, en.Excerpt, en.PublishedAt, en.DateTrusted})
			v.Blogs[h] = append(v.Blogs[h], string(raw))
		}
	}
	return v
}

// TestClearingTheCacheLosesNothing is the check from
// docs/design/architecture.md section 0.1.
func TestClearingTheCacheLosesNothing(t *testing.T) {
	e := newEnv(t)
	hexo := newSite(t, "127.0.0.1")
	hexo.feed("/atom.xml", "hexo/atom.xml", "https://hexo.example.com", `"h1"`)
	wp := newSite(t, "localhost")
	wp.feed("/feed/", "wordpress/feed.xml", "https://wp.example.com", `"w1"`)
	e.list(hexo, "/atom.xml")
	e.list(wp, "/feed/")

	if n := e.runOnce(); n != 2 {
		t.Fatalf("claimed %d, want 2", n)
	}
	before := e.view("127.0.0.1", "localhost")
	if len(before.Stream) == 0 || len(before.Blogs["localhost"]) != 2 {
		t.Fatalf("nothing to compare: %+v", before)
	}

	if err := e.s.ClearCache(context.Background()); err != nil {
		t.Fatal(err)
	}
	if empty := e.view(); len(empty.Stream) != 0 {
		t.Fatalf("stream after clearing = %v", empty.Stream)
	}

	// Both servers would answer 304 to the ETags still stored; the worker
	// must not ask, or the cache would stay empty.
	e.due("127.0.0.1", "localhost")
	e.runOnce()
	if hexo.conditionalRequests()+wp.conditionalRequests() != 0 {
		t.Fatal("conditional requests were sent while the cache was empty")
	}

	after := e.view("127.0.0.1", "localhost")
	a, _ := json.Marshal(after)
	b, _ := json.Marshal(before)
	if string(a) != string(b) {
		t.Errorf("rebuilt view differs\nbefore: %s\nafter:  %s", b, a)
	}
}

func TestFailuresBackOff(t *testing.T) {
	e := newEnv(t)
	s := newSite(t, "127.0.0.1")
	s.handle("/feed", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	b := e.list(s, "/feed")

	e.runOnce()
	got := e.blog(b.Host)
	if got.ConsecutiveFailures != 1 || got.LastError != "HTTP 500" || got.GoneSince != nil {
		t.Fatalf("after one failure = %+v", got)
	}
	near(t, "retry", got.NextFetchAt, time.Now().Add(time.Hour))

	e.due(b.Host)
	e.runOnce()
	got = e.blog(b.Host)
	near(t, "second retry", got.NextFetchAt, time.Now().Add(2*time.Hour))
}

func TestRetryAfter(t *testing.T) {
	e := newEnv(t)
	s := newSite(t, "127.0.0.1")
	s.handle("/feed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7200")
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	b := e.list(s, "/feed")
	e.runOnce()
	near(t, "retry", e.blog(b.Host).NextFetchAt, time.Now().Add(2*time.Hour))
}

func TestExitSignals(t *testing.T) {
	e := newEnv(t)
	gone := newSite(t, "127.0.0.1")
	gone.handle("/feed", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusGone) })
	refusing := newSite(t, "localhost")
	refusing.handle("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "User-agent: KiteExplore\nDisallow: /\n")
	})
	e.list(gone, "/feed")
	e.list(refusing, "/feed")

	e.runOnce()
	for _, h := range []string{"127.0.0.1", "localhost"} {
		if b := e.blog(h); b.GoneSince == nil {
			t.Errorf("%s: gone_since not set: %+v", h, b)
		}
	}
	if b := e.blog("localhost"); !strings.Contains(b.LastError, "robots.txt") {
		t.Errorf("robots refusal recorded as %q", b.LastError)
	}
}

func TestPermanentRedirectMovesTheFeed(t *testing.T) {
	e := newEnv(t)
	s := newSite(t, "127.0.0.1")
	s.handle("/old", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/atom.xml", http.StatusMovedPermanently)
	})
	s.feed("/atom.xml", "hexo/atom.xml", "https://hexo.example.com", "")
	b := e.list(s, "/old")

	e.runOnce()
	if got := e.blog(b.Host).FeedURL; got != s.base()+"/atom.xml" {
		t.Errorf("feed url = %s, want the redirect target", got)
	}
}

func TestBrokenFeedKeepsTheEntries(t *testing.T) {
	e := newEnv(t)
	s := newSite(t, "127.0.0.1")
	s.feed("/atom.xml", "hexo/atom.xml", "https://hexo.example.com", "")
	b := e.list(s, "/atom.xml")
	e.runOnce()

	s.handle("/atom.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><entry><title>cut`)
	})
	e.due(b.Host)
	e.runOnce()

	got := e.blog(b.Host)
	if got.ConsecutiveFailures != 1 || !strings.HasPrefix(got.LastError, "parse:") {
		t.Fatalf("broken feed recorded as %+v", got)
	}
	_, entries, err := e.s.VisibleBlog(context.Background(), b.Host)
	if err != nil || len(entries) != 2 {
		t.Errorf("entries after a broken feed = %v, %v; a parse failure must not empty the snapshot", entries, err)
	}
}

func TestMaintenanceRunsDaily(t *testing.T) {
	e := newEnv(t)
	now := time.Now()
	e.w.Now = func() time.Time { return now }
	e.w.maintain(context.Background())
	first := e.w.lastMaintenance
	if first.IsZero() {
		t.Fatal("maintenance did not run")
	}
	now = now.Add(time.Hour)
	e.w.maintain(context.Background())
	if !e.w.lastMaintenance.Equal(first) {
		t.Error("maintenance ran again within a day")
	}
	now = now.Add(policy.GoneRemovalAfter)
	e.w.maintain(context.Background())
	if e.w.lastMaintenance.Equal(first) {
		t.Error("maintenance did not run after a day")
	}
}

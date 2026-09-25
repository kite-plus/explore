package worker

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/store"
)

func TestSitemapsBringInOlderPosts(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newSite(t, "127.0.0.1")

	var mu sync.Mutex
	feedItems := []string{"new"}
	listed := []string{"/posts/new/", "/posts/old-1/", "/posts/old-2/", "/posts/draft/", "/posts/listing/", "/tags/go/", "https://other.example/posts/x/"}
	reads := map[string]int{}

	s.handle("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "User-agent: *\nAllow: /\n\nSitemap: /sitemap_index.xml # the index\n")
	})
	s.handle("/sitemap_index.xml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`+
			`<sitemap><loc>`+s.base()+`/post-sitemap.xml.gz</loc></sitemap>`+
			`<sitemap><loc>`+s.base()+`/page-sitemap.xml</loc></sitemap></sitemapindex>`)
	})
	s.handle("/post-sitemap.xml.gz", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var body strings.Builder
		body.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
		for i, path := range listed {
			loc := path
			if strings.HasPrefix(path, "/") {
				loc = s.base() + path
			}
			fmt.Fprintf(&body, `<url><loc>%s</loc><lastmod>2026-0%d-01</lastmod></url>`, loc, 9-i)
		}
		body.WriteString(`</urlset>`)
		var packed bytes.Buffer
		z := gzip.NewWriter(&packed)
		_, _ = io.WriteString(z, body.String())
		_ = z.Close()
		_, _ = w.Write(packed.Bytes())
	})
	s.handle("/page-sitemap.xml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>`+s.base()+`/about/</loc></url></urlset>`)
	})
	page := func(path, head string) {
		s.handle(path, func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			reads[path]++
			mu.Unlock()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, "<!doctype html><html><head>"+head+"</head><body><h1>Never read</h1></body></html>")
		})
	}
	page("/posts/new/", `<title>New | 127.0.0.1</title><meta property="og:type" content="article">`+
		`<meta property="article:published_time" content="`+time.Now().Add(-2*time.Hour).UTC().Format(time.RFC3339)+`">`+
		`<meta property="og:description" content="Fresh from the feed">`)
	page("/posts/old-1/", `<title>Old one | 127.0.0.1</title><meta property="og:type" content="article">`+
		`<meta property="article:published_time" content="2025-03-01T10:00:00+08:00">`+
		`<meta property="og:image" content="/covers/old-1.png"><meta property="og:description" content="An older post">`)
	// Inline styles push the JSON-LD past the first 256 KB, as some themes do.
	page("/posts/old-2/", `<title>127.0.0.1 - Old two</title><style>`+strings.Repeat("p{}", 100_000)+`</style>`+
		`<script type="application/ld+json">{"@context":"https://schema.org","@graph":[{"@type":"WebPage"},{"@type":"BlogPosting","datePublished":"2024-12-24T08:00:00Z"}]}</script>`)
	page("/posts/draft/", `<title>Draft</title><meta name="robots" content="noindex"><meta property="og:type" content="article">`+
		`<meta property="article:published_time" content="2025-01-01T00:00:00Z">`)
	page("/posts/listing/", `<title>All posts</title><meta property="og:type" content="website">`)
	page("/tags/go/", `<title>Go</title><meta property="og:type" content="website">`)

	s.handle("/feed.xml", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var items strings.Builder
		for _, id := range feedItems {
			title := strings.ToUpper(id[:1]) + id[1:]
			fmt.Fprintf(&items, `<item><guid>%s</guid><link>%s/posts/%s/</link><title>%s</title><description>Fresh from the feed</description><pubDate>%s</pubDate></item>`,
				id, s.base(), id, title, time.Now().Add(-2*time.Hour).Format(time.RFC1123Z))
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = io.WriteString(w, `<rss version="2.0"><channel><title>Blog</title><link>`+s.base()+`/</link>`+items.String()+`</channel></rss>`)
	})
	blog := e.list(s, "/feed.xml")
	e.runOnce()
	if n := e.w.SitemapOnce(ctx); n != 1 {
		t.Fatalf("sitemaps read = %d", n)
	}
	e.readPages()

	got := e.posts(s.host)
	want := []string{"feed New", "sitemap Old one 2025-03-01T02:00:00Z " + s.base() + "/covers/old-1.png An older post", "sitemap Old two 2024-12-24T08:00:00Z"}
	if !slices.Equal(got, want) {
		t.Fatalf("posts =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	mu.Lock()
	if reads["/tags/go/"] != 0 || reads["/about/"] != 0 {
		t.Errorf("pages that are no posts were read: %v", reads)
	}
	mu.Unlock()

	stream, err := e.s.Stream(ctx, store.StreamQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, se := range stream {
		titles = append(titles, se.Title)
	}
	if strings.Join(titles, ",") != "New,Old one,Old two" {
		t.Errorf("latest stream = %v", titles)
	}

	// Clearing the cache loses nothing the sitemap can give back.
	before := e.posts(s.host)
	if err := e.s.ClearCache(ctx); err != nil {
		t.Fatal(err)
	}
	e.due(s.host)
	e.runOnce()
	e.w.SitemapOnce(ctx)
	e.readPages()
	if after := e.posts(s.host); !slices.Equal(after, before) {
		t.Errorf("rebuilt =\n%s\nbefore\n%s", strings.Join(after, "\n"), strings.Join(before, "\n"))
	}

	// A post the sitemap stops listing goes.
	mu.Lock()
	listed = slices.DeleteFunc(listed, func(p string) bool { return p == "/posts/old-2/" })
	mu.Unlock()
	e.sitemapDue(blog.ID)
	e.w.SitemapOnce(ctx)
	if got := e.posts(s.host); len(got) != 2 || strings.Contains(strings.Join(got, "\n"), "Old two") {
		t.Errorf("after the sitemap dropped a post = %v", got)
	}

	// A hidden post leaving the feed stays hidden when the sitemap brings it
	// back, and showing it again works on the sitemap's entry.
	hidden, _, err := e.s.AdminEntries(ctx, "New", 10, 0)
	if err != nil || len(hidden) != 1 {
		t.Fatalf("admin entries = %v, %v", hidden, err)
	}
	if err := e.s.SetEntryHidden(ctx, hidden[0].ID, true, "spam", "maintainer"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	feedItems = []string{"newer"}
	mu.Unlock()
	e.due(s.host)
	e.runOnce()
	e.readPages()
	if got := e.posts(s.host); strings.Contains(strings.Join(got, "\n"), "sitemap New") || len(got) != 2 {
		t.Errorf("the hidden post came back: %v", got)
	}
	matches, _, err := e.s.AdminEntries(ctx, "New", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	var again []store.AdminEntry
	for _, m := range matches {
		if m.Title == "New" {
			again = append(again, m)
		}
	}
	if len(again) != 1 || !again[0].Hidden || !strings.HasPrefix(again[0].Identity, "sitemap:") {
		t.Fatalf("the sitemap's entry = %+v", again)
	}
	if err := e.s.SetEntryHidden(ctx, again[0].ID, false, "", "maintainer"); err != nil {
		t.Fatal(err)
	}
	if got := e.posts(s.host); len(got) != 3 || !slices.Contains(got, "sitemap New") {
		t.Errorf("after showing it again = %v", got)
	}
}

// posts lists a blog's entries as their source, title, date, cover and
// excerpt, the parts the sitemap and pages decide.
func (e *env) posts(host string) []string {
	e.t.Helper()
	_, entries, err := e.s.VisibleBlog(context.Background(), host)
	if err != nil {
		e.t.Fatal(err)
	}
	var out []string
	for _, en := range entries {
		parts := []string{e.source(en), en.Title}
		if en.PublishedAt != nil && !strings.HasPrefix(en.Title, "New") {
			parts = append(parts, en.PublishedAt.UTC().Format(time.RFC3339))
		}
		if en.ImageURL != "" {
			parts = append(parts, en.ImageURL)
		}
		if en.Excerpt != "" && !strings.HasPrefix(en.Title, "New") {
			parts = append(parts, en.Excerpt)
		}
		out = append(out, strings.Join(parts, " "))
	}
	return out
}

func (e *env) source(en model.Entry) string {
	if strings.HasPrefix(en.Identity, "sitemap:") {
		return "sitemap"
	}
	return "feed"
}

func (e *env) sitemapDue(blogID int64) {
	e.t.Helper()
	if err := e.s.PostponeSitemap(context.Background(), blogID, time.Now().Add(-time.Second)); err != nil {
		e.t.Fatal(err)
	}
}

package worker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/kite-plus/explore/internal/store"
)

func TestPageFromHTML(t *testing.T) {
	cases := []struct {
		name, head, image, excerpt string
	}{
		{"open graph", `<meta property="og:image" content="https://cdn.example.com/c.png"><meta property="og:description" content="Cats &amp; dogs">`,
			"https://cdn.example.com/c.png", "Cats & dogs"},
		{"relative image and fallbacks", `<meta name="twitter:image" content="/covers/1.png"><meta name="description" content="Plain">`,
			"https://blog.example.com/covers/1.png", "Plain"},
		{"open graph first", `<meta name="description" content="Meta"><meta property="og:description" content="Open Graph">`,
			"", "Open Graph"},
		{"og under name", `<meta name="og:image" content="c.png">`, "https://blog.example.com/posts/1/c.png", ""},
		{"only http", `<meta property="og:image" content="data:image/png;base64,AAAA">`, "", ""},
		{"noindex", `<meta name="robots" content="noindex"><meta property="og:image" content="/c.png"><meta property="og:description" content="D">`,
			"", ""},
		{"nosnippet for us", `<meta name="KiteExplore" content="nosnippet"><meta property="og:image" content="/c.png"><meta property="og:description" content="D">`,
			"https://blog.example.com/c.png", ""},
		{"noimageindex", `<meta name="robots" content="max-snippet:50, noimageindex"><meta property="og:image" content="/c.png"><meta property="og:description" content="D">`,
			"", "D"},
		{"another crawler's rule", `<meta name="googlebot" content="noindex"><meta property="og:image" content="/c.png">`,
			"https://blog.example.com/c.png", ""},
		{"body is not read", `</head><body><meta property="og:image" content="/late.png">`, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := "<!doctype html><html><head>" + c.head + "</head><body><p>Post</p></body></html>"
			image, excerpt := pageFromHTML([]byte(body), "https://blog.example.com/posts/1/")
			if image != c.image || excerpt != c.excerpt {
				t.Errorf("got (%q, %q), want (%q, %q)", image, excerpt, c.image, c.excerpt)
			}
		})
	}
}

func TestPagesFillWhatFeedsLeaveOut(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newSite(t, "127.0.0.1")

	var mu sync.Mutex
	requests := map[string]int{}
	page := func(path, head string) {
		s.handle(path, func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			requests[path]++
			mu.Unlock()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, "<!doctype html><html><head>"+head+"</head><body><p>Body text Explore never keeps.</p></body></html>")
		})
	}
	page("/posts/cover/", `<meta property="og:image" content="/covers/cover.png"><meta property="og:description" content="The whole opening, as its author wrote it.">`)
	page("/posts/cover-2/", `<meta property="og:image" content="/covers/cover-2.png">`)
	// The theme's defaults, shared by every post: a logo and a tagline.
	page("/posts/shared-1/", `<meta property="og:image" content="/logo.png"><meta property="og:description" content="Stories and notes">`)
	page("/posts/shared-2/", `<meta property="og:image" content="/logo.png"><meta property="og:description" content="Stories and notes">`)
	page("/posts/private/", `<meta name="robots" content="noimageindex, nosnippet"><meta property="og:image" content="/covers/private.png"><meta property="og:description" content="Private">`)
	page("/posts/pictured/", `<meta property="og:image" content="/covers/pictured.png">`)

	coverPath := "/posts/cover/"
	s.handle("/feed.xml", func(w http.ResponseWriter, _ *http.Request) {
		published := time.Now().Add(-48 * time.Hour)
		item := func(id, path, description string) string {
			published = published.Add(time.Hour)
			return fmt.Sprintf(`<item><guid>%s</guid><link>%s%s</link><title>Post %s</title><description>%s</description><pubDate>%s</pubDate></item>`,
				id, s.base(), path, id, description, published.Format(time.RFC1123Z))
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = io.WriteString(w, `<rss version="2.0"><channel><title>Blog</title><link>`+s.base()+`/</link>`+
			item("cover", coverPath, "Opening lines [&amp;#8230;]")+
			item("shared-1", "/posts/shared-1/", "Part one [&amp;#8230;]")+
			item("shared-2", "/posts/shared-2/", "Part two [&amp;#8230;]")+
			item("private", "/posts/private/", "Hidden [&amp;#8230;]")+
			item("gone", "/posts/gone/", "Missing [&amp;#8230;]")+
			item("pictured", "/posts/pictured/", `A full summary. &lt;img src="`+s.base()+`/feed.png"&gt;`)+
			`</channel></rss>`)
	})
	e.list(s, "/feed.xml")
	e.runOnce()
	e.readPages()

	got := e.shown(s.host)
	want := map[string][2]string{
		"cover":    {s.base() + "/covers/cover.png", "The whole opening, as its author wrote it."},
		"shared-1": {"", "Part one […]"},
		"shared-2": {"", "Part two […]"},
		"private":  {"", "Hidden […]"},
		"gone":     {"", "Missing […]"},
		"pictured": {s.base() + "/feed.png", "A full summary."},
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s shows %q, want %q", id, got[id], w)
		}
	}
	mu.Lock()
	if requests["/posts/pictured/"] != 0 {
		t.Error("a page was read although its feed had both an image and a full excerpt")
	}
	mu.Unlock()
	if n := e.w.PageOnce(ctx); n != 0 {
		t.Errorf("%d pages claimed again after they were read", n)
	}

	stream, err := e.s.Stream(ctx, store.StreamQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, se := range stream {
		if se.Identity == "cover" {
			if se.ImageURL != s.base()+"/covers/cover.png" {
				t.Errorf("stream image = %q", se.ImageURL)
			}
			if source, err := e.s.VisibleImageURL(ctx, se.ID); err != nil || source != s.base()+"/covers/cover.png" {
				t.Errorf("image proxy source = %q, %v", source, err)
			}
		}
	}

	// A new address is a new page: what the old one offered is dropped
	// and the new one is read.
	coverPath = "/posts/cover-2/"
	e.due(s.host)
	e.runOnce()
	if cover := e.shown(s.host)["cover"]; cover != [2]string{"", "Opening lines […]"} {
		t.Errorf("after the move, before reading = %q", cover)
	}
	e.readPages()
	if cover := e.shown(s.host)["cover"]; cover != [2]string{s.base() + "/covers/cover-2.png", "Opening lines […]"} {
		t.Errorf("after reading the new page = %q", cover)
	}
}

// readPages reads article pages until none is due.
func (e *env) readPages() {
	e.t.Helper()
	for range 20 {
		if e.w.PageOnce(context.Background()) == 0 {
			return
		}
	}
	e.t.Fatal("article pages kept coming")
}

// shown maps a blog's entries to the image and excerpt readers see.
func (e *env) shown(host string) map[string][2]string {
	e.t.Helper()
	_, entries, err := e.s.VisibleBlog(context.Background(), host)
	if err != nil {
		e.t.Fatal(err)
	}
	out := map[string][2]string{}
	for _, en := range entries {
		out[en.Identity] = [2]string{en.ImageURL, en.Excerpt}
	}
	return out
}

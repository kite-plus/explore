package sitemap

import (
	"bytes"
	"compress/gzip"
	"errors"
	"net/url"
	"testing"
)

func TestParseURLSet(t *testing.T) {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:image="http://www.google.com/schemas/sitemap-image/1.1">
  <url><loc> https://blog.example.com/posts/a/ </loc><lastmod>2026-05-08T16:49:29+08:00</lastmod>
    <image:image><image:loc>https://blog.example.com/a.png</image:loc></image:image></url>
  <url><loc>https://blog.example.com/posts/b/</loc><lastmod>2025-01-02</lastmod></url>
  <url><loc>https://blog.example.com/posts/c/</loc></url>
  <url><lastmod>2025-01-02</lastmod></url>
</urlset>`
	doc, err := Parse([]byte(body), 1<<20, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.URLs) != 3 || len(doc.Children) != 0 {
		t.Fatalf("doc = %+v", doc)
	}
	if doc.URLs[0].Loc != "https://blog.example.com/posts/a/" {
		t.Errorf("loc = %q; an image's loc must not leak into its post's", doc.URLs[0].Loc)
	}
	if got := doc.URLs[0].LastMod; got == nil || got.Format("2006-01-02T15:04:05Z") != "2026-05-08T08:49:29Z" {
		t.Errorf("lastmod = %v", got)
	}
	if got := doc.URLs[1].LastMod; got == nil || got.Format("2006-01-02") != "2025-01-02" {
		t.Errorf("date-only lastmod = %v", got)
	}
	if doc.URLs[2].LastMod != nil {
		t.Errorf("missing lastmod = %v", doc.URLs[2].LastMod)
	}
}

func TestParseIndexGzipAndLimits(t *testing.T) {
	index := `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>https://blog.example.com/post-sitemap.xml</loc></sitemap>
  <sitemap><loc>https://blog.example.com/page-sitemap.xml</loc></sitemap>
</sitemapindex>`
	var packed bytes.Buffer
	z := gzip.NewWriter(&packed)
	_, _ = z.Write([]byte(index))
	_ = z.Close()
	doc, err := Parse(packed.Bytes(), 1<<20, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Children) != 2 || doc.Children[0] != "https://blog.example.com/post-sitemap.xml" || len(doc.URLs) != 0 {
		t.Fatalf("index = %+v", doc)
	}
	if _, err := Parse(packed.Bytes(), 10, 100); !errors.Is(err, ErrTooLarge) {
		t.Errorf("oversized gzip = %v", err)
	}
	if doc, err := Parse([]byte(index), 1<<20, 1); err != nil || len(doc.Children) != 1 {
		t.Errorf("limited = %+v, %v", doc, err)
	}
	if _, err := Parse([]byte(`<!doctype html><html><body>Not found</body></html>`), 1<<20, 100); err == nil {
		t.Error("an HTML page parsed as a sitemap")
	}
	if _, err := Parse([]byte(`<rss version="2.0"><channel></channel></rss>`), 1<<20, 100); !errors.Is(err, ErrNotSitemap) {
		t.Errorf("a feed = %v", err)
	}
}

func TestPatterns(t *testing.T) {
	cases := []struct {
		name     string
		feed     []string
		match    []string
		mismatch []string
	}{
		{"hugo sections", []string{"https://b.example/posts/one/", "https://b.example/posts/two/"},
			[]string{"https://b.example/posts/older-post/"},
			[]string{"https://b.example/tags/go/", "https://b.example/about/", "https://b.example/posts/", "https://b.example/posts/a/b/"}},
		{"dated wordpress", []string{"https://b.example/2026/05/hello/", "https://b.example/2026/04/world/"},
			[]string{"https://b.example/2019/11/older/"},
			[]string{"https://b.example/category/tech/", "https://b.example/2019/11/", "https://b.example/sample-page/"}},
		{"halo ids", []string{"https://b.example/archives/1024", "https://b.example/archives/998"},
			[]string{"https://b.example/archives/17"},
			[]string{"https://b.example/archives/some-tag", "https://b.example/categories/1024"}},
		{"query permalinks", []string{"https://b.example/?p=120", "https://b.example/?p=119"},
			[]string{"https://b.example/?p=3"},
			[]string{"https://b.example/?page_id=2", "https://b.example/hello/"}},
		{"categories in the path", []string{"https://b.example/tech/a.html", "https://b.example/life/b.html"},
			[]string{"https://b.example/travel/c.html"},
			[]string{"https://b.example/travel/c/", "https://b.example/c.html"}},
		{"one example", []string{"https://b.example/p/first.html"},
			[]string{"https://b.example/p/second.html"},
			[]string{"https://b.example/q/second.html"}},
		{"nothing learned", nil, []string{"https://b.example/anything/at/all"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var examples []*url.URL
			for _, raw := range c.feed {
				u, _ := url.Parse(raw)
				examples = append(examples, u)
			}
			p := Learn(examples)
			for _, raw := range c.match {
				u, _ := url.Parse(raw)
				if !p.Match(u) {
					t.Errorf("%s should look like a post", raw)
				}
			}
			for _, raw := range c.mismatch {
				u, _ := url.Parse(raw)
				if p.Match(u) {
					t.Errorf("%s should not look like a post", raw)
				}
			}
		})
	}
}

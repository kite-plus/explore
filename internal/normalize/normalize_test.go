package normalize

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/kite-plus/explore/internal/feed"
	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/policy"
)

var update = flag.Bool("update", false, "rewrite golden files")

type goldenEntry struct {
	Identity    string     `json:"identity"`
	URL         string     `json:"url"`
	Title       string     `json:"title"`
	Excerpt     string     `json:"excerpt,omitempty"`
	PublishedAt *time.Time `json:"published_at"`
	DateTrusted bool       `json:"date_trusted"`
	Categories  []string   `json:"categories,omitempty"`
}

type golden struct {
	Entries []goldenEntry `json:"entries"`
	Stats   Stats         `json:"stats"`
}

func TestSnapshotGolden(t *testing.T) {
	cases := []struct {
		name, file, feedURL, host string
		extra                     []string
		hideExcerpt               bool
	}{
		{"wordpress", "wordpress/feed.xml", "https://wp.example.com/feed/", "wp.example.com", nil, false},
		{"halo-local", "halo/rss.xml", "http://localhost:8090/rss.xml", "localhost", nil, false},
		{"halo-external-url-unset", "halo/rss.xml", "https://halo.example.com/rss.xml", "halo.example.com", nil, false},
		{"halo-single-digit-day", "halo/single-digit-day.xml", "https://halo.example.com/rss.xml", "halo.example.com", nil, false},
		{"hugo-home", "hugo/index.xml", "https://hugo.example.com/index.xml", "hugo.example.com", nil, false},
		{"hugo-posts", "hugo/posts-index.xml", "https://hugo.example.com/posts/index.xml", "hugo.example.com", nil, false},
		{"hexo-atom", "hexo/atom.xml", "https://hexo.example.com/atom.xml", "hexo.example.com", nil, false},
		{"hexo-default-url", "hexo/atom-default-url.xml", "https://myblog.example.net/atom.xml", "myblog.example.net", nil, false},
		{"hexo-rss2-no-guid", "hexo/rss2-no-guid.xml", "https://hexo.example.com/rss2.xml", "hexo.example.com", nil, false},
		{"hexo-ci-dates", "hexo/ci-dates.xml", "https://ci.example.com/atom.xml", "ci.example.com", nil, false},
		{"kite-relative-links", "kite/rss.xml", "https://kite.example.com/rss.xml", "kite.example.com", nil, false},
		{"gbk", "edge/gbk.xml", "https://gbk.example.com/feed", "gbk.example.com", nil, false},
		{"json-feed", "edge/feed.json", "https://json.example.com/feed.json", "json.example.com", nil, false},
		{"tricky", "edge/tricky.xml", "https://blog.tricky.example.com/feed.xml", "blog.tricky.example.com", nil, false},
		{"tricky-extra-domain", "edge/tricky.xml", "https://blog.tricky.example.com/feed.xml", "blog.tricky.example.com", []string{"spam.example.org"}, false},
		{"wordpress-excerpt-hidden", "wordpress/feed.xml", "https://wp.example.com/feed/", "wp.example.com", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "feeds", c.file))
			if err != nil {
				t.Fatal(err)
			}
			f, err := feed.Parse(body)
			if err != nil {
				t.Fatal(err)
			}
			res := Snapshot(f, Blog{Host: c.host, ExtraDomains: c.extra, ShowExcerpt: !c.hideExcerpt, FeedURL: c.feedURL})
			compareGolden(t, filepath.Join("..", "..", "testdata", "golden", "normalize", c.name+".json"), view(res))
		})
	}
}

func view(res Result) golden {
	g := golden{Entries: []goldenEntry{}, Stats: res.Stats}
	for _, e := range res.Entries {
		g.Entries = append(g.Entries, goldenEntry{
			Identity: e.Identity, URL: e.URL, Title: e.Title, Excerpt: e.Excerpt,
			PublishedAt: e.PublishedAt, DateTrusted: e.DateTrusted, Categories: e.Categories,
		})
	}
	return g
}

func compareGolden(t *testing.T, path string, v any) {
	t.Helper()
	got, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from the golden file; run go test -update and review the diff\n--- got ---\n%s", path, got)
	}
}

func TestSnapshotKeepsNewestEntries(t *testing.T) {
	f := &feed.Feed{}
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	for i := range 30 {
		p := base.Add(time.Duration(i) * 24 * time.Hour)
		f.Items = append(f.Items, feed.Item{
			ID:        p.Format(time.DateOnly),
			Link:      "https://blog.example.com/" + p.Format(time.DateOnly),
			Title:     "Post",
			Published: &p,
		})
	}
	res := Snapshot(f, Blog{Host: "blog.example.com"})
	if len(res.Entries) != 20 {
		t.Fatalf("kept %d entries, want 20", len(res.Entries))
	}
	if got, want := res.Entries[0].Identity, "2026-01-30"; got != want {
		t.Errorf("newest = %s, want %s", got, want)
	}
	if got, want := res.Entries[19].Identity, "2026-01-11"; got != want {
		t.Errorf("oldest kept = %s, want %s", got, want)
	}
	if res.Stats.Valid != 30 || res.Stats.Trusted != 30 {
		t.Errorf("stats = %+v, want 30 valid and trusted", res.Stats)
	}
}

func TestPlainText(t *testing.T) {
	cases := map[string]string{
		"plain  text\n here":                           "plain text here",
		"<p>One.</p><p>Two.</p>":                       "One. Two.",
		"a<br>b":                                       "a b",
		"Using <T> generics":                           "Using <T> generics",
		"Vec<String> and <b>bold</b>":                  "Vec<String> and bold",
		"&lt;escaped&gt; &amp; entities":               "<escaped> & entities",
		"<script>alert(1)</script>kept":                "kept",
		"<style>p{}</style><noscript>x</noscript>ok":   "ok",
		"<img src=x.gif>after":                         "after",
		"<p>第一段。</p><p>第二段。</p>":                       "第一段。 第二段。",
		"cut in half <a href=\"https://example.com/do": "cut in half",
	}
	for in, want := range cases {
		if got := PlainText(in); got != want {
			t.Errorf("PlainText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSnapshotImage(t *testing.T) {
	article := "https://blog.example.com/posts/photo/"
	f := &feed.Feed{Items: []feed.Item{{
		ID: "photo", Link: article, Title: "Photo post",
		Content: `<img src="https://tracker.example.net/p.gif" width="1" height="1"><p><img src="../../photos/landscape.jpg" width="800" height="600"></p>`,
		Image:   "https://tracker.example.net/p.gif",
	}}}
	got := Snapshot(f, Blog{Host: "blog.example.com", ShowExcerpt: true, FeedURL: "https://blog.example.com/feed.xml"})
	if len(got.Entries) != 1 || got.Entries[0].ImageURL != "https://blog.example.com/photos/landscape.jpg" {
		t.Fatalf("image = %+v", got.Entries)
	}
	got = Snapshot(f, Blog{Host: "blog.example.com", FeedURL: "https://blog.example.com/feed.xml"})
	if got.Entries[0].ImageURL != "" {
		t.Errorf("image shown with excerpts disabled: %q", got.Entries[0].ImageURL)
	}
}

func TestImageCandidate(t *testing.T) {
	base, _ := url.Parse("https://blog.example.com/post/")
	for _, c := range []struct {
		item feed.Item
		want string
	}{
		{feed.Item{Image: "https://cdn.example.net/photo.webp"}, "https://cdn.example.net/photo.webp"},
		{feed.Item{Content: `<img src="https://blog.example.com/pixel.gif" width="1" height="1">`, Image: "https://blog.example.com/pixel.gif"}, ""},
		{feed.Item{Content: `<img src="https://blog.example.com/pixel.gif" width="1" height="1">`, Image: "https://cdn.example.net/photo.jpg"}, "https://cdn.example.net/photo.jpg"},
		{feed.Item{Content: `<img src="data:image/png;base64,aaa">`, Image: "javascript:alert(1)"}, ""},
		{feed.Item{Content: `<img src="data:image/png;base64,aaa" data-src="/photo.jpg">`}, "https://blog.example.com/photo.jpg"},
		{feed.Item{Image: "https://blog.example.com/icon.svg"}, ""},
	} {
		if got := imageURL(c.item, base); got != c.want {
			t.Errorf("imageURL(%+v) = %q, want %q", c.item, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate("short", 10); got != "short" {
		t.Errorf("got %q", got)
	}
	if got := Truncate("abcdefghij", 5); got != "abcd…" {
		t.Errorf("got %q", got)
	}
	if got := Truncate("一二三四五六", 4); got != "一二三…" {
		t.Errorf("got %q", got)
	}
	if got := Truncate("ab cdef", 4); got != "ab…" {
		t.Errorf("got %q, want trailing space trimmed", got)
	}
}

func TestSameSite(t *testing.T) {
	cases := []struct {
		link, blog string
		extra      []string
		want       bool
	}{
		{"blog.example.com", "blog.example.com", nil, true},
		{"example.com", "www.example.com", nil, true},
		{"www.example.com", "example.com", nil, true},
		{"cdn.blog.example.com", "blog.example.com", nil, true},
		{"example.org", "example.com", nil, false},
		{"alice.github.io", "alice.github.io", nil, true},
		{"bob.github.io", "alice.github.io", nil, false},
		{"blog.example.co.uk", "www.example.co.uk", nil, true},
		{"other.co.uk", "example.co.uk", nil, false},
		{"localhost", "localhost", nil, true},
		{"localhost", "halo.example.com", nil, false},
		{"127.0.0.1", "127.0.0.1", nil, true},
		{"10.0.0.1", "127.0.0.1", nil, false},
		{"mirror.example.org", "example.com", []string{"example.org"}, true},
		{"example.org", "example.com", []string{"example.org"}, true},
		{"notexample.org", "example.com", []string{"example.org"}, false},
		{"BLOG.Example.COM.", "blog.example.com", nil, true},
	}
	for _, c := range cases {
		if got := SameSite(c.link, c.blog, c.extra); got != c.want {
			t.Errorf("SameSite(%q, %q, %v) = %v, want %v", c.link, c.blog, c.extra, got, c.want)
		}
	}
}

func TestDetectGenerator(t *testing.T) {
	cases := map[string]model.Generator{
		"":                                    model.GeneratorUnknown,
		"https://wordpress.org/?v=7.1.2":      model.GeneratorWordPress,
		"WordPress 7.1.2":                     model.GeneratorWordPress,
		"Halo v2.26.1":                        model.GeneratorHalo,
		"Hugo":                                model.GeneratorHugo,
		"Hugo 0.157.0":                        model.GeneratorHugo,
		"Hexo https://hexo.io/":               model.GeneratorHexo,
		"Typecho 1.3.0":                       model.GeneratorTypecho,
		"Jekyll v4.4.1 https://jekyllrb.com/": model.GeneratorJekyll,
		"Ghost 6.65":                          model.GeneratorGhost,
		"Kite":                                model.GeneratorKite,
		"Some CMS":                            model.GeneratorOther,
	}
	for in, want := range cases {
		if got := DetectGenerator(in); got != want {
			t.Errorf("DetectGenerator(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLanguage(t *testing.T) {
	cases := map[string]string{
		"zh":         "zh",
		"zh-CN":      "zh-CN",
		"zh-cn":      "zh-CN",
		"zh_CN":      "zh-CN",
		"ZH_cn":      "zh-CN",
		"zh-Hans":    "zh-Hans",
		"zh-hans":    "zh-Hans",
		"zh_hant_tw": "zh-Hant-TW",
		"zh-TW":      "zh-TW",
		"en":         "en",
		"en-us":      "en-US",
		"en-US":      "en-US",
		"es-419":     "es-419",
		" en-us\n":   "en-US",
		"iw":         "he",
		"und":        "und",
		"":           "und",
		"  ":         "und",
		"English":    "und",
		"zh-CN,en":   "und",
		"中文":         "und",
	}
	for in, want := range cases {
		if got := Language(in); got != want {
			t.Errorf("Language(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCategories(t *testing.T) {
	raw := []string{"Go", "go", "Uncategorized", "  <b>Web</b>  ", "", "未分类", strings.Repeat("长", 60)}
	for i := range 12 {
		raw = append(raw, fmt.Sprintf("c%d", i))
	}
	got := categories(raw)
	if len(got) != policy.CategoriesPerEntry {
		t.Fatalf("len = %d, want %d: %q", len(got), policy.CategoriesPerEntry, got)
	}
	if got[0] != "Go" || got[1] != "Web" || utf8.RuneCountInString(got[2]) != policy.CategoryMaxRunes {
		t.Errorf("categories = %q", got[:3])
	}
}

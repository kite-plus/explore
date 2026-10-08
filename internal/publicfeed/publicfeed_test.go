package publicfeed

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kite-plus/explore/internal/feed"
)

func TestRSSRoundTrips(t *testing.T) {
	published := time.Date(2026, 9, 20, 2, 0, 0, 0, time.UTC)
	out, err := RSS(Channel{
		Title: "Explore", Link: "https://explore.kite.plus/", Self: "https://explore.kite.plus/feed.xml",
		Description: "New posts from independent blogs",
	}, []Item{
		{Title: "Tom & Jerry <3", Link: "https://blog.example.com/p/1?a=1&b=2", Excerpt: "Short.", PublishedAt: published, BlogName: "Example", BlogFeed: "https://blog.example.com/atom.xml"},
		{Title: "No excerpt", Link: "https://other.example.org/p/2", PublishedAt: published.Add(-time.Hour), BlogName: "Other", BlogFeed: "https://other.example.org/feed/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		`<source url="https://blog.example.com/atom.xml">Example</source>`,
		`<guid isPermaLink="true">https://blog.example.com/p/1?a=1&amp;b=2</guid>`,
		`<atom:link href="https://explore.kite.plus/feed.xml" rel="self" type="application/rss+xml"></atom:link>`,
		`<lastBuildDate>Sun, 20 Sep 2026 02:00:00 +0000</lastBuildDate>`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %s\n%s", want, s)
		}
	}
	if strings.Count(s, "<description>") != 2 {
		t.Errorf("an item without an excerpt must have no description:\n%s", s)
	}

	// Explore's own feed must read back through the parser it uses for others.
	f, err := feed.Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Items) != 2 || f.Items[0].Title != "Tom & Jerry <3" || f.Items[0].Link != "https://blog.example.com/p/1?a=1&b=2" {
		t.Errorf("round trip = %+v", f.Items)
	}
	if f.Items[0].Published == nil || !f.Items[0].Published.Equal(published) {
		t.Errorf("published = %v", f.Items[0].Published)
	}
}

func TestEmptyRSS(t *testing.T) {
	out, err := RSS(Channel{Title: "Explore", Link: "https://explore.kite.plus/", Self: "https://explore.kite.plus/feed.xml"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "lastBuildDate") {
		t.Errorf("an empty feed has no build date:\n%s", out)
	}
	if _, err := feed.Parse(out); err != nil {
		t.Errorf("empty feed does not parse: %v", err)
	}
}

// A post whose feed gave no date to trust is listed without one.
func TestRSSLeavesOutAnUntrustedDate(t *testing.T) {
	out, err := RSS(Channel{Title: "Explore", Link: "https://explore.kite.plus/", Self: "https://explore.kite.plus/f/t.xml"}, []Item{
		{Title: "Undated", Link: "https://other.example.org/p/2", BlogName: "Other", BlogFeed: "https://other.example.org/feed/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if s := string(out); strings.Contains(s, "pubDate") || strings.Contains(s, "lastBuildDate") {
		t.Errorf("an undated item has a date:\n%s", s)
	}
	if f, err := feed.Parse(out); err != nil || len(f.Items) != 1 || f.Items[0].Published != nil {
		t.Errorf("round trip = %+v, %v", f, err)
	}
}

func TestJSONFeed(t *testing.T) {
	published := time.Date(2026, 9, 20, 2, 0, 0, 0, time.UTC)
	out, err := JSON(Channel{
		Title: "Explore · Following", Link: "https://explore.kite.plus/", Self: "https://explore.kite.plus/f/t.json",
	}, []Item{
		{
			Title: "Tom & Jerry <3", Link: "https://blog.example.com/p/1", Excerpt: "Short.", PublishedAt: published,
			BlogName: "Example", BlogFeed: "https://blog.example.com/atom.xml", BlogSite: "https://blog.example.com/",
			BlogIcon: "https://explore.kite.plus/api/v1/blogs/blog.example.com/favicon",
		},
		{Title: "Undated, no excerpt", Link: "https://other.example.org/p/2", BlogName: "Other"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Version     string                       `json:"version"`
		Title       string                       `json:"title"`
		HomePageURL string                       `json:"home_page_url"`
		FeedURL     string                       `json:"feed_url"`
		Items       []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if got.Version != "https://jsonfeed.org/version/1.1" || got.HomePageURL != "https://explore.kite.plus/" ||
		got.FeedURL != "https://explore.kite.plus/f/t.json" || len(got.Items) != 2 {
		t.Fatalf("feed = %s", out)
	}
	first := got.Items[0]
	for key, want := range map[string]string{
		"id": `"https://blog.example.com/p/1"`, "url": `"https://blog.example.com/p/1"`, "title": `"Tom & Jerry <3"`,
		"content_text": `"Short."`, "summary": `"Short."`, "date_published": `"2026-09-20T02:00:00Z"`,
		"authors": `[{"name":"Example","url":"https://blog.example.com/","avatar":"https://explore.kite.plus/api/v1/blogs/blog.example.com/favicon"}]`,
	} {
		if compact(first[key]) != want {
			t.Errorf("first item's %s = %s, want %s", key, first[key], want)
		}
	}
	// JSON Feed requires content on every item, even an empty one.
	second := got.Items[1]
	if string(second["content_text"]) != `""` {
		t.Errorf("an item without an excerpt has content_text %s", second["content_text"])
	}
	for _, key := range []string{"summary", "date_published"} {
		if _, ok := second[key]; ok {
			t.Errorf("an undated item without an excerpt has %s", key)
		}
	}
}

func compact(raw json.RawMessage) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
}

func TestOPML(t *testing.T) {
	out, err := OPML("Explore blogs", []Outline{
		{Name: "Example & Co", SiteURL: "https://blog.example.com/", FeedURL: "https://blog.example.com/atom.xml"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<outline type="rss" text="Example &amp; Co" title="Example &amp; Co" xmlUrl="https://blog.example.com/atom.xml" htmlUrl="https://blog.example.com/"></outline>`
	if !strings.Contains(string(out), want) || !strings.Contains(string(out), `<opml version="2.0">`) {
		t.Errorf("OPML =\n%s", out)
	}
}

func TestReadOPML(t *testing.T) {
	// Folders as Feedly and Inoreader write them, an outline in lower case,
	// a folder-only outline and a site without a feed.
	const file = `<?xml version="1.0" encoding="UTF-8"?>
<opml version="1.0">
  <head><title>Reader subscriptions</title></head>
  <body>
    <outline text="Tech" title="Tech">
      <outline type="rss" text="Blog A" title="Blog A &amp; Co" xmlUrl="https://a.example.com/feed/" htmlUrl="https://a.example.com/"/>
      <outline type="rss" text="Blog B" xmlurl=" https://b.example.org/atom.xml "/>
    </outline>
    <outline text="Empty folder"></outline>
    <outline text="Blogroll only" htmlUrl="https://c.example.net/"/>
  </body>
</opml>`
	got, more, err := ReadOPML(strings.NewReader(file), 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []Outline{
		{Name: "Blog A & Co", SiteURL: "https://a.example.com/", FeedURL: "https://a.example.com/feed/"},
		{Name: "Blog B", FeedURL: "https://b.example.org/atom.xml"},
		{Name: "Blogroll only", SiteURL: "https://c.example.net/"},
	}
	if !slices.Equal(got, want) || more != 0 {
		t.Errorf("ReadOPML = %+v, %d more\nwant %+v", got, more, want)
	}

	got, more, err = ReadOPML(strings.NewReader(file), 2)
	if err != nil || len(got) != 2 || more != 1 {
		t.Errorf("with max 2: %d outlines, %d more, %v; want 2, 1", len(got), more, err)
	}

	gbk := "<?xml version=\"1.0\" encoding=\"GBK\"?><opml><body><outline text=\"\xd6\xd0\xce\xc4\" xmlUrl=\"https://zh.example.com/rss.xml\"/></body></opml>"
	if got, _, err := ReadOPML(strings.NewReader(gbk), 10); err != nil || len(got) != 1 || got[0].Name != "中文" {
		t.Errorf("GBK file = %+v, %v", got, err)
	}

	for _, bad := range []string{"", "not xml", `{"url": "x"}`, `<rss version="2.0"><channel/></rss>`, `<opml><body><outline`} {
		if _, _, err := ReadOPML(strings.NewReader(bad), 10); !errors.Is(err, ErrNotOPML) {
			t.Errorf("ReadOPML(%q) error = %v, want ErrNotOPML", bad, err)
		}
	}
}

package publicfeed

import (
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

package check

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kite-plus/explore/internal/fetch"
)

func surveyor() *Checker {
	return &Checker{
		Fetch: fetch.New(fetch.Options{UserAgent: "test", AllowPrivate: true, Timeout: 2 * time.Second}),
		Now:   func() time.Time { return now },
	}
}

func TestSurveyMeasuresTheFeed(t *testing.T) {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/"><channel><title>Survey</title>
<item><title>One</title><link>/1</link><pubDate>Tue, 22 Sep 2026 10:00:00 +0000</pubDate>
  <description>short summary</description><content:encoded><![CDATA[<p>` + strings.Repeat("长", 600) + `</p>]]></content:encoded></item>
<item><title>Two</title><link>/2</link><pubDate>Mon, 21 Sep 2026 08:00:10 +0000</pubDate><description>two</description></item>
<item><title>Three</title><link>/3</link><pubDate>Mon, 21 Sep 2026 08:00:50 +0000</pubDate><description>three</description></item>
<item><title>Old</title><link>/4</link><pubDate>Fri, 14 Aug 2026 10:00:00 +0000</pubDate></item>
<item><title>Unreadable</title><link>/5</link><pubDate>last week</pubDate></item>
<item><title>Undated</title><link>/6</link></item>
<item><title>Elsewhere</title><link>https://elsewhere.example/7</link><pubDate>Sun, 20 Sep 2026 10:00:00 +0000</pubDate></item>
<item><link>/8</link><pubDate>Sat, 19 Sep 2026 10:00:00 +0000</pubDate></item>
</channel></rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/feed.xml" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	rec := surveyor().Survey(context.Background(), Input{SiteURL: srv.URL, FeedURL: srv.URL + "/feed.xml"})
	if rec.Error != "" || rec.Report == nil || rec.Feed == nil {
		t.Fatalf("record = %+v", rec)
	}
	fs := *rec.Feed
	want := FeedStats{
		Bytes: len(body), Charset: "utf-8", Conditional: "304",
		Items: 8, Valid: 6, OffDomain: 1, NoTitle: 1, Undated: 2, Unreadable: 1,
		MaxSameMinute: 2, InWindow: 5, SpanDays: 39,
		// Title-only items count as no text, which puts the median at zero.
		SummaryRunes: 5, TextRunes: 0,
	}
	if fs != want {
		t.Errorf("stats =\n%+v\nwant\n%+v", fs, want)
	}
}

func TestSurveyWithoutValidators(t *testing.T) {
	s := newSite(t)
	s.serve("/", s.page("Hexo 8.1.2", "/atom.xml"))
	s.fixture("/atom.xml", "hexo/atom.xml", "https://hexo.example.com")

	rec := surveyor().Survey(context.Background(), Input{SiteURL: s.srv.URL})
	if rec.Feed == nil || rec.Feed.Conditional != "none" || rec.Report.DiscoveredBy != "autodiscovery" {
		t.Fatalf("record = %+v", rec)
	}
}

func TestSurveyWithoutFeed(t *testing.T) {
	s := newSite(t)
	s.serve("/", s.page("WordPress 7.1"))

	rec := surveyor().Survey(context.Background(), Input{SiteURL: s.srv.URL})
	if rec.Feed != nil || rec.Report == nil || rec.Report.Passed || rec.Report.Generator != "wordpress" {
		t.Fatalf("record = %+v", rec)
	}
}

func TestSurveyInvalidInput(t *testing.T) {
	rec := surveyor().Survey(context.Background(), Input{SiteURL: "ftp://example.com/"})
	if rec.Error == "" || rec.Report != nil {
		t.Fatalf("record = %+v", rec)
	}
}

func TestDeclaredCharset(t *testing.T) {
	cases := []struct {
		contentType, body, want string
	}{
		{"text/xml; charset=GBK", `<?xml version="1.0" encoding="utf-8"?><rss/>`, "gbk"},
		{"application/rss+xml", "\xef\xbb\xbf<?xml version='1.0' encoding='ISO-8859-1'?><rss/>", "iso-8859-1"},
		{"application/xml", `<?xml version="1.0"?><rss/>`, ""},
		{"application/feed+json", `{"version": "https://jsonfeed.org/version/1.1"}`, ""},
	}
	for _, c := range cases {
		if got := declaredCharset(c.contentType, []byte(c.body)); got != c.want {
			t.Errorf("declaredCharset(%q, %q) = %q, want %q", c.contentType, c.body, got, c.want)
		}
	}
}

func TestSurveyNamesTheSystemFromTheHomePage(t *testing.T) {
	s := newSite(t)
	s.serve("/", s.page("Hugo 0.150.0"))
	s.serve("/index.xml", `<?xml version="1.0"?><rss version="2.0"><channel><title>t</title>
<item><title>a</title><link>`+s.srv.URL+`/a/</link><pubDate>Mon, 21 Sep 2026 08:00:00 +0000</pubDate></item>
</channel></rss>`)

	rec := surveyor().Survey(context.Background(), Input{SiteURL: s.srv.URL, FeedURL: s.srv.URL + "/index.xml"})
	if rec.Report == nil || rec.Report.Generator != "unknown" || rec.System != "hugo" {
		t.Fatalf("record = %+v", rec)
	}
}

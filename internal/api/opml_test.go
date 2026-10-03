package api_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/kite-plus/explore/internal/publicfeed"
	"github.com/kite-plus/explore/internal/store"
)

type importOut struct {
	Outlines         int `json:"outlines"`
	Added            int `json:"added"`
	AlreadyFollowing int `json:"already_following"`
	Ignored          int `json:"ignored"`
	NotListed        []struct {
		Title   string `json:"title"`
		SiteURL string `json:"site_url"`
		FeedURL string `json:"feed_url"`
	} `json:"not_listed"`
}

func TestSubscriptionsOPML(t *testing.T) {
	e := newEnv(t, false)
	e.seed("a.example.com", "en")
	e.seed("b.example.org", "zh")
	e.seed("c.example.net", "en")
	if _, err := e.s.UpdateBlog(context.Background(), "c.example.net", store.BlogUpdate{ExtraDomains: &[]string{"c.example.dev"}}); err != nil {
		t.Fatal(err)
	}
	cookie, csrf := e.session("reader@example.com", false)
	auth := map[string]string{"Cookie": cookie, "X-CSRF-Token": csrf, "Content-Type": "text/x-opml"}
	if w := e.do(req{method: http.MethodPut, path: "/api/v1/me/subscriptions/b.example.org", header: auth}); w.Code != http.StatusNoContent {
		t.Fatalf("subscribe = %d", w.Code)
	}

	file := `<?xml version="1.0"?>
<opml version="2.0"><body>
  <outline text="Folder">
    <outline text="A" xmlUrl="https://www.a.example.com/feed/" htmlUrl="https://www.a.example.com/"/>
    <outline text="B" xmlUrl="https://b.example.org/feed.xml"/>
  </outline>
  <outline text="C by its extra domain" htmlUrl="https://c.example.dev/"/>
  <outline text="Not here" xmlUrl="https://unknown.example.com/rss.xml" htmlUrl="https://unknown.example.com/"/>
  <outline text="B again" htmlUrl="http://B.example.org"/>
</body></opml>`
	w := e.do(req{method: http.MethodPost, path: "/api/v1/me/subscriptions/import", body: file, header: auth})
	if w.Code != http.StatusOK {
		t.Fatalf("import = %d: %s", w.Code, w.Body.String())
	}
	got := decode[importOut](t, w)
	if got.Outlines != 5 || got.Added != 2 || got.AlreadyFollowing != 1 || got.Ignored != 0 ||
		len(got.NotListed) != 1 || got.NotListed[0].Title != "Not here" || got.NotListed[0].FeedURL != "https://unknown.example.com/rss.xml" {
		t.Fatalf("import = %+v", got)
	}

	export := e.do(req{method: http.MethodGet, path: "/api/v1/me/subscriptions.opml", header: auth})
	if export.Code != http.StatusOK || !strings.HasPrefix(export.Header().Get("Content-Type"), "text/x-opml") ||
		!strings.HasPrefix(export.Header().Get("Content-Disposition"), "attachment") ||
		export.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("export = %d %v", export.Code, export.Header())
	}
	outlines, _, err := publicfeed.ReadOPML(strings.NewReader(export.Body.String()), 10)
	if err != nil {
		t.Fatal(err)
	}
	var feeds []string
	for _, o := range outlines {
		feeds = append(feeds, o.FeedURL)
	}
	slices.Sort(feeds)
	if want := []string{"https://a.example.com/feed.xml", "https://b.example.org/feed.xml", "https://c.example.net/feed.xml"}; !slices.Equal(feeds, want) {
		t.Errorf("exported feeds = %v, want %v", feeds, want)
	}

	// The export imports back as it is.
	again := decode[importOut](t, e.do(req{method: http.MethodPost, path: "/api/v1/me/subscriptions/import", body: export.Body.String(), header: auth}))
	if again.Outlines != 3 || again.Added != 0 || again.AlreadyFollowing != 3 || len(again.NotListed) != 0 {
		t.Errorf("importing the export = %+v", again)
	}

	if w := e.do(req{method: http.MethodPost, path: "/api/v1/me/subscriptions/import", body: `{"url":"x"}`, header: auth}); w.Code != http.StatusBadRequest {
		t.Errorf("JSON import = %d", w.Code)
	} else if code, _ := errorCode(t, w); code != "invalid_opml" {
		t.Errorf("JSON import code = %s", code)
	}
	noCSRF := map[string]string{"Cookie": cookie}
	if w := e.do(req{method: http.MethodPost, path: "/api/v1/me/subscriptions/import", body: file, header: noCSRF}); w.Code != http.StatusForbidden {
		t.Errorf("import without CSRF = %d", w.Code)
	}
	if w := e.get("/api/v1/me/subscriptions.opml"); w.Code != http.StatusUnauthorized {
		t.Errorf("anonymous export = %d", w.Code)
	}
}

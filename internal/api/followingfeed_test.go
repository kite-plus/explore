package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/kite-plus/explore/internal/model"
)

type feedAddresses struct {
	JSON *string `json:"json_url"`
	RSS  *string `json:"rss_url"`
	OPML *string `json:"opml_url"`
}

// feedPath is a published address as a path on the test server.
func feedPath(address *string) string {
	return strings.TrimPrefix(*address, "https://explore.example.org")
}

func TestFollowingFeed(t *testing.T) {
	e := newEnv(t, false)
	e.seed("author.example.org", "en", post("one", 2, "An excerpt."), model.Entry{
		Identity: "undated", URL: "https://author.example.org/undated", Title: "Undated post",
	})
	e.seed("other.example.org", "en", post("elsewhere", 1, ""))
	cookie, csrf := e.session("reader@example.com", false)
	auth := map[string]string{"Cookie": cookie, "X-CSRF-Token": csrf}
	if w := e.do(req{method: http.MethodPut, path: "/api/v1/me/subscriptions/author.example.org", header: auth}); w.Code != http.StatusNoContent {
		t.Fatalf("subscribe = %d: %s", w.Code, w.Body.String())
	}
	settings := func() feedAddresses {
		return decode[feedAddresses](t, e.do(req{method: http.MethodGet, path: "/api/v1/me/following-feed", header: auth}))
	}
	publish := func() feedAddresses {
		w := e.do(req{method: http.MethodPost, path: "/api/v1/me/following-feed", header: auth})
		if w.Code != http.StatusOK {
			t.Fatalf("publish = %d: %s", w.Code, w.Body.String())
		}
		return decode[feedAddresses](t, w)
	}

	if off := settings(); off.JSON != nil || off.RSS != nil || off.OPML != nil {
		t.Fatalf("a new account publishes %+v", off)
	}
	if w := e.do(req{method: http.MethodPost, path: "/api/v1/me/following-feed", header: map[string]string{"Cookie": cookie}}); w.Code != http.StatusForbidden {
		t.Fatalf("publish without the CSRF token = %d", w.Code)
	}
	on := publish()
	if on.JSON == nil || !strings.HasPrefix(*on.JSON, "https://explore.example.org/f/") || !strings.HasSuffix(*on.JSON, ".json") ||
		on.RSS == nil || !strings.HasSuffix(*on.RSS, ".xml") || on.OPML == nil || !strings.HasSuffix(*on.OPML, ".opml") {
		t.Fatalf("published at %+v", on)
	}
	if got := settings(); got.JSON == nil || *got.JSON != *on.JSON {
		t.Fatalf("settings = %+v, want %+v", got, on)
	}

	w := e.get(feedPath(on.JSON))
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/feed+json; charset=utf-8" ||
		w.Header().Get("Cache-Control") != "public, max-age=300" || w.Header().Get("X-Robots-Tag") != "noindex" {
		t.Fatalf("json feed = %d %v", w.Code, w.Header())
	}
	feed := decode[struct {
		Version string `json:"version"`
		FeedURL string `json:"feed_url"`
		Items   []struct {
			URL           string `json:"url"`
			Title         string `json:"title"`
			Summary       string `json:"summary"`
			DatePublished string `json:"date_published"`
			Authors       []struct {
				Name   string `json:"name"`
				URL    string `json:"url"`
				Avatar string `json:"avatar"`
			} `json:"authors"`
		} `json:"items"`
	}](t, w)
	// Only the followed blog's posts, the undated one last.
	if feed.Version != "https://jsonfeed.org/version/1.1" || feed.FeedURL != *on.JSON || len(feed.Items) != 2 {
		t.Fatalf("json feed = %+v", feed)
	}
	first, undated := feed.Items[0], feed.Items[1]
	if first.URL != "https://posts.example/one" || first.Summary != "An excerpt." || first.DatePublished == "" || len(first.Authors) != 1 ||
		first.Authors[0].Name != "Blog author.example.org" || first.Authors[0].URL != "https://author.example.org/" ||
		first.Authors[0].Avatar != "https://explore.example.org/api/v1/blogs/author.example.org/favicon" {
		t.Errorf("first item = %+v", first)
	}
	if undated.Title != "Undated post" || undated.DatePublished != "" {
		t.Errorf("undated item = %+v", undated)
	}

	rss := e.get(feedPath(on.RSS))
	if body := rss.Body.String(); rss.Code != http.StatusOK || rss.Header().Get("Content-Type") != "application/rss+xml; charset=utf-8" ||
		!strings.Contains(body, `<source url="https://author.example.org/feed.xml">Blog author.example.org</source>`) ||
		strings.Contains(body, "elsewhere") {
		t.Errorf("rss = %d\n%s", rss.Code, body)
	}
	opml := e.get(feedPath(on.OPML))
	if body := opml.Body.String(); opml.Code != http.StatusOK || !strings.Contains(body, `xmlUrl="https://author.example.org/feed.xml"`) ||
		strings.Contains(body, "other.example.org") {
		t.Errorf("opml = %d\n%s", opml.Code, body)
	}

	token := strings.TrimSuffix(strings.TrimPrefix(feedPath(on.JSON), "/f/"), ".json")
	for _, path := range []string{"/f/" + token, "/f/" + token + ".txt", "/f/.json", "/f/unknown.json"} {
		if w := e.get(path); w.Code != http.StatusNotFound {
			t.Errorf("%s = %d", path, w.Code)
		}
	}

	// A new address replaces the one before, which stops answering at once.
	again := publish()
	if *again.JSON == *on.JSON {
		t.Fatal("the address did not change")
	}
	if w := e.get(feedPath(on.JSON)); w.Code != http.StatusNotFound {
		t.Errorf("replaced address = %d", w.Code)
	}
	if w := e.get(feedPath(again.JSON)); w.Code != http.StatusOK {
		t.Errorf("new address = %d", w.Code)
	}

	if w := e.do(req{method: http.MethodDelete, path: "/api/v1/me/following-feed", header: auth}); w.Code != http.StatusNoContent {
		t.Fatalf("unpublish = %d: %s", w.Code, w.Body.String())
	}
	if w := e.get(feedPath(again.JSON)); w.Code != http.StatusNotFound {
		t.Errorf("unpublished address = %d", w.Code)
	}
	if off := settings(); off.JSON != nil {
		t.Errorf("settings after unpublishing = %+v", off)
	}

	// Nor does a disabled account's.
	last := publish()
	ctx := context.Background()
	reader, err := e.s.UserByEmail(ctx, "reader@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.s.SetUserDisabled(ctx, reader.ID, true, "spam", adminEmail); err != nil {
		t.Fatal(err)
	}
	if w := e.get(feedPath(last.JSON)); w.Code != http.StatusNotFound {
		t.Errorf("disabled account's address = %d", w.Code)
	}
}

func TestFollowingFeedIsLimitedPerAddress(t *testing.T) {
	e := newEnv(t, false)
	cookie, csrf := e.session("busy@example.com", false)
	w := e.do(req{method: http.MethodPost, path: "/api/v1/me/following-feed", header: map[string]string{"Cookie": cookie, "X-CSRF-Token": csrf}})
	opml := feedPath(decode[feedAddresses](t, w).OPML)
	for i := range 600 {
		if w := e.get(opml); w.Code != http.StatusOK {
			t.Fatalf("request %d = %d", i+1, w.Code)
		}
	}
	w = e.get(opml)
	if code, _ := errorCode(t, w); w.Code != http.StatusTooManyRequests || code != "rate_limited" || w.Header().Get("Retry-After") == "" {
		t.Errorf("request 601 = %d %s, Retry-After %q", w.Code, code, w.Header().Get("Retry-After"))
	}
}

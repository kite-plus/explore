package fetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPublic(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8":                     true,
		"93.184.216.34":               true,
		"2606:4700:4700::1111":        true,
		"127.0.0.1":                   false,
		"10.1.2.3":                    false,
		"172.16.0.1":                  false,
		"192.168.1.1":                 false,
		"169.254.169.254":             false,
		"100.64.0.1":                  false,
		"0.0.0.0":                     false,
		"0.1.2.3":                     false,
		"224.0.0.1":                   false,
		"255.255.255.255":             false,
		"::1":                         false,
		"::":                          false,
		"fd00::1":                     false,
		"fe80::1":                     false,
		"::ffff:127.0.0.1":            false,
		"::ffff:10.0.0.1":             false,
		"64:ff9b::7f00:1":             false,
		"2002:7f00:1::1":              false,
		"2001:db8::1":                 false,
		"::ffff:8.8.8.8":              true,
		"2001:4860:4860::8888":        true,
		"fe80::1%en0":                 false,
		"2a00:1450:4001:82b::200e":    true,
		"198.51.100.7":                false,
		"203.0.113.9":                 false,
		"192.0.2.1":                   false,
		"198.18.0.1":                  false,
		"240.0.0.1":                   false,
		"100.127.255.254":             false,
		"100.128.0.1":                 true,
		"172.32.0.1":                  true,
		"11.0.0.1":                    true,
		"fc00::1":                     false,
		"ff02::1":                     false,
		"fec0::1":                     false,
		"64:ff9b:1::1":                false,
		"2001:db8:ffff:ffff::1":       false,
		"2002::1":                     false,
		"2003::1":                     true,
		"::ffff:169.254.169.254":      false,
		"::ffff:100.64.0.1":           false,
		"::ffff:192.168.0.1":          false,
		"::ffff:0.0.0.0":              false,
		"::ffff:224.0.0.1":            false,
		"::ffff:1.1.1.1":              true,
		"1.1.1.1":                     true,
		"2606:4700::6810:84e5":        true,
		"2400:cb00:2048:1::c629:d7a2": true,
	}
	for in, want := range cases {
		ip, err := netip.ParseAddr(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got := Public(ip); got != want {
			t.Errorf("Public(%s) = %v, want %v", in, got, want)
		}
	}
}

func TestControlBlocksPrivateAddresses(t *testing.T) {
	c := New(Options{UserAgent: "test"})
	for _, addr := range []string{"127.0.0.1:80", "10.0.0.1:443", "[::1]:443", "169.254.169.254:80"} {
		if err := c.control("tcp", addr, nil); !errors.Is(err, ErrBlockedAddress) {
			t.Errorf("control(%s) = %v, want ErrBlockedAddress", addr, err)
		}
	}
	if err := c.control("tcp", "93.184.216.34:8080", nil); !errors.Is(err, ErrBlockedAddress) {
		t.Errorf("non-standard port = %v, want ErrBlockedAddress", err)
	}
	if err := c.control("tcp", "93.184.216.34:443", nil); err != nil {
		t.Errorf("public address refused: %v", err)
	}
}

func TestGetRefusesPrivateURLs(t *testing.T) {
	c := New(Options{UserAgent: "test"})
	cases := []string{
		"http://127.0.0.1/feed",
		"http://[::1]/feed",
		"http://169.254.169.254/latest/meta-data/",
		"http://example.com:8080/feed",
		"ftp://example.com/feed",
		"file:///etc/passwd",
		"http://user:pass@example.com/feed",
		"javascript:alert(1)",
	}
	for _, u := range cases {
		_, err := c.Get(context.Background(), Request{URL: u})
		if !errors.Is(err, ErrBadURL) && !errors.Is(err, ErrBlockedAddress) {
			t.Errorf("Get(%s) = %v, want it refused", u, err)
		}
	}
}

func TestGetRefusesHostsResolvingToPrivateAddresses(t *testing.T) {
	// localhost passes the URL check but must be stopped at connect time.
	c := New(Options{UserAgent: "test"})
	_, err := c.Get(context.Background(), Request{URL: "http://localhost/feed"})
	if !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("err = %v, want ErrBlockedAddress", err)
	}
}

// newServer serves robots.txt and the given handler, and returns a client
// allowed to reach it.
func newServer(t *testing.T, robots string, h http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		if robots == "" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(robots))
	})
	mux.HandleFunc("/", h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, New(Options{UserAgent: UserAgent("0.1", "https://explore.kite.plus"), AllowPrivate: true, Timeout: 2 * time.Second})
}

func TestGetSendsIdentityAndConditionalHeaders(t *testing.T) {
	srv, c := newServer(t, "", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "KiteExplore/0.1 (+https://explore.kite.plus/bot)" {
			t.Errorf("User-Agent = %q", got)
		}
		if !strings.Contains(r.Header.Get("Accept"), "application/rss+xml") {
			t.Errorf("Accept = %q", r.Header.Get("Accept"))
		}
		if r.Header.Get("If-None-Match") != "" && r.Header.Get("If-Modified-Since") != "" {
			t.Error("both validators sent; WordPress then wants both to match")
		}
		if r.Header.Get("If-Modified-Since") == "Sun, 20 Sep 2026 02:00:00 GMT" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Last-Modified", "Sun, 20 Sep 2026 02:00:00 GMT")
		_, _ = w.Write([]byte("<rss/>"))
	})

	first, err := c.Get(context.Background(), Request{URL: srv.URL + "/feed"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != 200 || first.ETag != `"v1"` || first.LastModified == "" {
		t.Fatalf("first = %+v", first)
	}
	second, err := c.Get(context.Background(), Request{URL: srv.URL + "/feed", ETag: first.ETag, LastModified: first.LastModified})
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != http.StatusNotModified {
		t.Fatalf("second status = %d, want 304", second.Status)
	}
}

func TestGetSendsETagWithoutLastModified(t *testing.T) {
	srv, c := newServer(t, "", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `W/"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write([]byte("<rss/>"))
	})
	resp, err := c.Get(context.Background(), Request{URL: srv.URL + "/feed", ETag: `W/"v1"`})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", resp.Status)
	}
}

func TestProbeFallsBackToBoundedGet(t *testing.T) {
	methods := make(chan string, 2)
	srv, client := newServer(t, "", func(w http.ResponseWriter, r *http.Request) {
		methods <- r.Method
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Range") != "bytes=0-0" {
			t.Errorf("Range = %q", r.Header.Get("Range"))
		}
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("x"))
	})
	status, err := client.Probe(context.Background(), srv.URL+"/post")
	if err != nil || status != http.StatusPartialContent {
		t.Fatalf("Probe = %d, %v", status, err)
	}
	if first, second := <-methods, <-methods; first != http.MethodHead || second != http.MethodGet {
		t.Errorf("methods = %s, %s", first, second)
	}
}

func TestProbeHonorsRobotsAndAddressRules(t *testing.T) {
	srv, client := newServer(t, "User-agent: KiteExplore\nDisallow: /post\n", func(w http.ResponseWriter, r *http.Request) {
		t.Error("blocked post was requested")
	})
	if _, err := client.Probe(context.Background(), srv.URL+"/post"); !errors.Is(err, ErrRobotsDisallowed) {
		t.Errorf("robots refusal = %v", err)
	}
	if _, err := New(Options{UserAgent: "test"}).Probe(context.Background(), "http://127.0.0.1/post"); !errors.Is(err, ErrBlockedAddress) {
		t.Errorf("private address = %v", err)
	}
}

func TestGetRedirects(t *testing.T) {
	srv, c := newServer(t, "", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/moved":
			http.Redirect(w, r, "/moved-again", http.StatusMovedPermanently)
		case "/moved-again":
			http.Redirect(w, r, "/feed", http.StatusPermanentRedirect)
		case "/temporary":
			http.Redirect(w, r, "/moved", http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusMovedPermanently)
		default:
			_, _ = w.Write([]byte("<rss/>"))
		}
	})

	res, err := c.Get(context.Background(), Request{URL: srv.URL + "/moved"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Redirected || !res.PermanentRedirect || res.URL != srv.URL+"/feed" {
		t.Errorf("permanent chain = %+v", res)
	}

	res, err = c.Get(context.Background(), Request{URL: srv.URL + "/temporary"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Redirected || res.PermanentRedirect {
		t.Errorf("chain with a 302 = %+v, want not permanent", res)
	}

	if _, err := c.Get(context.Background(), Request{URL: srv.URL + "/loop"}); !errors.Is(err, ErrTooManyRedirects) {
		t.Errorf("loop err = %v, want ErrTooManyRedirects", err)
	}
}

func TestRedirectTargetsAreChecked(t *testing.T) {
	// CheckRedirect runs checkURL on every hop; control covers the resolved
	// address, as TestGetRefusesHostsResolvingToPrivateAddresses shows.
	c := New(Options{UserAgent: "test"})
	u, err := url.Parse("http://169.254.169.254/latest/meta-data/")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.checkURL(u); !errors.Is(err, ErrBlockedAddress) {
		t.Errorf("checkURL = %v, want ErrBlockedAddress", err)
	}
}

func TestGetBoundsTheBody(t *testing.T) {
	srv, c := newServer(t, "", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 2048)))
	})
	if _, err := c.Get(context.Background(), Request{URL: srv.URL + "/big", MaxBytes: 1024}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("err = %v, want ErrTooLarge", err)
	}
	res, err := c.Get(context.Background(), Request{URL: srv.URL + "/big", MaxBytes: 2048})
	if err != nil || len(res.Body) != 2048 {
		t.Errorf("exact limit: res=%v err=%v", res, err)
	}
}

func TestGetRetryAfter(t *testing.T) {
	srv, c := newServer(t, "", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	res, err := c.Get(context.Background(), Request{URL: srv.URL + "/feed"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != 503 || res.RetryAfter != 2*time.Minute {
		t.Errorf("res = %+v", res)
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 9, 20, 2, 0, 0, 0, time.UTC)
	cases := map[string]time.Duration{
		"":                              0,
		"30":                            30 * time.Second,
		"-5":                            0,
		"soon":                          0,
		"Sun, 20 Sep 2026 02:10:00 GMT": 10 * time.Minute,
		"Sun, 20 Sep 2026 01:00:00 GMT": 0,
	}
	for in, want := range cases {
		if got := parseRetryAfter(in, now); got != want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestRobots(t *testing.T) {
	robots := "User-agent: KiteExplore\nDisallow: /private\n\nUser-agent: *\nDisallow: /\n"
	srv, c := newServer(t, robots, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<rss/>"))
	})
	if _, err := c.Get(context.Background(), Request{URL: srv.URL + "/private/feed"}); !errors.Is(err, ErrRobotsDisallowed) {
		t.Errorf("private: err = %v, want ErrRobotsDisallowed", err)
	}
	// The group for KiteExplore wins over the * group that disallows everything.
	if _, err := c.Get(context.Background(), Request{URL: srv.URL + "/feed"}); err != nil {
		t.Errorf("public: %v", err)
	}
}

func TestRobotsWildcardGroupApplies(t *testing.T) {
	srv, c := newServer(t, "User-agent: *\nDisallow: /\n", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<rss/>"))
	})
	if _, err := c.Get(context.Background(), Request{URL: srv.URL + "/feed"}); !errors.Is(err, ErrRobotsDisallowed) {
		t.Errorf("err = %v, want ErrRobotsDisallowed", err)
	}
}

func TestRobotsNamesWhoIsRefused(t *testing.T) {
	for _, tc := range []struct {
		robots   string
		explicit bool
	}{
		{"User-agent: KiteExplore\nDisallow: /\n", true},
		{"User-agent: *\nDisallow: /feed\n", false},
	} {
		srv, c := newServer(t, tc.robots, func(w http.ResponseWriter, r *http.Request) {})
		_, err := c.Get(context.Background(), Request{URL: srv.URL + "/feed"})
		var refused *RobotsError
		if !errors.As(err, &refused) || refused.Explicit != tc.explicit {
			t.Errorf("%q: err = %v, want a refusal with Explicit = %v", tc.robots, err, tc.explicit)
		}
	}
}

func TestRobotsApplyToWhereARedirectLeads(t *testing.T) {
	srv, c := newServer(t, "User-agent: *\nDisallow: /private/\n", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/feed" {
			http.Redirect(w, r, "/private/feed", http.StatusMovedPermanently)
			return
		}
		t.Errorf("fetched %s, which robots.txt disallows", r.URL.Path)
	})
	if _, err := c.Get(context.Background(), Request{URL: srv.URL + "/feed"}); !errors.Is(err, ErrRobotsDisallowed) {
		t.Fatalf("err = %v, want ErrRobotsDisallowed", err)
	}
}

func TestRobotsFileMayRedirect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/robots-live.txt", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/robots-live.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /private\n"))
	})
	mux.HandleFunc("/feed", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<rss/>")) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := New(Options{UserAgent: "test", AllowPrivate: true})
	if _, err := c.Get(context.Background(), Request{URL: srv.URL + "/feed"}); err != nil {
		t.Fatalf("err = %v", err)
	}
	if _, err := c.Get(context.Background(), Request{URL: srv.URL + "/private"}); !errors.Is(err, ErrRobotsDisallowed) {
		t.Fatalf("err = %v, want the redirected robots.txt to apply", err)
	}
}

func TestRobotsStatusCodes(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusNotFound, nil},
		{http.StatusForbidden, nil},
		{http.StatusTooManyRequests, ErrRobotsUnreachable},
		{http.StatusInternalServerError, ErrRobotsUnreachable},
		{http.StatusServiceUnavailable, ErrRobotsUnreachable},
	}
	for _, tc := range cases {
		mux := http.NewServeMux()
		mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status) })
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<rss/>")) })
		srv := httptest.NewServer(mux)
		c := New(Options{UserAgent: "test", AllowPrivate: true})
		_, err := c.Get(context.Background(), Request{URL: srv.URL + "/feed"})
		srv.Close()
		if tc.want == nil && err != nil {
			t.Errorf("robots %d: err = %v, want allowed", tc.status, err)
		}
		if tc.want != nil && !errors.Is(err, tc.want) {
			t.Errorf("robots %d: err = %v, want %v", tc.status, err, tc.want)
		}
	}
}

func TestRobotsIsCached(t *testing.T) {
	hits := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<rss/>")) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(Options{UserAgent: "test", AllowPrivate: true})
	for range 3 {
		if _, err := c.Get(context.Background(), Request{URL: srv.URL + "/feed"}); err != nil {
			t.Fatal(err)
		}
	}
	if hits != 1 {
		t.Errorf("robots.txt fetched %d times, want 1", hits)
	}
}

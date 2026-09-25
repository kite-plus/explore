package api_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/api"
	"github.com/kite-plus/explore/internal/check"
	"github.com/kite-plus/explore/internal/feed"
	"github.com/kite-plus/explore/internal/fetch"
	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/store"
	"github.com/kite-plus/explore/internal/store/storetest"
)

// adminEmail is the account behind admin: true requests; reviews record it.
const adminEmail = "alice@example.com"

func init() { gin.SetMode(gin.TestMode) }

type env struct {
	t    *testing.T
	s    *store.Store
	srv  *api.Server
	h    http.Handler
	logs *bytes.Buffer
	// The admin account's session, sent by requests marked admin.
	adminCookie, adminCSRF string
}

func newEnv(t *testing.T, admins bool) *env {
	return newEnvWithPrivate(t, admins, true)
}

func newEnvWithPrivate(t *testing.T, admins, allowPrivate bool) *env {
	st := storetest.New(t)
	logs := &bytes.Buffer{}
	srv := &api.Server{
		Store:        st,
		Checker:      &check.Checker{Fetch: fetch.New(fetch.Options{UserAgent: "test", AllowPrivate: true, Timeout: 3 * time.Second})},
		ImageFetch:   fetch.New(fetch.Options{UserAgent: "test", AllowPrivate: true, Timeout: 3 * time.Second}),
		LinkFetch:    fetch.New(fetch.Options{UserAgent: "test", AllowPrivate: true, Timeout: 3 * time.Second}),
		PublicURL:    "https://explore.example.org",
		AllowPrivate: allowPrivate,
		Log:          slog.New(slog.NewTextHandler(logs, nil)),
	}
	h, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, s: st, srv: srv, h: h, logs: logs}
	if admins {
		e.adminCookie, e.adminCSRF = e.session(adminEmail, true)
	}
	return e
}

// session creates an account and signs it in the way login does, returning
// the cookie and the CSRF token its writes need.
func (e *env) session(email string, admin bool) (cookie, csrf string) {
	e.t.Helper()
	ctx := context.Background()
	u, err := e.s.CreateUser(ctx, email, "no password", "Alice")
	if err != nil {
		e.t.Fatal(err)
	}
	if admin {
		if err := e.s.SetUserAdmin(ctx, email, true); err != nil {
			e.t.Fatal(err)
		}
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		e.t.Fatal(err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	if err := e.s.CreateSession(ctx, u.ID, sha256.Sum256([]byte(token)), time.Now().Add(time.Hour)); err != nil {
		e.t.Fatal(err)
	}
	cookie = "explore_session=" + token
	me := decode[struct {
		CSRFToken string `json:"csrf_token"`
	}](e.t, e.do(req{method: http.MethodGet, path: "/api/v1/me", header: map[string]string{"Cookie": cookie}}))
	return cookie, me.CSRFToken
}

type req struct {
	method, path, body string
	header             map[string]string
	admin              bool
}

func (e *env) do(r req) *httptest.ResponseRecorder {
	e.t.Helper()
	var body io.Reader
	if r.body != "" {
		body = strings.NewReader(r.body)
	}
	hr := httptest.NewRequest(r.method, r.path, body)
	if r.body != "" {
		hr.Header.Set("Content-Type", "application/json")
	}
	for k, v := range r.header {
		hr.Header.Set(k, v)
	}
	if r.admin {
		hr.Header.Set("Cookie", e.adminCookie)
		hr.Header.Set("X-CSRF-Token", e.adminCSRF)
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, hr)
	return w
}

func (e *env) get(path string) *httptest.ResponseRecorder {
	return e.do(req{method: http.MethodGet, path: path})
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("body is not JSON: %v\n%s", err, w.Body.String())
	}
	return v
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) (string, string) {
	t.Helper()
	v := decode[struct {
		Error struct{ Code, Message string } `json:"error"`
	}](t, w)
	return v.Error.Code, v.Error.Message
}

// seed lists a blog and gives it entries, as the worker would.
func (e *env) seed(host, lang string, entries ...model.Entry) {
	e.t.Helper()
	ctx := context.Background()
	b, err := e.s.CreateBlog(ctx, store.NewBlog{
		Host: host, Name: "Blog " + host, SiteURL: "https://" + host + "/", FeedURL: "https://" + host + "/feed.xml",
		Language: lang, ShowExcerpt: true, Generator: model.GeneratorHugo,
	})
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.s.SyncSnapshot(ctx, b.ID, entries, store.FetchState{FetchInterval: time.Hour, NextFetchAt: time.Now().Add(time.Hour)}); err != nil {
		e.t.Fatal(err)
	}
}

func post(id string, hoursAgo int, excerpt string) model.Entry {
	p := time.Now().Add(-time.Duration(hoursAgo) * time.Hour).Truncate(time.Second)
	return model.Entry{Identity: id, URL: "https://posts.example/" + id, Title: "Post " + id, Excerpt: excerpt, PublishedAt: &p, DateTrusted: true}
}

type entryOut struct {
	ID            string           `json:"id"`
	Title         string           `json:"title"`
	URL           string           `json:"url"`
	Excerpt       *string          `json:"excerpt"`
	ImageURL      *string          `json:"image_url"`
	PublishedAt   *time.Time       `json:"published_at"`
	LinkStatus    model.LinkStatus `json:"link_status"`
	LinkCheckedAt *time.Time       `json:"link_checked_at"`
	Tags          []string         `json:"tags"`
	Blog          *struct {
		Host     string `json:"host"`
		Name     string `json:"name"`
		SiteURL  string `json:"site_url"`
		Language string `json:"language"`
	} `json:"blog"`
}

type pageOut struct {
	Data       []entryOut `json:"data"`
	NextCursor *string    `json:"next_cursor"`
}

func TestEntries(t *testing.T) {
	e := newEnv(t, false)
	e.seed("zh.example.com", "zh-CN", post("a", 1, "摘要"), post("b", 3, ""))
	e.seed("en.example.com", "en", post("c", 2, "Excerpt"))

	w := e.get("/api/v1/entries")
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "public, max-age=60" || w.Header().Get("ETag") == "" {
		t.Fatalf("status %d, headers %v", w.Code, w.Header())
	}
	page := decode[pageOut](t, w)
	if len(page.Data) != 3 || page.NextCursor != nil {
		t.Fatalf("page = %+v", page)
	}
	first := page.Data[0]
	if first.Title != "Post a" || first.Excerpt == nil || *first.Excerpt != "摘要" || first.Blog == nil || first.Blog.Host != "zh.example.com" || first.Blog.Language != "zh-CN" {
		t.Errorf("first entry = %+v", first)
	}
	if page.Data[2].Excerpt != nil {
		t.Errorf("an empty excerpt must be null: %+v", page.Data[2])
	}
	if first.PublishedAt.Location() != time.UTC {
		t.Errorf("times must be UTC: %v", first.PublishedAt)
	}
	if first.LinkStatus != model.LinkUnknown || first.LinkCheckedAt != nil {
		t.Errorf("new entries should await a link check: %+v", first)
	}
	if !strings.Contains(w.Body.String(), `"site_url"`) || strings.Contains(w.Body.String(), `"SiteURL"`) {
		t.Errorf("fields must be snake_case: %s", w.Body.String())
	}

	// A matching ETag gets 304.
	w2 := e.do(req{method: http.MethodGet, path: "/api/v1/entries", header: map[string]string{"If-None-Match": w.Header().Get("ETag")}})
	if w2.Code != http.StatusNotModified || w2.Body.Len() != 0 {
		t.Errorf("revalidation = %d", w2.Code)
	}

	// Paging with the cursor visits every entry once.
	var titles []string
	path := "/api/v1/entries?limit=1"
	for range 10 {
		p := decode[pageOut](t, e.get(path))
		for _, en := range p.Data {
			titles = append(titles, en.Title)
		}
		if p.NextCursor == nil {
			break
		}
		path = "/api/v1/entries?limit=1&cursor=" + url.QueryEscape(*p.NextCursor)
	}
	if strings.Join(titles, ",") != "Post a,Post c,Post b" {
		t.Errorf("paged titles = %v", titles)
	}

	zh := decode[pageOut](t, e.get("/api/v1/entries?lang=zh"))
	if len(zh.Data) != 2 {
		t.Errorf("zh entries = %+v", zh.Data)
	}

	for path, code := range map[string]string{
		"/api/v1/entries?cursor=%21%21%21": "invalid_cursor",
		"/api/v1/entries?cursor=bm9wZQ":    "invalid_cursor",
		"/api/v1/entries?limit=0":          "invalid_request",
		"/api/v1/entries?limit=101":        "invalid_request",
		"/api/v1/entries?lang=zh_CN%25":    "invalid_request",
		"/api/v1/entries?lang=toolongxx":   "invalid_request",
	} {
		w := e.get(path)
		if got, _ := errorCode(t, w); w.Code != http.StatusBadRequest || got != code {
			t.Errorf("%s: %d %s, want 400 %s", path, w.Code, got, code)
		}
	}
}

func TestReaderChecksPendingArticleLink(t *testing.T) {
	e := newEnv(t, false)
	checks := 0
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.WriteHeader(http.StatusOK)
			return
		}
		checks++
		w.WriteHeader(http.StatusOK)
	}))
	defer source.Close()

	article := post("reader-check", 1, "")
	article.URL = source.URL + "/reader-check"
	e.seed("reader-check.example.com", "en", article)
	page := decode[pageOut](t, e.get("/api/v1/entries"))
	id := page.Data[0].ID

	path := "/api/v1/entries/" + id + "/check"
	checked := e.do(req{method: http.MethodPost, path: path, header: map[string]string{"Content-Type": "application/json"}})
	if checked.Code != http.StatusOK {
		t.Fatalf("check = %d %s", checked.Code, checked.Body.String())
	}
	state := decode[struct {
		Status    model.LinkStatus `json:"link_status"`
		CheckedAt *time.Time       `json:"link_checked_at"`
		Checking  bool             `json:"checking"`
	}](t, checked)
	if state.Status != model.LinkAvailable || state.CheckedAt == nil || state.Checking || checks != 1 {
		t.Fatalf("link state = %+v, probes = %d", state, checks)
	}
	if repeat := e.do(req{method: http.MethodPost, path: path, header: map[string]string{"Content-Type": "application/json"}}); repeat.Code != http.StatusOK || checks != 1 {
		t.Errorf("repeat = %d, probes = %d", repeat.Code, checks)
	}
	if current := e.get(path); current.Code != http.StatusOK || current.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("link state GET = %d %s", current.Code, current.Header())
	}
	if missing := e.do(req{method: http.MethodPost, path: "/api/v1/entries/999999/check", header: map[string]string{"Content-Type": "application/json"}}); missing.Code != http.StatusNotFound {
		t.Errorf("missing link = %d", missing.Code)
	}
}

func TestReaderLinkChecksAreRateLimited(t *testing.T) {
	e := newEnv(t, false)
	for i := range 11 {
		w := e.do(req{method: http.MethodPost, path: "/api/v1/entries/1/check", header: map[string]string{"Content-Type": "application/json"}})
		if i < 10 && w.Code != http.StatusNotFound {
			t.Fatalf("request %d = %d", i+1, w.Code)
		}
		if i == 10 && (w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "") {
			t.Errorf("eleventh request = %d %s", w.Code, w.Header())
		}
	}
}

func TestReaderWaitsForExistingLinkCheck(t *testing.T) {
	e := newEnv(t, false)
	e.seed("waiting-links.example.com", "en", post("already-claimed", 1, ""))
	page := decode[pageOut](t, e.get("/api/v1/entries"))
	id, err := strconv.ParseInt(page.Data[0].ID, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	job, claimed, err := e.s.ClaimLinkOnDemand(context.Background(), id)
	if err != nil || !claimed {
		t.Fatalf("initial claim = %t, %v", claimed, err)
	}
	path := "/api/v1/entries/" + page.Data[0].ID + "/check"
	response := e.do(req{method: http.MethodPost, path: path, header: map[string]string{"Content-Type": "application/json"}})
	if response.Code != http.StatusAccepted || !decode[struct {
		Checking bool `json:"checking"`
	}](t, response).Checking {
		t.Fatalf("pending request = %d %s", response.Code, response.Body.String())
	}
	if err := e.s.RecordLinkStatus(context.Background(), job, model.LinkAvailable, time.Hour); err != nil {
		t.Fatal(err)
	}
	if response := e.get(path); response.Code != http.StatusOK {
		t.Errorf("completed request = %d %s", response.Code, response.Body.String())
	}
}

func TestEntryImageProxy(t *testing.T) {
	makePNG := func(width, height int) []byte {
		var body bytes.Buffer
		if err := png.Encode(&body, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
			t.Fatal(err)
		}
		return body.Bytes()
	}
	large, tiny := makePNG(120, 100), makePNG(1, 1)
	fetches := 0
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			_, _ = io.WriteString(w, "User-agent: *\nAllow: /\n")
		case "/large.png":
			fetches++
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(large)
		case "/tiny.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(tiny)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(origin.Close)

	e := newEnv(t, false)
	photo := post("photo", 1, "")
	photo.ImageURL = origin.URL + "/large.png"
	pixel := post("pixel", 2, "")
	pixel.ImageURL = origin.URL + "/tiny.png"
	e.seed("photos.example.com", "en", photo, pixel)
	page := decode[pageOut](t, e.get("/api/v1/entries"))
	if page.Data[0].ImageURL == nil || page.Data[1].ImageURL == nil {
		t.Fatalf("image URLs missing: %+v", page.Data)
	}
	path := *page.Data[0].ImageURL
	if strings.Contains(path, origin.URL) {
		t.Fatalf("source URL leaked: %s", path)
	}
	for range 2 {
		w := e.get(path)
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" || !bytes.Equal(w.Body.Bytes(), large) {
			t.Errorf("image response: %d %v", w.Code, w.Header())
		}
	}
	if fetches != 1 {
		t.Errorf("origin fetched %d times, want one", fetches)
	}
	if w := e.get(*page.Data[1].ImageURL); w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("tracking pixel status = %d", w.Code)
	}
}

func TestBlogFavicon(t *testing.T) {
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 32, 32))); err != nil {
		t.Fatal(err)
	}
	fetches := 0
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			_, _ = io.WriteString(w, "User-agent: *\nAllow: /\n")
		case "/site/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, `<html><head><link rel="shortcut icon" href="icon.png"></head></html>`)
		case "/site/icon.png":
			fetches++
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(picture.Bytes())
		case "/missing/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, "<html><head></head></html>")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(origin.Close)

	e := newEnv(t, false)
	for host, path := range map[string]string{"icons.example.com": "/site/", "missing.example.com": "/missing/"} {
		blog, err := e.s.CreateBlog(context.Background(), store.NewBlog{
			Host: host, Name: host, SiteURL: origin.URL + path, FeedURL: origin.URL + "/feed.xml",
			Language: "en", ShowExcerpt: true, Generator: model.GeneratorHugo,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := e.s.SyncSnapshot(context.Background(), blog.ID, nil, store.FetchState{
			FetchInterval: time.Hour, NextFetchAt: time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		w := e.get("/api/v1/blogs/ICONS.example.com/favicon")
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" || !bytes.Equal(w.Body.Bytes(), picture.Bytes()) {
			t.Errorf("favicon response: %d %v", w.Code, w.Header())
		}
	}
	if fetches != 1 {
		t.Errorf("favicon fetched %d times, want one", fetches)
	}
	w := e.get("/api/v1/blogs/missing.example.com/favicon")
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" || w.Body.Len() == 0 {
		t.Errorf("missing favicon fallback: %d %v", w.Code, w.Header())
	}
	if w := e.get("/api/v1/blogs/unknown.example.com/favicon"); w.Code != http.StatusNotFound {
		t.Errorf("unknown blog favicon status = %d", w.Code)
	}
}

func TestBlogs(t *testing.T) {
	e := newEnv(t, false)
	e.seed("recent.example.com", "en", post("a", 1, ""))
	e.seed("older.example.com", "zh-CN", post("b", 30, ""))
	recent, err := e.s.Blog(context.Background(), "recent.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.s.SetDescription(context.Background(), recent.ID, "A blog about code and life"); err != nil {
		t.Fatal(err)
	}

	list := decode[struct {
		Data []struct {
			Host            string     `json:"host"`
			Description     string     `json:"description"`
			FeedURL         string     `json:"feed_url"`
			Generator       string     `json:"generator"`
			LastPublishedAt *time.Time `json:"last_published_at"`
		} `json:"data"`
	}](t, e.get("/api/v1/blogs"))
	if len(list.Data) != 2 || list.Data[0].Host != "recent.example.com" || list.Data[0].Description != "A blog about code and life" || list.Data[0].Generator != "hugo" || list.Data[0].LastPublishedAt == nil {
		t.Fatalf("blogs = %+v", list.Data)
	}

	w := e.get("/api/v1/blogs/RECENT.example.com")
	if w.Code != http.StatusOK {
		t.Fatalf("blog page = %d %s", w.Code, w.Body.String())
	}
	page := decode[struct {
		Blog    struct{ Host, Description string } `json:"blog"`
		Entries []entryOut                         `json:"entries"`
	}](t, w)
	if page.Blog.Host != "recent.example.com" || page.Blog.Description != "A blog about code and life" || len(page.Entries) != 1 || page.Entries[0].Blog != nil {
		t.Errorf("blog page = %+v", page)
	}

	w = e.do(req{method: http.MethodGet, path: "/api/v1/blogs/missing.example.com", header: map[string]string{"Accept-Language": "zh-CN,zh;q=0.9"}})
	code, msg := errorCode(t, w)
	if w.Code != http.StatusNotFound || code != "not_found" || msg != "没有找到。" || w.Header().Get("Vary") != "Accept-Language" {
		t.Errorf("missing blog = %d %s %q", w.Code, code, msg)
	}
	if w := e.get("/no/such/route"); w.Code != http.StatusNotFound {
		t.Errorf("unknown route = %d", w.Code)
	}
}

func TestFeedAndOPML(t *testing.T) {
	e := newEnv(t, false)
	e.seed("blog.example.com", "en", post("a", 1, "Short & sweet"))

	w := e.get("/feed.xml")
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/rss+xml") ||
		w.Header().Get("Cache-Control") != "public, max-age=300" {
		t.Fatalf("feed = %d %v", w.Code, w.Header())
	}
	f, err := feed.Parse(w.Body.Bytes())
	if err != nil || len(f.Items) != 1 || f.Items[0].Link != "https://posts.example/a" || f.Items[0].Summary != "Short & sweet" {
		t.Fatalf("feed = %+v, %v", f, err)
	}
	if !strings.Contains(w.Body.String(), `<source url="https://blog.example.com/feed.xml">Blog blog.example.com</source>`) {
		t.Errorf("feed lacks the source element:\n%s", w.Body.String())
	}

	w = e.get("/blogs.opml")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `xmlUrl="https://blog.example.com/feed.xml"`) {
		t.Errorf("opml = %d\n%s", w.Code, w.Body.String())
	}
	if w := e.get("/healthz"); w.Code != http.StatusOK {
		t.Errorf("healthz = %d", w.Code)
	}
	if w := e.get("/readyz"); w.Code != http.StatusOK {
		t.Errorf("readyz = %d", w.Code)
	}
}

// blogSite is a fake Hexo blog on localhost for submissions to check.
func blogSite(t *testing.T, withFeed bool) string {
	t.Helper()
	var base string
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `<html><head><meta name="generator" content="Hexo 8.1.2">`+
			`<link rel="alternate" type="application/atom+xml" href="/atom.xml"></head><body></body></html>`)
	})
	if withFeed {
		raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "feeds", "hexo", "atom.xml"))
		if err != nil {
			t.Fatal(err)
		}
		mux.HandleFunc("/atom.xml", func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, strings.ReplaceAll(string(raw), "https://hexo.example.com", base))
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	base = "http://localhost:" + u.Port()
	return base
}

type submissionOut struct {
	ID          string             `json:"id"`
	Status      string             `json:"status"`
	Host        string             `json:"host"`
	FeedURL     string             `json:"feed_url"`
	ReviewNote  string             `json:"review_note"`
	CheckReport *model.CheckReport `json:"check_report"`
}

func TestSubmissionFlow(t *testing.T) {
	e := newEnv(t, true)
	site := blogSite(t, true)

	w := e.do(req{method: http.MethodPost, path: "/api/v1/submissions", body: `{"site_url":"` + site + `","note":"my blog"}`,
		header: map[string]string{"Accept-Language": "zh-CN"}})
	if w.Code != http.StatusCreated {
		t.Fatalf("submit = %d %s", w.Code, w.Body.String())
	}
	sub := decode[submissionOut](t, w)
	if sub.Status != "pending" || sub.Host != "localhost" || sub.FeedURL != site+"/atom.xml" || !sub.CheckReport.Passed {
		t.Fatalf("submission = %+v", sub)
	}
	if hint := sub.CheckReport.Problems[0].Hint; !strings.Contains(hint, "条件请求") {
		t.Errorf("hints must follow Accept-Language: %q", hint)
	}

	got := decode[submissionOut](t, e.get("/api/v1/submissions/"+sub.ID))
	if hint := got.CheckReport.Problems[0].Hint; !strings.Contains(hint, "conditional requests") {
		t.Errorf("the stored report must localize again, in English here: %q", hint)
	}
	if strings.Contains(e.get("/api/v1/submissions/"+sub.ID).Body.String(), "reviewed_by") {
		t.Error("the reviewer name must not be public")
	}

	w = e.do(req{method: http.MethodPost, path: "/api/v1/submissions", body: `{"site_url":"` + site + `"}`})
	if code, _ := errorCode(t, w); w.Code != http.StatusConflict || code != "already_pending" || !strings.Contains(w.Body.String(), sub.ID) {
		t.Errorf("second submission = %d %s", w.Code, w.Body.String())
	}

	w = e.do(req{method: http.MethodPost, path: "/api/v1/admin/submissions/" + sub.ID + "/approve", body: `{"language":"zh_cn"}`, admin: true})
	if w.Code != http.StatusCreated {
		t.Fatalf("approve = %d %s", w.Code, w.Body.String())
	}
	blog := decode[struct {
		Host, Name, Generator, Language string
		Visible                         bool
	}](t, w)
	if blog.Host != "localhost" || blog.Name != "Example Hexo Blog" || blog.Generator != "hexo" || blog.Language != "zh-CN" || blog.Visible {
		t.Errorf("approved blog = %+v (not visible until the worker fetches it)", blog)
	}

	w = e.do(req{method: http.MethodPost, path: "/api/v1/submissions", body: `{"site_url":"` + site + `"}`})
	if code, _ := errorCode(t, w); w.Code != http.StatusConflict || code != "already_listed" {
		t.Errorf("after approval = %d %s", w.Code, w.Body.String())
	}
	if w := e.do(req{method: http.MethodPost, path: "/api/v1/admin/submissions/" + sub.ID + "/approve", admin: true}); w.Code != http.StatusConflict {
		t.Errorf("approving twice = %d", w.Code)
	}
}

func TestDirectListingRemovesPendingSubmission(t *testing.T) {
	e := newEnv(t, true)
	site := blogSite(t, true)

	w := e.do(req{method: http.MethodPost, path: "/api/v1/submissions", body: `{"site_url":"` + site + `"}`})
	if w.Code != http.StatusCreated {
		t.Fatalf("submit = %d %s", w.Code, w.Body.String())
	}
	sub := decode[submissionOut](t, w)

	w = e.do(req{method: http.MethodPost, path: "/api/v1/admin/blogs", body: `{"site_url":"` + site + `"}`, admin: true})
	if w.Code != http.StatusCreated {
		t.Fatalf("direct listing = %d %s", w.Code, w.Body.String())
	}

	w = e.do(req{method: http.MethodGet, path: "/api/v1/admin/submissions?status=pending", admin: true})
	pending := decode[struct {
		Data []submissionOut `json:"data"`
	}](t, w)
	if w.Code != http.StatusOK || len(pending.Data) != 0 {
		t.Fatalf("pending after direct listing = %d %s", w.Code, w.Body.String())
	}

	w = e.get("/api/v1/submissions/" + sub.ID)
	got := decode[submissionOut](t, w)
	if got.Status != "approved" {
		t.Errorf("submission status after direct listing = %q", got.Status)
	}
	stored, err := e.s.Submission(context.Background(), sub.ID)
	if err != nil || stored.ReviewedBy != adminEmail || stored.BlogID == nil {
		t.Errorf("stored review after direct listing = %+v, %v", stored, err)
	}
}

func TestFetchQueueShowsWorkerAndAttemptState(t *testing.T) {
	e := newEnv(t, true)
	ctx := context.Background()
	blog, err := e.s.CreateBlog(ctx, store.NewBlog{
		Host: "queue.example.com", Name: "Queue Blog", SiteURL: "https://queue.example.com/",
		FeedURL: "https://queue.example.com/feed", Language: "en", ShowExcerpt: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	w := e.do(req{method: http.MethodGet, path: "/api/v1/admin/fetch-queue", admin: true})
	queue := decode[struct {
		WorkerOnline bool `json:"worker_online"`
		Data         []struct {
			Host        string `json:"host"`
			QueueStatus string `json:"queue_status"`
		} `json:"data"`
	}](t, w)
	if w.Code != http.StatusOK || queue.WorkerOnline || len(queue.Data) != 1 ||
		queue.Data[0].Host != blog.Host || queue.Data[0].QueueStatus != "queued" {
		t.Fatalf("offline queue = %d %s", w.Code, w.Body.String())
	}
	if err := e.s.RecordWorkerHeartbeat(ctx, "test-worker"); err != nil {
		t.Fatal(err)
	}
	claimed, err := e.s.ClaimDue(ctx, 1, time.Minute)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claimed = %+v, %v", claimed, err)
	}
	w = e.do(req{method: http.MethodGet, path: "/api/v1/admin/fetch-queue", admin: true})
	queue = decode[struct {
		WorkerOnline bool `json:"worker_online"`
		Data         []struct {
			Host        string `json:"host"`
			QueueStatus string `json:"queue_status"`
		} `json:"data"`
	}](t, w)
	if !queue.WorkerOnline || queue.Data[0].QueueStatus != "running" {
		t.Errorf("running queue = %d %s", w.Code, w.Body.String())
	}
	w = e.do(req{method: http.MethodGet, path: "/api/v1/admin/blogs/queue.example.com/fetch-attempts", admin: true})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"outcome":"running"`) {
		t.Errorf("fetch attempts = %d %s", w.Code, w.Body.String())
	}
}

func TestFailedCheckStoresNothing(t *testing.T) {
	e := newEnv(t, false)
	site := blogSite(t, false)

	w := e.do(req{method: http.MethodPost, path: "/api/v1/submissions", body: `{"site_url":"` + site + `"}`})
	if code, _ := errorCode(t, w); w.Code != http.StatusUnprocessableEntity || code != "check_failed" {
		t.Fatalf("submit = %d %s", w.Code, w.Body.String())
	}
	report := decode[struct {
		CheckReport model.CheckReport `json:"check_report"`
	}](t, w).CheckReport
	if report.Passed || report.Problems[0].Code != model.ProblemFeedNotFound || !strings.Contains(report.Problems[0].Hint, "hexo-generator-feed") {
		t.Errorf("report = %+v", report)
	}
	state, err := e.s.HostState(context.Background(), "localhost")
	if err != nil || state.PendingID != "" {
		t.Errorf("a failed check must not be stored: %+v %v", state, err)
	}
}

func TestSubmissionValidation(t *testing.T) {
	e := newEnv(t, true)
	cases := map[string]string{
		`{"site_url":"ftp://example.com"}`: "invalid_url",
		`{"site_url":""}`:                  "invalid_url",
		`{"site_url":"https://example.com","feed_url":"javascript:x"}`:                 "invalid_url",
		`{"site_url":"https://example.com","note":"` + strings.Repeat("长", 501) + `"}`: "invalid_request",
		`not json`: "invalid_request",
	}
	for body, want := range cases {
		w := e.do(req{method: http.MethodPost, path: "/api/v1/submissions", body: body})
		if code, _ := errorCode(t, w); w.Code != http.StatusBadRequest || code != want {
			t.Errorf("%.40s: %d %s, want 400 %s", body, w.Code, code, want)
		}
	}
}

func TestProductionRefusesAddressesThatAreNotDomains(t *testing.T) {
	st := storetest.New(t)
	srv := &api.Server{Store: st, Checker: &check.Checker{Fetch: fetch.New(fetch.Options{UserAgent: "test"})}, PublicURL: "https://x.example"}
	h, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	for _, site := range []string{"http://127.0.0.1/", "http://localhost/", "http://[::1]/"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/submissions", strings.NewReader(`{"site_url":"`+site+`"}`))
		r.Header.Set("Content-Type", "application/json")
		h.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s, want 400", site, w.Code, w.Body.String())
		}
	}
}

func TestExcludedHostIsRefused(t *testing.T) {
	e := newEnv(t, true)
	e.seed("leaving.example.com", "en", post("a", 1, ""))
	if w := e.do(req{method: http.MethodDelete, path: "/api/v1/admin/blogs/leaving.example.com?exclude=opt_out&note=asked", admin: true}); w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	w := e.do(req{method: http.MethodPost, path: "/api/v1/submissions", body: `{"site_url":"https://leaving.example.com/"}`})
	if code, _ := errorCode(t, w); w.Code != http.StatusForbidden || code != "excluded" {
		t.Errorf("resubmitting an opted-out blog = %d %s", w.Code, w.Body.String())
	}
}

func TestAdminAuth(t *testing.T) {
	e := newEnv(t, true)
	if w := e.get("/api/v1/admin/blogs"); w.Code != http.StatusUnauthorized {
		t.Errorf("no session = %d", w.Code)
	}
	bearer := map[string]string{"Authorization": "Bearer anything"}
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/admin/blogs", header: bearer}); w.Code != http.StatusUnauthorized {
		t.Errorf("a bearer token = %d; only admin accounts get in", w.Code)
	}
	reader, _ := e.session("reader@example.com", false)
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/admin/blogs", header: map[string]string{"Cookie": reader}}); w.Code != http.StatusUnauthorized {
		t.Errorf("a reader's session = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/admin/blogs", admin: true}); w.Code != http.StatusOK {
		t.Errorf("an admin's session = %d", w.Code)
	}
	noCSRF := map[string]string{"Cookie": e.adminCookie}
	if w := e.do(req{method: http.MethodPatch, path: "/api/v1/admin/settings/site_notice", body: `{"value":"x"}`, header: noCSRF}); w.Code != http.StatusForbidden {
		t.Errorf("an admin's write without the CSRF token = %d", w.Code)
	}
}

func TestSetup(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	required := func() bool {
		return decode[struct {
			Required bool `json:"required"`
		}](t, e.get("/api/v1/setup")).Required
	}
	if !required() {
		t.Fatal("a fresh install needs setup")
	}
	code, err := e.s.SetupCode(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := e.s.SetupCode(ctx); err != nil || again != code {
		t.Errorf("the code changed between starts: %q, then %q (%v)", code, again, err)
	}

	verify := func(c string) int {
		return e.do(req{method: http.MethodPost, path: "/api/v1/setup/verify", body: `{"code":"` + c + `"}`}).Code
	}
	if got := verify("AAAA-AAAA-AAAA"); got != http.StatusForbidden {
		t.Errorf("a wrong code = %d", got)
	}
	if got := verify(strings.ToLower(strings.ReplaceAll(code, "-", " "))); got != http.StatusNoContent {
		t.Errorf("the code typed in lower case with spaces = %d", got)
	}

	setup := func(c, email, password string) *httptest.ResponseRecorder {
		return e.do(req{method: http.MethodPost, path: "/api/v1/setup",
			body: `{"code":"` + c + `","email":"` + email + `","password":"` + password + `","display_name":"Owner","registration_enabled":false}`})
	}
	if w := setup(code, "owner@example.com", "short"); w.Code != http.StatusBadRequest {
		t.Errorf("a short password = %d", w.Code)
	}
	if w := setup("AAAA-AAAA-AAAA", "owner@example.com", "long enough password"); w.Code != http.StatusForbidden {
		t.Errorf("a wrong code = %d", w.Code)
	}
	w := setup(code, "Owner@Example.com", "long enough password")
	if w.Code != http.StatusOK {
		t.Fatalf("setup = %d %s", w.Code, w.Body.String())
	}
	owner := decode[struct {
		Email   string `json:"email"`
		IsAdmin bool   `json:"is_admin"`
	}](t, w)
	if owner.Email != "owner@example.com" || !owner.IsAdmin {
		t.Errorf("owner = %+v", owner)
	}
	cookie := strings.Split(w.Header().Get("Set-Cookie"), ";")[0]
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/admin/session", header: map[string]string{"Cookie": cookie}}); w.Code != http.StatusNoContent {
		t.Errorf("setup signs the owner in: %d", w.Code)
	}
	if v, _ := e.s.Setting(ctx, "registration_enabled"); v != "false" {
		t.Errorf("registration_enabled = %q", v)
	}
	if v, _ := e.s.Setting(ctx, "submissions_enabled"); v != "true" {
		t.Errorf("a setting setup left alone changed: submissions_enabled = %q", v)
	}

	if required() {
		t.Error("setup is still required once an admin exists")
	}
	if w := setup(code, "second@example.com", "long enough password"); w.Code != http.StatusConflict {
		t.Errorf("a second setup = %d", w.Code)
	}
	if got := verify(code); got != http.StatusConflict {
		t.Errorf("verify after setup = %d", got)
	}
}

func TestSetupHasOneWinner(t *testing.T) {
	e := newEnv(t, false)
	code, err := e.s.SetupCode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	codes := make([]int, 4)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = e.do(req{method: http.MethodPost, path: "/api/v1/setup",
				body: `{"code":"` + code + `","email":"owner` + strconv.Itoa(i) + `@example.com","password":"long enough password","display_name":"Owner"}`}).Code
		}()
	}
	wg.Wait()
	won := 0
	for _, c := range codes {
		if c == http.StatusOK {
			won++
		} else if c != http.StatusConflict {
			t.Errorf("a losing setup = %d, want 409", c)
		}
	}
	if won != 1 {
		t.Errorf("%d setups succeeded: %v", won, codes)
	}
}

func TestAdminManagesBlogs(t *testing.T) {
	e := newEnv(t, true)
	site := blogSite(t, true)

	w := e.do(req{method: http.MethodPost, path: "/api/v1/admin/check", body: `{"url":"` + site + `"}`, admin: true})
	if w.Code != http.StatusOK || !decode[model.CheckReport](t, w).Passed {
		t.Fatalf("admin check = %d %s", w.Code, w.Body.String())
	}

	w = e.do(req{method: http.MethodPost, path: "/api/v1/admin/blogs", body: `{"site_url":"` + site + `","language":"zh_cn","extra_domains":["CDN.Example.com"]}`, admin: true})
	if w.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	created := decode[struct {
		Host         string   `json:"host"`
		Language     string   `json:"language"`
		ExtraDomains []string `json:"extra_domains"`
	}](t, w)
	if created.Language != "zh-CN" || len(created.ExtraDomains) != 1 || created.ExtraDomains[0] != "cdn.example.com" {
		t.Errorf("created = %+v", created)
	}

	w = e.do(req{method: http.MethodPatch, path: "/api/v1/admin/blogs/localhost", body: `{"status":"paused","status_note":"reports"}`, admin: true})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"paused"`) {
		t.Errorf("pause = %d %s", w.Code, w.Body.String())
	}
	if w := e.do(req{method: http.MethodPatch, path: "/api/v1/admin/blogs/localhost", body: `{"status":"deleted"}`, admin: true}); w.Code != http.StatusBadRequest {
		t.Errorf("unknown status = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodPost, path: "/api/v1/admin/blogs/localhost/fetch", admin: true}); w.Code != http.StatusAccepted {
		t.Errorf("fetch now = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodDelete, path: "/api/v1/admin/blogs/localhost?exclude=gone-forever", admin: true}); w.Code != http.StatusBadRequest {
		t.Errorf("unknown exclusion = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodDelete, path: "/api/v1/admin/blogs/localhost?exclude=blocked", admin: true}); w.Code != http.StatusNoContent {
		t.Errorf("delete = %d", w.Code)
	}

	list := e.do(req{method: http.MethodGet, path: "/api/v1/admin/excluded-hosts", admin: true})
	if !strings.Contains(list.Body.String(), `"reason":"blocked"`) {
		t.Errorf("exclusions = %s", list.Body.String())
	}
	if w := e.do(req{method: http.MethodDelete, path: "/api/v1/admin/excluded-hosts/localhost", admin: true}); w.Code != http.StatusNoContent {
		t.Errorf("lift exclusion = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodDelete, path: "/api/v1/admin/excluded-hosts/localhost", admin: true}); w.Code != http.StatusNotFound {
		t.Errorf("lifting twice = %d", w.Code)
	}
}

func TestAdminBlogListCountsEntries(t *testing.T) {
	e := newEnv(t, true)
	e.seed("busy.example.com", "en", post("a", 1, ""), post("b", 2, ""))
	e.seed("quiet.example.com", "en")

	w := e.do(req{method: http.MethodGet, path: "/api/v1/admin/blogs", admin: true})
	list := decode[struct {
		Data []struct {
			Host       string `json:"host"`
			EntryCount *int64 `json:"entry_count"`
		} `json:"data"`
	}](t, w)
	counts := map[string]int64{}
	for _, b := range list.Data {
		if b.EntryCount == nil {
			t.Fatalf("%s has no entry_count: %s", b.Host, w.Body.String())
		}
		counts[b.Host] = *b.EntryCount
	}
	if len(counts) != 2 || counts["busy.example.com"] != 2 || counts["quiet.example.com"] != 0 {
		t.Errorf("entry counts = %v", counts)
	}
}

func TestLanguageFilterFindsRelabeledBlog(t *testing.T) {
	e := newEnv(t, true)
	e.seed("relabeled.example.com", "en", post("a", 1, ""))
	e.seed("en.example.com", "en", post("b", 2, ""))

	w := e.do(req{method: http.MethodPatch, path: "/api/v1/admin/blogs/relabeled.example.com", body: `{"language":"zh_CN"}`, admin: true})
	if w.Code != http.StatusOK || decode[struct{ Language string }](t, w).Language != "zh-CN" {
		t.Fatalf("relabel = %d %s", w.Code, w.Body.String())
	}

	entries := decode[pageOut](t, e.get("/api/v1/entries?lang=zh"))
	if len(entries.Data) != 1 || entries.Data[0].Blog == nil || entries.Data[0].Blog.Host != "relabeled.example.com" || entries.Data[0].Blog.Language != "zh-CN" {
		t.Errorf("zh entries = %+v", entries.Data)
	}
	blogs := decode[struct {
		Data []struct{ Host, Language string } `json:"data"`
	}](t, e.get("/api/v1/blogs?lang=zh"))
	if len(blogs.Data) != 1 || blogs.Data[0].Host != "relabeled.example.com" || blogs.Data[0].Language != "zh-CN" {
		t.Errorf("zh blogs = %+v", blogs.Data)
	}
}

func TestRejection(t *testing.T) {
	e := newEnv(t, true)
	site := blogSite(t, true)
	sub := decode[submissionOut](t, e.do(req{method: http.MethodPost, path: "/api/v1/submissions", body: `{"site_url":"` + site + `"}`}))

	if w := e.do(req{method: http.MethodPost, path: "/api/v1/admin/submissions/" + sub.ID + "/reject", body: `{"review_note":" "}`, admin: true}); w.Code != http.StatusBadRequest {
		t.Errorf("a rejection needs a note: %d", w.Code)
	}
	if w := e.do(req{method: http.MethodPost, path: "/api/v1/admin/submissions/" + sub.ID + "/reject", body: `{"review_note":"Not a personal blog."}`, admin: true}); w.Code != http.StatusNoContent {
		t.Fatalf("reject = %d %s", w.Code, w.Body.String())
	}
	got := decode[submissionOut](t, e.get("/api/v1/submissions/"+sub.ID))
	if got.Status != "rejected" || got.ReviewNote != "Not a personal blog." {
		t.Errorf("rejected submission = %+v", got)
	}
	queue := e.do(req{method: http.MethodGet, path: "/api/v1/admin/submissions?status=rejected", admin: true})
	if !strings.Contains(queue.Body.String(), `"reviewed_by":"`+adminEmail+`"`) {
		t.Errorf("maintainers see who reviewed: %s", queue.Body.String())
	}
	if w := e.get("/api/v1/submissions/not-a-uuid"); w.Code != http.StatusNotFound {
		t.Errorf("bad id = %d", w.Code)
	}
}

func TestLogsCarryNoClientAddress(t *testing.T) {
	e := newEnv(t, false)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/entries?lang=zh", nil)
	r.RemoteAddr = "198.51.100.23:5555"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	r.Header.Set("User-Agent", "SecretBrowser/1.0")
	e.h.ServeHTTP(httptest.NewRecorder(), r)

	logs := e.logs.String()
	if !strings.Contains(logs, "route=/api/v1/entries") {
		t.Fatalf("the request was not logged: %s", logs)
	}
	for _, leak := range []string{"198.51.100.23", "203.0.113.9", "SecretBrowser", "lang=zh"} {
		if strings.Contains(logs, leak) {
			t.Errorf("logs contain %q: %s", leak, logs)
		}
	}
}

func TestSubmissionsAreRateLimited(t *testing.T) {
	e := newEnvWithPrivate(t, false, false)
	for i := range 6 {
		w := e.do(req{method: http.MethodPost, path: "/api/v1/submissions", body: `not json`})
		if i < 5 && w.Code != http.StatusBadRequest {
			t.Fatalf("request %d = %d", i+1, w.Code)
		}
		if i == 5 {
			if code, _ := errorCode(t, w); w.Code != http.StatusTooManyRequests || code != "rate_limited" || w.Header().Get("Retry-After") == "" {
				t.Errorf("sixth request = %d %s", w.Code, w.Header())
			}
		}
	}
}

func TestTags(t *testing.T) {
	e := newEnv(t, true)
	e.seed("tags.example.com", "zh", post("a", 1, ""), post("b", 2, ""))
	ctx := context.Background()
	jobs, err := e.s.Untagged(ctx, 10)
	if err != nil || len(jobs) != 2 {
		t.Fatalf("untagged = %+v, %v", jobs, err)
	}
	if err := e.s.SetTags(ctx, jobs[0].EntryID, jobs[0].Title, []string{"ai", "tools"}); err != nil {
		t.Fatal(err)
	}

	w := e.get("/api/v1/tags")
	list := decode[struct {
		Data []struct {
			Slug string            `json:"slug"`
			Name map[string]string `json:"name"`
		} `json:"data"`
	}](t, w)
	if w.Code != http.StatusOK || len(list.Data) != len(model.Tags) || list.Data[0].Name["zh"] != model.Tags[0].ZH {
		t.Fatalf("tags = %d %+v", w.Code, list)
	}

	page := decode[pageOut](t, e.get("/api/v1/entries?tag=ai"))
	if len(page.Data) != 1 || page.Data[0].Title != "Post a" || strings.Join(page.Data[0].Tags, ",") != "ai,tools" {
		t.Fatalf("entries tagged ai = %+v", page.Data)
	}
	all := decode[pageOut](t, e.get("/api/v1/entries"))
	if len(all.Data) != 2 || all.Data[1].Tags == nil || len(all.Data[1].Tags) != 0 {
		t.Errorf("an untagged entry should carry an empty list: %+v", all.Data)
	}
	if w := e.get("/api/v1/entries?tag=nonsense"); w.Code != http.StatusBadRequest {
		t.Errorf("unknown tag = %d, want 400", w.Code)
	}

	w = e.do(req{method: http.MethodPatch, path: "/api/v1/admin/blogs/tags.example.com", body: `{"default_tags":["ai","ops"]}`, admin: true})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"default_tags":["ai","ops"]`) {
		t.Fatalf("set default tags = %d %s", w.Code, w.Body.String())
	}
	for _, body := range []string{`{"default_tags":["nonsense"]}`, `{"default_tags":["ai","ops","data","life"]}`} {
		if w := e.do(req{method: http.MethodPatch, path: "/api/v1/admin/blogs/tags.example.com", body: body, admin: true}); w.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", body, w.Code)
		}
	}
}

func TestBlogPagePages(t *testing.T) {
	e := newEnv(t, false)
	undated := model.Entry{Identity: "undated", URL: "https://posts.example/undated", Title: "Post undated"}
	e.seed("paged.example.com", "en", post("p1", 1, ""), post("p2", 2, ""), post("p3", 3, ""), post("p4", 4, ""), undated)

	var titles []string
	path := "/api/v1/blogs/paged.example.com?limit=2"
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatal("the blog page kept giving cursors")
		}
		w := e.get(path)
		if w.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", path, w.Code, w.Body.String())
		}
		page := decode[struct {
			Entries    []entryOut `json:"entries"`
			NextCursor *string    `json:"next_cursor"`
		}](t, w)
		for _, en := range page.Entries {
			titles = append(titles, en.Title)
		}
		if page.NextCursor == nil {
			break
		}
		path = "/api/v1/blogs/paged.example.com?limit=2&cursor=" + url.QueryEscape(*page.NextCursor)
	}
	if got := strings.Join(titles, ","); got != "Post p1,Post p2,Post p3,Post p4,Post undated" {
		t.Errorf("paged blog = %s", got)
	}
	if w := e.get("/api/v1/blogs/paged.example.com?cursor=nonsense"); w.Code != http.StatusBadRequest {
		t.Errorf("bad cursor = %d", w.Code)
	}
}

package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kite-plus/explore/internal/model"
)

var update = flag.Bool("update", false, "rewrite golden files")

// newTestStore mirrors storetest.New, which this package cannot import.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("EXPLORE_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("EXPLORE_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	schema := "test_" + hex.EncodeToString(b[:])
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	s, err := OpenSchema(ctx, url, schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		_ = admin.Close(context.Background())
	})
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return s
}

func (s *Store) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := s.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func listBlog(t *testing.T, s *Store, host, lang string) model.Blog {
	t.Helper()
	b, err := s.CreateBlog(context.Background(), NewBlog{
		Host: host, Name: host, SiteURL: "https://" + host + "/", FeedURL: "https://" + host + "/feed",
		Language: lang, ShowExcerpt: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func at(t time.Time) *time.Time { return &t }

func entry(id string, published *time.Time, trusted bool) model.Entry {
	return model.Entry{Identity: id, URL: "https://x/" + id, Title: "Title " + id, Excerpt: "Excerpt " + id, PublishedAt: published, DateTrusted: trusted}
}

func sync(t *testing.T, s *Store, blogID int64, entries ...model.Entry) {
	t.Helper()
	err := s.SyncSnapshot(context.Background(), blogID, entries, FetchState{
		ETag: `"v1"`, BodyHash: []byte{1}, FetchInterval: time.Hour, NextFetchAt: time.Now().Add(time.Hour),
		Generator: model.GeneratorHugo,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func identities[T any](items []T, id func(T) string) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, id(it))
	}
	return out
}

func TestMigrateTwice(t *testing.T) {
	s := newTestStore(t)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("second migration run: %v", err)
	}
}

// TestSchemaColumns is the guard from docs/design/data-model.md section 6:
// any new column must show up in the golden file and so in review.
func TestSchemaColumns(t *testing.T) {
	s := newTestStore(t)
	rows, err := s.pool.Query(context.Background(), `
		SELECT table_name || '.' || column_name || ' ' || data_type
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name NOT LIKE 'goose%'
		ORDER BY table_name, column_name`)
	if err != nil {
		t.Fatal(err)
	}
	cols, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	got := []byte(strings.Join(cols, "\n") + "\n")
	path := filepath.Join("..", "..", "testdata", "schema", "columns.txt")
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
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("schema columns changed; review, then run go test -update\n%s", got)
	}
	for _, c := range cols {
		if strings.HasPrefix(c, "entries.") && strings.Contains(c, "content") {
			t.Errorf("entries must never hold content: %s", c)
		}
	}
}

func TestSyncSnapshotMirrorsTheFeed(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	b := listBlog(t, s, "blog.example.com", "en")
	now := time.Now()

	sync(t, s, b.ID, entry("a", at(now.Add(-3*time.Hour)), true), entry("b", at(now.Add(-2*time.Hour)), true), entry("c", nil, false))
	_, got, err := s.VisibleBlog(ctx, b.Host)
	if err != nil {
		t.Fatal(err)
	}
	if ids := identities(got, func(e model.Entry) string { return e.Identity }); strings.Join(ids, ",") != "b,a,c" {
		t.Fatalf("after first sync = %v, want b,a,c (undated last)", ids)
	}

	changed := entry("b", at(now.Add(-2*time.Hour)), true)
	changed.Title = "Renamed"
	changed.Excerpt = ""
	sync(t, s, b.ID, changed, entry("d", at(now.Add(-time.Hour)), true))
	_, got, err = s.VisibleBlog(ctx, b.Host)
	if err != nil {
		t.Fatal(err)
	}
	if ids := identities(got, func(e model.Entry) string { return e.Identity }); strings.Join(ids, ",") != "d,b" {
		t.Fatalf("after second sync = %v, want d,b", ids)
	}
	if got[1].Title != "Renamed" || got[1].Excerpt != "" {
		t.Errorf("updated entry = %+v", got[1])
	}

	blog, err := s.Blog(ctx, b.Host)
	if err != nil {
		t.Fatal(err)
	}
	if blog.ETag != `"v1"` || blog.Generator != model.GeneratorHugo || blog.LastSucceededAt == nil || blog.ConsecutiveFailures != 0 {
		t.Errorf("fetch state = %+v", blog)
	}
}

func TestExcerptLimitIsEnforcedByTheDatabase(t *testing.T) {
	s := newTestStore(t)
	b := listBlog(t, s, "blog.example.com", "en")
	long := entry("a", nil, false)
	long.Excerpt = strings.Repeat("x", 141)
	err := s.SyncSnapshot(context.Background(), b.ID, []model.Entry{long}, FetchState{NextFetchAt: time.Now()})
	if err == nil {
		t.Fatal("a 141 character excerpt was stored")
	}
}

func TestStreamRules(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Add(-24 * time.Hour)

	zh := listBlog(t, s, "zh.example.com", "zh-CN")
	sync(t, s, zh.ID,
		entry("zh1", at(day.Add(1*time.Hour)), true),
		entry("zh2", at(day.Add(2*time.Hour)), true),
		entry("zh3", at(day.Add(3*time.Hour)), true),
		entry("zh4", at(day.Add(4*time.Hour)), true),
		entry("zh5", at(day.Add(5*time.Hour)), true),
		entry("untrusted", at(now.Add(-2*time.Hour)), false),
		entry("undated", nil, false),
		entry("future", at(now.Add(72*time.Hour)), true),
		entry("soon", at(now.Add(30*time.Minute)), true),
		entry("old", at(now.Add(-40*24*time.Hour)), true),
	)
	en := listBlog(t, s, "en.example.com", "en")
	sync(t, s, en.ID, entry("en1", at(now.Add(-3*time.Hour)), true), entry("en2", at(now.Add(-50*time.Hour)), true))

	paused := listBlog(t, s, "paused.example.com", "en")
	sync(t, s, paused.ID, entry("p1", at(now.Add(-time.Hour)), true))
	st := model.BlogPaused
	if _, err := s.UpdateBlog(ctx, paused.Host, BlogUpdate{Status: &st}); err != nil {
		t.Fatal(err)
	}
	gone := listBlog(t, s, "gone.example.com", "en")
	sync(t, s, gone.ID, entry("g1", at(now.Add(-time.Hour)), true))
	if err := s.RecordFailure(ctx, gone.ID, Failure{Error: "HTTP 410", NextFetchAt: now, Gone: true}); err != nil {
		t.Fatal(err)
	}
	stale := listBlog(t, s, "stale.example.com", "en")
	sync(t, s, stale.ID, entry("s1", at(now.Add(-time.Hour)), true))
	s.exec(t, `UPDATE blogs SET last_succeeded_at = now() - interval '8 days' WHERE id = $1`, stale.ID)

	all, err := s.Stream(ctx, StreamQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(identities(all, func(e StreamEntry) string { return e.Identity }), ",")
	if want := "soon,en1,zh5,zh4,zh3,en2"; got != want {
		t.Fatalf("stream = %s, want %s", got, want)
	}

	zhOnly, err := s.Stream(ctx, StreamQuery{Lang: "zh", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(identities(zhOnly, func(e StreamEntry) string { return e.Identity }), ","); got != "soon,zh5,zh4,zh3" {
		t.Errorf("zh stream = %s", got)
	}
	if zhOnly[0].Blog.Host != "zh.example.com" || zhOnly[0].Blog.Language != "zh-CN" {
		t.Errorf("blog ref = %+v", zhOnly[0].Blog)
	}

	// Paging with a cursor visits the same entries in the same order.
	var paged []string
	var cursor *Cursor
	for {
		page, err := s.Stream(ctx, StreamQuery{Limit: 2, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for _, e := range page {
			paged = append(paged, e.Identity)
		}
		last := page[len(page)-1]
		cursor = &Cursor{At: *last.PublishedAt, ID: last.ID}
	}
	if strings.Join(paged, ",") != got {
		t.Errorf("paged = %v, want %s", paged, got)
	}
}

func TestDirectoryAndVisibleBlog(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	recent := listBlog(t, s, "recent.example.com", "en")
	sync(t, s, recent.ID, entry("r1", at(now.Add(-time.Hour)), true), entry("r2", at(now.Add(72*time.Hour)), true))
	older := listBlog(t, s, "older.example.com", "zh-CN")
	sync(t, s, older.ID, entry("o1", at(now.Add(-48*time.Hour)), true))
	undated := listBlog(t, s, "undated.example.com", "en")
	sync(t, s, undated.ID, entry("u1", nil, false))
	listBlog(t, s, "never-fetched.example.com", "en")

	blogs, err := s.Directory(ctx, DirectoryQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(identities(blogs, func(b ListedBlog) string { return b.Host }), ",")
	if want := "recent.example.com,older.example.com,undated.example.com"; got != want {
		t.Fatalf("directory = %s, want %s", got, want)
	}
	if blogs[0].LastPublishedAt == nil || blogs[0].LastPublishedAt.After(now) {
		t.Errorf("last published must ignore future entries: %v", blogs[0].LastPublishedAt)
	}
	if blogs[2].LastPublishedAt != nil {
		t.Errorf("undated blog last published = %v", blogs[2].LastPublishedAt)
	}

	var paged []string
	var cursor *Cursor
	for {
		page, err := s.Directory(ctx, DirectoryQuery{Limit: 1, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		last := page[0]
		paged = append(paged, last.Host)
		c := Cursor{At: time.Unix(0, 0), ID: last.ID}
		if last.LastPublishedAt != nil {
			c.At = *last.LastPublishedAt
		}
		cursor = &c
	}
	if strings.Join(paged, ",") != got {
		t.Errorf("paged directory = %v, want %s", paged, got)
	}

	zh, err := s.Directory(ctx, DirectoryQuery{Lang: "zh", Limit: 100})
	if err != nil || len(zh) != 1 || zh[0].Host != "older.example.com" {
		t.Errorf("zh directory = %v, %v", zh, err)
	}

	_, entries, err := s.VisibleBlog(ctx, "recent.example.com")
	if err != nil || len(entries) != 1 || entries[0].Identity != "r1" {
		t.Errorf("visible blog entries = %v, %v (future entries must be hidden)", entries, err)
	}
	if _, _, err := s.VisibleBlog(ctx, "never-fetched.example.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("never fetched blog: err = %v, want ErrNotFound", err)
	}
	if _, _, err := s.VisibleBlog(ctx, "missing.example.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing blog: err = %v, want ErrNotFound", err)
	}
}

func TestClaimDue(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for i := range 3 {
		listBlog(t, s, fmt.Sprintf("b%d.example.com", i), "en")
	}
	paused := listBlog(t, s, "paused.example.com", "en")
	st := model.BlogPaused
	if _, err := s.UpdateBlog(ctx, paused.Host, BlogUpdate{Status: &st}); err != nil {
		t.Fatal(err)
	}

	first, err := s.ClaimDue(ctx, 2, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.ClaimDue(ctx, 2, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	third, err := s.ClaimDue(ctx, 2, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || len(second) != 1 || len(third) != 0 {
		t.Fatalf("claimed %d, %d, %d; want 2, 1, 0 and never the paused blog", len(first), len(second), len(third))
	}
	c := first[0]
	if c.HasEntries || c.FetchInterval != time.Hour || c.FeedURL == "" || !c.ShowExcerpt {
		t.Errorf("claimed = %+v", c)
	}

	sync(t, s, c.ID, entry("a", nil, false))
	s.exec(t, `UPDATE blogs SET next_fetch_at = now() - interval '1 second' WHERE id = $1`, c.ID)
	again, err := s.ClaimDue(ctx, 5, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 || !again[0].HasEntries || again[0].ETag != `"v1"` {
		t.Errorf("reclaimed = %+v", again)
	}
}

func TestFailureGoneAndMaintenance(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()
	b := listBlog(t, s, "blog.example.com", "en")
	sync(t, s, b.ID, entry("a", at(now.Add(-time.Hour)), true))

	if err := s.RecordFailure(ctx, b.ID, Failure{Error: "HTTP 410", NextFetchAt: now, Gone: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordFailure(ctx, b.ID, Failure{Error: "timeout", NextFetchAt: now}); err != nil {
		t.Fatal(err)
	}
	blog, err := s.Blog(ctx, b.Host)
	if err != nil {
		t.Fatal(err)
	}
	if blog.ConsecutiveFailures != 2 || blog.LastError != "timeout" || blog.GoneSince == nil {
		t.Fatalf("after failures = %+v; a plain failure must keep gone_since", blog)
	}
	if _, _, err := s.VisibleBlog(ctx, b.Host); !errors.Is(err, ErrNotFound) {
		t.Errorf("a gone blog is still visible")
	}

	if err := s.RecordUnchanged(ctx, b.ID, FetchState{FetchInterval: time.Hour, NextFetchAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	blog, _ = s.Blog(ctx, b.Host)
	if blog.GoneSince != nil || blog.ConsecutiveFailures != 0 || blog.ETag != `"v1"` {
		t.Fatalf("success must clear the exit signal and keep the ETag: %+v", blog)
	}

	if err := s.RecordFailure(ctx, b.ID, Failure{Error: "HTTP 410", NextFetchAt: now, Gone: true}); err != nil {
		t.Fatal(err)
	}
	s.exec(t, `UPDATE blogs SET gone_since = now() - interval '8 days' WHERE id = $1`, b.ID)

	sub, err := s.CreateSubmission(ctx, model.Submission{Host: "old.example.org", SiteURL: "https://old.example.org/", FeedURL: "https://old.example.org/feed", Report: model.CheckReport{Passed: true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RejectSubmission(ctx, sub.ID, "alice", "not a blog"); err != nil {
		t.Fatal(err)
	}
	s.exec(t, `UPDATE submissions SET reviewed_at = now() - interval '100 days' WHERE id = $1::uuid`, sub.ID)

	m, err := s.Maintain(ctx, now.Add(-7*24*time.Hour), now.Add(-90*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !m.Ran || len(m.Removed) != 1 || m.Removed[0] != b.Host || m.Purged != 1 {
		t.Fatalf("maintenance = %+v", m)
	}
	hs, err := s.HostState(ctx, b.Host)
	if err != nil {
		t.Fatal(err)
	}
	if hs.Listed || hs.Excluded == nil || hs.Excluded.Reason != model.ExcludedOptOut {
		t.Errorf("a removed gone blog must be excluded as opted out: %+v", hs)
	}
	var entries int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM entries`).Scan(&entries); err != nil || entries != 0 {
		t.Errorf("entries left = %d, %v", entries, err)
	}
}

func TestSubmissionLifecycle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	host := "new.example.com"

	hs, err := s.HostState(ctx, host)
	if err != nil || hs.Listed || hs.Excluded != nil || hs.PendingID != "" {
		t.Fatalf("fresh host state = %+v, %v", hs, err)
	}

	report := model.CheckReport{
		InputURL: "https://new.example.com/", FeedURL: "https://new.example.com/atom.xml", Generator: model.GeneratorHexo,
		Title: "New Blog", Language: "zh-CN", Passed: true,
		Problems: []model.Problem{{Code: model.ProblemNoConditionalGet, Severity: model.SeverityInfo, Hint: "localized text"}},
	}
	sub, err := s.CreateSubmission(ctx, model.Submission{Host: host, SiteURL: "https://new.example.com/", FeedURL: report.FeedURL, Note: "hello", Report: report})
	if err != nil {
		t.Fatal(err)
	}
	if sub.Status != model.SubmissionPending || len(sub.ID) != 36 {
		t.Fatalf("created = %+v", sub)
	}
	if sub.Report.Problems[0].Hint != "" {
		t.Errorf("hints must not be stored: %+v", sub.Report.Problems)
	}
	if _, err := s.CreateSubmission(ctx, model.Submission{Host: host, SiteURL: "x", FeedURL: "y"}); !errors.Is(err, ErrPending) {
		t.Errorf("second pending submission: err = %v, want ErrPending", err)
	}
	if hs, _ := s.HostState(ctx, host); hs.PendingID != sub.ID {
		t.Errorf("pending id = %q, want %q", hs.PendingID, sub.ID)
	}
	if _, err := s.Submission(ctx, "not-a-uuid"); !errors.Is(err, ErrNotFound) {
		t.Errorf("bad id: err = %v", err)
	}
	pending, err := s.Submissions(ctx, model.SubmissionPending, 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending = %v, %v", pending, err)
	}

	blog, err := s.ApproveSubmission(ctx, sub.ID, Approval{Reviewer: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if blog.Host != host || blog.Name != "New Blog" || blog.Language != "zh-CN" || blog.Generator != model.GeneratorHexo ||
		blog.FeedURL != report.FeedURL || !blog.ShowExcerpt {
		t.Errorf("approved blog = %+v", blog)
	}
	got, err := s.Submission(ctx, sub.ID)
	if err != nil || got.Status != model.SubmissionApproved || got.ReviewedBy != "alice" || got.BlogID == nil || *got.BlogID != blog.ID {
		t.Errorf("approved submission = %+v, %v", got, err)
	}
	if _, err := s.ApproveSubmission(ctx, sub.ID, Approval{Reviewer: "bob"}); !errors.Is(err, ErrNotPending) {
		t.Errorf("approving twice: err = %v", err)
	}
	if hs, _ := s.HostState(ctx, host); !hs.Listed || hs.PendingID != "" {
		t.Errorf("after approval host state = %+v", hs)
	}

	other, err := s.CreateSubmission(ctx, model.Submission{Host: "other.example.com", SiteURL: "https://other.example.com/", FeedURL: "https://other.example.com/feed"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RejectSubmission(ctx, other.ID, "alice", "not a personal blog"); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Submission(ctx, other.ID)
	if got.Status != model.SubmissionRejected || got.ReviewNote != "not a personal blog" {
		t.Errorf("rejected = %+v", got)
	}
	if err := s.RejectSubmission(ctx, other.ID, "alice", "again"); !errors.Is(err, ErrNotPending) {
		t.Errorf("rejecting twice: err = %v", err)
	}
}

func TestExclusions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	b := listBlog(t, s, "leaving.example.com", "en")
	sync(t, s, b.ID, entry("a", nil, false))

	if err := s.DeleteBlog(ctx, b.Host, model.ExcludedOptOut, "asked by email"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteBlog(ctx, b.Host, "", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleting twice: err = %v", err)
	}
	if _, err := s.CreateBlog(ctx, NewBlog{Host: b.Host, Name: "x", SiteURL: "x", FeedURL: "x", Language: "en"}); !errors.Is(err, ErrExcluded) {
		t.Errorf("relisting an excluded host: err = %v", err)
	}
	ex, err := s.ExcludedHosts(ctx)
	if err != nil || len(ex) != 1 || ex[0].Note != "asked by email" {
		t.Fatalf("exclusions = %v, %v", ex, err)
	}
	if err := s.DeleteExcludedHost(ctx, b.Host); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBlog(ctx, NewBlog{Host: b.Host, Name: "x", SiteURL: "x", FeedURL: "x", Language: "en"}); err != nil {
		t.Errorf("relisting after the exclusion was lifted: %v", err)
	}
	if _, err := s.CreateBlog(ctx, NewBlog{Host: b.Host, Name: "x", SiteURL: "x", FeedURL: "x", Language: "en"}); !errors.Is(err, ErrListed) {
		t.Errorf("listing twice: err = %v", err)
	}
}

func TestUpdateBlogForgetsTheFetchCache(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	b := listBlog(t, s, "blog.example.com", "en")
	sync(t, s, b.ID, entry("a", nil, false))

	name := "New name"
	blog, err := s.UpdateBlog(ctx, b.Host, BlogUpdate{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if blog.Name != name || blog.ETag == "" || blog.BodyHash == nil {
		t.Fatalf("a rename must keep the fetch cache: %+v", blog)
	}

	off := false
	blog, err = s.UpdateBlog(ctx, b.Host, BlogUpdate{ShowExcerpt: &off})
	if err != nil {
		t.Fatal(err)
	}
	if blog.ShowExcerpt || blog.ETag != "" || blog.BodyHash != nil || time.Until(blog.NextFetchAt) > time.Minute {
		t.Fatalf("hiding excerpts must force a rebuild: %+v", blog)
	}

	note := "spam reports"
	paused := model.BlogPaused
	blog, err = s.UpdateBlog(ctx, b.Host, BlogUpdate{Status: &paused, StatusNote: &note})
	if err != nil || blog.Status != model.BlogPaused || blog.StatusNote != note {
		t.Fatalf("pause = %+v, %v", blog, err)
	}
	if _, err := s.UpdateBlog(ctx, "missing.example.com", BlogUpdate{Name: &name}); !errors.Is(err, ErrNotFound) {
		t.Errorf("updating a missing blog: err = %v", err)
	}
	if err := s.FetchNow(ctx, "missing.example.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("fetching a missing blog: err = %v", err)
	}
}

func TestTagsFollowTheirTitle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	b := listBlog(t, s, "tags.example.com", "zh")
	now := time.Now().UTC()
	first := entry("a", at(now.Add(-time.Hour)), true)
	first.Categories = []string{"Go", "Web"}
	second := entry("b", at(now.Add(-2*time.Hour)), true)
	sync(t, s, b.ID, first, second)

	jobs, err := s.Untagged(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := identities(jobs, func(j TagJob) string { return j.Title }); strings.Join(got, ",") != "Title a,Title b" {
		t.Fatalf("untagged = %v, want newest first", got)
	}
	if strings.Join(jobs[0].Categories, ",") != "Go,Web" || jobs[0].Language != "zh" {
		t.Errorf("job = %+v", jobs[0])
	}
	for _, j := range jobs {
		if err := s.SetTags(ctx, j.EntryID, j.Title, []string{"backend"}); err != nil {
			t.Fatal(err)
		}
	}
	// A title that changed after the job was read is left for its own turn.
	if err := s.SetTags(ctx, jobs[0].EntryID, "an older title", []string{"ai"}); err != nil {
		t.Fatal(err)
	}

	// The same title keeps its tags through a sync; a new one loses them.
	second.Title = "a new title"
	sync(t, s, b.ID, first, second)
	jobs, err = s.Untagged(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Title != "a new title" {
		t.Fatalf("untagged after sync = %+v", jobs)
	}

	page, err := s.Stream(ctx, StreamQuery{Tag: "backend", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 || page[0].Title != "Title a" || strings.Join(page[0].Tags, ",") != "backend" {
		t.Fatalf("stream tagged backend = %+v", page)
	}
	if page, err := s.Stream(ctx, StreamQuery{Tag: "design", Limit: 10}); err != nil || len(page) != 0 {
		t.Errorf("stream tagged design = %+v, %v", page, err)
	}

	// New default tags send the blog's entries back to the tagger.
	tags := []string{"backend", "ops"}
	got, err := s.UpdateBlog(ctx, b.Host, BlogUpdate{DefaultTags: &tags})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.DefaultTags, ",") != "backend,ops" {
		t.Errorf("default tags = %v", got.DefaultTags)
	}
	if jobs, err := s.Untagged(ctx, 10); err != nil || len(jobs) != 2 || strings.Join(jobs[0].BlogTags, ",") != "backend,ops" {
		t.Errorf("untagged after new default tags = %+v, %v", jobs, err)
	}
}

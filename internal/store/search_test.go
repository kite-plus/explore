package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kite-plus/explore/internal/model"
)

func TestParseSearch(t *testing.T) {
	cases := []struct {
		q    string
		want []string // query, then title, of each term
	}{
		{"摄影", []string{"('摄影')", "('摄影':A)"}},
		{"独立博客", []string{"('独立' <-> '立博' <-> '博客')", "('独立':A <-> '立博':A <-> '博客':A)"}},
		// One character may begin a longer run in a post.
		{"猫", []string{"('猫':*)", "('猫':*A)"}},
		// A run followed by more of the term ends there in a post too.
		{"博客RSS", []string{"('博客' <-> '客' <-> 'rss')", "('博客':A <-> '客':A <-> 'rss':A)"}},
		{"用Rust写", []string{"('用' <-> 'rust' <-> '写':*)", "('用':A <-> 'rust':A <-> '写':*A)"}},
		// Punctuation alone splits a word; inside a word it stays.
		{"你好，世界 node.js", []string{"('你好')", "('你好':A)", "('世界')", "('世界':A)", "('node.js')", "('node.js':A)"}},
		{"it's a\\b", []string{"('it''s')", "('it''s':A)", "('a\\\\b')", "('a\\\\b':A)"}},
		{"  !!! ", nil},
	}
	for _, c := range cases {
		var got []string
		for _, term := range parseSearch(c.q) {
			got = append(got, term.query, term.title)
		}
		if strings.Join(got, " | ") != strings.Join(c.want, " | ") {
			t.Errorf("parseSearch(%q) = %q, want %q", c.q, got, c.want)
		}
	}

	terms := parseSearch("AI 摄 rust a b c d e f g")
	if len(terms) != MaxSearchTerms {
		t.Fatalf("terms = %d, want %d", len(terms), MaxSearchTerms)
	}
	if terms[0].substr || !terms[1].substr || !terms[2].substr {
		t.Errorf("substr = %v %v %v, want false true true", terms[0].substr, terms[1].substr, terms[2].substr)
	}
}

func post(id, title, excerpt string, published *time.Time) model.Entry {
	return model.Entry{Identity: id, URL: "https://x/" + id, Title: title, Excerpt: excerpt, PublishedAt: published, DateTrusted: published != nil}
}

func searchIDs(t *testing.T, s *Store, q SearchQuery) string {
	t.Helper()
	found, err := s.SearchEntries(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(identities(found, func(e FoundEntry) string { return e.Identity }), ",")
}

func TestSearchEntries(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	ago := func(h int) *time.Time { return at(now.Add(-time.Duration(h) * time.Hour)) }

	zh := listBlog(t, s, "zh.example.com", "zh-CN")
	sync(t, s, zh.ID,
		post("film", "胶片摄影入门", "从一台旧相机开始", ago(30)),
		post("workflow", "我的摄影后期流程", "", ago(10)),
		post("kyoto", "冬天的京都", "全程手机摄影，分享几个构图习惯", ago(5)),
		post("today", "今天写了什么", "", ago(2)),
		post("apart", "独立的博客", "", ago(3)),
		post("rust", "我用Rust写了一个博客", "", ago(4)),
		post("kitten", "小猫", "", ago(6)),
		post("cat", "猫咪的一天", "", ago(7)),
		post("undated", "摄影随笔", "", nil),
		post("future", "未来的摄影", "", at(now.Add(72*time.Hour))),
	)
	en := listBlog(t, s, "en.example.com", "en")
	sync(t, s, en.ID,
		post("ai", "AI is everywhere", "", ago(1)),
		post("said", "He said hello", "and sent an email", ago(1)),
		post("photo", "Street photography", "摄影 in English", ago(8)),
	)
	paused := listBlog(t, s, "paused.example.com", "zh-CN")
	sync(t, s, paused.ID, post("hidden", "摄影", "", ago(1)))
	st := model.BlogPaused
	if _, err := s.UpdateBlog(ctx, paused.Host, BlogUpdate{Status: &st}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		q, lang, want string
	}{
		// Title matches first, newest first; then excerpt matches; undated last
		// in its group; no future post, no paused blog.
		{"摄影", "", "workflow,film,undated,kyoto,photo"},
		{"摄影", "zh", "workflow,film,undated,kyoto"},
		{"写了", "", "today,rust"},
		{"独立博客", "", ""},
		{"用Rust写", "", "rust"},
		{"rust 博客", "", "rust"},
		{"猫", "", "kitten,cat"},
		{"AI", "", "ai"},
		{"EMAIL", "", "said"},
		{"!!!", "", ""},
	}
	for _, c := range cases {
		if got := searchIDs(t, s, SearchQuery{Text: c.q, Lang: c.lang, Limit: 100}); got != c.want {
			t.Errorf("search %q (lang %q) = %s, want %s", c.q, c.lang, got, c.want)
		}
	}

	// Paging with a cursor visits the same entries in the same order.
	var paged []string
	var cursor *SearchCursor
	for {
		page, err := s.SearchEntries(ctx, SearchQuery{Text: "摄影", Limit: 2, Cursor: cursor})
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
		cursor = &SearchCursor{Top: last.Top, At: last.SortAt, ID: last.ID}
	}
	if got := strings.Join(paged, ","); got != "workflow,film,undated,kyoto,photo" {
		t.Errorf("paged = %s", got)
	}

	if n, err := s.CountSearchEntries(ctx, "摄影", "", 1000); err != nil || n != 5 {
		t.Errorf("count = %d, %v, want 5", n, err)
	}
	if n, err := s.CountSearchEntries(ctx, "摄影", "", 3); err != nil || n != 3 {
		t.Errorf("capped count = %d, %v, want 3", n, err)
	}
}

func TestSearchBlogs(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	add := func(host, lang, name, description string, latest time.Duration) {
		b := listBlog(t, s, host, lang)
		s.exec(t, `UPDATE blogs SET name = $2, description = $3 WHERE id = $1`, b.ID, name, description)
		sync(t, s, b.ID, entry(host, at(now.Add(-latest)), true))
	}
	add("www.ruanyifeng.com", "zh-CN", "阮一峰的网络日志", "", time.Hour)
	add("mactalk.example.com", "zh-CN", "MacTalk", "池建强的随想录", 2*time.Hour)
	add("lens.example.com", "zh-CN", "光影手记", "一个摄影爱好者的胶片记录", 3*time.Hour)
	add("photo.example.com", "zh-CN", "摄影札记", "", 4*time.Hour)
	add("daily.example.com", "en", "Daily Notes", "Notes on Rust and AI", 5*time.Hour)

	search := func(q, lang string) (string, int) {
		t.Helper()
		found, total, err := s.SearchBlogs(ctx, SearchQuery{Text: q, Lang: lang, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(identities(found, func(b FoundBlog) string { return b.Host }), ","), total
	}
	cases := []struct {
		q, lang, want string
	}{
		{"ruanyifeng", "", "www.ruanyifeng.com"},
		{"一峰", "", "www.ruanyifeng.com"},
		{"talk", "", "mactalk.example.com"},
		// A name match comes before a description match, though older.
		{"摄影", "", "photo.example.com,lens.example.com"},
		// Short words match whole words only: "ai" is not in "daily".
		{"ai", "", "daily.example.com"},
		{"ai", "zh", ""},
		{"rust 光影", "", ""},
	}
	for _, c := range cases {
		if got, _ := search(c.q, c.lang); got != c.want {
			t.Errorf("search blogs %q (lang %q) = %s, want %s", c.q, c.lang, got, c.want)
		}
	}
	if _, total := search("example", ""); total != 4 {
		t.Errorf("total = %d, want 4", total)
	}

	var paged []string
	var cursor *SearchCursor
	for {
		page, _, err := s.SearchBlogs(ctx, SearchQuery{Text: "example", Limit: 1, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		paged = append(paged, page[0].Host)
		cursor = &SearchCursor{Top: page[0].Top, At: page[0].SortAt, ID: page[0].ID}
	}
	if got, _ := search("example", ""); strings.Join(paged, ",") != got {
		t.Errorf("paged = %v, want %s", paged, got)
	}
}

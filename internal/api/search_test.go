package api_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/kite-plus/explore/internal/model"
)

type searchOut struct {
	Data        []entryOut `json:"data"`
	NextCursor  *string    `json:"next_cursor"`
	Total       *int       `json:"total"`
	TotalCapped bool       `json:"total_capped"`
}

func titled(id string, hoursAgo int, title, excerpt string) model.Entry {
	p := post(id, hoursAgo, excerpt)
	p.Title = title
	return p
}

func searchPath(kind string, params ...string) string {
	q := url.Values{}
	for i := 0; i+1 < len(params); i += 2 {
		q.Set(params[i], params[i+1])
	}
	return "/api/v1/search/" + kind + "?" + q.Encode()
}

func TestSearchEntries(t *testing.T) {
	e := newEnv(t, false)
	e.seed("zh.example.com", "zh-CN",
		titled("film", 3, "胶片摄影入门", ""),
		titled("kyoto", 1, "冬天的京都", "全程手机摄影"),
		titled("other", 2, "读书笔记", ""),
	)
	e.seed("en.example.com", "en", titled("street", 4, "Street photography", "摄影 in English"))

	w := e.get(searchPath("entries", "q", "摄影"))
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Cache-Control"), "public") {
		t.Fatalf("search = %d %s", w.Code, w.Body.String())
	}
	all := decode[searchOut](t, w)
	var ids []string
	for _, d := range all.Data {
		ids = append(ids, d.Title)
	}
	// Title matches first, then the rest, newest first within each.
	if strings.Join(ids, ",") != "胶片摄影入门,冬天的京都,Street photography" || all.NextCursor != nil {
		t.Fatalf("search = %v, next %v", ids, all.NextCursor)
	}
	if all.Total == nil || *all.Total != 3 || all.TotalCapped {
		t.Errorf("total = %v %v", all.Total, all.TotalCapped)
	}
	if b := all.Data[0].Blog; b == nil || b.Host != "zh.example.com" || b.Language != "zh-CN" {
		t.Errorf("blog = %+v", b)
	}

	first := decode[searchOut](t, e.get(searchPath("entries", "q", "摄影", "limit", "2")))
	if len(first.Data) != 2 || first.NextCursor == nil || first.Total == nil || *first.Total != 3 {
		t.Fatalf("first page = %+v", first)
	}
	second := decode[searchOut](t, e.get(searchPath("entries", "q", "摄影", "limit", "2", "cursor", *first.NextCursor)))
	if len(second.Data) != 1 || second.Data[0].Title != "Street photography" || second.NextCursor != nil || second.Total != nil {
		t.Errorf("second page = %+v", second)
	}

	en := decode[searchOut](t, e.get(searchPath("entries", "q", "摄影", "lang", "en")))
	if len(en.Data) != 1 || en.Data[0].Title != "Street photography" {
		t.Errorf("lang en = %+v", en.Data)
	}
	none := decode[searchOut](t, e.get(searchPath("entries", "q", "!!!")))
	if len(none.Data) != 0 || none.Total == nil || *none.Total != 0 {
		t.Errorf("punctuation only = %+v", none)
	}

	for _, c := range []struct{ path, code string }{
		{"/api/v1/search/entries", "invalid_request"},
		{searchPath("entries", "q", "   "), "invalid_request"},
		{searchPath("entries", "q", strings.Repeat("长", 101)), "invalid_request"},
		{searchPath("entries", "q", "摄影", "limit", "0"), "invalid_request"},
		{searchPath("entries", "q", "摄影", "lang", "x"), "invalid_request"},
		{searchPath("entries", "q", "摄影", "cursor", "not-a-cursor"), "invalid_cursor"},
		{searchPath("blogs", "q", ""), "invalid_request"},
	} {
		w := e.get(c.path)
		if code, _ := errorCode(t, w); w.Code != http.StatusBadRequest || code != c.code {
			t.Errorf("%s = %d %s, want 400 %s", c.path, w.Code, code, c.code)
		}
	}
}

func TestSearchBlogs(t *testing.T) {
	e := newEnv(t, false)
	e.seed("lens.example.com", "zh-CN", post("a", 1, ""))
	e.seed("photo.example.com", "zh-CN", post("b", 2, ""))
	e.seed("words.example.com", "en", post("c", 3, ""))
	ctx := context.Background()
	lens, err := e.s.Blog(ctx, "lens.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.s.SetDescription(ctx, lens.ID, "一个摄影爱好者的胶片记录"); err != nil {
		t.Fatal(err)
	}

	type blogsOut struct {
		Data []struct {
			Host string `json:"host"`
		} `json:"data"`
		NextCursor *string `json:"next_cursor"`
		Total      *int    `json:"total"`
	}
	hosts := func(out blogsOut) string {
		var h []string
		for _, b := range out.Data {
			h = append(h, b.Host)
		}
		return strings.Join(h, ",")
	}

	out := decode[blogsOut](t, e.get(searchPath("blogs", "q", "摄影")))
	if hosts(out) != "lens.example.com" || out.Total == nil || *out.Total != 1 {
		t.Errorf("description search = %s %v", hosts(out), out.Total)
	}
	// Hosts match as a substring.
	out = decode[blogsOut](t, e.get(searchPath("blogs", "q", "photo")))
	if hosts(out) != "photo.example.com" {
		t.Errorf("host search = %s", hosts(out))
	}
	out = decode[blogsOut](t, e.get(searchPath("blogs", "q", "example", "limit", "2")))
	if hosts(out) != "lens.example.com,photo.example.com" || out.NextCursor == nil || out.Total == nil || *out.Total != 3 {
		t.Fatalf("first page = %s %v %v", hosts(out), out.NextCursor, out.Total)
	}
	out = decode[blogsOut](t, e.get(searchPath("blogs", "q", "example", "limit", "2", "cursor", *out.NextCursor)))
	if hosts(out) != "words.example.com" || out.NextCursor != nil || out.Total != nil {
		t.Errorf("second page = %s %v %v", hosts(out), out.NextCursor, out.Total)
	}
	out = decode[blogsOut](t, e.get(searchPath("blogs", "q", "example", "lang", "en")))
	if hosts(out) != "words.example.com" {
		t.Errorf("lang en = %s", hosts(out))
	}
}

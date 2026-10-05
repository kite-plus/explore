package api_test

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"
)

type noticeOut struct {
	ID         string  `json:"id"`
	Kind       string  `json:"kind"`
	Title      string  `json:"title"`
	Body       *string `json:"body"`
	URL        string  `json:"url"`
	SourceName string  `json:"source_name"`
	Position   int     `json:"position"`
}

func TestNoticesAPI(t *testing.T) {
	e := newEnv(t, true)
	create := func(body string) int64 {
		t.Helper()
		w := e.do(req{method: http.MethodPost, path: "/api/v1/admin/notices", body: body, admin: true})
		if w.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", w.Code, w.Body.String())
		}
		return decode[struct{ ID int64 }](t, w).ID
	}
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)

	pinned := create(`{"kind":"notice","title":"Kite for iOS 上架了","body":"第一段。\n\n第二段。","source_name":"Kite Plus","position":0,"enabled":true}`)
	ad := create(`{"kind":"ad","title":"一元建站","url":"https://ads.example.com/kite","source_name":"某某云","position":4,"audience":"zh","starts_at":"` + past + `","enabled":true}`)
	create(`{"kind":"notice","title":"草稿","body":"还没发布。","source_name":"Kite Plus","enabled":false}`)
	create(`{"kind":"notice","title":"明天才显示","body":"稍后。","source_name":"Kite Plus","starts_at":"` + future + `","enabled":true}`)
	create(`{"kind":"ad","title":"English only","url":"https://ads.example.com/en","source_name":"Acme","position":2,"audience":"en","enabled":true}`)

	list := func(path string) []noticeOut {
		t.Helper()
		w := e.get(path)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if got := w.Header().Get("Cache-Control"); got != "public, max-age=60" {
			t.Fatalf("%s cache: %q", path, got)
		}
		return decode[struct{ Data []noticeOut }](t, w).Data
	}
	everyone := list("/api/v1/notices")
	if len(everyone) != 1 || everyone[0].ID != strconv.FormatInt(pinned, 10) || everyone[0].Body != nil {
		t.Fatalf("notices for everyone: %+v", everyone)
	}
	zh := list("/api/v1/notices?audience=zh")
	if len(zh) != 2 || zh[0].Position != 0 || zh[1].Kind != "ad" || zh[1].URL != "https://ads.example.com/kite" {
		t.Fatalf("notices for zh: %+v", zh)
	}
	if got := e.get("/api/v1/notices?audience=fr"); got.Code != http.StatusBadRequest {
		t.Fatalf("unknown audience: %d", got.Code)
	}

	one := e.get(fmt.Sprintf("/api/v1/notices/%d", pinned))
	if one.Code != http.StatusOK {
		t.Fatalf("one: %d %s", one.Code, one.Body.String())
	}
	if n := decode[noticeOut](t, one); n.Body == nil || *n.Body != "第一段。\n\n第二段。" {
		t.Fatalf("body: %+v", n)
	}

	// Taking it down hides it from readers but keeps it for maintainers.
	update := e.do(req{method: http.MethodPatch, path: fmt.Sprintf("/api/v1/admin/notices/%d", ad), admin: true,
		body: `{"kind":"ad","title":"一元建站","url":"https://ads.example.com/kite","source_name":"某某云","position":4,"audience":"zh","enabled":false}`})
	if update.Code != http.StatusOK {
		t.Fatalf("update: %d %s", update.Code, update.Body.String())
	}
	if got := e.get(fmt.Sprintf("/api/v1/notices/%d", ad)); got.Code != http.StatusNotFound {
		t.Fatalf("disabled notice is public: %d", got.Code)
	}
	all := decode[struct{ Data []struct{ ID int64 } }](t, e.do(req{method: http.MethodGet, path: "/api/v1/admin/notices", admin: true}))
	if len(all.Data) != 5 {
		t.Fatalf("admin list: %+v", all)
	}

	for _, body := range []string{
		`{"kind":"banner","title":"x","body":"x","source_name":"x"}`,
		`{"kind":"notice","title":"","body":"x","source_name":"x"}`,
		`{"kind":"notice","title":"x","source_name":"x"}`,
		`{"kind":"ad","title":"x","url":"javascript:alert(1)","source_name":"x"}`,
		`{"kind":"notice","title":"x","body":"x","source_name":"x","position":51}`,
		`{"kind":"notice","title":"x","body":"x","source_name":"x","starts_at":"` + future + `","ends_at":"` + past + `"}`,
	} {
		if w := e.do(req{method: http.MethodPost, path: "/api/v1/admin/notices", body: body, admin: true}); w.Code != http.StatusBadRequest {
			t.Errorf("accepted %s: %d", body, w.Code)
		}
	}

	if w := e.do(req{method: http.MethodDelete, path: fmt.Sprintf("/api/v1/admin/notices/%d", pinned), admin: true}); w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", w.Code)
	}
	if got := e.get(fmt.Sprintf("/api/v1/notices/%d", pinned)); got.Code != http.StatusNotFound {
		t.Fatalf("deleted notice: %d", got.Code)
	}
	if w := e.get("/api/v1/admin/notices"); w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
		t.Fatalf("admin list without a session: %d", w.Code)
	}
}

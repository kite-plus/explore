package api_test

import (
	"net/http"
	"strconv"
	"testing"
)

func TestAdminOperationsAPI(t *testing.T) {
	e := newEnv(t, true)
	e.seed("operations.example", "zh", post("one", 2, "sample"))

	overview := e.do(req{method: http.MethodGet, path: "/api/v1/admin/overview", admin: true})
	if overview.Code != http.StatusOK {
		t.Fatalf("overview: %d %s", overview.Code, overview.Body.String())
	}
	stats := decode[struct {
		Stats struct{ Blogs, Entries int64 } `json:"stats"`
	}](t, overview)
	if stats.Stats.Blogs != 1 || stats.Stats.Entries != 1 {
		t.Fatalf("overview stats: %+v", stats)
	}

	list := e.do(req{method: http.MethodGet, path: "/api/v1/admin/entries?q=operations.example", admin: true})
	if list.Code != http.StatusOK {
		t.Fatalf("entries: %d %s", list.Code, list.Body.String())
	}
	entries := decode[struct{ Data []struct{ ID int64 } }](t, list)
	if len(entries.Data) != 1 {
		t.Fatalf("entries: %+v", entries)
	}
	id := strconv.FormatInt(entries.Data[0].ID, 10)
	hide := e.do(req{method: http.MethodPatch, path: "/api/v1/admin/entries/" + id, body: `{"hidden":true,"reason":"审核隐藏"}`, admin: true})
	if hide.Code != http.StatusNoContent {
		t.Fatalf("hide: %d %s", hide.Code, hide.Body.String())
	}
	stream := e.get("/api/v1/entries")
	if stream.Code != http.StatusOK {
		t.Fatalf("stream: %d", stream.Code)
	}
	public := decode[struct{ Data []entryOut }](t, stream)
	if len(public.Data) != 0 {
		t.Fatalf("hidden entry in stream: %+v", public.Data)
	}
	restore := e.do(req{method: http.MethodPatch, path: "/api/v1/admin/entries/" + id, body: `{"hidden":false}`, admin: true})
	if restore.Code != http.StatusNoContent {
		t.Fatalf("restore: %d", restore.Code)
	}

	request := e.do(req{method: http.MethodPost, path: "/api/v1/admin/takedowns", body: `{"target_type":"blog","blog_host":"operations.example","reason":"所有者要求移除博客"}`, admin: true})
	if request.Code != http.StatusCreated {
		t.Fatalf("create request: %d %s", request.Code, request.Body.String())
	}
	queue := e.do(req{method: http.MethodGet, path: "/api/v1/admin/takedowns", admin: true})
	items := decode[struct{ Data []struct{ ID string } }](t, queue)
	if len(items.Data) != 1 {
		t.Fatalf("takedown queue: %+v", items)
	}
	approve := e.do(req{method: http.MethodPost, path: "/api/v1/admin/takedowns/" + items.Data[0].ID + "/review", body: `{"decision":"approved"}`, admin: true})
	if approve.Code != http.StatusNoContent {
		t.Fatalf("approve takedown: %d %s", approve.Code, approve.Body.String())
	}
	if got := e.get("/api/v1/blogs/operations.example"); got.Code != http.StatusNotFound {
		t.Fatalf("takedown still public: %d", got.Code)
	}

	setting := e.do(req{method: http.MethodPatch, path: "/api/v1/admin/settings/registration_enabled", body: `{"value":"false"}`, admin: true})
	if setting.Code != http.StatusNoContent {
		t.Fatalf("setting: %d", setting.Code)
	}
	config := decode[struct {
		RegistrationEnabled bool `json:"registration_enabled"`
	}](t, e.get("/api/v1/site-config"))
	if config.RegistrationEnabled {
		t.Fatal("registration setting not exposed")
	}
	registration := e.do(req{method: http.MethodPost, path: "/api/v1/auth/register", body: `{"email":"user@example.org","password":"long-password-123","display_name":"Reader"}`})
	if registration.Code != http.StatusForbidden {
		t.Fatalf("registration was not disabled: %d", registration.Code)
	}
}

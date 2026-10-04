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

func TestAdminUsersAPI(t *testing.T) {
	e := newEnv(t, true)
	readerCookie, _ := e.session("reader@example.org", false)

	type userOut struct {
		ID             string  `json:"id"`
		Email          string  `json:"email"`
		DisplayName    string  `json:"display_name"`
		DisabledReason string  `json:"disabled_reason"`
		DisabledBy     string  `json:"disabled_by"`
		LastSeenAt     *string `json:"last_seen_at"`
	}
	list := e.do(req{method: http.MethodGet, path: "/api/v1/admin/users?status=active&role=reader&sort=seen&order=desc", admin: true})
	if list.Code != http.StatusOK {
		t.Fatalf("users: %d %s", list.Code, list.Body.String())
	}
	page := decode[struct {
		Data   []userOut
		Total  int64
		Counts struct{ Active, Disabled, Admin, Reader int64 }
	}](t, list)
	if page.Total != 1 || len(page.Data) != 1 || page.Data[0].Email != "reader@example.org" || page.Data[0].LastSeenAt == nil ||
		page.Counts.Active != 1 || page.Counts.Admin != 1 || page.Counts.Reader != 1 {
		t.Fatalf("active readers: %+v", page)
	}
	reader := "/api/v1/admin/users/" + page.Data[0].ID
	for _, query := range []string{"status=gone", "role=owner", "sort=email", "order=up"} {
		if got := e.do(req{method: http.MethodGet, path: "/api/v1/admin/users?" + query, admin: true}); got.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", query, got.Code)
		}
	}

	detail := e.do(req{method: http.MethodGet, path: reader, admin: true})
	if detail.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", detail.Code, detail.Body.String())
	}
	if d := decode[struct{ Sessions []struct{} }](t, detail); len(d.Sessions) != 1 {
		t.Fatalf("detail sessions: %s", detail.Body.String())
	}

	for _, body := range []string{`{"disabled":true}`, `{"disabled":true,"reason":"  "}`, `{"disabled":false,"is_admin":true}`, `{"display_name":""}`, `{}`} {
		if got := e.do(req{method: http.MethodPatch, path: reader, body: body, admin: true}); got.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", body, got.Code)
		}
	}
	if got := e.do(req{method: http.MethodPatch, path: reader, body: `{"disabled":true,"reason":"spam"}`, admin: true}); got.Code != http.StatusNoContent {
		t.Fatalf("disable: %d %s", got.Code, got.Body.String())
	}
	if got := e.do(req{method: http.MethodGet, path: "/api/v1/me", header: map[string]string{"Cookie": readerCookie}}); got.Code != http.StatusUnauthorized {
		t.Fatalf("disabled reader still signed in: %d", got.Code)
	}
	if d := decode[userOut](t, e.do(req{method: http.MethodGet, path: reader, admin: true})); d.DisabledReason != "spam" || d.DisabledBy != adminEmail {
		t.Fatalf("disabled detail: %+v", d)
	}
	if got := e.do(req{method: http.MethodPatch, path: reader, body: `{"is_admin":true}`, admin: true}); got.Code != http.StatusConflict {
		t.Fatalf("admin access for a disabled account: %d", got.Code)
	} else if code, _ := errorCode(t, got); code != "account_disabled" {
		t.Fatalf("admin access for a disabled account: %s", code)
	}
	if got := e.do(req{method: http.MethodPatch, path: reader, body: `{"disabled":false}`, admin: true}); got.Code != http.StatusNoContent {
		t.Fatalf("restore: %d", got.Code)
	}
	if got := e.do(req{method: http.MethodPatch, path: reader, body: `{"display_name":" Renamed "}`, admin: true}); got.Code != http.StatusNoContent {
		t.Fatalf("rename: %d", got.Code)
	}
	revoke := e.do(req{method: http.MethodDelete, path: reader + "/sessions", admin: true})
	if revoke.Code != http.StatusOK || decode[struct{ Revoked int64 }](t, revoke).Revoked != 0 {
		t.Fatalf("revoke: %d %s", revoke.Code, revoke.Body.String())
	}
	if got := e.do(req{method: http.MethodDelete, path: reader + "/blogs/nothing.example", admin: true}); got.Code != http.StatusNotFound {
		t.Fatalf("release a blog not owned: %d", got.Code)
	}

	self := decode[struct{ Data []userOut }](t, e.do(req{method: http.MethodGet, path: "/api/v1/admin/users?role=admin", admin: true}))
	if len(self.Data) != 1 {
		t.Fatalf("admins: %+v", self)
	}
	own := "/api/v1/admin/users/" + self.Data[0].ID
	for _, r := range []req{
		{method: http.MethodPatch, path: own, body: `{"disabled":true,"reason":"oops"}`},
		{method: http.MethodPatch, path: own, body: `{"is_admin":false}`},
		{method: http.MethodDelete, path: own + "/sessions"},
		{method: http.MethodDelete, path: own},
	} {
		r.admin = true
		got := e.do(r)
		if code, _ := errorCode(t, got); got.Code != http.StatusForbidden || code != "own_account" {
			t.Fatalf("%s %s on own account: %d %s", r.method, r.path, got.Code, code)
		}
	}
	if got := e.do(req{method: http.MethodPatch, path: own, body: `{"display_name":"Me"}`, admin: true}); got.Code != http.StatusNoContent {
		t.Fatalf("rename own account: %d", got.Code)
	}

	if got := e.do(req{method: http.MethodDelete, path: reader, admin: true}); got.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", got.Code, got.Body.String())
	}
	if got := e.do(req{method: http.MethodGet, path: reader, admin: true}); got.Code != http.StatusNotFound {
		t.Fatalf("deleted account: %d", got.Code)
	}
}

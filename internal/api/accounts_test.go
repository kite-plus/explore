package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kite-plus/explore/internal/model"
)

func TestReaderAccountSubscriptionAndClaim(t *testing.T) {
	e := newEnv(t, false)
	e.seed("author.example.org", "en", post("one", 1, ""), model.Entry{
		Identity: "undated", URL: "https://author.example.org/undated", Title: "Undated post",
	})
	register := e.do(req{method: http.MethodPost, path: "/api/v1/auth/register",
		body: `{"email":"author@example.org","password":"long-password-123","display_name":"Author"}`})
	if register.Code != http.StatusOK {
		t.Fatalf("register = %d: %s", register.Code, register.Body.String())
	}
	account := decode[struct {
		CSRFToken string `json:"csrf_token"`
	}](t, register)
	cookie := strings.Split(register.Header().Get("Set-Cookie"), ";")[0]
	auth := map[string]string{"Cookie": cookie, "X-CSRF-Token": account.CSRFToken}
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/admin/session", header: auth}); w.Code != http.StatusUnauthorized {
		t.Fatalf("reader admin access = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodPut, path: "/api/v1/me/subscriptions/author.example.org", header: map[string]string{"Cookie": cookie}}); w.Code != http.StatusForbidden {
		t.Fatalf("subscription without CSRF = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodPut, path: "/api/v1/me/subscriptions/author.example.org", header: auth}); w.Code != http.StatusNoContent {
		t.Fatalf("subscribe = %d: %s", w.Code, w.Body.String())
	}
	following := e.do(req{method: http.MethodGet, path: "/api/v1/me/entries", header: auth})
	if following.Code != http.StatusOK {
		t.Fatalf("following = %d: %s", following.Code, following.Body.String())
	}
	page := decode[pageOut](t, following)
	if len(page.Data) != 2 || page.Data[0].Blog.Host != "author.example.org" || page.Data[1].PublishedAt != nil {
		t.Fatalf("following = %+v", page)
	}
	if w := e.get("/api/v1/me/entries"); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous following = %d", w.Code)
	}
	challenge := e.do(req{method: http.MethodPost, path: "/api/v1/me/blog-claims/author.example.org", header: auth})
	if challenge.Code != http.StatusOK {
		t.Fatalf("challenge = %d: %s", challenge.Code, challenge.Body.String())
	}
	record := decode[struct{ Record, Value string }](t, challenge)
	e.srv.LookupTXT = func(_ context.Context, name string) ([]string, error) {
		if name != record.Record {
			t.Errorf("record = %s", name)
		}
		return []string{record.Value}, nil
	}
	if w := e.do(req{method: http.MethodPost, path: "/api/v1/me/blog-claims/author.example.org/verify", header: auth}); w.Code != http.StatusNoContent {
		t.Fatalf("verify = %d: %s", w.Code, w.Body.String())
	}
	owned := e.do(req{method: http.MethodGet, path: "/api/v1/me/blogs", header: auth})
	if !strings.Contains(owned.Body.String(), "author.example.org") {
		t.Fatalf("owned = %s", owned.Body.String())
	}
	if err := e.s.SetUserAdmin(context.Background(), "author@example.org", true); err != nil {
		t.Fatal(err)
	}
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/admin/session", header: auth}); w.Code != http.StatusNoContent {
		t.Fatalf("admin session = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodPost, path: "/api/v1/auth/logout", header: auth}); w.Code != http.StatusNoContent {
		t.Fatalf("logout = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/me", header: auth}); w.Code != http.StatusUnauthorized {
		t.Fatalf("after logout = %d", w.Code)
	}
}

func TestReaderDeletesTheirAccount(t *testing.T) {
	e := newEnv(t, false)
	cookie, csrf := e.session("leaver@example.com", false)
	auth := map[string]string{"Cookie": cookie, "X-CSRF-Token": csrf}
	if w := e.do(req{method: http.MethodDelete, path: "/api/v1/me", header: map[string]string{"Cookie": cookie}}); w.Code != http.StatusForbidden {
		t.Fatalf("delete without the CSRF token = %d", w.Code)
	}
	w := e.do(req{method: http.MethodDelete, path: "/api/v1/me", header: auth})
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Set-Cookie"); !strings.Contains(got, "explore_session=;") || !strings.Contains(got, "Max-Age=0") {
		t.Errorf("session cookie not cleared: %q", got)
	}
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/me", header: auth}); w.Code != http.StatusUnauthorized {
		t.Fatalf("after delete = %d", w.Code)
	}

	owner, ownerCSRF := e.session("owner@example.com", true)
	w = e.do(req{method: http.MethodDelete, path: "/api/v1/me", header: map[string]string{"Cookie": owner, "X-CSRF-Token": ownerCSRF}})
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), `"last_admin"`) {
		t.Fatalf("only admin delete = %d: %s", w.Code, w.Body.String())
	}
}

func TestChangePassword(t *testing.T) {
	e := newEnv(t, false)
	signIn := func(password string) *httptest.ResponseRecorder {
		return e.do(req{method: http.MethodPost, path: "/api/v1/auth/login",
			body: `{"email":"changer@example.org","password":"` + password + `"}`})
	}
	register := e.do(req{method: http.MethodPost, path: "/api/v1/auth/register",
		body: `{"email":"changer@example.org","password":"old-password-123","display_name":"Changer"}`})
	if register.Code != http.StatusOK {
		t.Fatalf("register = %d: %s", register.Code, register.Body.String())
	}
	other := signIn("old-password-123")
	if other.Code != http.StatusOK {
		t.Fatalf("second sign-in = %d", other.Code)
	}
	here := strings.Split(register.Header().Get("Set-Cookie"), ";")[0]
	elsewhere := strings.Split(other.Header().Get("Set-Cookie"), ";")[0]
	csrf := decode[struct {
		CSRFToken string `json:"csrf_token"`
	}](t, register).CSRFToken
	auth := map[string]string{"Cookie": here, "X-CSRF-Token": csrf}
	change := func(header map[string]string, body string) *httptest.ResponseRecorder {
		return e.do(req{method: http.MethodPut, path: "/api/v1/me/password", header: header, body: body})
	}

	if w := change(map[string]string{"Cookie": here}, `{"current_password":"old-password-123","new_password":"new-password-456"}`); w.Code != http.StatusForbidden {
		t.Errorf("without the CSRF token = %d", w.Code)
	}
	if w := change(auth, `{"current_password":"not-the-password","new_password":"new-password-456"}`); w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), `"wrong_password"`) {
		t.Errorf("wrong current password = %d %s", w.Code, w.Body.String())
	}
	if w := change(auth, `{"current_password":"old-password-123","new_password":"short"}`); w.Code != http.StatusBadRequest {
		t.Errorf("short new password = %d", w.Code)
	}
	if w := change(auth, `{"current_password":"old-password-123","new_password":"new-password-456"}`); w.Code != http.StatusNoContent {
		t.Fatalf("change = %d %s", w.Code, w.Body.String())
	}

	if w := e.do(req{method: http.MethodGet, path: "/api/v1/me", header: map[string]string{"Cookie": here}}); w.Code != http.StatusOK {
		t.Errorf("this session after the change = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/me", header: map[string]string{"Cookie": elsewhere}}); w.Code != http.StatusUnauthorized {
		t.Errorf("another session after the change = %d", w.Code)
	}
	if w := signIn("old-password-123"); w.Code == http.StatusOK {
		t.Error("the old password still signs in")
	}
	if w := signIn("new-password-456"); w.Code != http.StatusOK {
		t.Errorf("the new password = %d", w.Code)
	}
}

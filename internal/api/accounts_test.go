package api_test

import (
	"context"
	"net/http"
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

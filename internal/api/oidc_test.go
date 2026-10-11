package api_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/kite-plus/explore/internal/api"
	"github.com/kite-plus/explore/internal/idp"
	"github.com/kite-plus/explore/internal/idp/idptest"
	"github.com/kite-plus/explore/internal/store/storetest"
)

const webhookSecret = "hook-secret"

// newOIDCEnv serves the API with sign-in through a fake identity provider.
func newOIDCEnv(t *testing.T, publicURL string, configure func(*api.Server)) (*env, *idptest.Provider) {
	t.Helper()
	p := idptest.New(t)
	st := storetest.New(t)
	logs := &bytes.Buffer{}
	srv := &api.Server{
		Store:     st,
		PublicURL: publicURL,
		Log:       slog.New(slog.NewTextHandler(logs, nil)),
		OIDC: idp.New(idp.Config{Issuer: p.URL, ClientID: idptest.ClientID, ClientSecret: idptest.ClientSecret,
			RedirectURL: publicURL + "/auth/callback"}),
		IdentityAccountURL:    p.URL + "/ui/console/users/me",
		IdentityWebhookSecret: webhookSecret,
		AllowPrivate:          true,
	}
	if configure != nil {
		configure(srv)
	}
	h, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return &env{t: t, s: st, srv: srv, h: h, logs: logs}, p
}

// browser keeps cookies between requests, as a browser would.
type browser struct {
	e       *env
	cookies map[string]string
}

func (e *env) browser() *browser { return &browser{e: e, cookies: map[string]string{}} }

func (b *browser) do(method, path, body string, header map[string]string) *httptest.ResponseRecorder {
	b.e.t.Helper()
	h := map[string]string{}
	var pairs []string
	for k, v := range b.cookies {
		pairs = append(pairs, k+"="+v)
	}
	if len(pairs) > 0 {
		h["Cookie"] = strings.Join(pairs, "; ")
	}
	for k, v := range header {
		h[k] = v
	}
	w := b.e.do(req{method: method, path: path, body: body, header: h})
	for _, c := range (&http.Response{Header: w.Header()}).Cookies() {
		if c.MaxAge < 0 || c.Value == "" {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c.Value
		}
	}
	return w
}

func (b *browser) get(path string) *httptest.ResponseRecorder {
	return b.do(http.MethodGet, path, "", nil)
}

// provider follows a redirect to the fake provider's authorization endpoint
// and returns where it sends the browser back.
func provider(t *testing.T, location string) *url.URL {
	t.Helper()
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noFollow.Get(location)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("provider = %d", resp.StatusCode)
	}
	back, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return back
}

// signIn starts a sign-in at path, lets the provider sign the reader in and
// returns the callback's response.
func (b *browser) signIn(path string) *httptest.ResponseRecorder {
	b.e.t.Helper()
	start := b.get(path)
	if start.Code != http.StatusFound {
		b.e.t.Fatalf("start %s = %d: %s", path, start.Code, start.Body.String())
	}
	back := provider(b.e.t, start.Header().Get("Location"))
	return b.get("/auth/callback?" + back.RawQuery)
}

func location(t *testing.T, w *httptest.ResponseRecorder) *url.URL {
	t.Helper()
	if w.Code != http.StatusFound && w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	u, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return u
}

type meOut struct {
	ID               string     `json:"id"`
	Email            string     `json:"email"`
	DisplayName      string     `json:"display_name"`
	CSRFToken        string     `json:"csrf_token"`
	PasswordLogin    bool       `json:"password_login"`
	IdentityLinkedAt *time.Time `json:"identity_linked_at"`
}

func TestSignInIsOffWithoutOIDC(t *testing.T) {
	e := newEnv(t, false)
	for _, path := range []string{"/auth/login", "/auth/callback?state=x", "/auth/app/start?challenge=x", "/app/auth/done",
		"/.well-known/apple-app-site-association", "/.well-known/assetlinks.json"} {
		if w := e.get(path); w.Code != http.StatusNotFound {
			t.Errorf("%s = %d", path, w.Code)
		}
	}
	for _, path := range []string{"/auth/backchannel-logout", "/hooks/idp", "/api/v1/auth/app/exchange"} {
		if w := e.do(req{method: http.MethodPost, path: path, body: "{}"}); w.Code != http.StatusNotFound {
			t.Errorf("POST %s = %d", path, w.Code)
		}
	}
	w := e.get("/api/v1/auth/options")
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"oidc":null,"password":true}` {
		t.Fatalf("options = %d %s", w.Code, w.Body.String())
	}
	register := e.do(req{method: http.MethodPost, path: "/api/v1/auth/register",
		body: `{"email":"reader@example.org","password":"long-password-123","display_name":"Reader"}`})
	if register.Code != http.StatusOK || !strings.HasPrefix(register.Header().Get("Set-Cookie"), "explore_session=") {
		t.Fatalf("register = %d %s", register.Code, register.Header().Get("Set-Cookie"))
	}
}

func TestWebSignIn(t *testing.T) {
	e, p := newOIDCEnv(t, "https://explore.example.org", nil)
	verified := true
	p.Next = idptest.User{Subject: "sub-1", SessionID: "sid-1", Email: "Reader@Example.org", Name: "Reader", EmailVerified: &verified}

	options := e.do(req{method: http.MethodGet, path: "/api/v1/auth/options", header: map[string]string{"Accept-Language": "zh-CN"}})
	if !strings.Contains(options.Body.String(), `"name":"Kite Plus 账号"`) || !strings.Contains(options.Body.String(), `"account_url":"`+p.URL+`/ui/console/users/me"`) {
		t.Fatalf("options = %s", options.Body.String())
	}

	b := e.browser()
	start := b.get("/auth/login?return=" + url.QueryEscape("/following?tag=go"))
	if start.Header().Get("Cache-Control") != "no-store" || b.cookies["__Host-explore_login"] == "" {
		t.Fatalf("start: %v %v", start.Header(), b.cookies)
	}
	if !strings.Contains(start.Header().Get("Set-Cookie"), "Secure") || !strings.Contains(start.Header().Get("Set-Cookie"), "HttpOnly") {
		t.Fatalf("login cookie = %s", start.Header().Get("Set-Cookie"))
	}
	back := provider(t, start.Header().Get("Location"))
	auth := p.Authorizations()[0]
	if auth.RedirectURI != "https://explore.example.org/auth/callback" || auth.ChallengeMethod != "S256" || auth.Nonce == "" || auth.Prompt != "" {
		t.Fatalf("authorization = %+v", auth)
	}
	done := b.get("/auth/callback?" + back.RawQuery)
	if loc := location(t, done); loc.String() != "/following?tag=go" {
		t.Fatalf("callback went to %s", loc)
	}
	if b.cookies["__Host-explore_session"] == "" || b.cookies["__Host-explore_login"] != "" {
		t.Fatalf("cookies after sign-in = %v", b.cookies)
	}
	if !strings.Contains(strings.Join(done.Header().Values("Set-Cookie"), "\n"), "\nexplore_session=; Path=/; Max-Age=0") {
		t.Fatalf("old cookie not cleared: %v", done.Header().Values("Set-Cookie"))
	}
	me := decode[meOut](t, b.get("/api/v1/me"))
	if me.Email != "reader@example.org" || me.DisplayName != "Reader" || me.PasswordLogin || me.IdentityLinkedAt == nil {
		t.Fatalf("me = %+v", me)
	}
	// The old name is no longer accepted over https.
	token := b.cookies["__Host-explore_session"]
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/me", header: map[string]string{"Cookie": "explore_session=" + token}}); w.Code != http.StatusUnauthorized {
		t.Fatalf("old cookie name = %d", w.Code)
	}
	detail, err := e.s.AdminUserDetail(context.Background(), me.ID)
	if err != nil || len(detail.Sessions) != 1 || detail.Sessions[0].Client != "web" || len(detail.Identities) != 1 ||
		detail.Identities[0].Subject != "sub-1" || detail.Identities[0].Issuer != p.URL {
		t.Fatalf("detail = %+v, %v", detail, err)
	}

	// A second sign-in finds the same account.
	again := e.browser()
	if loc := location(t, again.signIn("/auth/login")); loc.String() != "/" {
		t.Fatalf("default return = %s", loc)
	}
	if decode[meOut](t, again.get("/api/v1/me")).ID != me.ID {
		t.Fatal("a second sign-in made another account")
	}

	// Password registration is closed once Kite Plus is configured.
	register := e.do(req{method: http.MethodPost, path: "/api/v1/auth/register",
		body: `{"email":"new@example.org","password":"long-password-123","display_name":"New"}`})
	if code, _ := errorCode(t, register); register.Code != http.StatusForbidden || code != "feature_disabled" {
		t.Fatalf("register = %d %s", register.Code, code)
	}
	// No password to sign in with.
	login := e.do(req{method: http.MethodPost, path: "/api/v1/auth/login", body: `{"email":"reader@example.org","password":"unused-password-value"}`})
	if login.Code != http.StatusUnauthorized {
		t.Fatalf("password login of an identity account = %d", login.Code)
	}
	if w := b.do(http.MethodPut, "/api/v1/me/password", `{"current_password":"x","new_password":"long-password-123"}`,
		map[string]string{"X-CSRF-Token": me.CSRFToken}); w.Code != http.StatusConflict {
		t.Fatalf("change password = %d", w.Code)
	}
}

func TestReturnPathsStayOnSite(t *testing.T) {
	e, _ := newOIDCEnv(t, "https://explore.example.org", nil)
	for _, ret := range []string{"//evil.example/x", "https://evil.example/", "/\\evil.example", "/auth/login", "/login?next=/x",
		"/en/login", "javascript:alert(1)", "/a\nb", "/app/auth/done"} {
		b := e.browser()
		if loc := location(t, b.signIn("/auth/login?return="+url.QueryEscape(ret))); loc.String() != "/" {
			t.Errorf("return %q went to %s", ret, loc)
		}
	}
	b := e.browser()
	if loc := location(t, b.signIn("/auth/login?return="+url.QueryEscape("/en/blogs/x.example"))); loc.String() != "/en/blogs/x.example" {
		t.Errorf("went to %s", loc)
	}
}

func TestCallbackRefusals(t *testing.T) {
	e, p := newOIDCEnv(t, "https://explore.example.org", nil)
	wantError := func(w *httptest.ResponseRecorder, path, code string) {
		t.Helper()
		loc := location(t, w)
		if loc.Path != path || loc.Query().Get("error") != code {
			t.Fatalf("went to %s, want %s?error=%s", loc, path, code)
		}
	}

	// The state is bound to the browser that started the sign-in.
	b := e.browser()
	start := b.get("/auth/login?return=/en/following")
	back := provider(t, start.Header().Get("Location"))
	other := e.browser()
	other.cookies["__Host-explore_login"] = strings.Repeat("a", 43)
	wantError(other.get("/auth/callback?"+back.RawQuery), "/login", "login_expired")
	wantError(e.browser().get("/auth/callback?"+back.RawQuery), "/login", "login_expired")
	done := b.get("/auth/callback?" + back.RawQuery)
	if loc := location(t, done); loc.Path != "/en/following" {
		t.Fatalf("binding check consumed the flow: %s", loc)
	}
	// Each state is good for one callback.
	b.cookies["__Host-explore_login"] = other.cookies["__Host-explore_login"]
	wantError(b.get("/auth/callback?"+back.RawQuery), "/login", "login_expired")

	// The reader said no at the provider.
	p.Deny = true
	denied := e.browser().signIn("/auth/login?return=/en/following")
	wantError(denied, "/en/login", "access_denied")
	if location(t, denied).Query().Get("next") != "/en/following" {
		t.Fatalf("next = %s", location(t, denied))
	}
	p.Deny = false

	// A token that does not check out.
	for name, tamper := range map[string]func(map[string]any){
		"audience": func(c map[string]any) { c["aud"] = "someone-else"; delete(c, "azp") },
		"issuer":   func(c map[string]any) { c["iss"] = "https://evil.example" },
		"nonce":    func(c map[string]any) { c["nonce"] = "replayed" },
		"expired":  func(c map[string]any) { c["exp"] = time.Now().Add(-time.Minute).Unix() },
	} {
		p.Tamper = tamper
		br := e.browser()
		wantError(br.signIn("/auth/login"), "/login", "login_failed")
		if br.cookies["__Host-explore_session"] != "" {
			t.Fatalf("%s: a session was set", name)
		}
	}
	p.Tamper = nil

	// A forged code or none at all.
	b = e.browser()
	start = b.get("/auth/login")
	back = provider(t, start.Header().Get("Location"))
	q := back.Query()
	q.Set("code", "forged")
	wantError(b.get("/auth/callback?"+q.Encode()), "/login", "login_failed")
}

func TestSignInRules(t *testing.T) {
	e, p := newOIDCEnv(t, "https://explore.example.org", nil)
	ctx := context.Background()
	wantError := func(w *httptest.ResponseRecorder, code string) {
		t.Helper()
		if got := location(t, w).Query().Get("error"); got != code {
			t.Fatalf("error = %q, want %q", got, code)
		}
	}
	// Never merged by email: the owner of the local account links instead.
	if _, err := e.s.CreateUser(ctx, "local@example.org", "hash", "Local"); err != nil {
		t.Fatal(err)
	}
	p.Next = idptest.User{Subject: "sub-local", SessionID: "s", Email: "local@example.org", Name: "Local"}
	wantError(e.browser().signIn("/auth/login"), "email_in_use")

	unverified := false
	p.Next = idptest.User{Subject: "sub-unverified", SessionID: "s", Email: "unverified@example.org", EmailVerified: &unverified}
	wantError(e.browser().signIn("/auth/login"), "email_unverified")

	p.Next = idptest.User{Subject: "sub-closed", SessionID: "s", Email: "closed@example.org", Username: "closed"}
	if err := e.s.SetSetting(ctx, "registration_enabled", "false", "admin@example.org"); err != nil {
		t.Fatal(err)
	}
	wantError(e.browser().signIn("/auth/login"), "registration_closed")
	if err := e.s.SetSetting(ctx, "registration_enabled", "true", "admin@example.org"); err != nil {
		t.Fatal(err)
	}
	b := e.browser()
	b.signIn("/auth/login")
	me := decode[meOut](t, b.get("/api/v1/me"))
	if me.DisplayName != "closed" {
		t.Fatalf("name from the username = %q", me.DisplayName)
	}
	if err := e.s.SetUserDisabled(ctx, me.ID, true, "spam", "admin@example.org"); err != nil {
		t.Fatal(err)
	}
	wantError(e.browser().signIn("/auth/login"), "account_disabled")
}

func TestLinkIdentity(t *testing.T) {
	e, p := newOIDCEnv(t, "https://explore.example.org", nil)
	ctx := context.Background()
	register := func(email string) (*browser, meOut) {
		t.Helper()
		if err := e.s.SetSetting(ctx, "registration_enabled", "true", "x"); err != nil {
			t.Fatal(err)
		}
		b := e.browser()
		if _, err := e.s.CreateUser(ctx, email, mustHash(t, "long-password-123"), "Local"); err != nil {
			t.Fatal(err)
		}
		w := b.do(http.MethodPost, "/api/v1/auth/login", `{"email":"`+email+`","password":"long-password-123"}`, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("login = %d %s", w.Code, w.Body.String())
		}
		return b, decode[meOut](t, w)
	}
	b, me := register("local@example.org")
	if !me.PasswordLogin || me.IdentityLinkedAt != nil {
		t.Fatalf("before linking: %+v", me)
	}
	form := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	if w := b.do(http.MethodPost, "/auth/link", "return=/en/account", form); w.Code != http.StatusForbidden {
		t.Fatalf("link without the CSRF token = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodPost, path: "/auth/link", body: "csrf_token=" + me.CSRFToken, header: form}); w.Code != http.StatusUnauthorized {
		t.Fatalf("link signed out = %d", w.Code)
	}
	p.Next = idptest.User{Subject: "sub-link", SessionID: "sid-link", Email: "another@example.org", Name: "Another"}
	start := b.do(http.MethodPost, "/auth/link", "csrf_token="+me.CSRFToken+"&return=/en/account", form)
	back := provider(t, location(t, start).String())
	done := b.get("/auth/callback?" + back.RawQuery)
	if loc := location(t, done); loc.String() != "/en/account?linked=1" {
		t.Fatalf("link went to %s", loc)
	}
	after := decode[meOut](t, b.get("/api/v1/me"))
	if after.ID != me.ID || after.PasswordLogin || after.IdentityLinkedAt == nil || after.Email != "local@example.org" {
		t.Fatalf("after linking: %+v", after)
	}
	if w := e.do(req{method: http.MethodPost, path: "/api/v1/auth/login", body: `{"email":"local@example.org","password":"long-password-123"}`}); w.Code != http.StatusUnauthorized {
		t.Fatalf("password after linking = %d", w.Code)
	}
	// Signing in with Kite Plus now reaches the same account.
	k := e.browser()
	k.signIn("/auth/login")
	if decode[meOut](t, k.get("/api/v1/me")).ID != me.ID {
		t.Fatal("sign-in after linking reached another account")
	}
	// Linking again is refused; so is an identity linked elsewhere.
	if w := b.do(http.MethodPost, "/auth/link", "csrf_token="+after.CSRFToken, form); location(t, w).String() != "/account?error=already_linked" {
		t.Fatalf("second link went to %s", location(t, w))
	}
	b2, me2 := register("second@example.org")
	start = b2.do(http.MethodPost, "/auth/link", "csrf_token="+me2.CSRFToken, form)
	back = provider(t, location(t, start).String())
	if loc := location(t, b2.get("/auth/callback?"+back.RawQuery)); loc.String() != "/account?error=identity_taken" {
		t.Fatalf("taken identity went to %s", loc)
	}
	if decode[meOut](t, b2.get("/api/v1/me")).PasswordLogin != true {
		t.Fatal("a refused link turned the password off")
	}
	// The browser must still be signed in to the account that started it.
	b3, me3 := register("third@example.org")
	p.Next = idptest.User{Subject: "sub-third", SessionID: "s3", Email: "third-kp@example.org"}
	start = b3.do(http.MethodPost, "/auth/link", "csrf_token="+me3.CSRFToken, form)
	back = provider(t, location(t, start).String())
	b3.do(http.MethodPost, "/api/v1/auth/logout", "", map[string]string{"X-CSRF-Token": me3.CSRFToken})
	if loc := location(t, b3.get("/auth/callback?"+back.RawQuery)); loc.String() != "/account?error=login_expired" {
		t.Fatalf("link after signing out went to %s", loc)
	}
}

func TestBackchannelLogout(t *testing.T) {
	e, p := newOIDCEnv(t, "https://explore.example.org", nil)
	p.Next = idptest.User{Subject: "sub-1", SessionID: "sid-1", Email: "reader@example.org"}
	first := e.browser()
	first.signIn("/auth/login")
	p.Next.SessionID = "sid-2"
	second := e.browser()
	second.signIn("/auth/login")
	logout := func(token string) *httptest.ResponseRecorder {
		return e.do(req{method: http.MethodPost, path: "/auth/backchannel-logout", body: "logout_token=" + url.QueryEscape(token),
			header: map[string]string{"Content-Type": "application/x-www-form-urlencoded"}})
	}
	token := p.LogoutToken("sub-1", "sid-1", nil)
	if w := logout(token); w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("logout = %d %s", w.Code, w.Body.String())
	}
	if w := first.get("/api/v1/me"); w.Code != http.StatusUnauthorized {
		t.Fatalf("session of sid-1 = %d", w.Code)
	}
	if w := second.get("/api/v1/me"); w.Code != http.StatusOK {
		t.Fatalf("session of sid-2 = %d", w.Code)
	}
	if w := logout(token); w.Code != http.StatusBadRequest {
		t.Fatalf("replay = %d", w.Code)
	}
	for name, bad := range map[string]string{
		"nonce":   p.LogoutToken("sub-1", "sid-2", map[string]any{"nonce": "n"}),
		"no sid":  p.LogoutToken("", "", nil),
		"old":     p.LogoutToken("sub-1", "sid-2", map[string]any{"iat": time.Now().Add(-time.Hour).Unix()}),
		"garbage": "x.y.z",
		"empty":   "",
	} {
		if w := logout(bad); w.Code != http.StatusBadRequest {
			t.Errorf("%s = %d", name, w.Code)
		}
	}
	if w := second.get("/api/v1/me"); w.Code != http.StatusOK {
		t.Fatalf("a refused token ended a session: %d", w.Code)
	}
	if w := logout(p.LogoutToken("sub-1", "", nil)); w.Code != http.StatusOK {
		t.Fatalf("sub only = %d", w.Code)
	}
	if w := second.get("/api/v1/me"); w.Code != http.StatusUnauthorized {
		t.Fatalf("session after sub-only logout = %d", w.Code)
	}
}

func TestIdentityWebhook(t *testing.T) {
	e, p := newOIDCEnv(t, "https://explore.example.org", nil)
	p.Next = idptest.User{Subject: "sub-1", SessionID: "sid-1", Email: "reader@example.org"}
	b := e.browser()
	b.signIn("/auth/login")
	me := decode[meOut](t, b.get("/api/v1/me"))
	event := func(kind string, seq int, at time.Time) string {
		return `{"aggregateID":"sub-1","aggregateType":"user","resourceOwner":"org","instanceID":"inst","version":"v2",` +
			`"sequence":` + strconv.Itoa(seq) + `,"event_type":"` + kind + `","created_at":"` + at.UTC().Format(time.RFC3339Nano) + `","event_payload":{}}`
	}
	send := func(body, signature string) *httptest.ResponseRecorder {
		return e.do(req{method: http.MethodPost, path: "/hooks/idp", body: body, header: map[string]string{idp.SignatureHeader: signature}})
	}
	signed := func(body string) *httptest.ResponseRecorder {
		return send(body, idp.Sign([]byte(body), webhookSecret, time.Now()))
	}
	t0 := time.Now().Add(-time.Hour)

	locked := event("user.locked", 5, t0)
	if w := send(locked, idp.Sign([]byte(locked), "wrong", time.Now())); w.Code != http.StatusUnauthorized {
		t.Fatalf("bad signature = %d", w.Code)
	}
	if w := send(locked, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("no signature = %d", w.Code)
	}
	if w := b.get("/api/v1/me"); w.Code != http.StatusOK {
		t.Fatal("an unsigned event was applied")
	}
	if w := signed(locked); w.Code != http.StatusOK {
		t.Fatalf("locked = %d %s", w.Code, w.Body.String())
	}
	if w := b.get("/api/v1/me"); w.Code != http.StatusUnauthorized {
		t.Fatalf("session after the lock = %d", w.Code)
	}
	if w := e.browser().signIn("/auth/login"); location(t, w).Query().Get("error") != "account_disabled" {
		t.Fatalf("sign-in while locked went to %s", location(t, w))
	}
	if w := signed(event("user.unlocked", 6, t0.Add(time.Minute))); w.Code != http.StatusOK {
		t.Fatalf("unlocked = %d", w.Code)
	}
	b = e.browser()
	b.signIn("/auth/login")
	if w := b.get("/api/v1/me"); w.Code != http.StatusOK {
		t.Fatalf("sign-in after unlocking = %d", w.Code)
	}
	// The same delivery again is acknowledged and not applied.
	if w := signed(locked); w.Code != http.StatusOK {
		t.Fatalf("redelivery = %d", w.Code)
	}
	if w := b.get("/api/v1/me"); w.Code != http.StatusOK {
		t.Fatal("a redelivered event was applied again")
	}
	if w := signed(`{"aggregateID":"sub-1","aggregateType":"user","sequence":7,"event_type":"user.human.profile.changed","created_at":"2026-10-10T08:00:00Z"}`); w.Code != http.StatusOK {
		t.Fatalf("other event = %d", w.Code)
	}
	if w := signed(`not json`); w.Code != http.StatusBadRequest {
		t.Fatalf("garbage = %d", w.Code)
	}
	if w := signed(event("user.removed", 8, t0.Add(2*time.Minute))); w.Code != http.StatusOK {
		t.Fatalf("removed = %d", w.Code)
	}
	if _, err := e.s.AdminUserDetail(context.Background(), me.ID); err == nil {
		t.Fatal("the account outlived its identity")
	}
}

func challengeOf(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func TestAppSignIn(t *testing.T) {
	e, p := newOIDCEnv(t, "https://explore.example.org", nil)
	p.Next = idptest.User{Subject: "sub-app", SessionID: "sid-app", Email: "app@example.org", Name: "App"}
	verifier := strings.Repeat("v", 50)
	challenge := challengeOf(verifier)

	for name, path := range map[string]string{
		"no challenge":    "/auth/app/start?client=ios",
		"short challenge": "/auth/app/start?client=ios&challenge=abc",
		"no client":       "/auth/app/start?challenge=" + challenge,
		"web client":      "/auth/app/start?client=web&challenge=" + challenge,
		"odd callback":    "/auth/app/start?client=ios&callback=ftp&challenge=" + challenge,
	} {
		if w := e.get(path); w.Code != http.StatusBadRequest {
			t.Errorf("%s = %d", name, w.Code)
		}
	}

	appSignIn := func(query string) *url.URL {
		t.Helper()
		b := e.browser()
		done := b.signIn("/auth/app/start?" + query)
		if len(b.cookies) != 0 {
			t.Fatalf("an app sign-in left cookies: %v", b.cookies)
		}
		return location(t, done)
	}
	loc := appSignIn("client=ios&switch=1&challenge=" + challenge)
	if loc.Scheme != "https" || loc.Host != "explore.example.org" || loc.Path != "/app/auth/done" || len(loc.Query().Get("code")) != 43 {
		t.Fatalf("app callback = %s", loc)
	}
	if prompt := p.Authorizations()[0].Prompt; prompt != "select_account" {
		t.Fatalf("prompt = %q", prompt)
	}
	code := loc.Query().Get("code")
	exchange := func(code, verifier string) *httptest.ResponseRecorder {
		return e.do(req{method: http.MethodPost, path: "/api/v1/auth/app/exchange", body: `{"code":"` + code + `","verifier":"` + verifier + `"}`})
	}
	wantGrantError := func(w *httptest.ResponseRecorder) {
		t.Helper()
		if code, _ := errorCode(t, w); w.Code != http.StatusBadRequest || code != "invalid_grant" {
			t.Fatalf("exchange = %d %s", w.Code, code)
		}
	}
	// A wrong verifier uses the code up.
	wantGrantError(exchange(code, strings.Repeat("w", 50)))
	wantGrantError(exchange(code, verifier))

	code = appSignIn("client=android&challenge=" + challenge).Query().Get("code")
	if p.Authorizations()[1].Prompt != "" {
		t.Fatal("select_account without switch=1")
	}
	ok := exchange(code, verifier)
	if ok.Code != http.StatusOK || ok.Header().Get("Cache-Control") != "no-store" || ok.Header().Get("Set-Cookie") != "" {
		t.Fatalf("exchange = %d %s %v", ok.Code, ok.Body.String(), ok.Header())
	}
	session := decode[struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
		User      meOut     `json:"user"`
	}](t, ok)
	if len(session.Token) != 43 || time.Until(session.ExpiresAt) < 29*24*time.Hour || session.User.Email != "app@example.org" {
		t.Fatalf("session = %+v", session)
	}
	bearer := map[string]string{"Authorization": "Bearer " + session.Token}
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/me", header: bearer}); w.Code != http.StatusOK {
		t.Fatalf("bearer me = %d", w.Code)
	}
	// Bearer writes need no CSRF header; a browser never sends the header on its own.
	if w := e.do(req{method: http.MethodPatch, path: "/api/v1/me", body: `{"display_name":"Renamed"}`, header: bearer}); w.Code != http.StatusOK {
		t.Fatalf("bearer write = %d %s", w.Code, w.Body.String())
	}
	if w := e.do(req{method: http.MethodPatch, path: "/api/v1/me", body: `{"display_name":"Cookie"}`,
		header: map[string]string{"Cookie": "__Host-explore_session=" + session.Token}}); w.Code != http.StatusForbidden {
		t.Fatalf("cookie write without CSRF = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/me", header: map[string]string{"Authorization": "Basic " + session.Token,
		"Cookie": "__Host-explore_session=" + session.Token}}); w.Code != http.StatusUnauthorized {
		t.Fatalf("another scheme fell back to the cookie: %d", w.Code)
	}
	detail, err := e.s.AdminUserDetail(context.Background(), session.User.ID)
	if err != nil || len(detail.Sessions) != 1 || detail.Sessions[0].Client != "android" {
		t.Fatalf("sessions = %+v, %v", detail.Sessions, err)
	}
	// Trying the code again ends the session it gave.
	wantGrantError(exchange(code, verifier))
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/me", header: bearer}); w.Code != http.StatusUnauthorized {
		t.Fatalf("session after the code was replayed = %d", w.Code)
	}

	// The code expires after two minutes.
	e.srv.Now = func() time.Time { return time.Now().Add(-3 * time.Minute) }
	code = appSignIn("client=ios&challenge=" + challenge).Query().Get("code")
	e.srv.Now = time.Now
	wantGrantError(exchange(code, verifier))
	wantGrantError(exchange("short", verifier))
	wantGrantError(exchange(strings.Repeat("c", 43), "short"))
	if w := e.do(req{method: http.MethodPost, path: "/api/v1/auth/app/exchange", body: `garbage`}); w.Code != http.StatusBadRequest {
		t.Fatalf("garbage = %d", w.Code)
	}

	// A web sign-in hands out no code, and an app sign-in no cookie.
	web := e.browser()
	if loc := location(t, web.signIn("/auth/login?return=/x")); loc.Query().Get("code") != "" || loc.Path != "/x" {
		t.Fatalf("web sign-in went to %s", loc)
	}

	// Without app links the custom scheme is allowed, for servers the app cannot claim.
	if loc := appSignIn("client=ios&callback=scheme&challenge=" + challenge); loc.Scheme != "explore" || loc.Host != "app" || loc.Path != "/auth/done" {
		t.Fatalf("scheme callback = %s", loc)
	}
	// A refused sign-in reaches the app with the error.
	p.Deny = true
	if loc := appSignIn("client=ios&challenge=" + challenge); loc.Path != "/app/auth/done" || loc.Query().Get("error") != "access_denied" {
		t.Fatalf("denied app sign-in = %s", loc)
	}
	p.Deny = false

	page := e.do(req{method: http.MethodGet, path: "/app/auth/done?code=" + code, header: map[string]string{"Accept-Language": "zh-CN"}})
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "请在 Kite 应用中完成登录") || strings.Contains(page.Body.String(), code) ||
		!strings.Contains(page.Header().Get("Content-Security-Policy"), "default-src 'none'") || page.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("done page = %d %v\n%s", page.Code, page.Header(), page.Body.String())
	}
}

func TestAppLinks(t *testing.T) {
	fp := strings.TrimSuffix(strings.Repeat("AB:", 32), ":")
	e, _ := newOIDCEnv(t, "https://explore.example.org", func(s *api.Server) {
		s.AppleAppIDs = []string{"ABCDE12345.plus.kite.explore"}
		s.AndroidPackage = "plus.kite.explore"
		s.AndroidCertSHA256 = []string{fp}
	})
	aasa := e.get("/.well-known/apple-app-site-association")
	if aasa.Code != http.StatusOK || aasa.Header().Get("Content-Type") != "application/json" ||
		!strings.Contains(aasa.Body.String(), `"appIDs":["ABCDE12345.plus.kite.explore"]`) ||
		!strings.Contains(aasa.Body.String(), `"/":"/app/auth/done"`) || !strings.Contains(aasa.Body.String(), `"webcredentials":{"apps":["ABCDE12345.plus.kite.explore"]}`) {
		t.Fatalf("aasa = %d %s", aasa.Code, aasa.Body.String())
	}
	links := e.get("/.well-known/assetlinks.json")
	if links.Code != http.StatusOK || !strings.Contains(links.Body.String(), `"package_name":"plus.kite.explore"`) ||
		!strings.Contains(links.Body.String(), fp) || !strings.Contains(links.Body.String(), "delegate_permission/common.handle_all_urls") {
		t.Fatalf("assetlinks = %d %s", links.Code, links.Body.String())
	}
	// With the address claimed, codes never go to a scheme any app could register.
	if w := e.get("/auth/app/start?client=ios&callback=scheme&challenge=" + challengeOf(strings.Repeat("v", 50))); w.Code != http.StatusBadRequest {
		t.Fatalf("scheme callback with app links = %d", w.Code)
	}
}

func TestCookieNamesOverHTTP(t *testing.T) {
	e, p := newOIDCEnv(t, "http://localhost:4321", nil)
	p.Next = idptest.User{Subject: "sub-1", SessionID: "sid-1", Email: "reader@example.org"}
	b := e.browser()
	start := b.get("/auth/login")
	if b.cookies["explore_login"] == "" || strings.Contains(start.Header().Get("Set-Cookie"), "Secure") {
		t.Fatalf("login cookie over http = %s", start.Header().Get("Set-Cookie"))
	}
	back := provider(t, start.Header().Get("Location"))
	if p.Authorizations()[0].RedirectURI != "http://localhost:4321/auth/callback" {
		t.Fatalf("redirect_uri = %s", p.Authorizations()[0].RedirectURI)
	}
	b.get("/auth/callback?" + back.RawQuery)
	token := b.cookies["explore_session"]
	if token == "" {
		t.Fatalf("cookies = %v", b.cookies)
	}
	// Today's app writes this cookie by hand; it keeps working over http.
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/me", header: map[string]string{"Cookie": "explore_session=" + token}}); w.Code != http.StatusOK {
		t.Fatalf("explore_session over http = %d", w.Code)
	}
	if w := e.get("/app/auth/done"); !strings.Contains(w.Body.String(), "Finish signing in") {
		t.Fatalf("done page in English = %s", w.Body.String())
	}
}

func TestAppPasswordLoginAndBearer(t *testing.T) {
	e := newEnv(t, false)
	if _, err := e.s.CreateUser(context.Background(), "reader@example.org", mustHash(t, "long-password-123"), "Reader"); err != nil {
		t.Fatal(err)
	}
	if err := e.s.SetUserAdmin(context.Background(), "reader@example.org", true); err != nil {
		t.Fatal(err)
	}
	w := e.do(req{method: http.MethodPost, path: "/api/v1/auth/login", body: `{"email":"reader@example.org","password":"long-password-123","client":"ios"}`})
	if w.Code != http.StatusOK || w.Header().Get("Set-Cookie") != "" {
		t.Fatalf("app login = %d %v", w.Code, w.Header())
	}
	session := decode[struct {
		Token string `json:"token"`
		User  meOut  `json:"user"`
	}](t, w)
	if len(session.Token) != 43 || !session.User.PasswordLogin {
		t.Fatalf("app login = %s", w.Body.String())
	}
	bearer := map[string]string{"Authorization": "bearer " + session.Token}
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/admin/session", header: bearer}); w.Code != http.StatusNoContent {
		t.Fatalf("bearer admin = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodPatch, path: "/api/v1/admin/settings/site_notice", body: `{"value":"hi"}`, header: bearer}); w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Fatalf("bearer admin write = %d %s", w.Code, w.Body.String())
	}
	// Without OIDC the old cookie name stays, over https too.
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/me", header: map[string]string{"Cookie": "explore_session=" + session.Token}}); w.Code != http.StatusOK {
		t.Fatalf("explore_session without OIDC = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodPost, path: "/api/v1/auth/logout", header: bearer}); w.Code != http.StatusNoContent {
		t.Fatalf("bearer logout = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodGet, path: "/api/v1/me", header: bearer}); w.Code != http.StatusUnauthorized {
		t.Fatalf("after logout = %d", w.Code)
	}
	if w := e.do(req{method: http.MethodPost, path: "/api/v1/auth/login", body: `{"email":"reader@example.org","password":"long-password-123","client":"tv"}`}); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown client = %d", w.Code)
	}
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(hash)
}

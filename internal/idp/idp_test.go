package idp_test

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kite-plus/explore/internal/idp"
	"github.com/kite-plus/explore/internal/idp/idptest"
	"github.com/kite-plus/explore/internal/model"
)

const redirect = "https://explore.example.org/auth/callback"

func newClient(p *idptest.Provider) *idp.Client {
	return idp.New(idp.Config{Issuer: p.URL, ClientID: idptest.ClientID, ClientSecret: idptest.ClientSecret, RedirectURL: redirect})
}

// signIn runs the authorization endpoint and returns the code it hands back.
func signIn(t *testing.T, c *idp.Client, req idp.AuthRequest) string {
	t.Helper()
	u, err := c.AuthURL(req)
	if err != nil {
		t.Fatal(err)
	}
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noFollow.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil || resp.StatusCode != http.StatusFound {
		t.Fatalf("authorize = %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	if loc.Query().Get("state") != req.State {
		t.Fatalf("state = %q", loc.Query().Get("state"))
	}
	return loc.Query().Get("code")
}

func TestExchange(t *testing.T) {
	p := idptest.New(t)
	verified := true
	p.SetUser(idptest.User{Subject: "u-42", SessionID: "s-42", Email: "Reader@Example.org", Name: " Reader ", Username: "reader", EmailVerified: &verified})
	c := newClient(p)
	verifier := idp.NewVerifier()
	code := signIn(t, c, idp.AuthRequest{State: "st", Nonce: "n-1", Verifier: verifier, SelectAccount: true})
	a := p.Authorizations()[0]
	if a.Prompt != "select_account" || a.ChallengeMethod != "S256" || a.Nonce != "n-1" || a.RedirectURI != redirect ||
		!strings.Contains(a.Scope, "openid") || !strings.Contains(a.Scope, "email") {
		t.Fatalf("authorization = %+v", a)
	}
	id, err := c.Exchange(t.Context(), code, verifier, "n-1")
	if err != nil {
		t.Fatal(err)
	}
	if id.Issuer != p.URL || id.Subject != "u-42" || id.SessionID != "s-42" || id.Email != "Reader@Example.org" ||
		id.Name != "Reader" || id.Username != "reader" || id.EmailVerified == nil || !*id.EmailVerified {
		t.Fatalf("identity = %+v", id)
	}
	if _, err := c.Exchange(t.Context(), code, verifier, "n-1"); !errors.Is(err, idp.ErrInvalidToken) {
		t.Fatalf("reused code: %v", err)
	}
}

func TestExchangeRefusesBadTokens(t *testing.T) {
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		tamper   func(map[string]any)
		signWith *rsa.PrivateKey
		nonce    string
		verifier string
	}{
		"signature":        {signWith: other},
		"issuer":           {tamper: func(c map[string]any) { c["iss"] = "https://evil.example" }},
		"audience":         {tamper: func(c map[string]any) { c["aud"] = "someone-else"; delete(c, "azp") }},
		"authorized party": {tamper: func(c map[string]any) { c["azp"] = "someone-else" }},
		"expired":          {tamper: func(c map[string]any) { c["exp"] = time.Now().Add(-time.Minute).Unix() }},
		"nonce":            {nonce: "another-nonce"},
		"missing nonce":    {tamper: func(c map[string]any) { delete(c, "nonce") }},
		"pkce":             {verifier: idp.NewVerifier()},
		"subject":          {tamper: func(c map[string]any) { c["sub"] = "" }},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := idptest.New(t)
			p.SetTamper(tc.tamper)
			p.SetSigner(tc.signWith)
			c := newClient(p)
			verifier := idp.NewVerifier()
			code := signIn(t, c, idp.AuthRequest{State: "st", Nonce: "n-1", Verifier: verifier})
			nonce := "n-1"
			if tc.nonce != "" {
				nonce = tc.nonce
			}
			if tc.verifier != "" {
				verifier = tc.verifier
			}
			if _, err := c.Exchange(t.Context(), code, verifier, nonce); !errors.Is(err, idp.ErrInvalidToken) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestUnreachableProvider(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	c := idp.New(idp.Config{Issuer: srv.URL, ClientID: "x", ClientSecret: "y", RedirectURL: redirect})
	if _, err := c.AuthURL(idp.AuthRequest{State: "s", Nonce: "n", Verifier: idp.NewVerifier()}); !errors.Is(err, idp.ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestVerifyLogout(t *testing.T) {
	p := idptest.New(t)
	c := newClient(p)
	// Discovery happens on first use.
	if _, err := c.AuthURL(idp.AuthRequest{State: "s", Nonce: "n", Verifier: idp.NewVerifier()}); err != nil {
		t.Fatal(err)
	}
	lt, err := c.VerifyLogout(t.Context(), p.LogoutToken("u-1", "s-1", nil))
	if err != nil {
		t.Fatal(err)
	}
	if lt.Subject != "u-1" || lt.SessionID != "s-1" || lt.TokenID == "" || lt.Issuer != p.URL {
		t.Fatalf("logout = %+v", lt)
	}
	if _, err := c.VerifyLogout(t.Context(), p.LogoutToken("", "s-1", nil)); err != nil {
		t.Fatalf("sid only: %v", err)
	}
	now := time.Now()
	bad := map[string]string{
		"no sub or sid": p.LogoutToken("", "", nil),
		"nonce":         p.LogoutToken("u-1", "s-1", map[string]any{"nonce": "n"}),
		"no event":      p.LogoutToken("u-1", "s-1", map[string]any{"events": map[string]any{}}),
		"no jti":        p.LogoutToken("u-1", "s-1", map[string]any{"jti": nil}),
		"no iat":        p.LogoutToken("u-1", "s-1", map[string]any{"iat": nil}),
		"old":           p.LogoutToken("u-1", "s-1", map[string]any{"iat": now.Add(-time.Hour).Unix()}),
		"future":        p.LogoutToken("u-1", "s-1", map[string]any{"iat": now.Add(time.Hour).Unix()}),
		"expired":       p.LogoutToken("u-1", "s-1", map[string]any{"exp": now.Add(-time.Minute).Unix()}),
		"no exp":        p.LogoutToken("u-1", "s-1", map[string]any{"exp": nil}),
		"issuer":        p.LogoutToken("u-1", "s-1", map[string]any{"iss": "https://evil.example"}),
		"audience":      p.LogoutToken("u-1", "s-1", map[string]any{"aud": "someone-else"}),
		"id token":      p.Sign(map[string]any{"iss": p.URL, "aud": idptest.ClientID, "sub": "u-1", "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(), "nonce": "n"}),
		"garbage":       "not.a.token",
	}
	for name, raw := range bad {
		if _, err := c.VerifyLogout(t.Context(), raw); !errors.Is(err, idp.ErrInvalidToken) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	// Another provider's key under the same key ID.
	forged := idptest.New(t)
	forged.URL = p.URL
	if _, err := c.VerifyLogout(t.Context(), forged.LogoutToken("u-1", "s-1", map[string]any{"iss": p.URL})); !errors.Is(err, idp.ErrInvalidToken) {
		t.Errorf("other key: err = %v", err)
	}
}

func TestSignature(t *testing.T) {
	body := []byte(`{"event_type":"user.locked"}`)
	now := time.Unix(1_800_000_000, 0)
	header := idp.Sign(body, "secret", now)
	if !strings.HasPrefix(header, "t=1800000000,v1=") {
		t.Fatalf("header = %s", header)
	}
	if err := idp.VerifySignature(header, body, "secret", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := idp.VerifySignature("t=1800000000,v1=00,"+header[len("t=1800000000,"):], body, "secret", now); err != nil {
		t.Fatalf("second signature: %v", err)
	}
	for name, tc := range map[string]struct {
		header string
		body   []byte
		at     time.Time
	}{
		"wrong secret": {idp.Sign(body, "other", now), body, now},
		"changed body": {header, []byte(`{"event_type":"user.unlocked"}`), now},
		"too old":      {header, body, now.Add(10 * time.Minute)},
		"future":       {header, body, now.Add(-10 * time.Minute)},
		"empty":        {"", body, now},
		"no time":      {header[strings.Index(header, ",")+1:], body, now},
	} {
		if err := idp.VerifySignature(tc.header, tc.body, "secret", tc.at); !errors.Is(err, idp.ErrBadSignature) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseEvent(t *testing.T) {
	e, ok, err := idp.ParseEvent([]byte(`{"aggregateID":"u-1","aggregateType":"user","resourceOwner":"o","instanceID":"i",
		"version":"v2","sequence":7,"event_type":"user.locked","created_at":"2026-10-10T08:00:00.123456Z","userID":"admin"}`))
	if err != nil || !ok {
		t.Fatalf("ok = %v, err = %v", ok, err)
	}
	if e.Subject != "u-1" || e.Type != "user.locked" || e.ID != "u-1/7" || !e.At.Equal(time.Date(2026, 10, 10, 8, 0, 0, 123456000, time.UTC)) {
		t.Fatalf("event = %+v", e)
	}
	if _, ok, err := idp.ParseEvent([]byte(`{"aggregateID":"u-1","aggregateType":"user","sequence":8,"event_type":"user.human.profile.changed","created_at":"2026-10-10T08:00:00Z"}`)); ok || err != nil {
		t.Fatalf("profile change: ok = %v, err = %v", ok, err)
	}
	if _, _, err := idp.ParseEvent([]byte(`{"aggregateType":"user","event_type":"user.removed"}`)); err == nil {
		t.Fatal("event without an aggregate was accepted")
	}
	if _, _, err := idp.ParseEvent([]byte(`not json`)); err == nil {
		t.Fatal("garbage was accepted")
	}
}

func TestDirectory(t *testing.T) {
	p := idptest.New(t)
	t0 := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	p.Record(idptest.Event{Subject: "u-1", Sequence: 3, Type: "user.locked", At: t0})
	p.Record(idptest.Event{Subject: "u-2", Sequence: 9, Type: "user.human.added", At: t0.Add(time.Minute)})
	p.Record(idptest.Event{Subject: "u-1", Sequence: 4, Type: "user.unlocked", At: t0.Add(2 * time.Minute)})
	p.Record(idptest.Event{Subject: "u-3", Sequence: 1, Type: "user.removed", At: t0.Add(3 * time.Minute)})
	d := &idp.Directory{Issuer: p.URL, Token: idptest.DirectoryToken, PageSize: 2}
	events, err := d.Events(t.Context(), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].Type != "user.locked" || events[1].Type != "user.unlocked" || events[2].Subject != "u-3" ||
		events[2].ID != "u-3/1" || !events[2].At.Equal(t0.Add(3*time.Minute)) {
		t.Fatalf("events = %+v", events)
	}
	searches := p.Searches()
	if _, has := searches[0]["from"]; has || searches[0]["asc"] != true || searches[0]["limit"] != float64(2) {
		t.Fatalf("first search = %v", searches[0])
	}
	if searches[1]["from"] != "2026-10-10T08:02:00Z" || len(searches[1]["eventTypes"].([]any)) != 5 {
		t.Fatalf("second search = %v", searches[1])
	}
	if later, err := d.Events(t.Context(), t0.Add(2*time.Minute)); err != nil || len(later) != 1 || later[0].Subject != "u-3" {
		t.Fatalf("after a cursor = %+v, %v", later, err)
	}

	p.SetState("u-1", "USER_STATE_LOCKED")
	p.SetState("u-2", "USER_STATE_ACTIVE")
	p.SetState("u-4", "USER_STATE_INACTIVE")
	p.SetState("u-5", "USER_STATE_SOMETHING")
	for sub, want := range map[string]model.IdentityState{"u-1": model.IdentityLocked, "u-2": model.IdentityActive,
		"u-3": model.IdentityRemoved, "u-4": model.IdentityDeactivated} {
		if got, err := d.State(t.Context(), sub); err != nil || got != want {
			t.Errorf("%s: %q, %v", sub, got, err)
		}
	}
	if _, err := d.State(t.Context(), "u-5"); err == nil {
		t.Error("an unknown state was accepted")
	}
	bad := &idp.Directory{Issuer: p.URL, Token: "wrong"}
	if _, err := bad.Events(t.Context(), time.Time{}); err == nil {
		t.Error("an unauthorized search succeeded")
	}
	if _, err := bad.State(t.Context(), "u-3"); err == nil {
		t.Error("an unauthorized read counted as removed")
	}
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	if _, err := (&idp.Directory{Issuer: srv.URL, Token: "x"}).State(t.Context(), "u-1"); !errors.Is(err, idp.ErrUnavailable) {
		t.Errorf("unreachable: %v", err)
	}
}

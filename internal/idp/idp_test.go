package idp_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"

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
		"version":"v2","sequence":7,"event_type":"user.deactivated","created_at":"2026-10-10T08:00:00.123456Z","userID":"admin"}`))
	if err != nil || !ok {
		t.Fatalf("ok = %v, err = %v", ok, err)
	}
	if e.Subject != "u-1" || e.Type != "user.deactivated" || e.ID != "u-1/7" || !e.At.Equal(time.Date(2026, 10, 10, 8, 0, 0, 123456000, time.UTC)) {
		t.Fatalf("event = %+v", e)
	}
	for _, kind := range []string{"user.human.profile.changed", "user.locked", "user.unlocked"} {
		if _, ok, err := idp.ParseEvent([]byte(`{"aggregateID":"u-1","aggregateType":"user","sequence":8,"event_type":"` + kind + `","created_at":"2026-10-10T08:00:00Z"}`)); ok || err != nil {
			t.Fatalf("%s: ok = %v, err = %v", kind, ok, err)
		}
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
	p.Record(idptest.Event{Subject: "u-1", Sequence: 3, Type: "user.deactivated", At: t0})
	p.Record(idptest.Event{Subject: "u-5", Sequence: 2, Type: "user.locked", At: t0.Add(30 * time.Second)})
	p.Record(idptest.Event{Subject: "u-2", Sequence: 9, Type: "user.human.added", At: t0.Add(time.Minute)})
	p.Record(idptest.Event{Subject: "u-1", Sequence: 4, Type: "user.reactivated", At: t0.Add(2 * time.Minute)})
	p.Record(idptest.Event{Subject: "u-3", Sequence: 1, Type: "user.removed", At: t0.Add(3 * time.Minute)})
	d := &idp.Directory{Issuer: p.URL, Token: idptest.DirectoryToken, PageSize: 2}
	events, err := d.Events(t.Context(), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].Type != "user.deactivated" || events[1].Type != "user.reactivated" || events[2].Subject != "u-3" ||
		events[2].ID != "u-3/1" || !events[2].At.Equal(t0.Add(3*time.Minute)) {
		t.Fatalf("events = %+v", events)
	}
	searches := p.Searches()
	if _, has := searches[0]["from"]; has || searches[0]["asc"] != true || searches[0]["limit"] != float64(2) ||
		len(searches[0]["eventTypes"].([]any)) != 3 {
		t.Fatalf("first search = %v", searches[0])
	}
	if later, err := d.Events(t.Context(), t0.Add(2*time.Minute)); err != nil || len(later) != 1 || later[0].Subject != "u-3" {
		t.Fatalf("after a cursor = %+v, %v", later, err)
	}
	if removed, err := d.Removed(t.Context(), "u-3"); err != nil || !removed {
		t.Fatalf("removed u-3 = %v, %v", removed, err)
	}
	if removed, err := d.Removed(t.Context(), "u-1"); err != nil || removed {
		t.Fatalf("removed u-1 = %v, %v", removed, err)
	}
	if last := p.Searches()[len(p.Searches())-1]; last["aggregateId"] != "u-1" {
		t.Fatalf("removal search = %v", last)
	}

	p.SetState("u-1", "USER_STATE_LOCKED")
	p.SetState("u-2", "USER_STATE_ACTIVE")
	p.SetState("u-4", "USER_STATE_INACTIVE")
	p.SetState("u-5", "USER_STATE_SOMETHING")
	// A locked user counts as active: locking suspends nothing.
	for sub, want := range map[string]model.IdentityState{"u-1": model.IdentityActive, "u-2": model.IdentityActive,
		"u-3": model.IdentityRemoved, "u-4": model.IdentityDeactivated} {
		if got, err := d.State(t.Context(), sub); err != nil || got != want {
			t.Errorf("%s: %q, %v", sub, got, err)
		}
	}
	if _, err := d.State(t.Context(), "u-5"); err == nil {
		t.Error("an unknown state was accepted")
	}
	// Only ZITADEL's missing-user error means removed, not any 404.
	p.SetUsersGone(true)
	if state, err := d.State(t.Context(), "u-3"); err == nil {
		t.Errorf("a 404 that is not a missing user = %q", state)
	}
	p.SetUsersGone(false)
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

func TestEventsAtPageBoundaries(t *testing.T) {
	p := idptest.New(t)
	t0 := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	p.Record(idptest.Event{Subject: "u-1", Sequence: 1, Type: "user.deactivated", At: t0.Add(-time.Minute)})
	// Two events of one time, which a page of two cuts between.
	p.Record(idptest.Event{Subject: "u-2", Sequence: 1, Type: "user.deactivated", At: t0})
	p.Record(idptest.Event{Subject: "u-3", Sequence: 1, Type: "user.deactivated", At: t0})
	p.Record(idptest.Event{Subject: "u-4", Sequence: 1, Type: "user.deactivated", At: t0.Add(time.Minute)})
	d := &idp.Directory{Issuer: p.URL, Token: idptest.DirectoryToken, PageSize: 2}
	events, err := d.Events(t.Context(), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	var subjects []string
	for _, e := range events {
		subjects = append(subjects, e.Subject)
	}
	if strings.Join(subjects, ",") != "u-1,u-2,u-3,u-4" {
		t.Fatalf("subjects = %v", subjects)
	}
}

func TestKeys(t *testing.T) {
	p := idptest.New(t)
	now := time.Now()
	c := idp.New(idp.Config{Issuer: p.URL, ClientID: idptest.ClientID, ClientSecret: idptest.ClientSecret, RedirectURL: redirect,
		Now: func() time.Time { return now }})
	// Keys that cannot be fetched make the provider unavailable, not the token invalid.
	p.SetKeysDown(true)
	if _, err := c.VerifyLogout(t.Context(), p.LogoutToken("u-1", "s-1", nil)); !errors.Is(err, idp.ErrUnavailable) {
		t.Fatalf("keys down: %v", err)
	}
	p.SetKeysDown(false)
	now = now.Add(time.Minute)
	if _, err := c.VerifyLogout(t.Context(), p.LogoutToken("u-1", "s-1", map[string]any{"iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})); err != nil {
		t.Fatalf("keys back: %v", err)
	}
	// Tokens naming keys the provider never had fetch the keys again at most every 30 seconds.
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	fetches := p.KeyFetches()
	for range 5 {
		if _, err := c.VerifyLogout(t.Context(), signWith(t, other, "made-up", p.URL, now)); !errors.Is(err, idp.ErrInvalidToken) {
			t.Fatalf("unknown key: %v", err)
		}
	}
	if got := p.KeyFetches() - fetches; got != 1 {
		t.Fatalf("refetches for unknown keys = %d", got)
	}
	now = now.Add(time.Minute)
	if _, err := c.VerifyLogout(t.Context(), signWith(t, other, "made-up", p.URL, now)); !errors.Is(err, idp.ErrInvalidToken) {
		t.Fatalf("unknown key later: %v", err)
	}
	if got := p.KeyFetches() - fetches; got != 2 {
		t.Fatalf("refetches after the wait = %d", got)
	}
}

// signWith signs a logout token with key under key ID kid.
func signWith(t *testing.T, key *rsa.PrivateKey, kid, issuer string, now time.Time) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithHeader("kid", kid))
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"iss": issuer, "aud": idptest.ClientID, "sub": "u-1", "iat": now.Unix(),
		"exp": now.Add(time.Minute).Unix(), "jti": kid + now.String(),
		"events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": map[string]any{}}})
	obj, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := obj.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

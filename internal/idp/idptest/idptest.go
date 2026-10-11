// Package idptest runs an in-process OpenID Connect provider for tests:
// discovery, signing keys, an authorization endpoint that signs whoever the
// test names straight in, and a token endpoint that checks PKCE and the
// client secret before handing out a signed ID Token.
package idptest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// Client credentials the provider accepts.
const (
	ClientID     = "explore-test"
	ClientSecret = "explore-test-secret"
	keyID        = "test-key"
)

// User is who the authorization endpoint signs in next.
type User struct {
	Subject, SessionID, Email, Name, Username string
	EmailVerified                             *bool
}

// Authorization is what one request to the authorization endpoint asked for.
type Authorization struct {
	ClientID, RedirectURI, State, Nonce, Challenge, ChallengeMethod, Prompt, Scope string
}

// Provider is the fake provider. Its fields may be changed between requests.
type Provider struct {
	URL string
	// Next is signed in by the authorization endpoint.
	Next User
	// Tamper edits the claims of each ID Token before it is signed, to test
	// how the client refuses bad tokens.
	Tamper func(claims map[string]any)
	// SignWith signs ID Tokens with another key, which the client must refuse.
	SignWith *rsa.PrivateKey
	// Deny makes the authorization endpoint answer error=access_denied.
	Deny bool

	key    *rsa.PrivateKey
	server *httptest.Server
	mu     sync.Mutex
	codes  map[string]grant
	auths  []Authorization
}

type grant struct {
	user                       User
	nonce, challenge, redirect string
}

// New starts a provider that stops when the test ends.
func New(t testing.TB) *Provider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &Provider{key: key, codes: map[string]grant{},
		Next: User{Subject: "user-1", SessionID: "sid-1", Email: "reader@example.org", Name: "Reader"}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("GET /keys", p.keys)
	mux.HandleFunc("GET /authorize", p.authorize)
	mux.HandleFunc("POST /token", p.token)
	p.server = httptest.NewServer(mux)
	p.URL = p.server.URL
	t.Cleanup(p.server.Close)
	return p
}

// Key is the provider's signing key.
func (p *Provider) Key() *rsa.PrivateKey { return p.key }

// Authorizations lists the authorization requests so far.
func (p *Provider) Authorizations() []Authorization {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Authorization(nil), p.auths...)
}

func (p *Provider) discovery(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                p.URL,
		"authorization_endpoint":                p.URL + "/authorize",
		"token_endpoint":                        p.URL + "/token",
		"jwks_uri":                              p.URL + "/keys",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"code_challenge_methods_supported":      []string{"S256"},
		"backchannel_logout_supported":          true,
		"backchannel_logout_session_supported":  true,
	})
}

func (p *Provider) keys(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
		{Key: p.key.Public(), KeyID: keyID, Algorithm: "RS256", Use: "sig"},
	}})
}

func (p *Provider) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	a := Authorization{ClientID: q.Get("client_id"), RedirectURI: q.Get("redirect_uri"), State: q.Get("state"),
		Nonce: q.Get("nonce"), Challenge: q.Get("code_challenge"), ChallengeMethod: q.Get("code_challenge_method"),
		Prompt: q.Get("prompt"), Scope: q.Get("scope")}
	p.mu.Lock()
	p.auths = append(p.auths, a)
	deny, user := p.Deny, p.Next
	p.mu.Unlock()
	if a.ClientID != ClientID || a.RedirectURI == "" || q.Get("response_type") != "code" {
		http.Error(w, "bad authorization request", http.StatusBadRequest)
		return
	}
	back, err := url.Parse(a.RedirectURI)
	if err != nil {
		http.Error(w, "bad redirect_uri", http.StatusBadRequest)
		return
	}
	params := url.Values{"state": {a.State}}
	if deny {
		params.Set("error", "access_denied")
	} else {
		code := random()
		p.mu.Lock()
		p.codes[code] = grant{user: user, nonce: a.Nonce, challenge: a.Challenge, redirect: a.RedirectURI}
		p.mu.Unlock()
		params.Set("code", code)
	}
	back.RawQuery = params.Encode()
	http.Redirect(w, r, back.String(), http.StatusFound)
}

func (p *Provider) token(w http.ResponseWriter, r *http.Request) {
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.PostFormValue("client_id"), r.PostFormValue("client_secret")
	}
	fail := func(code string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
	}
	if id != ClientID || secret != ClientSecret {
		fail("invalid_client")
		return
	}
	p.mu.Lock()
	g, found := p.codes[r.PostFormValue("code")]
	delete(p.codes, r.PostFormValue("code"))
	p.mu.Unlock()
	sum := sha256.Sum256([]byte(r.PostFormValue("code_verifier")))
	if !found || r.PostFormValue("grant_type") != "authorization_code" || r.PostFormValue("redirect_uri") != g.redirect ||
		base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge {
		fail("invalid_grant")
		return
	}
	now := time.Now()
	claims := map[string]any{
		"iss": p.URL, "aud": []string{ClientID, "project-1"}, "azp": ClientID, "sub": g.user.Subject,
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(), "nonce": g.nonce, "sid": g.user.SessionID,
		"email": g.user.Email, "name": g.user.Name, "preferred_username": g.user.Username,
	}
	if g.user.EmailVerified != nil {
		claims["email_verified"] = *g.user.EmailVerified
	}
	if p.Tamper != nil {
		p.Tamper(claims)
	}
	key := p.key
	if p.SignWith != nil {
		key = p.SignWith
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": random(), "token_type": "Bearer", "expires_in": 3600,
		"id_token": sign(key, claims, "")})
}

// LogoutToken signs a back-channel logout token for sub and sid (either may
// be empty), with extra claims merged over the defaults; a nil value removes
// a claim.
func (p *Provider) LogoutToken(sub, sid string, extra map[string]any) string {
	now := time.Now()
	claims := map[string]any{
		"iss": p.URL, "aud": ClientID, "iat": now.Unix(), "exp": now.Add(2 * time.Minute).Unix(), "jti": random(),
		"events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": map[string]any{}},
	}
	if sub != "" {
		claims["sub"] = sub
	}
	if sid != "" {
		claims["sid"] = sid
	}
	maps.Copy(claims, extra)
	maps.DeleteFunc(claims, func(_ string, v any) bool { return v == nil })
	return sign(p.key, claims, "logout+jwt")
}

// Sign signs arbitrary claims with the provider's key.
func (p *Provider) Sign(claims map[string]any) string { return sign(p.key, claims, "") }

func sign(key *rsa.PrivateKey, claims map[string]any, typ string) string {
	opts := (&jose.SignerOptions{}).WithHeader("kid", keyID)
	if typ != "" {
		opts = opts.WithType(jose.ContentType(typ))
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, opts)
	if err != nil {
		panic(err)
	}
	payload, _ := json.Marshal(claims)
	obj, err := signer.Sign(payload)
	if err != nil {
		panic(err)
	}
	out, err := obj.CompactSerialize()
	if err != nil {
		panic(err)
	}
	return out
}

func random() string {
	var b [24]byte
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

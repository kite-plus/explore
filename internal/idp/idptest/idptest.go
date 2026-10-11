// Package idptest runs an in-process OpenID Connect provider for tests:
// discovery, signing keys, an authorization endpoint that signs whoever the
// test names straight in, a token endpoint that checks PKCE and the client
// secret before handing out a signed ID Token, and ZITADEL's event search
// and user API for reconciliation.
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
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// Credentials the provider accepts.
const (
	ClientID     = "explore-test"
	ClientSecret = "explore-test-secret"
	// DirectoryToken is the bearer token of the event search and user API.
	DirectoryToken = "directory-token"
	keyID          = "test-key"
)

// User is who the authorization endpoint signs in.
type User struct {
	Subject, SessionID, Email, Name, Username string
	EmailVerified                             *bool
}

// Authorization is what one request to the authorization endpoint asked for.
type Authorization struct {
	ClientID, RedirectURI, State, Nonce, Challenge, ChallengeMethod, Prompt, Scope string
}

// Event is an account event the event search returns.
type Event struct {
	Subject  string
	Sequence int
	Type     string
	At       time.Time
}

// Provider is the fake provider; change it through its methods, which are
// safe while it serves.
type Provider struct {
	URL string

	key    *rsa.PrivateKey
	server *httptest.Server

	mu       sync.Mutex
	next     User
	tamper   func(claims map[string]any)
	signWith *rsa.PrivateKey
	deny     bool
	codes    map[string]grant
	auths    []Authorization
	states   map[string]string
	events   []Event
	searches []map[string]any
	// usersGone answers every user lookup with a 404 that is not ZITADEL's
	// missing-user error, as a wrong path or instance would.
	usersGone bool
	keysDown  bool
	keyFetch  int
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
	p := &Provider{key: key, codes: map[string]grant{}, states: map[string]string{},
		next: User{Subject: "user-1", SessionID: "sid-1", Email: "reader@example.org", Name: "Reader"}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("GET /keys", p.keys)
	mux.HandleFunc("GET /authorize", p.authorize)
	mux.HandleFunc("POST /token", p.token)
	mux.HandleFunc("POST /admin/v1/events/_search", p.searchEvents)
	mux.HandleFunc("GET /v2/users/{id}", p.user)
	p.server = httptest.NewServer(mux)
	p.URL = p.server.URL
	t.Cleanup(p.server.Close)
	return p
}

// SetUser names who the authorization endpoint signs in next.
func (p *Provider) SetUser(u User) { p.mu.Lock(); p.next = u; p.mu.Unlock() }

// SetSession changes the provider session of the next sign-in.
func (p *Provider) SetSession(sid string) { p.mu.Lock(); p.next.SessionID = sid; p.mu.Unlock() }

// SetTamper edits the claims of each ID Token before it is signed, to test
// how the client refuses bad tokens; nil stops it.
func (p *Provider) SetTamper(f func(claims map[string]any)) { p.mu.Lock(); p.tamper = f; p.mu.Unlock() }

// SetSigner signs ID Tokens with another key, which the client must refuse;
// nil goes back to the provider's own.
func (p *Provider) SetSigner(key *rsa.PrivateKey) { p.mu.Lock(); p.signWith = key; p.mu.Unlock() }

// SetDeny makes the authorization endpoint answer error=access_denied.
func (p *Provider) SetDeny(deny bool) { p.mu.Lock(); p.deny = deny; p.mu.Unlock() }

// SetState sets what the user API says of subject, a ZITADEL state such as
// "USER_STATE_LOCKED"; "" makes the user unknown, as a deleted one is.
func (p *Provider) SetState(subject, state string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if state == "" {
		delete(p.states, subject)
		return
	}
	p.states[subject] = state
}

// SetUsersGone makes every user lookup a 404 that is not a missing user.
func (p *Provider) SetUsersGone(gone bool) { p.mu.Lock(); p.usersGone = gone; p.mu.Unlock() }

// SetKeysDown makes the signing keys unreachable.
func (p *Provider) SetKeysDown(down bool) { p.mu.Lock(); p.keysDown = down; p.mu.Unlock() }

// KeyFetches counts requests for the signing keys.
func (p *Provider) KeyFetches() int { p.mu.Lock(); defer p.mu.Unlock(); return p.keyFetch }

// Record adds an event for the event search to return.
func (p *Provider) Record(e Event) { p.mu.Lock(); p.events = append(p.events, e); p.mu.Unlock() }

// Searches lists the bodies of the event searches so far.
func (p *Provider) Searches() []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.searches)
}

// Key is the provider's signing key.
func (p *Provider) Key() *rsa.PrivateKey { return p.key }

// Authorizations lists the authorization requests so far.
func (p *Provider) Authorizations() []Authorization {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.auths)
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
	p.mu.Lock()
	p.keyFetch++
	down := p.keysDown
	p.mu.Unlock()
	if down {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
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
	deny, user := p.deny, p.next
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
	tamper, key := p.tamper, p.key
	if p.signWith != nil {
		key = p.signWith
	}
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
	if tamper != nil {
		tamper(claims)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": random(), "token_type": "Bearer", "expires_in": 3600,
		"id_token": sign(key, claims, "")})
}

func (p *Provider) authorized(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Authorization") != "Bearer "+DirectoryToken {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":16,"message":"Errors.Token.Invalid (AUTH-7fs1e)"}`))
		return false
	}
	return true
}

// searchEvents answers like ZITADEL's ListEvents: events after "from",
// oldest first, at most "limit".
func (p *Provider) searchEvents(w http.ResponseWriter, r *http.Request) {
	if !p.authorized(w, r) {
		return
	}
	var q struct {
		Asc         bool     `json:"asc"`
		Limit       int      `json:"limit"`
		From        string   `json:"from"`
		EventTypes  []string `json:"eventTypes"`
		AggregateID string   `json:"aggregateId"`
	}
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	b, _ := json.Marshal(raw)
	_ = json.Unmarshal(b, &q)
	var from time.Time
	if q.From != "" {
		var err error
		if from, err = time.Parse(time.RFC3339Nano, q.From); err != nil {
			http.Error(w, "bad from", http.StatusBadRequest)
			return
		}
	}
	p.mu.Lock()
	p.searches = append(p.searches, raw)
	events := slices.Clone(p.events)
	p.mu.Unlock()
	slices.SortStableFunc(events, func(a, b Event) int { return a.At.Compare(b.At) })
	type out struct {
		Aggregate    map[string]any `json:"aggregate"`
		Sequence     string         `json:"sequence"`
		CreationDate string         `json:"creationDate"`
		Type         map[string]any `json:"type"`
	}
	page := []out{}
	for _, e := range events {
		if !e.At.After(from) || (len(q.EventTypes) > 0 && !slices.Contains(q.EventTypes, e.Type)) ||
			(q.AggregateID != "" && e.Subject != q.AggregateID) {
			continue
		}
		if q.Limit > 0 && len(page) == q.Limit {
			break
		}
		page = append(page, out{
			Aggregate: map[string]any{"id": e.Subject, "type": map[string]any{"type": "user"}, "resourceOwner": "org"},
			Sequence:  strconv.Itoa(e.Sequence), CreationDate: e.At.UTC().Format(time.RFC3339Nano),
			Type: map[string]any{"type": e.Type, "localized": map[string]any{"key": e.Type}},
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"events": page})
}

// user answers like ZITADEL's GetUserByID.
func (p *Provider) user(w http.ResponseWriter, r *http.Request) {
	if !p.authorized(w, r) {
		return
	}
	p.mu.Lock()
	state, ok := p.states[r.PathValue("id")]
	gone := p.usersGone
	p.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if gone {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":5,"message":"Not Found"}`))
		return
	}
	if !ok {
		// ZITADEL v4's answer, its message translated for the instance.
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":5,"message":"User could not be found (QUERY-Dfbg2)","details":[{"@type":"type.googleapis.com/zitadel.v1.ErrorDetail","id":"QUERY-Dfbg2","message":"User could not be found"}]}`))
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"userId": r.PathValue("id"), "state": state}})
}

// LogoutToken signs a back-channel logout token for sub and sid (either may
// be empty), with extra claims merged over the defaults; a nil value removes
// a claim.
func (p *Provider) LogoutToken(sub, sid string, extra map[string]any) string {
	now := time.Now()
	claims := map[string]any{
		"iss": p.URL, "aud": ClientID, "iat": now.Unix(), "exp": now.Add(15 * time.Minute).Unix(), "jti": random(),
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

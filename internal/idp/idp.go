// Package idp talks to the Kite Plus identity provider (OpenID Connect):
// sign-in with Authorization Code and PKCE, back-channel logout tokens, the
// signed user events it pushes, and reading those events back for
// reconciliation. docs/design/accounts.md section 1 is the contract.
package idp

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Config names the provider and Explore's client at it.
type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	// RedirectURL is the callback registered with the provider.
	RedirectURL string
	// HTTPClient talks to the provider; nil uses one with a 10 second timeout.
	HTTPClient *http.Client
	Now        func() time.Time
}

// Client signs readers in through the provider. Discovery happens on first
// use, so Explore starts while the provider is unreachable.
type Client struct {
	cfg Config

	mu       sync.Mutex
	provider *oidc.Provider
	retryAt  time.Time
}

// ErrUnavailable means the provider could not be reached or discovered.
var ErrUnavailable = errors.New("identity provider unavailable")

// discoveryRetry spaces out discovery attempts after one fails.
const discoveryRetry = 10 * time.Second

// New returns a client for cfg.
func New(cfg Config) *Client {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Client{cfg: cfg}
}

// Issuer is the issuer every identity of this provider carries.
func (c *Client) Issuer() string { return c.cfg.Issuer }

func (c *Client) discover() (*oidc.Provider, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.provider != nil {
		return c.provider, nil
	}
	now := c.cfg.Now()
	if now.Before(c.retryAt) {
		return nil, ErrUnavailable
	}
	// The provider keeps this context for fetching signing keys later, so it
	// must not be a request's; the HTTP client's timeout bounds each call.
	p, err := oidc.NewProvider(oidc.ClientContext(context.Background(), c.cfg.HTTPClient), c.cfg.Issuer)
	if err != nil {
		c.retryAt = now.Add(discoveryRetry)
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	c.provider = p
	return p, nil
}

func (c *Client) oauth(p *oidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.cfg.ClientID,
		ClientSecret: c.cfg.ClientSecret,
		Endpoint:     p.Endpoint(),
		RedirectURL:  c.cfg.RedirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}
}

func (c *Client) verifier(p *oidc.Provider) *oidc.IDTokenVerifier {
	return p.Verifier(&oidc.Config{ClientID: c.cfg.ClientID, Now: c.cfg.Now})
}

// AuthRequest is one sign-in's parameters. Verifier is the PKCE verifier,
// from NewVerifier.
type AuthRequest struct {
	State, Nonce, Verifier string
	// SelectAccount asks the provider to let the reader pick another account.
	SelectAccount bool
}

// NewVerifier returns a random PKCE verifier.
func NewVerifier() string { return oauth2.GenerateVerifier() }

// AuthURL is where the browser goes to sign in.
func (c *Client) AuthURL(r AuthRequest) (string, error) {
	p, err := c.discover()
	if err != nil {
		return "", err
	}
	opts := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(r.Verifier), oidc.Nonce(r.Nonce)}
	if r.SelectAccount {
		opts = append(opts, oauth2.SetAuthURLParam("prompt", "select_account"))
	}
	return c.oauth(p).AuthCodeURL(r.State, opts...), nil
}

// Identity is who the provider signed in. Only Issuer and Subject identify
// the reader; the rest is for display.
type Identity struct {
	Issuer, Subject string
	// SessionID is the provider's session ("sid"), which back-channel logout
	// names.
	SessionID     string
	Email         string
	EmailVerified *bool
	Name          string
	Username      string
}

// ErrInvalidToken means the provider's answer did not check out.
var ErrInvalidToken = errors.New("invalid token")

// Exchange redeems an authorization code and verifies the ID Token it
// brings: signature, issuer, audience, expiry and nonce.
func (c *Client) Exchange(ctx context.Context, code, verifier, nonce string) (Identity, error) {
	p, err := c.discover()
	if err != nil {
		return Identity{}, err
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.cfg.HTTPClient)
	tok, err := c.oauth(p).Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) {
			return Identity{}, fmt.Errorf("%w: token endpoint: %w", ErrInvalidToken, err)
		}
		return Identity{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	raw, _ := tok.Extra("id_token").(string)
	if raw == "" {
		return Identity{}, fmt.Errorf("%w: no id_token", ErrInvalidToken)
	}
	idt, err := c.verifier(p).Verify(ctx, raw)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if nonce == "" || subtle.ConstantTimeCompare([]byte(idt.Nonce), []byte(nonce)) != 1 {
		return Identity{}, fmt.Errorf("%w: nonce mismatch", ErrInvalidToken)
	}
	var claims struct {
		AuthorizedParty   string `json:"azp"`
		SessionID         string `json:"sid"`
		Email             string `json:"email"`
		EmailVerified     *bool  `json:"email_verified"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idt.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("%w: claims: %w", ErrInvalidToken, err)
	}
	// ZITADEL adds its project to the audience; azp then names the client.
	if claims.AuthorizedParty != "" && claims.AuthorizedParty != c.cfg.ClientID {
		return Identity{}, fmt.Errorf("%w: azp %q", ErrInvalidToken, claims.AuthorizedParty)
	}
	if idt.Subject == "" {
		return Identity{}, fmt.Errorf("%w: no subject", ErrInvalidToken)
	}
	return Identity{
		Issuer:        idt.Issuer,
		Subject:       idt.Subject,
		SessionID:     claims.SessionID,
		Email:         strings.TrimSpace(claims.Email),
		EmailVerified: claims.EmailVerified,
		Name:          strings.TrimSpace(claims.Name),
		Username:      strings.TrimSpace(claims.PreferredUsername),
	}, nil
}

// Logout is a verified back-channel logout token.
type Logout struct {
	Issuer, Subject, SessionID string
	// TokenID ("jti") lets the caller refuse a replay.
	TokenID  string
	IssuedAt time.Time
	Expiry   time.Time
}

// Logout tokens are delivered as they are made; one issued longer ago than
// this, or this far ahead of our clock, is refused.
const (
	LogoutMaxAge  = 10 * time.Minute
	logoutMaxSkew = 5 * time.Minute
)

// VerifyLogout checks a back-channel logout token: signature against the
// provider's keys, issuer, audience, expiry, issue time, the logout event,
// no nonce, and a sub or sid. Replays are the caller's to refuse by TokenID.
func (c *Client) VerifyLogout(ctx context.Context, raw string) (Logout, error) {
	p, err := c.discover()
	if err != nil {
		return Logout{}, err
	}
	lt, err := c.verifier(p).VerifyLogout(ctx, raw)
	if err != nil {
		return Logout{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	now := c.cfg.Now()
	if lt.IssuedAt.IsZero() || lt.IssuedAt.After(now.Add(logoutMaxSkew)) || lt.IssuedAt.Before(now.Add(-LogoutMaxAge)) {
		return Logout{}, fmt.Errorf("%w: iat %v", ErrInvalidToken, lt.IssuedAt)
	}
	return Logout{Issuer: lt.Issuer, Subject: lt.Subject, SessionID: lt.SessionID, TokenID: lt.TokenID,
		IssuedAt: lt.IssuedAt, Expiry: lt.Expiry}, nil
}

package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/idp"
	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/store"
)

// What the identity provider calls on its own: back-channel logout and its
// pushed user events. See docs/design/accounts.md section 1.

// receiptLifetime is how long a pushed event's ID is kept against a second
// delivery.
const receiptLifetime = 7 * 24 * time.Hour

// backchannelLogout ends the sessions a logout token names, when the reader
// signs out at the provider. A token that does not check out, or one seen
// before, is a 400, as OpenID Connect Back-Channel Logout 1.0 section 2.8
// says.
func (s *Server) backchannelLogout(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	refuse := func(status int, code string) {
		c.AbortWithStatusJSON(status, gin.H{"error": code})
	}
	raw := c.PostForm("logout_token")
	if raw == "" {
		refuse(http.StatusBadRequest, "invalid_request")
		return
	}
	ctx := c.Request.Context()
	lt, err := s.OIDC.VerifyLogout(ctx, raw)
	if errors.Is(err, idp.ErrUnavailable) {
		s.Log.Warn("identity provider unavailable", "error", err)
		refuse(http.StatusServiceUnavailable, "temporarily_unavailable")
		return
	}
	if err != nil {
		s.Log.Warn("logout token refused", "error", err)
		refuse(http.StatusBadRequest, "invalid_request")
		return
	}
	// Kept past the latest time the token could still be accepted.
	keep := lt.IssuedAt.Add(idp.LogoutMaxAge)
	if lt.Expiry.After(keep) {
		keep = lt.Expiry
	}
	keep = keep.Add(time.Hour)
	fresh, err := s.Store.RememberReceipt(ctx, store.ReceiptLogoutToken, lt.Issuer+" "+lt.TokenID, keep)
	if err != nil {
		s.storeError(c, err)
		return
	}
	if !fresh {
		s.Log.Warn("logout token replayed")
		refuse(http.StatusBadRequest, "invalid_request")
		return
	}
	n, err := s.Store.EndIdentitySessions(ctx, s.OIDC.Issuer(), lt.Subject, lt.SessionID)
	if err != nil {
		s.storeError(c, err)
		return
	}
	s.Log.Info("back-channel logout", "sessions", n)
	c.Status(http.StatusOK)
}

// maxEventBody bounds a pushed event.
const maxEventBody = 64 << 10

// identityLookupTimeout keeps the answer to a push within the 10 seconds
// the provider waits for it.
const identityLookupTimeout = 5 * time.Second

// identityHook receives a user event the provider pushes, signed with the
// shared secret. The event only names the account: other services' tokens
// can read the signing key, so the account's state is read back from the
// provider and that is what gets applied. A delivery seen before is
// acknowledged and not applied again.
func (s *Server) identityHook(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxEventBody+1))
	if err != nil || len(body) > maxEventBody {
		s.fail(c, http.StatusRequestEntityTooLarge, codeInvalidRequest)
		return
	}
	if idp.VerifySignature(c.GetHeader(idp.SignatureHeader), body, s.IdentityWebhookSecret, s.Now()) != nil {
		s.Log.Warn("identity event with a bad signature")
		s.fail(c, http.StatusUnauthorized, codeUnauthorized)
		return
	}
	e, ok, err := idp.ParseEvent(body)
	if err != nil {
		s.Log.Warn("identity event unreadable", "error", err)
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if !ok {
		c.Status(http.StatusOK)
		return
	}
	ctx := c.Request.Context()
	seen, err := s.Store.SeenReceipt(ctx, store.ReceiptEvent, e.ID)
	if err != nil {
		s.storeError(c, err)
		return
	}
	if seen {
		c.Status(http.StatusOK)
		return
	}
	outcome := store.IdentityIgnored
	if _, err := s.Store.UserByIdentity(ctx, s.OIDC.Issuer(), e.Subject); err == nil {
		lookup, cancel := context.WithTimeout(ctx, identityLookupTimeout)
		state, err := s.IdentityDirectory.State(lookup, e.Subject)
		cancel()
		if err != nil {
			// The hourly reconciliation reads it again.
			s.Log.Warn("reading an account from the identity provider failed", "error", err)
			s.fail(c, http.StatusServiceUnavailable, codeInternal)
			return
		}
		if state == model.IdentityRemoved && e.Type != model.RemovalEvent {
			s.Log.Warn("identity provider has no such account, but the event is not a removal", "type", e.Type)
			c.Status(http.StatusOK)
			return
		}
		if outcome, err = s.Store.ApplyIdentityState(ctx, s.OIDC.Issuer(), e.Subject, state); err != nil {
			s.storeError(c, err)
			return
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		s.storeError(c, err)
		return
	}
	if _, err := s.Store.RememberReceipt(ctx, store.ReceiptEvent, e.ID, s.Now().Add(receiptLifetime)); err != nil {
		s.storeError(c, err)
		return
	}
	s.Log.Info("identity event", "type", e.Type, "outcome", outcome)
	c.Status(http.StatusOK)
}

// appleAppSiteAssociation lets the iOS app claim the sign-in callback, both
// as a universal link and for ASWebAuthenticationSession's https callback.
func (s *Server) appleAppSiteAssociation(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, "application/json", encode(gin.H{
		"applinks": gin.H{"details": []gin.H{{
			"appIDs":     s.AppleAppIDs,
			"components": []gin.H{{"/": "/app/auth/done", "comment": "The end of signing in through Explore"}},
		}}},
		"webcredentials": gin.H{"apps": s.AppleAppIDs},
	}))
}

// assetLinks lets the Android app claim the sign-in callback as an App Link.
func (s *Server) assetLinks(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, "application/json", encode([]gin.H{{
		"relation": []string{"delegate_permission/common.handle_all_urls"},
		"target": gin.H{"namespace": "android_app", "package_name": s.AndroidPackage,
			"sha256_cert_fingerprints": s.AndroidCertSHA256},
	}}))
}

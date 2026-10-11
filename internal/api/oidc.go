package api

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/kite-plus/explore/internal/i18n"
	"github.com/kite-plus/explore/internal/idp"
	"github.com/kite-plus/explore/internal/store"
)

// Sign-in with the Kite Plus identity provider; docs/design/accounts.md
// section 1 describes the flows and section 1.1 the apps' part.

// flowLifetime is how long a sign-in may take at the provider; signing up
// there can mean waiting for a verification email.
const flowLifetime = 30 * time.Minute

// Flow modes: a sign-in on the site, linking an identity to the signed-in
// account, and an app's sign-in.
const (
	modeWeb  = "web"
	modeLink = "link"
	modeApp  = "app"
)

// Why a sign-in failed, passed to the page the reader lands on.
const (
	flowExpired            = "login_expired"
	flowFailed             = "login_failed"
	flowDenied             = "access_denied"
	flowUnavailable        = "unavailable"
	flowDisabled           = "account_disabled"
	flowEmailInUse         = "email_in_use"
	flowEmailUnverified    = "email_unverified"
	flowRegistrationClosed = "registration_closed"
	flowIdentityTaken      = "identity_taken"
	flowAlreadyLinked      = "already_linked"
)

// authOptions tells clients how this server signs readers in.
func (s *Server) authOptions(c *gin.Context) {
	var oidc gin.H
	if s.OIDC != nil {
		name := "Kite Plus"
		if lang(c) == i18n.Chinese {
			name = "Kite Plus 账号"
		}
		oidc = gin.H{"name": name, "account_url": s.IdentityAccountURL}
	}
	c.Header("Vary", "Accept-Language")
	cached(c, time.Minute, "application/json; charset=utf-8", encode(gin.H{"password": true, "oidc": oidc}))
}

// safeReturn keeps where a sign-in returns to on this site: a path from its
// root, never one a browser reads as another host ("//" or "/\"), nor the
// sign-in pages, which would go round in circles.
func safeReturn(raw, fallback string) string {
	if len(raw) > 1024 || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "/\\") ||
		strings.ContainsFunc(raw, func(r rune) bool { return r == '\\' || unicode.IsControl(r) || unicode.IsSpace(r) }) {
		return fallback
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil {
		return fallback
	}
	for _, p := range []string{"/auth/", "/login", "/en/login", "/app/"} {
		if u.Path == strings.TrimSuffix(p, "/") || strings.HasPrefix(u.Path, p) {
			return fallback
		}
	}
	return raw
}

// localePath puts path under /en when where is an English page.
func localePath(where, path string) string {
	if where == "/en" || strings.HasPrefix(where, "/en/") || strings.HasPrefix(where, "/en?") {
		return "/en" + path
	}
	return path
}

// withQuery adds a parameter to a path from safeReturn.
func withQuery(path, key, value string) string {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + key + "=" + url.QueryEscape(value)
}

// browserBinding returns the value of the cookie that ties sign-ins to this
// browser, setting one when there is none. A browser keeps one value for
// all its sign-ins, so two started side by side both finish.
func (s *Server) browserBinding(c *gin.Context) (string, error) {
	if v, err := c.Cookie(s.cookieName(loginCookie)); err == nil && len(v) == 43 {
		s.setCookie(c, loginCookie, v, int(flowLifetime/time.Second))
		return v, nil
	}
	v, err := randomToken()
	if err != nil {
		return "", err
	}
	s.setCookie(c, loginCookie, v, int(flowLifetime/time.Second))
	return v, nil
}

// prepareFlow records a sign-in bound to this browser and returns where at
// the provider it goes on. An error is the provider being unreachable, or
// the database failing (err is then wrapped in neither).
func (s *Server) prepareFlow(c *gin.Context, f store.LoginFlow, selectAccount bool) (string, error) {
	state, err1 := randomToken()
	nonce, err2 := randomToken()
	browser, err3 := s.browserBinding(c)
	if err := errors.Join(err1, err2, err3); err != nil {
		return "", err
	}
	verifier := idp.NewVerifier()
	target, err := s.OIDC.AuthURL(idp.AuthRequest{State: state, Nonce: nonce, Verifier: verifier, SelectAccount: selectAccount})
	if err != nil {
		s.Log.Warn("identity provider unavailable", "error", err)
		return "", err
	}
	f.StateHash, f.BrowserHash = sha256.Sum256([]byte(state)), sha256.Sum256([]byte(browser))
	f.Nonce, f.Verifier, f.Expires = nonce, verifier, s.Now().Add(flowLifetime)
	if err := s.Store.CreateLoginFlow(c.Request.Context(), f); err != nil {
		return "", err
	}
	return target, nil
}

// startFlow records a sign-in and sends the browser to the provider.
func (s *Server) startFlow(c *gin.Context, f store.LoginFlow, selectAccount bool) {
	c.Header("Cache-Control", "no-store")
	target, err := s.prepareFlow(c, f, selectAccount)
	switch {
	case errors.Is(err, idp.ErrUnavailable):
		s.flowError(c, f, flowUnavailable)
	case err != nil:
		s.storeError(c, err)
	default:
		c.Redirect(http.StatusFound, target)
	}
}

// flowError sends the reader to where the sign-in would have ended, saying
// why it did not: the app's callback, the account page for a link, or the
// sign-in page.
func (s *Server) flowError(c *gin.Context, f store.LoginFlow, code string) {
	c.Header("Cache-Control", "no-store")
	switch f.Mode {
	case modeApp:
		c.Redirect(http.StatusFound, withQuery(f.AppRedirect, "error", code))
	case modeLink:
		c.Redirect(http.StatusFound, withQuery(localePath(f.ReturnPath, "/account"), "error", code))
	default:
		target := withQuery(localePath(f.ReturnPath, "/login"), "error", code)
		if f.ReturnPath != "" {
			target = withQuery(target, "next", f.ReturnPath)
		}
		c.Redirect(http.StatusFound, target)
	}
}

// oidcLogin starts a sign-in on the site: GET /auth/login?return=/path.
func (s *Server) oidcLogin(c *gin.Context) {
	s.startFlow(c, store.LoginFlow{Mode: modeWeb, ReturnPath: safeReturn(c.Query("return"), "/")}, false)
}

// oidcLink starts linking a Kite Plus account to the signed-in one:
// POST /api/v1/auth/link {password, return}, with the session's CSRF
// header. The password is asked again because a linked identity outlives
// the session, so a stolen session must not be enough to attach one. It
// answers with the provider's address for the page to go to, rather than a
// redirect, which the site's CSP (form-action 'self') would stop.
func (s *Server) oidcLink(c *gin.Context) {
	var body struct {
		Password string `json:"password"`
		Return   string `json:"return"`
	}
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") || c.ShouldBindJSON(&body) != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	u := currentUser(c)
	if u.LinkedAt != nil || u.PasswordHash == "" {
		s.fail(c, http.StatusConflict, codeAlreadyLinked)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(body.Password)) != nil {
		s.fail(c, http.StatusForbidden, codeWrongPassword)
		return
	}
	// The provider asks which account to link rather than taking whichever
	// is signed in there.
	target, err := s.prepareFlow(c, store.LoginFlow{Mode: modeLink, ReturnPath: safeReturn(body.Return, "/account"), UserID: u.ID}, true)
	switch {
	case errors.Is(err, idp.ErrUnavailable):
		s.fail(c, http.StatusServiceUnavailable, codeIdentityUnavailable)
	case err != nil:
		s.storeError(c, err)
	default:
		writeJSON(c, http.StatusOK, gin.H{"url": target})
	}
}

// oidcCallback is where the provider sends the browser back. It checks the
// state against the browser that started the sign-in, then the ID Token,
// and finishes the sign-in as its mode says.
func (s *Server) oidcCallback(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	state := c.Query("state")
	browser, _ := c.Cookie(s.cookieName(loginCookie))
	if state == "" || browser == "" {
		s.flowError(c, store.LoginFlow{}, flowExpired)
		return
	}
	ctx := c.Request.Context()
	binding := sha256.Sum256([]byte(browser))
	f, err := s.Store.TakeLoginFlow(ctx, sha256.Sum256([]byte(state)), binding)
	// A custom-scheme app sign-in still needs the cookie for its
	// confirmation; otherwise it goes once no other sign-in waits for it.
	if f.AppRedirect != schemeCallback {
		s.releaseBinding(c, binding)
	}
	if errors.Is(err, store.ErrNotFound) {
		s.flowError(c, store.LoginFlow{}, flowExpired)
		return
	}
	if err != nil {
		s.storeError(c, err)
		return
	}
	if e := c.Query("error"); e != "" {
		code := flowFailed
		if e == "access_denied" {
			code = flowDenied
		}
		s.flowError(c, f, code)
		return
	}
	id, err := s.OIDC.Exchange(ctx, c.Query("code"), f.Verifier, f.Nonce)
	if err != nil {
		code := flowFailed
		if errors.Is(err, idp.ErrUnavailable) {
			code = flowUnavailable
		}
		s.Log.Warn("sign-in with the identity provider failed", "mode", f.Mode, "error", err)
		s.flowError(c, f, code)
		return
	}
	if f.Mode == modeLink {
		s.finishLink(c, f, id)
		return
	}
	u, code, err := s.identityUser(ctx, id)
	if err != nil {
		s.storeError(c, err)
		return
	}
	if code != "" {
		s.flowError(c, f, code)
		return
	}
	if f.Mode == modeApp && f.AppRedirect == schemeCallback {
		s.askAppConfirmation(c, f, u, id.SessionID, binding)
		return
	}
	if f.Mode == modeApp {
		s.issueAppCode(c, f, u, id.SessionID)
		return
	}
	token, _, err := s.newSession(ctx, u, id.SessionID, store.ClientWeb)
	if err != nil {
		s.storeError(c, err)
		return
	}
	s.setSessionCookie(c, token)
	c.Redirect(http.StatusFound, f.ReturnPath)
}

// releaseBinding clears the cookie that binds sign-ins to this browser once
// none of its sign-ins waits for it.
func (s *Server) releaseBinding(c *gin.Context, binding [sha256.Size]byte) {
	if pending, err := s.Store.LoginFlowsPending(c.Request.Context(), binding); err == nil && !pending {
		s.setCookie(c, loginCookie, "", -1)
	}
}

// identityUser finds the account linked to id or creates one from its
// claims. Accounts are never matched by email: an email a local account
// already uses stops the sign-in, and its owner links the identity from
// the account page instead. code is why the reader cannot sign in.
func (s *Server) identityUser(ctx context.Context, id idp.Identity) (store.User, string, error) {
	issuer := s.OIDC.Issuer()
	email := validEmail(id.Email)
	u, err := s.Store.UserByIdentity(ctx, issuer, id.Subject)
	if err == nil {
		if email != "" {
			if err := s.Store.RecordIdentityEmail(ctx, issuer, id.Subject, email); err != nil {
				return store.User{}, "", err
			}
		}
		return signInAs(u)
	}
	if !errors.Is(err, store.ErrNotFound) {
		return store.User{}, "", err
	}
	open, err := s.Store.Setting(ctx, "registration_enabled")
	if err != nil {
		return store.User{}, "", err
	}
	if open != "true" {
		return store.User{}, flowRegistrationClosed, nil
	}
	if email == "" {
		s.Log.Warn("identity provider sent no usable email")
		return store.User{}, flowFailed, nil
	}
	if id.EmailVerified != nil && !*id.EmailVerified {
		return store.User{}, flowEmailUnverified, nil
	}
	u, err = s.Store.CreateIdentityUser(ctx, issuer, id.Subject, email, displayName(id, email))
	if errors.Is(err, store.ErrEmailTaken) || errors.Is(err, store.ErrIdentityTaken) {
		// Another sign-in of the same reader that ran alongside may have
		// created the account; only then is the email someone else's.
		if u, err := s.Store.UserByIdentity(ctx, issuer, id.Subject); err == nil {
			return signInAs(u)
		} else if !errors.Is(err, store.ErrNotFound) {
			return store.User{}, "", err
		}
		return store.User{}, flowEmailInUse, nil
	}
	if err != nil {
		return store.User{}, "", err
	}
	s.Log.Info("account created from an identity")
	return u, "", nil
}

func signInAs(u store.User) (store.User, string, error) {
	if u.Disabled {
		return store.User{}, flowDisabled, nil
	}
	return u, "", nil
}

// displayName takes the name the provider knows, then the username, then
// the part of the email before the @, cut to the 80 characters names have.
func displayName(id idp.Identity, email string) string {
	name := id.Name
	if name == "" {
		name = id.Username
	}
	if name == "" {
		name, _, _ = strings.Cut(email, "@")
	}
	name = strings.TrimSpace(strings.ToValidUTF8(name, ""))
	if utf8.RuneCountInString(name) > 80 {
		name = string([]rune(name)[:80])
	}
	if name == "" {
		name = "Kite Plus"
	}
	return name
}

// finishLink attaches the identity to the account that started the link,
// which the browser must still be signed in to. As after a password change,
// the account's other sessions end; this one stays.
func (s *Server) finishLink(c *gin.Context, f store.LoginFlow, id idp.Identity) {
	u, token, _, err := s.sessionUser(c)
	if errors.Is(err, store.ErrNotFound) || (err == nil && u.ID != f.UserID) {
		s.flowError(c, f, flowExpired)
		return
	}
	if err != nil {
		s.storeError(c, err)
		return
	}
	err = s.Store.LinkIdentity(c.Request.Context(), f.UserID, s.OIDC.Issuer(), id.Subject, validEmail(id.Email),
		sha256.Sum256([]byte(token)))
	switch {
	case errors.Is(err, store.ErrIdentityTaken):
		s.flowError(c, f, flowIdentityTaken)
	case errors.Is(err, store.ErrAlreadyLinked):
		s.flowError(c, f, flowAlreadyLinked)
	case errors.Is(err, store.ErrUserDisabled), errors.Is(err, store.ErrNotFound):
		s.flowError(c, f, flowDisabled)
	case err != nil:
		s.storeError(c, err)
	default:
		s.Log.Info("identity linked to an account")
		c.Redirect(http.StatusFound, withQuery(f.ReturnPath, "linked", "1"))
	}
}

// myIdentity is the Kite Plus account linked to the signed-in one, for the
// account page: GET /api/v1/me/identity gives {email, linked_at}, or 404.
func (s *Server) myIdentity(c *gin.Context) {
	i, err := s.Store.IdentityOf(c.Request.Context(), currentUser(c).ID)
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"email": i.Email, "linked_at": i.LinkedAt.UTC()})
}

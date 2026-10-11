package api

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/store"
)

// Cookie names before the __Host- prefix that hostCookies adds.
const (
	sessionCookie = "explore_session"
	loginCookie   = "explore_login"
)

// sessionLifetime is how long a session lasts, on the web and in the apps.
const sessionLifetime = 30 * 24 * time.Hour

var unusedPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("unused-password-value"), bcrypt.DefaultCost)

func randomToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func csrfToken(token string) string {
	hash := sha256.Sum256([]byte("csrf:" + token))
	return hex.EncodeToString(hash[:])
}

func (s *Server) secure() bool { return strings.HasPrefix(s.PublicURL, "https://") }

// hostCookies reports whether cookie names take the __Host- prefix, which
// keeps a sibling subdomain of the site from planting one. It comes with
// sign-in through the identity provider, and needs https; see
// docs/design/accounts.md section 1.
func (s *Server) hostCookies() bool { return s.OIDC != nil && s.secure() }

func (s *Server) cookieName(name string) string {
	if s.hostCookies() {
		return "__Host-" + name
	}
	return name
}

// setCookie sets or, with maxAge -1, clears a cookie of this site.
func (s *Server) setCookie(c *gin.Context, name, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{Name: s.cookieName(name), Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: s.secure(), SameSite: http.SameSiteLaxMode})
}

// setSessionCookie sets the session cookie, or clears it for "". Under the
// __Host- name it also clears one left under the old name.
func (s *Server) setSessionCookie(c *gin.Context, token string) {
	maxAge := int(sessionLifetime / time.Second)
	if token == "" {
		maxAge = -1
	}
	s.setCookie(c, sessionCookie, token, maxAge)
	if s.hostCookies() {
		http.SetCookie(c.Writer, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, Secure: true,
			SameSite: http.SameSiteLaxMode})
	}
}

// sessionToken reads the session token from an Authorization: Bearer
// header, as the apps send it, or else from the session cookie. bearer
// reports a request with an Authorization header; it never falls back to
// the cookie.
func (s *Server) sessionToken(c *gin.Context) (token string, bearer bool) {
	if h := c.GetHeader("Authorization"); h != "" {
		scheme, value, _ := strings.Cut(h, " ")
		if !strings.EqualFold(scheme, "Bearer") {
			return "", true
		}
		return strings.TrimSpace(value), true
	}
	value, _ := c.Cookie(s.cookieName(sessionCookie))
	return value, false
}

// sessionUser is the account behind the request's session.
func (s *Server) sessionUser(c *gin.Context) (u store.User, token string, bearer bool, err error) {
	token, bearer = s.sessionToken(c)
	if len(token) != 43 {
		return store.User{}, "", bearer, store.ErrNotFound
	}
	u, err = s.Store.UserBySession(c.Request.Context(), sha256.Sum256([]byte(token)))
	return u, token, bearer, err
}

// csrfOK checks the CSRF header that cookie sessions need for writes. A
// browser never adds an Authorization header on its own, so a Bearer
// request needs none.
func csrfOK(c *gin.Context, token string, bearer bool) bool {
	if bearer || c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(c.GetHeader("X-CSRF-Token")), []byte(csrfToken(token))) == 1
}

func (s *Server) requireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, token, bearer, err := s.sessionUser(c)
		if errors.Is(err, store.ErrNotFound) {
			s.fail(c, http.StatusUnauthorized, codeUnauthorized)
			return
		}
		if err != nil {
			s.storeError(c, err)
			return
		}
		if !csrfOK(c, token, bearer) {
			s.fail(c, http.StatusForbidden, codeInvalidRequest)
			return
		}
		c.Set("user", u)
		c.Set("session_token", token)
		c.Next()
	}
}

func currentUser(c *gin.Context) store.User {
	u, _ := c.Get("user")
	return u.(store.User)
}

func validEmail(raw string) string {
	email := strings.ToLower(strings.TrimSpace(raw))
	a, err := mail.ParseAddress(email)
	if err != nil || a.Address != email || len(email) > 254 || !strings.Contains(email, ".") {
		return ""
	}
	return email
}

// Passwords are bounded in bytes; bcrypt reads no more than 72.
const (
	minPasswordLength = 12
	maxPasswordLength = 72
)

// validAccount checks the fields register and setup share, returning the
// normalized email and name.
func validAccount(rawEmail, rawName, password string) (email, name string, ok bool) {
	email = validEmail(rawEmail)
	name = strings.TrimSpace(rawName)
	runes := len([]rune(name))
	if email == "" || runes < 1 || runes > 80 || len(password) < minPasswordLength || len(password) > maxPasswordLength {
		return "", "", false
	}
	return email, name, true
}

func (s *Server) register(c *gin.Context) {
	enabled, err := s.Store.Setting(c.Request.Context(), "registration_enabled")
	if err != nil {
		s.storeError(c, err)
		return
	}
	if enabled != "true" {
		s.fail(c, http.StatusForbidden, codeFeatureDisabled)
		return
	}
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		s.fail(c, http.StatusUnsupportedMediaType, codeInvalidRequest)
		return
	}
	var body struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if c.ShouldBindJSON(&body) != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	email, name, ok := validAccount(body.Email, body.DisplayName, body.Password)
	if !ok {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		s.storeError(c, err)
		return
	}
	u, err := s.Store.CreateUser(c.Request.Context(), email, string(hash), name)
	if errors.Is(err, store.ErrConflict) {
		s.fail(c, http.StatusConflict, codeConflict)
		return
	}
	if err != nil {
		s.storeError(c, err)
		return
	}
	s.startSession(c, u, store.ClientWeb)
}

// login signs in with a password. An app names itself in client ("ios" or
// "android") and gets the token in the response, as the app code exchange
// gives it, instead of a cookie.
func (s *Server) login(c *gin.Context) {
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		s.fail(c, http.StatusUnsupportedMediaType, codeInvalidRequest)
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Client   string `json:"client"`
	}
	if c.ShouldBindJSON(&body) != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	client := cmp.Or(body.Client, store.ClientWeb)
	if client != store.ClientWeb && client != store.ClientIOS && client != store.ClientAndroid {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	u, err := s.Store.UserByEmail(c.Request.Context(), validEmail(body.Email))
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.storeError(c, err)
		return
	}
	// With no account, or one that signs in with Kite Plus and has no
	// password, a password is compared all the same, so the answer takes as
	// long as for a wrong one.
	hash := []byte(u.PasswordHash)
	if err != nil || u.PasswordHash == "" {
		hash = unusedPasswordHash
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(body.Password)) != nil || err != nil || u.PasswordHash == "" || u.Disabled {
		s.fail(c, http.StatusUnauthorized, codeBadCredentials)
		return
	}
	s.startSession(c, u, client)
}

// newSession creates a session for u and returns its token. sid is the
// identity provider's session it came from, if any.
func (s *Server) newSession(ctx context.Context, u store.User, sid, client string) (string, time.Time, error) {
	token, err := randomToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := s.Now().Add(sessionLifetime)
	err = s.Store.CreateSession(ctx, store.Session{UserID: u.ID, TokenHash: sha256.Sum256([]byte(token)), Expires: expires,
		SID: sid, Client: client})
	return token, expires, err
}

// startSession signs u in after a password: a cookie and the account for
// the web, the token for an app.
func (s *Server) startSession(c *gin.Context, u store.User, client string) {
	token, expires, err := s.newSession(c.Request.Context(), u, "", client)
	if err != nil {
		s.storeError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	if client != store.ClientWeb {
		writeJSON(c, http.StatusOK, s.appSessionJSON(u, token, expires))
		return
	}
	s.setSessionCookie(c, token)
	writeJSON(c, http.StatusOK, s.userJSON(u, token))
}

func (s *Server) appSessionJSON(u store.User, token string, expires time.Time) gin.H {
	return gin.H{"token": token, "expires_at": expires.UTC().Truncate(time.Second), "user": s.userJSON(u, token)}
}

// userJSON is the account as GET /api/v1/me gives it. password_login and
// temporary_password are false for every account once Kite Plus is on, as
// no password works then; a session from before the switch must not offer
// to change one.
func (s *Server) userJSON(u store.User, token string) gin.H {
	var linked *time.Time
	if u.LinkedAt != nil {
		t := u.LinkedAt.UTC()
		linked = &t
	}
	passwords := !s.kitePlusOnly()
	return gin.H{"id": u.ID, "number": u.Number, "created_at": u.CreatedAt.UTC(), "temporary_password": u.TemporaryPassword && passwords,
		"email": u.Email, "display_name": u.DisplayName,
		"is_admin": u.IsAdmin, "csrf_token": csrfToken(token),
		"password_login": u.PasswordHash != "" && passwords, "identity_linked_at": linked}
}

func (s *Server) me(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	writeJSON(c, http.StatusOK, s.userJSON(currentUser(c), c.GetString("session_token")))
}

func (s *Server) logout(c *gin.Context) {
	token := c.GetString("session_token")
	if err := s.Store.DeleteSession(c.Request.Context(), sha256.Sum256([]byte(token))); err != nil {
		s.storeError(c, err)
		return
	}
	s.setSessionCookie(c, "")
	c.Status(http.StatusNoContent)
}

func (s *Server) updateMe(c *gin.Context) {
	var body struct {
		DisplayName string `json:"display_name"`
	}
	if c.ShouldBindJSON(&body) != nil || len([]rune(strings.TrimSpace(body.DisplayName))) < 1 || len([]rune(strings.TrimSpace(body.DisplayName))) > 80 {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	u := currentUser(c)
	u.DisplayName = strings.TrimSpace(body.DisplayName)
	if err := s.Store.UpdateUserName(c.Request.Context(), u.ID, u.DisplayName); err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, s.userJSON(u, c.GetString("session_token")))
}

// changePassword sets a new password once the current one checks out, and
// signs out every other session of the account.
func (s *Server) changePassword(c *gin.Context) {
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		s.fail(c, http.StatusUnsupportedMediaType, codeInvalidRequest)
		return
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if c.ShouldBindJSON(&body) != nil || len(body.NewPassword) < minPasswordLength || len(body.NewPassword) > maxPasswordLength {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	u := currentUser(c)
	if u.PasswordHash == "" {
		s.fail(c, http.StatusConflict, codePasswordDisabled)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(body.CurrentPassword)) != nil {
		s.fail(c, http.StatusForbidden, codeWrongPassword)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		s.storeError(c, err)
		return
	}
	if err := s.Store.SetPassword(c.Request.Context(), u.ID, string(hash), sha256.Sum256([]byte(c.GetString("session_token")))); err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) deleteMe(c *gin.Context) {
	if err := s.Store.DeleteUser(c.Request.Context(), currentUser(c).ID); err != nil {
		if errors.Is(err, store.ErrLastAdmin) {
			s.fail(c, http.StatusConflict, codeLastAdmin)
			return
		}
		s.storeError(c, err)
		return
	}
	s.setSessionCookie(c, "")
	c.Status(http.StatusNoContent)
}

func (s *Server) subscriptions(c *gin.Context) {
	blogs, err := s.Store.Subscriptions(c.Request.Context(), currentUser(c).ID)
	if err != nil {
		s.storeError(c, err)
		return
	}
	out := make([]blogJSON, 0, len(blogs))
	for _, b := range blogs {
		out = append(out, toBlog(b))
	}
	c.Header("Cache-Control", "private, no-store")
	writeJSON(c, http.StatusOK, gin.H{"data": out})
}

func (s *Server) addSubscription(c *gin.Context) {
	if err := s.Store.AddSubscription(c.Request.Context(), currentUser(c).ID, strings.ToLower(c.Param("host"))); err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) removeSubscription(c *gin.Context) {
	if err := s.Store.RemoveSubscription(c.Request.Context(), currentUser(c).ID, strings.ToLower(c.Param("host"))); err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) following(c *gin.Context) {
	cur, limit, language, ok := s.pageParams(c)
	if !ok {
		return
	}
	tag := c.Query("tag")
	if _, known := model.TagBySlug(tag); tag != "" && !known {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	rows, err := s.Store.FollowingStream(c.Request.Context(), currentUser(c).ID, store.StreamQuery{Lang: language, Tag: tag, Limit: limit + 1, Cursor: cur})
	if err != nil {
		s.storeError(c, err)
		return
	}
	out := listJSON[entryJSON]{Data: make([]entryJSON, 0, len(rows))}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		next := encodeCursor(store.Cursor{At: last.SortAt, ID: last.ID})
		out.NextCursor = &next
	}
	for _, row := range rows {
		e := toEntry(row.Entry)
		e.Blog = &blogRefJSON{Host: row.Blog.Host, Name: row.Blog.Name, SiteURL: row.Blog.SiteURL, Language: row.Blog.Language}
		out.Data = append(out.Data, e)
	}
	c.Header("Cache-Control", "private, no-store")
	writeJSON(c, http.StatusOK, out)
}

func (s *Server) ownedBlogs(c *gin.Context) {
	blogs, err := s.Store.OwnedBlogs(c.Request.Context(), currentUser(c).ID)
	if err != nil {
		s.storeError(c, err)
		return
	}
	out := make([]blogJSON, 0, len(blogs))
	for _, b := range blogs {
		out = append(out, toBlog(b))
	}
	c.Header("Cache-Control", "private, no-store")
	writeJSON(c, http.StatusOK, gin.H{"data": out})
}

func (s *Server) startBlogClaim(c *gin.Context) {
	host := strings.ToLower(c.Param("host"))
	token, err := randomToken()
	if err != nil {
		s.storeError(c, err)
		return
	}
	if err := s.Store.CreateBlogClaim(c.Request.Context(), currentUser(c).ID, host, sha256.Sum256([]byte(token)), s.Now().Add(30*time.Minute)); err != nil {
		s.storeError(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	writeJSON(c, http.StatusOK, gin.H{"record": "_explore-claim." + host, "value": "explore-claim=" + token, "expires_in_seconds": 1800})
}

func (s *Server) verifyBlogClaim(c *gin.Context) {
	host := strings.ToLower(c.Param("host"))
	want, err := s.Store.BlogClaimHash(c.Request.Context(), currentUser(c).ID, host)
	if err != nil {
		s.storeError(c, err)
		return
	}
	lookup := s.LookupTXT
	if lookup == nil {
		lookup = net.DefaultResolver.LookupTXT
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	records, err := lookup(ctx, "_explore-claim."+host)
	if err != nil {
		s.fail(c, http.StatusBadGateway, codeVerificationFailed)
		return
	}
	for _, record := range records {
		value, ok := strings.CutPrefix(strings.TrimSpace(record), "explore-claim=")
		if !ok {
			continue
		}
		hash := sha256.Sum256([]byte(value))
		if subtle.ConstantTimeCompare(hash[:], want[:]) == 1 {
			if err := s.Store.ConfirmBlogClaim(c.Request.Context(), currentUser(c).ID, host, hash); err != nil {
				s.storeError(c, err)
				return
			}
			c.Status(http.StatusNoContent)
			return
		}
	}
	s.fail(c, http.StatusConflict, codeVerificationFailed)
}

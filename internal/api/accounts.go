package api

import (
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

const sessionCookie = "explore_session"

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

func (s *Server) sessionUser(c *gin.Context) (store.User, string, error) {
	value, err := c.Cookie(sessionCookie)
	if err != nil || len(value) != 43 {
		return store.User{}, "", store.ErrNotFound
	}
	u, err := s.Store.UserBySession(c.Request.Context(), sha256.Sum256([]byte(value)))
	return u, value, err
}

func (s *Server) requireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, token, err := s.sessionUser(c)
		if errors.Is(err, store.ErrNotFound) {
			s.fail(c, http.StatusUnauthorized, codeUnauthorized)
			return
		}
		if err != nil {
			s.storeError(c, err)
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			if subtle.ConstantTimeCompare([]byte(c.GetHeader("X-CSRF-Token")), []byte(csrfToken(token))) != 1 {
				s.fail(c, http.StatusForbidden, codeInvalidRequest)
				return
			}
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
	email := validEmail(body.Email)
	name := strings.TrimSpace(body.DisplayName)
	if email == "" || len([]rune(name)) < 1 || len([]rune(name)) > 80 || len(body.Password) < 12 || len(body.Password) > 72 {
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
	s.startSession(c, u)
}

func (s *Server) login(c *gin.Context) {
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		s.fail(c, http.StatusUnsupportedMediaType, codeInvalidRequest)
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&body) != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	u, err := s.Store.UserByEmail(c.Request.Context(), validEmail(body.Email))
	if errors.Is(err, store.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(unusedPasswordHash, []byte(body.Password))
	}
	if errors.Is(err, store.ErrNotFound) || (err == nil && (u.Disabled || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(body.Password)) != nil)) {
		s.fail(c, http.StatusUnauthorized, codeBadCredentials)
		return
	}
	if err != nil {
		s.storeError(c, err)
		return
	}
	s.startSession(c, u)
}

func (s *Server) startSession(c *gin.Context, u store.User) {
	token, err := randomToken()
	if err != nil {
		s.storeError(c, err)
		return
	}
	if err := s.Store.CreateSession(c.Request.Context(), u.ID, sha256.Sum256([]byte(token)), s.Now().Add(30*24*time.Hour)); err != nil {
		s.storeError(c, err)
		return
	}
	secure := strings.HasPrefix(s.PublicURL, "https://")
	http.SetCookie(c.Writer, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", MaxAge: 30 * 24 * 60 * 60,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
	c.Header("Cache-Control", "no-store")
	writeJSON(c, http.StatusOK, userJSON(u, token))
}

func userJSON(u store.User, token string) gin.H {
	return gin.H{"id": u.ID, "email": u.Email, "display_name": u.DisplayName,
		"is_admin": u.IsAdmin, "csrf_token": csrfToken(token)}
}

func (s *Server) me(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	writeJSON(c, http.StatusOK, userJSON(currentUser(c), c.GetString("session_token")))
}

func (s *Server) logout(c *gin.Context) {
	token := c.GetString("session_token")
	if err := s.Store.DeleteSession(c.Request.Context(), sha256.Sum256([]byte(token))); err != nil {
		s.storeError(c, err)
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true,
		Secure: strings.HasPrefix(s.PublicURL, "https://"), SameSite: http.SameSiteLaxMode})
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
	writeJSON(c, http.StatusOK, userJSON(u, c.GetString("session_token")))
}

func (s *Server) deleteMe(c *gin.Context) {
	if err := s.Store.DeleteUser(c.Request.Context(), currentUser(c).ID); err != nil {
		s.storeError(c, err)
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true,
		Secure: strings.HasPrefix(s.PublicURL, "https://"), SameSite: http.SameSiteLaxMode})
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

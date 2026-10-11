package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/i18n"
	"github.com/kite-plus/explore/internal/store"
)

// App sign-in through the server; docs/design/accounts.md section 1.1.

// appCodeLifetime is how long an app has to exchange its code, and
// confirmLifetime how long the reader has to confirm a custom-scheme app
// sign-in.
const (
	appCodeLifetime = 2 * time.Minute
	confirmLifetime = 10 * time.Minute
)

// schemeCallback is the app's custom-scheme callback, for servers whose
// addresses the app cannot claim.
const schemeCallback = "explore://app/auth/done"

func (s *Server) appCallback() string { return s.PublicURL + "/app/auth/done" }

// appLinksPublished reports whether this server lets the apps claim its
// callback address, in which case it never hands codes to a custom scheme
// any app could register.
func (s *Server) appLinksPublished() bool {
	return len(s.AppleAppIDs) > 0 || len(s.AndroidCertSHA256) > 0
}

// only reports whether v has nothing but ASCII letters, digits and the
// characters in extra.
func only(v, extra string) bool {
	return !strings.ContainsFunc(v, func(r rune) bool {
		alnum := r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		return !alnum && !strings.ContainsRune(extra, r)
	})
}

// pkceChallenge is a base64url SHA-256, 43 characters.
func pkceChallenge(v string) bool { return len(v) == 43 && only(v, "-_") }

// pkceVerifier follows RFC 7636 section 4.1.
func pkceVerifier(v string) bool { return len(v) >= 43 && len(v) <= 128 && only(v, "-._~") }

// appStart begins an app's sign-in in the system browser:
// GET /auth/app/start?challenge=...&client=ios|android, with switch=1 to pick
// another account and callback=scheme for the custom-scheme callback.
func (s *Server) appStart(c *gin.Context) {
	challenge, client := c.Query("challenge"), c.Query("client")
	if !pkceChallenge(challenge) || (client != store.ClientIOS && client != store.ClientAndroid) {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	redirect := s.appCallback()
	switch c.Query("callback") {
	case "", "https":
	case "scheme":
		if s.appLinksPublished() {
			s.fail(c, http.StatusBadRequest, codeInvalidRequest)
			return
		}
		redirect = schemeCallback
	default:
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	s.startFlow(c, store.LoginFlow{Mode: modeApp, Challenge: challenge, Client: client, AppRedirect: redirect},
		c.Query("switch") == "1")
}

// issueAppCode ends an app's sign-in in the browser: rather than a cookie,
// the app's callback gets a one-time code that only the app's verifier
// redeems.
func (s *Server) issueAppCode(c *gin.Context, f store.LoginFlow, u store.User, sid string) {
	code, err := randomToken()
	if err != nil {
		s.storeError(c, err)
		return
	}
	if err := s.Store.CreateAppCode(c.Request.Context(), store.AppCode{CodeHash: sha256.Sum256([]byte(code)), UserID: u.ID,
		SID: sid, Challenge: f.Challenge, Client: f.Client, Expires: s.Now().Add(appCodeLifetime)}); err != nil {
		s.storeError(c, err)
		return
	}
	c.Redirect(http.StatusFound, withQuery(f.AppRedirect, "code", code))
}

// appExchange redeems an app's code with its verifier for a session token:
// POST /api/v1/auth/app/exchange {code, verifier}. Whatever is wrong, the
// answer is the same 400 invalid_grant.
func (s *Server) appExchange(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var body struct {
		Code     string `json:"code"`
		Verifier string `json:"verifier"`
	}
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") || c.ShouldBindJSON(&body) != nil ||
		len(body.Code) != 43 || !pkceVerifier(body.Verifier) {
		s.fail(c, http.StatusBadRequest, codeInvalidGrant)
		return
	}
	token, err := randomToken()
	if err != nil {
		s.storeError(c, err)
		return
	}
	sum := sha256.Sum256([]byte(body.Verifier))
	answer := base64.RawURLEncoding.EncodeToString(sum[:])
	expires := s.Now().Add(sessionLifetime)
	u, err := s.Store.RedeemAppCode(c.Request.Context(), sha256.Sum256([]byte(body.Code)),
		func(challenge string) bool { return subtle.ConstantTimeCompare([]byte(answer), []byte(challenge)) == 1 },
		sha256.Sum256([]byte(token)), expires)
	if errors.Is(err, store.ErrInvalidGrant) {
		s.fail(c, http.StatusBadRequest, codeInvalidGrant)
		return
	}
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, appSessionJSON(u, token, expires))
}

// askAppConfirmation stops an app sign-in bound for the custom scheme in
// the browser, asking the reader to confirm it. Any app can register the
// scheme and any web page can start the sign-in, and a reader already
// signed in at the provider comes straight back, so without this step a
// page could hand the reader's session to whatever app owns the scheme.
// The confirmation belongs to this browser, through the sign-in's binding
// cookie, and the page cannot be framed.
func (s *Server) askAppConfirmation(c *gin.Context, f store.LoginFlow, u store.User, sid string, binding [sha256.Size]byte) {
	token, err := randomToken()
	if err != nil {
		s.storeError(c, err)
		return
	}
	if err := s.Store.CreateAppConfirmation(c.Request.Context(), store.AppConfirmation{TokenHash: sha256.Sum256([]byte(token)),
		BrowserHash: binding, UserID: u.ID, SID: sid, Challenge: f.Challenge, Client: f.Client,
		Expires: s.Now().Add(confirmLifetime)}); err != nil {
		s.storeError(c, err)
		return
	}
	text := appConfirmText[i18n.English]
	if lang(c) == i18n.Chinese {
		text = appConfirmText[i18n.Chinese]
	}
	who := u.DisplayName + " (" + u.Email + ")"
	if lang(c) == i18n.Chinese {
		who = u.DisplayName + "（" + u.Email + "）"
	}
	host := s.PublicURL
	if parsed, err := url.Parse(s.PublicURL); err == nil {
		host = parsed.Host
	}
	page := appConfirmPage{Lang: text.Lang, Title: text.Title, Question: fmt.Sprintf(text.Question, who),
		Detail: fmt.Sprintf(text.Detail, host), Continue: text.Continue, Cancel: text.Cancel, Token: token,
		CancelURL: template.URL(withQuery(schemeCallback, "error", flowDenied))}
	s.writePage(c, appConfirmTemplate, page, "form-action 'self' explore:")
}

// appConfirm issues the code once the reader confirms a custom-scheme app
// sign-in: POST /auth/app/confirm from the confirmation page, in the
// browser that started the sign-in.
func (s *Server) appConfirm(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if origin := c.GetHeader("Origin"); origin != "" && origin != s.publicOrigin() {
		s.fail(c, http.StatusForbidden, codeInvalidRequest)
		return
	}
	browser, _ := c.Cookie(s.cookieName(loginCookie))
	token := c.PostForm("token")
	if browser == "" || token == "" {
		c.Redirect(http.StatusSeeOther, withQuery(schemeCallback, "error", flowExpired))
		return
	}
	ctx := c.Request.Context()
	binding := sha256.Sum256([]byte(browser))
	conf, err := s.Store.TakeAppConfirmation(ctx, sha256.Sum256([]byte(token)), binding)
	s.releaseBinding(c, binding)
	if errors.Is(err, store.ErrNotFound) {
		c.Redirect(http.StatusSeeOther, withQuery(schemeCallback, "error", flowExpired))
		return
	}
	if err != nil {
		s.storeError(c, err)
		return
	}
	code, err := randomToken()
	if err != nil {
		s.storeError(c, err)
		return
	}
	if err := s.Store.CreateAppCode(ctx, store.AppCode{CodeHash: sha256.Sum256([]byte(code)), UserID: conf.UserID,
		SID: conf.SID, Challenge: conf.Challenge, Client: conf.Client, Expires: s.Now().Add(appCodeLifetime)}); err != nil {
		s.storeError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, withQuery(schemeCallback, "code", code))
}

func (s *Server) publicOrigin() string {
	u, err := url.Parse(s.PublicURL)
	if err != nil {
		return s.PublicURL
	}
	return u.Scheme + "://" + u.Host
}

// writePage writes one of the few pages serve draws itself: not cached, not
// indexed, never framed, with a policy that allows the page's own style and
// formAction for its forms.
func (s *Server) writePage(c *gin.Context, t *template.Template, data any, formAction string) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Robots-Tag", "noindex")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Vary", "Accept-Language")
	c.Header("Content-Security-Policy", "default-src 'none'; style-src '"+pageStyleHash+"'; base-uri 'none'; "+
		formAction+"; frame-ancestors 'none'")
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusOK)
	_ = t.Execute(c.Writer, data)
}

type appConfirmPage struct {
	Lang, Title, Question, Detail, Continue, Cancel, Token string
	// CancelURL is the app's custom scheme, which html/template would
	// otherwise take for an unsafe address.
	CancelURL template.URL
}

type appConfirmCopy struct{ Lang, Title, Question, Detail, Continue, Cancel string }

var appConfirmText = map[i18n.Lang]appConfirmCopy{
	i18n.Chinese: {"zh-CN", "登录 Kite 应用", "以 %s的身份登录 Kite 应用？",
		"Kite 应用将登录 %s 上的这个账号。如果你没有在 Kite 应用里点「登录」，请取消。", "继续", "取消"},
	i18n.English: {"en", "Sign in to the Kite app", "Continue to the Kite app as %s?",
		"The Kite app will be signed in to this account on %s. If you did not tap Sign in in the Kite app, cancel.", "Continue", "Cancel"},
}

var appConfirmTemplate = template.Must(template.New("confirm").Parse(`<!doctype html>
<html lang="{{.Lang}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex"><title>{{.Title}}</title><style>` + pageStyle + `</style></head>
<body><h1>{{.Question}}</h1><p>{{.Detail}}</p>
<form method="post" action="/auth/app/confirm"><input type="hidden" name="token" value="{{.Token}}">
<button type="submit">{{.Continue}}</button> <a href="{{.CancelURL}}">{{.Cancel}}</a></form></body></html>
`))

// appDone is the callback address opened in the browser, which happens when
// the app is not installed or its claim on the address is not in effect. It
// neither shows nor uses the code, which expires unused.
func (s *Server) appDone(c *gin.Context) {
	text := appDoneText[i18n.English]
	if lang(c) == i18n.Chinese {
		text = appDoneText[i18n.Chinese]
	}
	s.writePage(c, appDonePage, text, "form-action 'none'")
}

type appDoneCopy struct{ Lang, Title, Body, Home string }

var appDoneText = map[i18n.Lang]appDoneCopy{
	i18n.Chinese: {"zh-CN", "请在 Kite 应用中完成登录",
		"登录没有回到 Kite 应用。请回到应用，再点一次「登录」。这个页面不使用地址里的授权码，它两分钟后就会失效。", "返回 Explore"},
	i18n.English: {"en", "Finish signing in in the Kite app",
		"Signing in did not get back to the Kite app. Go back to the app and tap Sign in again. This page does not use the code in its address, which stops working in two minutes.", "Go to Explore"},
}

const pageStyle = `body{font:16px/1.6 system-ui,sans-serif;max-width:32rem;margin:15vh auto;padding:0 1.5rem;color:#1f2933}` +
	`@media(prefers-color-scheme:dark){body{background:#111827;color:#e5e7eb}a{color:#93c5fd}}h1{font-size:1.25rem}` +
	`button{font:inherit;padding:.5rem 1.25rem;margin-right:1rem;border:0;border-radius:.375rem;background:#0f766e;color:#fff;cursor:pointer}`

var pageStyleHash = func() string {
	sum := sha256.Sum256([]byte(pageStyle))
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
}()

var appDonePage = template.Must(template.New("done").Parse(`<!doctype html>
<html lang="{{.Lang}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex"><title>{{.Title}}</title><style>` + pageStyle + `</style></head>
<body><h1>{{.Title}}</h1><p>{{.Body}}</p><p><a href="/">{{.Home}}</a></p></body></html>
`))

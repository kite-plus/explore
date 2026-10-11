package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/i18n"
	"github.com/kite-plus/explore/internal/store"
)

// App sign-in through the server; docs/design/accounts.md section 1.1.

// appCodeLifetime is how long an app has to exchange its code.
const appCodeLifetime = 2 * time.Minute

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

// appDone is the callback address opened in the browser, which happens when
// the app is not installed or its claim on the address is not in effect. It
// neither shows nor uses the code, which expires unused.
func (s *Server) appDone(c *gin.Context) {
	text := appDoneText[i18n.English]
	if lang(c) == i18n.Chinese {
		text = appDoneText[i18n.Chinese]
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Robots-Tag", "noindex")
	c.Header("Vary", "Accept-Language")
	c.Header("Content-Security-Policy", "default-src 'none'; style-src '"+appDoneStyleHash+"'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = appDonePage.Execute(c.Writer, text)
}

type appDoneCopy struct{ Lang, Title, Body, Home string }

var appDoneText = map[i18n.Lang]appDoneCopy{
	i18n.Chinese: {"zh-CN", "请在 Kite 应用中完成登录",
		"登录没有回到 Kite 应用。请回到应用，再点一次「登录」。这个页面不使用地址里的授权码，它两分钟后就会失效。", "返回 Explore"},
	i18n.English: {"en", "Finish signing in in the Kite app",
		"Signing in did not get back to the Kite app. Go back to the app and tap Sign in again. This page does not use the code in its address, which stops working in two minutes.", "Go to Explore"},
}

const appDoneStyle = `body{font:16px/1.6 system-ui,sans-serif;max-width:32rem;margin:15vh auto;padding:0 1.5rem;color:#1f2933}` +
	`@media(prefers-color-scheme:dark){body{background:#111827;color:#e5e7eb}a{color:#93c5fd}}h1{font-size:1.25rem}`

var appDoneStyleHash = func() string {
	sum := sha256.Sum256([]byte(appDoneStyle))
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
}()

var appDonePage = template.Must(template.New("done").Parse(`<!doctype html>
<html lang="{{.Lang}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex"><title>{{.Title}}</title><style>` + appDoneStyle + `</style></head>
<body><h1>{{.Title}}</h1><p>{{.Body}}</p><p><a href="/">{{.Home}}</a></p></body></html>
`))

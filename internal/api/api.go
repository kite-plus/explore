// Package api serves Explore's HTTP interface with Gin: the public reading
// API, submissions, maintainer endpoints, and the feed and OPML outputs.
// docs/design/api.md is the contract.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/check"
	"github.com/kite-plus/explore/internal/fetch"
	"github.com/kite-plus/explore/internal/store"
)

// Server holds what the handlers need.
type Server struct {
	Store      *store.Store
	Checker    *check.Checker
	ImageFetch *fetch.Client
	LinkFetch  *fetch.Client
	PublicURL  string
	LookupTXT  func(context.Context, string) ([]string, error)
	// TrustedProxies may set X-Forwarded-For; see
	// docs/design/project-layout.md section 6.
	TrustedProxies []string
	// AllowPrivate accepts submissions of hosts that are IP addresses or
	// single labels, for local development.
	AllowPrivate bool
	Log          *slog.Logger
	Now          func() time.Time

	readLimit     *limiter
	submitLimit   *limiter
	previewLimit  *limiter
	linkLimit     *limiter
	linkSlots     chan struct{}
	imageMu       sync.Mutex
	registerLimit *limiter
	loginLimit    *limiter
	images        map[string]cachedImage
	faviconMu     sync.Mutex
	favicons      map[string]cachedImage
}

// Handler builds the router.
func (s *Server) Handler() (http.Handler, error) {
	if s.Log == nil {
		s.Log = slog.Default()
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	s.readLimit = newLimiter(300, time.Minute, s.Now)
	if s.AllowPrivate {
		s.submitLimit = newLimiter(10000, time.Hour, s.Now)
		s.previewLimit = newLimiter(10000, time.Hour, s.Now)
	} else {
		s.submitLimit = newLimiter(5, time.Hour, s.Now)
		s.previewLimit = newLimiter(60, time.Hour, s.Now)
	}
	s.linkLimit = newLimiter(10, time.Hour, s.Now)
	s.registerLimit = newLimiter(5, time.Hour, s.Now)
	s.loginLimit = newLimiter(10, time.Minute, s.Now)
	s.linkSlots = make(chan struct{}, 4)

	// Release mode keeps Gin's startup chatter out of the logs, and gin.New
	// rather than gin.Default leaves out the logger that writes addresses.
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	if err := r.SetTrustedProxies(s.TrustedProxies); err != nil {
		return nil, err
	}
	r.Use(s.logRequests(), s.recover())
	r.NoRoute(func(c *gin.Context) { s.fail(c, http.StatusNotFound, codeNotFound) })

	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/readyz", s.ready)
	r.GET("/feed.xml", s.limit(s.readLimit), s.feedXML)
	r.GET("/blogs.opml", s.limit(s.readLimit), s.blogsOPML)

	v1 := r.Group("/api/v1")
	v1.GET("/entries", s.limit(s.readLimit), s.entries)
	v1.GET("/entries/:id/image", s.limit(s.readLimit), s.entryImage)
	v1.GET("/entries/:id/check", s.limit(s.readLimit), s.entryLinkState)
	v1.POST("/entries/:id/check", s.limit(s.linkLimit), s.checkEntryLink)
	v1.GET("/tags", s.limit(s.readLimit), s.tags)
	v1.GET("/blogs", s.limit(s.readLimit), s.blogs)
	v1.GET("/blogs/:host/favicon", s.limit(s.readLimit), s.blogFavicon)
	v1.GET("/blogs/:host", s.limit(s.readLimit), s.blog)
	v1.GET("/site-config", s.limit(s.readLimit), s.siteConfig)
	v1.POST("/submissions", s.limit(s.submitLimit), s.submit)
	v1.POST("/submissions/preview", s.limit(s.previewLimit), s.previewSubmission)
	v1.GET("/submissions/:id", s.limit(s.readLimit), s.submission)
	v1.POST("/auth/register", s.limit(s.registerLimit), s.register)
	v1.POST("/auth/login", s.limit(s.loginLimit), s.login)
	v1.GET("/setup", s.limit(s.readLimit), s.setupState)
	v1.POST("/setup/verify", s.limit(s.loginLimit), s.verifySetupCode)
	v1.POST("/setup", s.limit(s.loginLimit), s.completeSetup)
	account := v1.Group("/", func(c *gin.Context) { c.Header("Cache-Control", "private, no-store"); c.Next() }, s.requireUser())
	account.GET("/me", s.me)
	account.PATCH("/me", s.updateMe)
	account.DELETE("/me", s.deleteMe)
	account.POST("/auth/logout", s.logout)
	account.GET("/me/subscriptions", s.subscriptions)
	account.PUT("/me/subscriptions/:host", s.addSubscription)
	account.DELETE("/me/subscriptions/:host", s.removeSubscription)
	account.GET("/me/entries", s.following)
	account.GET("/me/blogs", s.ownedBlogs)
	account.POST("/me/blog-claims/:host", s.startBlogClaim)
	account.POST("/me/blog-claims/:host/verify", s.verifyBlogClaim)
	account.POST("/reports", s.createReport)

	admin := v1.Group("/admin", s.requireAdmin())
	admin.GET("/session", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	admin.GET("/overview", s.adminOverview)
	admin.GET("/users", s.adminUsers)
	admin.PATCH("/users/:id", s.adminUpdateUser)
	admin.GET("/entries", s.adminEntries)
	admin.PATCH("/entries/:id", s.adminUpdateEntry)
	admin.GET("/takedowns", s.adminTakedowns)
	admin.POST("/takedowns", s.adminCreateTakedown)
	admin.POST("/takedowns/:id/review", s.adminReviewTakedown)
	admin.GET("/settings", s.adminSettings)
	admin.PATCH("/settings/:key", s.adminUpdateSetting)
	admin.GET("/submissions", s.adminSubmissions)
	admin.POST("/submissions/:id/approve", s.adminApprove)
	admin.POST("/submissions/:id/reject", s.adminReject)
	admin.GET("/blogs", s.adminBlogs)
	admin.GET("/fetch-queue", s.adminFetchQueue)
	admin.POST("/blogs", s.adminCreateBlog)
	admin.GET("/blogs/:host/fetch-attempts", s.adminFetchAttempts)
	admin.PATCH("/blogs/:host", s.adminUpdateBlog)
	admin.DELETE("/blogs/:host", s.adminDeleteBlog)
	admin.POST("/blogs/:host/fetch", s.adminFetchNow)
	admin.GET("/excluded-hosts", s.adminExcluded)
	admin.DELETE("/excluded-hosts/:host", s.adminDeleteExcluded)
	admin.POST("/check", s.adminCheck)
	return r, nil
}

func (s *Server) ready(c *gin.Context) {
	if err := s.Store.Ping(c.Request.Context()); err != nil {
		s.Log.Error("database unreachable", "error", err)
		c.String(http.StatusServiceUnavailable, "database unreachable")
		return
	}
	c.String(http.StatusOK, "ok")
}

// Package api serves Explore's HTTP interface with Gin: the public reading
// API, submissions, maintainer endpoints, and the feed and OPML outputs.
// docs/design/api.md is the contract.
package api

import (
	"crypto/sha256"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/check"
	"github.com/kite-plus/explore/internal/fetch"
	"github.com/kite-plus/explore/internal/store"
)

// AdminToken is a maintainer credential: a name and the SHA-256 of the
// token.
type AdminToken struct {
	Name string
	Hash [sha256.Size]byte
}

// Server holds what the handlers need.
type Server struct {
	Store      *store.Store
	Checker    *check.Checker
	ImageFetch *fetch.Client
	LinkFetch  *fetch.Client
	PublicURL  string
	Admins     []AdminToken
	// TrustedProxies may set X-Forwarded-For; see
	// docs/design/project-layout.md section 6.
	TrustedProxies []string
	// AllowPrivate accepts submissions of hosts that are IP addresses or
	// single labels, for local development.
	AllowPrivate bool
	Log          *slog.Logger
	Now          func() time.Time

	readLimit    *limiter
	submitLimit  *limiter
	previewLimit *limiter
	linkLimit    *limiter
	linkSlots    chan struct{}
	imageMu      sync.Mutex
	images       map[string]cachedImage
	faviconMu    sync.Mutex
	favicons     map[string]cachedImage
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
	v1.POST("/submissions", s.limit(s.submitLimit), s.submit)
	v1.POST("/submissions/preview", s.limit(s.previewLimit), s.previewSubmission)
	v1.GET("/submissions/:id", s.limit(s.readLimit), s.submission)

	// Without tokens the admin routes do not exist at all.
	if len(s.Admins) > 0 {
		admin := v1.Group("/admin", s.requireAdmin())
		admin.GET("/submissions", s.adminSubmissions)
		admin.POST("/submissions/:id/approve", s.adminApprove)
		admin.POST("/submissions/:id/reject", s.adminReject)
		admin.GET("/blogs", s.adminBlogs)
		admin.POST("/blogs", s.adminCreateBlog)
		admin.PATCH("/blogs/:host", s.adminUpdateBlog)
		admin.DELETE("/blogs/:host", s.adminDeleteBlog)
		admin.POST("/blogs/:host/fetch", s.adminFetchNow)
		admin.GET("/excluded-hosts", s.adminExcluded)
		admin.DELETE("/excluded-hosts/:host", s.adminDeleteExcluded)
		admin.POST("/check", s.adminCheck)
	}
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

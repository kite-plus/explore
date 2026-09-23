package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// logRequests records the route template, never the client address, the
// user agent or the query string.
func (s *Server) logRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		s.Log.Info("request", "method", c.Request.Method, "route", route,
			"status", c.Writer.Status(), "duration", time.Since(start))
	}
}

func (s *Server) recover() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		s.Log.Error("panic", "error", fmt.Sprint(recovered), "stack", string(debug.Stack()))
		s.fail(c, http.StatusInternalServerError, codeInternal)
	})
}

// limit applies a per-client limit. The address is used only as a key in
// memory.
func (s *Server) limit(l *limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ok, retry := l.allow(c.ClientIP())
		if !ok {
			c.Header("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
			s.fail(c, http.StatusTooManyRequests, codeRateLimited)
			return
		}
		c.Next()
	}
}

// requireAdmin accepts a bearer token whose SHA-256 matches a configured
// maintainer. Every configured hash is compared, in constant time, so the
// timing does not reveal which one came close.
func (s *Server) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || token == "" {
			s.fail(c, http.StatusUnauthorized, codeUnauthorized)
			return
		}
		sum := sha256.Sum256([]byte(token))
		name := ""
		for _, a := range s.Admins {
			if subtle.ConstantTimeCompare(sum[:], a.Hash[:]) == 1 {
				name = a.Name
			}
		}
		if name == "" {
			s.fail(c, http.StatusUnauthorized, codeUnauthorized)
			return
		}
		c.Set("admin", name)
		c.Next()
	}
}

func reviewer(c *gin.Context) string { return c.GetString("admin") }

// limiter counts requests per key in fixed windows, in memory only.
type limiter struct {
	max    int
	window time.Duration
	now    func() time.Time

	mu     sync.Mutex
	counts map[string]window
	calls  int
}

type window struct {
	start time.Time
	n     int
}

func newLimiter(max int, per time.Duration, now func() time.Time) *limiter {
	return &limiter{max: max, window: per, now: now, counts: make(map[string]window)}
}

func (l *limiter) allow(key string) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.calls++
	if l.calls%1024 == 0 {
		for k, w := range l.counts {
			if now.Sub(w.start) >= l.window {
				delete(l.counts, k)
			}
		}
	}

	w := l.counts[key]
	if now.Sub(w.start) >= l.window {
		w = window{start: now}
	}
	if w.n >= l.max {
		return false, w.start.Add(l.window).Sub(now)
	}
	w.n++
	l.counts[key] = w
	return true, 0
}

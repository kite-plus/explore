package api

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
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

// requireAdmin accepts the session of an account with admin access; writes
// also need the session's CSRF token.
func (s *Server) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, session, err := s.sessionUser(c)
		if err != nil || !u.IsAdmin {
			s.fail(c, http.StatusUnauthorized, codeUnauthorized)
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead &&
			subtle.ConstantTimeCompare([]byte(c.GetHeader("X-CSRF-Token")), []byte(csrfToken(session))) != 1 {
			s.fail(c, http.StatusForbidden, codeInvalidRequest)
			return
		}
		c.Set("admin", u.Email)
		c.Set("admin_user_id", u.ID)
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

package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/policy"
)

type linkStateJSON struct {
	LinkStatus    model.LinkStatus `json:"link_status"`
	LinkCheckedAt *time.Time       `json:"link_checked_at"`
	Checking      bool             `json:"checking"`
}

func entryID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	return id, err == nil && id > 0
}

func (s *Server) writeLinkState(c *gin.Context, id int64) {
	state, err := s.Store.VisibleLinkState(c.Request.Context(), id)
	if err != nil {
		s.storeError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	status := http.StatusOK
	if state.Checking {
		status = http.StatusAccepted
	}
	writeJSON(c, status, linkStateJSON{LinkStatus: state.Status, LinkCheckedAt: utc(state.CheckedAt), Checking: state.Checking})
}

func (s *Server) entryLinkState(c *gin.Context) {
	id, ok := entryID(c)
	if !ok {
		s.fail(c, http.StatusNotFound, codeNotFound)
		return
	}
	s.writeLinkState(c, id)
}

func (s *Server) checkEntryLink(c *gin.Context) {
	id, ok := entryID(c)
	if !ok {
		s.fail(c, http.StatusNotFound, codeNotFound)
		return
	}
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if s.LinkFetch == nil {
		s.fail(c, http.StatusServiceUnavailable, codeInternal)
		return
	}
	select {
	case s.linkSlots <- struct{}{}:
		defer func() { <-s.linkSlots }()
	default:
		c.Header("Retry-After", "10")
		s.fail(c, http.StatusTooManyRequests, codeRateLimited)
		return
	}

	job, claimed, err := s.Store.ClaimLinkOnDemand(c.Request.Context(), id)
	if err != nil {
		s.storeError(c, err)
		return
	}
	if !claimed {
		s.writeLinkState(c, id)
		return
	}

	code, probeErr := s.LinkFetch.Probe(c.Request.Context(), job.URL)
	status := model.LinkUnknown
	switch {
	case probeErr == nil && code >= 200 && code < 300:
		status = model.LinkAvailable
	case probeErr == nil && (code == http.StatusNotFound || code == http.StatusGone):
		status = model.LinkUnavailable
	}
	interval := policy.LinkCheckInterval
	if status == model.LinkUnknown {
		interval = policy.LinkRetryInterval
	}
	if err := s.Store.RecordLinkStatus(c.Request.Context(), job, status, interval); err != nil {
		s.storeError(c, err)
		return
	}
	if probeErr != nil && c.Request.Context().Err() == nil {
		s.Log.Warn("reader article link check was inconclusive", "entry", id, "error", probeErr)
	}
	s.writeLinkState(c, id)
}

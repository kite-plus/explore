package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/idna"

	"github.com/kite-plus/explore/internal/check"
	"github.com/kite-plus/explore/internal/model"
)

// checkTimeout bounds a submission's check; see docs/design/api.md 2.4.
const checkTimeout = 30 * time.Second

// maxBody bounds a request body; submissions are a few hundred bytes.
const maxBody = 16 << 10

type submitRequest struct {
	SiteURL string `json:"site_url"`
	FeedURL string `json:"feed_url"`
	Note    string `json:"note"`
}

type submissionJSON struct {
	ID          string             `json:"id"`
	Status      string             `json:"status"`
	Host        string             `json:"host"`
	SiteURL     string             `json:"site_url"`
	FeedURL     string             `json:"feed_url"`
	CheckReport *model.CheckReport `json:"check_report"`
	ReviewNote  string             `json:"review_note,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	ReviewedAt  *time.Time         `json:"reviewed_at"`
}

func toSubmission(sub model.Submission, c *gin.Context) submissionJSON {
	report := sub.Report
	check.Localize(&report, lang(c))
	return submissionJSON{
		ID: sub.ID, Status: string(sub.Status), Host: sub.Host, SiteURL: sub.SiteURL, FeedURL: sub.FeedURL,
		CheckReport: &report, ReviewNote: sub.ReviewNote, CreatedAt: sub.CreatedAt.UTC(), ReviewedAt: utc(sub.ReviewedAt),
	}
}

// target is a checked address and the host it will be listed under.
type target struct {
	site string
	feed string
	host string
}

// parseTarget validates what an author typed. Blogs live on public domain
// names, so IP addresses and single-label hosts are refused unless the
// server runs for local development.
func (s *Server) parseTarget(siteURL, feedURL string) (target, bool) {
	site, err := check.ParseURL(siteURL)
	if err != nil {
		return target{}, false
	}
	host, err := idna.Lookup.ToASCII(strings.TrimSuffix(site.Hostname(), "."))
	if err != nil || host == "" {
		return target{}, false
	}
	if !s.AllowPrivate && (net.ParseIP(host) != nil || !strings.Contains(host, ".")) {
		return target{}, false
	}
	t := target{site: site.String(), host: strings.ToLower(host)}
	if strings.TrimSpace(feedURL) != "" {
		feed, err := check.ParseURL(feedURL)
		if err != nil {
			return target{}, false
		}
		t.feed = feed.String()
	}
	return t, true
}

// runCheck checks a target and localizes the report for the caller.
func (s *Server) runCheck(c *gin.Context, t target) (*model.CheckReport, bool) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), checkTimeout)
	defer cancel()
	report, err := s.Checker.Run(ctx, check.Input{SiteURL: t.site, FeedURL: t.feed})
	if errors.Is(err, check.ErrInvalidURL) {
		s.fail(c, http.StatusBadRequest, codeInvalidURL)
		return nil, false
	}
	if err != nil {
		s.storeError(c, err)
		return nil, false
	}
	return report, true
}

func (s *Server) submit(c *gin.Context) {
	var req submitRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBody)
	if err := c.ShouldBindJSON(&req); err != nil || utf8.RuneCountInString(req.Note) > 500 {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	t, ok := s.parseTarget(req.SiteURL, req.FeedURL)
	if !ok {
		s.fail(c, http.StatusBadRequest, codeInvalidURL)
		return
	}

	state, err := s.Store.HostState(c.Request.Context(), t.host)
	if err != nil {
		s.storeError(c, err)
		return
	}
	switch {
	case state.Excluded != nil:
		s.fail(c, http.StatusForbidden, codeExcluded)
		return
	case state.Listed:
		s.fail(c, http.StatusConflict, codeAlreadyListed)
		return
	case state.PendingID != "":
		s.fail(c, http.StatusConflict, codeAlreadyPending, gin.H{"submission_id": state.PendingID})
		return
	}

	report, ok := s.runCheck(c, t)
	if !ok {
		return
	}
	if !report.Passed {
		// A failed check is only reported; nothing is written.
		check.Localize(report, lang(c))
		s.fail(c, http.StatusUnprocessableEntity, codeCheckFailed, gin.H{"check_report": report})
		return
	}

	sub, err := s.Store.CreateSubmission(c.Request.Context(), model.Submission{
		Host: t.host, SiteURL: t.site, FeedURL: report.FeedURL, Note: strings.TrimSpace(req.Note), Report: *report,
	})
	if err != nil {
		s.storeError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Vary", "Accept-Language")
	writeJSON(c, http.StatusCreated, toSubmission(sub, c))
}

func (s *Server) submission(c *gin.Context) {
	sub, err := s.Store.Submission(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.storeError(c, err)
		return
	}
	// Reviewer names stay internal: submissionJSON has no field for them.
	c.Header("Cache-Control", "no-store")
	c.Header("Vary", "Accept-Language")
	writeJSON(c, http.StatusOK, toSubmission(sub, c))
}

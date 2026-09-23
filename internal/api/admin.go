package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/check"
	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/policy"
	"github.com/kite-plus/explore/internal/store"
)

type adminBlogJSON struct {
	Host                string     `json:"host"`
	Name                string     `json:"name"`
	Description         string     `json:"description"`
	SiteURL             string     `json:"site_url"`
	FeedURL             string     `json:"feed_url"`
	Language            string     `json:"language"`
	Generator           string     `json:"generator"`
	Status              string     `json:"status"`
	StatusNote          string     `json:"status_note"`
	ShowExcerpt         bool       `json:"show_excerpt"`
	ExtraDomains        []string   `json:"extra_domains"`
	DefaultTags         []string   `json:"default_tags"`
	Visible             bool       `json:"visible"`
	FetchIntervalSec    int64      `json:"fetch_interval_seconds"`
	NextFetchAt         time.Time  `json:"next_fetch_at"`
	LastFetchedAt       *time.Time `json:"last_fetched_at"`
	LastSucceededAt     *time.Time `json:"last_succeeded_at"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	LastError           string     `json:"last_error"`
	GoneSince           *time.Time `json:"gone_since"`
	CreatedAt           time.Time  `json:"created_at"`
}

func (s *Server) toAdminBlog(b model.Blog) adminBlogJSON {
	visible := b.Status == model.BlogActive && b.GoneSince == nil &&
		b.LastSucceededAt != nil && s.Now().Sub(*b.LastSucceededAt) < policy.UnhealthyAfter
	return adminBlogJSON{
		Host: b.Host, Name: b.Name, Description: b.Description, SiteURL: b.SiteURL, FeedURL: b.FeedURL, Language: b.Language,
		Generator: string(b.Generator), Status: string(b.Status), StatusNote: b.StatusNote,
		ShowExcerpt: b.ShowExcerpt, ExtraDomains: b.ExtraDomains, DefaultTags: b.DefaultTags, Visible: visible,
		FetchIntervalSec: int64(b.FetchInterval / time.Second), NextFetchAt: b.NextFetchAt.UTC(),
		LastFetchedAt: utc(b.LastFetchedAt), LastSucceededAt: utc(b.LastSucceededAt),
		ConsecutiveFailures: b.ConsecutiveFailures, LastError: b.LastError, GoneSince: utc(b.GoneSince),
		CreatedAt: b.CreatedAt.UTC(),
	}
}

type adminSubmissionJSON struct {
	submissionJSON
	Note       string `json:"note"`
	ReviewedBy string `json:"reviewed_by,omitempty"`
}

func (s *Server) adminSubmissions(c *gin.Context) {
	status := model.SubmissionStatus(c.DefaultQuery("status", string(model.SubmissionPending)))
	switch status {
	case model.SubmissionPending, model.SubmissionApproved, model.SubmissionRejected:
	default:
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	subs, err := s.Store.Submissions(c.Request.Context(), status, 200)
	if err != nil {
		s.storeError(c, err)
		return
	}
	out := make([]adminSubmissionJSON, 0, len(subs))
	for _, sub := range subs {
		out = append(out, adminSubmissionJSON{submissionJSON: toSubmission(sub, c), Note: sub.Note, ReviewedBy: sub.ReviewedBy})
	}
	writeJSON(c, http.StatusOK, gin.H{"data": out})
}

type approveRequest struct {
	Name         string   `json:"name"`
	Language     string   `json:"language"`
	FeedURL      string   `json:"feed_url"`
	ExtraDomains []string `json:"extra_domains"`
	ShowExcerpt  *bool    `json:"show_excerpt"`
}

func (s *Server) adminApprove(c *gin.Context) {
	var req approveRequest
	if !s.bindOptional(c, &req) {
		return
	}
	blog, err := s.Store.ApproveSubmission(c.Request.Context(), c.Param("id"), store.Approval{
		Reviewer: reviewer(c), Name: strings.TrimSpace(req.Name), Language: strings.TrimSpace(req.Language),
		FeedURL: strings.TrimSpace(req.FeedURL), ExtraDomains: lowerAll(req.ExtraDomains), ShowExcerpt: req.ShowExcerpt,
	})
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, s.toAdminBlog(blog))
}

func (s *Server) adminReject(c *gin.Context) {
	var req struct {
		ReviewNote string `json:"review_note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.ReviewNote) == "" {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if err := s.Store.RejectSubmission(c.Request.Context(), c.Param("id"), reviewer(c), strings.TrimSpace(req.ReviewNote)); err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) adminBlogs(c *gin.Context) {
	blogs, err := s.Store.Blogs(c.Request.Context(), c.Query("health") == "unhealthy", 1000)
	if err != nil {
		s.storeError(c, err)
		return
	}
	out := make([]adminBlogJSON, 0, len(blogs))
	for _, b := range blogs {
		out = append(out, s.toAdminBlog(b))
	}
	writeJSON(c, http.StatusOK, gin.H{"data": out})
}

type fetchAttemptJSON struct {
	ID         int64      `json:"id"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Outcome    string     `json:"outcome"`
	HTTPStatus *int       `json:"http_status"`
	EntryCount *int       `json:"entry_count"`
	Error      string     `json:"error"`
}

func toFetchAttempt(attempt store.FetchAttempt) fetchAttemptJSON {
	return fetchAttemptJSON{
		ID: attempt.ID, StartedAt: attempt.StartedAt.UTC(), FinishedAt: utc(attempt.FinishedAt),
		Outcome: attempt.Outcome, HTTPStatus: attempt.HTTPStatus, EntryCount: attempt.EntryCount, Error: attempt.Error,
	}
}

type fetchQueueItemJSON struct {
	Host                string            `json:"host"`
	Name                string            `json:"name"`
	QueueStatus         string            `json:"queue_status"`
	NextFetchAt         time.Time         `json:"next_fetch_at"`
	LastFetchedAt       *time.Time        `json:"last_fetched_at"`
	LastSucceededAt     *time.Time        `json:"last_succeeded_at"`
	ConsecutiveFailures int               `json:"consecutive_failures"`
	LastError           string            `json:"last_error"`
	LastAttempt         *fetchAttemptJSON `json:"last_attempt"`
}

func (s *Server) adminFetchQueue(c *gin.Context) {
	items, err := s.Store.FetchQueue(c.Request.Context(), 1000)
	if err != nil {
		s.storeError(c, err)
		return
	}
	workerCount, lastSeen, err := s.Store.WorkerHeartbeats(c.Request.Context())
	if err != nil {
		s.storeError(c, err)
		return
	}
	now := s.Now()
	workerOnline := workerCount > 0
	out := make([]fetchQueueItemJSON, 0, len(items))
	for _, item := range items {
		status := "scheduled"
		switch {
		case item.Status == string(model.BlogPaused):
			status = "paused"
		case item.LastAttempt != nil && item.LastAttempt.Outcome == "running" &&
			(!workerOnline || now.Sub(item.LastAttempt.StartedAt) > policy.FetchLease):
			status = "stalled"
		case item.LastAttempt != nil && item.LastAttempt.Outcome == "running":
			status = "running"
		case !item.NextFetchAt.After(now):
			status = "queued"
		case item.ConsecutiveFailures > 0:
			status = "retry"
		}
		row := fetchQueueItemJSON{
			Host: item.Host, Name: item.Name, QueueStatus: status,
			NextFetchAt: item.NextFetchAt.UTC(), LastFetchedAt: utc(item.LastFetchedAt),
			LastSucceededAt: utc(item.LastSucceededAt), ConsecutiveFailures: item.ConsecutiveFailures,
			LastError: item.LastError,
		}
		if item.LastAttempt != nil {
			attempt := toFetchAttempt(*item.LastAttempt)
			row.LastAttempt = &attempt
		}
		out = append(out, row)
	}
	writeJSON(c, http.StatusOK, gin.H{
		"worker_online": workerOnline, "worker_count": workerCount,
		"worker_last_seen_at": utc(lastSeen), "data": out,
	})
}

func (s *Server) adminFetchAttempts(c *gin.Context) {
	attempts, err := s.Store.FetchAttempts(c.Request.Context(), strings.ToLower(c.Param("host")), 20)
	if err != nil {
		s.storeError(c, err)
		return
	}
	out := make([]fetchAttemptJSON, 0, len(attempts))
	for _, attempt := range attempts {
		out = append(out, toFetchAttempt(attempt))
	}
	writeJSON(c, http.StatusOK, gin.H{"data": out})
}

type createBlogRequest struct {
	SiteURL      string   `json:"site_url"`
	FeedURL      string   `json:"feed_url"`
	Name         string   `json:"name"`
	Language     string   `json:"language"`
	ExtraDomains []string `json:"extra_domains"`
	ShowExcerpt  *bool    `json:"show_excerpt"`
}

// adminCreateBlog lists a blog directly, after the same check a submission
// gets.
func (s *Server) adminCreateBlog(c *gin.Context) {
	var req createBlogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	t, ok := s.parseTarget(req.SiteURL, req.FeedURL)
	if !ok {
		s.fail(c, http.StatusBadRequest, codeInvalidURL)
		return
	}
	report, ok := s.runCheck(c, t)
	if !ok {
		return
	}
	if !report.Passed {
		check.Localize(report, lang(c))
		s.fail(c, http.StatusUnprocessableEntity, codeCheckFailed, gin.H{"check_report": report})
		return
	}
	blog, err := s.Store.CreateBlog(c.Request.Context(), store.NewBlog{
		Host: t.host, Reviewer: reviewer(c), SiteURL: t.site, FeedURL: report.FeedURL,
		Name:         firstNonEmpty(strings.TrimSpace(req.Name), report.Title, t.host),
		Language:     firstNonEmpty(strings.TrimSpace(req.Language), report.Language, "und"),
		Generator:    report.Generator,
		ShowExcerpt:  req.ShowExcerpt == nil || *req.ShowExcerpt,
		ExtraDomains: lowerAll(req.ExtraDomains),
	})
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, s.toAdminBlog(blog))
}

type updateBlogRequest struct {
	Name         *string   `json:"name"`
	Language     *string   `json:"language"`
	FeedURL      *string   `json:"feed_url"`
	ExtraDomains *[]string `json:"extra_domains"`
	ShowExcerpt  *bool     `json:"show_excerpt"`
	Status       *string   `json:"status"`
	StatusNote   *string   `json:"status_note"`
	DefaultTags  *[]string `json:"default_tags"`
}

func (s *Server) adminUpdateBlog(c *gin.Context) {
	var req updateBlogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	u := store.BlogUpdate{Name: req.Name, Language: req.Language, ShowExcerpt: req.ShowExcerpt, StatusNote: req.StatusNote}
	if req.FeedURL != nil {
		feed, err := check.ParseURL(*req.FeedURL)
		if err != nil {
			s.fail(c, http.StatusBadRequest, codeInvalidURL)
			return
		}
		f := feed.String()
		u.FeedURL = &f
	}
	if req.ExtraDomains != nil {
		d := lowerAll(*req.ExtraDomains)
		u.ExtraDomains = &d
	}
	if req.DefaultTags != nil {
		tags := *req.DefaultTags
		if len(tags) > model.MaxTagsPerEntry {
			s.fail(c, http.StatusBadRequest, codeInvalidRequest)
			return
		}
		for _, t := range tags {
			if _, ok := model.TagBySlug(t); !ok {
				s.fail(c, http.StatusBadRequest, codeInvalidRequest)
				return
			}
		}
		u.DefaultTags = &tags
	}
	if req.Status != nil {
		st := model.BlogStatus(*req.Status)
		if st != model.BlogActive && st != model.BlogPaused {
			s.fail(c, http.StatusBadRequest, codeInvalidRequest)
			return
		}
		u.Status = &st
	}
	blog, err := s.Store.UpdateBlog(c.Request.Context(), strings.ToLower(c.Param("host")), u)
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, s.toAdminBlog(blog))
}

// adminDeleteBlog removes a blog. With exclude=opt_out, as for an author
// who asked to leave, or exclude=blocked, the host cannot be listed again.
func (s *Server) adminDeleteBlog(c *gin.Context) {
	reason := model.ExclusionReason(c.Query("exclude"))
	switch reason {
	case "", model.ExcludedOptOut, model.ExcludedBlocked:
	default:
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if err := s.Store.DeleteBlog(c.Request.Context(), strings.ToLower(c.Param("host")), reason, c.Query("note")); err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) adminFetchNow(c *gin.Context) {
	if err := s.Store.FetchNow(c.Request.Context(), strings.ToLower(c.Param("host"))); err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

type excludedJSON struct {
	Host      string    `json:"host"`
	Reason    string    `json:"reason"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Server) adminExcluded(c *gin.Context) {
	hosts, err := s.Store.ExcludedHosts(c.Request.Context())
	if err != nil {
		s.storeError(c, err)
		return
	}
	out := make([]excludedJSON, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, excludedJSON{Host: h.Host, Reason: string(h.Reason), Note: h.Note, CreatedAt: h.CreatedAt.UTC()})
	}
	writeJSON(c, http.StatusOK, gin.H{"data": out})
}

func (s *Server) adminDeleteExcluded(c *gin.Context) {
	if err := s.Store.DeleteExcludedHost(c.Request.Context(), strings.ToLower(c.Param("host"))); err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// adminCheck runs a check without storing anything.
func (s *Server) adminCheck(c *gin.Context) {
	var req struct {
		URL     string `json:"url"`
		FeedURL string `json:"feed_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	t, ok := s.parseTarget(req.URL, req.FeedURL)
	if !ok {
		s.fail(c, http.StatusBadRequest, codeInvalidURL)
		return
	}
	report, ok := s.runCheck(c, t)
	if !ok {
		return
	}
	check.Localize(report, lang(c))
	writeJSON(c, http.StatusOK, report)
}

// bindOptional accepts an empty body as the zero value.
func (s *Server) bindOptional(c *gin.Context, v any) bool {
	if c.Request.ContentLength == 0 {
		return true
	}
	if err := c.ShouldBindJSON(v); err != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return false
	}
	return true
}

func lowerAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

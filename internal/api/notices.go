package api

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/store"
)

// noticeJSON is a notice as readers get it; docs/design/notices.md section 3.
type noticeJSON struct {
	ID         string  `json:"id"`
	Kind       string  `json:"kind"`
	Title      string  `json:"title"`
	Summary    string  `json:"summary"`
	Body       *string `json:"body,omitempty"`
	URL        string  `json:"url"`
	SourceName string  `json:"source_name"`
	Position   int     `json:"position"`
	// PublishedAt is when it started showing, or when it was created.
	PublishedAt time.Time `json:"published_at"`
}

func toNotice(n store.Notice, withBody bool) noticeJSON {
	out := noticeJSON{
		ID: strconv.FormatInt(n.ID, 10), Kind: n.Kind, Title: n.Title, Summary: n.Summary, URL: n.URL,
		SourceName: n.SourceName, Position: n.Position, PublishedAt: n.CreatedAt.UTC(),
	}
	if n.StartsAt != nil {
		out.PublishedAt = n.StartsAt.UTC()
	}
	if withBody {
		body := n.Body
		out.Body = &body
	}
	return out
}

func (s *Server) notices(c *gin.Context) {
	audience := c.Query("audience")
	if audience != "" && audience != "zh" && audience != "en" {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	notices, err := s.Store.ActiveNotices(c.Request.Context(), audience)
	if err != nil {
		s.storeError(c, err)
		return
	}
	out := struct {
		Data []noticeJSON `json:"data"`
	}{Data: make([]noticeJSON, 0, len(notices))}
	for _, n := range notices {
		out.Data = append(out.Data, toNotice(n, false))
	}
	cached(c, time.Minute, "application/json; charset=utf-8", encode(out))
}

func (s *Server) notice(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		s.fail(c, http.StatusNotFound, codeNotFound)
		return
	}
	n, err := s.Store.ActiveNotice(c.Request.Context(), id)
	if err != nil {
		s.storeError(c, err)
		return
	}
	cached(c, time.Minute, "application/json; charset=utf-8", encode(toNotice(n, true)))
}

func (s *Server) adminNotices(c *gin.Context) {
	notices, err := s.Store.Notices(c.Request.Context())
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"data": notices})
}

func (s *Server) adminCreateNotice(c *gin.Context) {
	in, ok := s.noticeInput(c)
	if !ok {
		return
	}
	n, err := s.Store.CreateNotice(c.Request.Context(), in, reviewer(c))
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, n)
}

func (s *Server) adminUpdateNotice(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		s.fail(c, http.StatusNotFound, codeNotFound)
		return
	}
	in, ok := s.noticeInput(c)
	if !ok {
		return
	}
	n, err := s.Store.UpdateNotice(c.Request.Context(), id, in, reviewer(c))
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, n)
}

func (s *Server) adminDeleteNotice(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		s.fail(c, http.StatusNotFound, codeNotFound)
		return
	}
	if err := s.Store.DeleteNotice(c.Request.Context(), id); err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// noticeInput reads and checks a notice from the request, by the rules in
// docs/design/notices.md section 1, and answers 400 when they fail.
func (s *Server) noticeInput(c *gin.Context) (store.NoticeInput, bool) {
	var body struct {
		Kind       string     `json:"kind"`
		Title      string     `json:"title"`
		Summary    string     `json:"summary"`
		Body       string     `json:"body"`
		URL        string     `json:"url"`
		SourceName string     `json:"source_name"`
		Position   int        `json:"position"`
		Audience   string     `json:"audience"`
		StartsAt   *time.Time `json:"starts_at"`
		EndsAt     *time.Time `json:"ends_at"`
		Enabled    bool       `json:"enabled"`
	}
	if c.ShouldBindJSON(&body) != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return store.NoticeInput{}, false
	}
	in := store.NoticeInput{
		Kind: body.Kind, Title: strings.TrimSpace(body.Title), Summary: strings.TrimSpace(body.Summary),
		Body: strings.TrimSpace(body.Body), URL: strings.TrimSpace(body.URL), SourceName: strings.TrimSpace(body.SourceName),
		Position: body.Position, Audience: body.Audience, StartsAt: body.StartsAt, EndsAt: body.EndsAt, Enabled: body.Enabled,
	}
	runes := utf8.RuneCountInString
	valid := (in.Kind == "notice" || in.Kind == "ad") &&
		runes(in.Title) >= 1 && runes(in.Title) <= 120 &&
		runes(in.Summary) <= 280 &&
		runes(in.Body) <= 20000 &&
		runes(in.SourceName) >= 1 && runes(in.SourceName) <= 60 &&
		in.Position >= 0 && in.Position <= 50 &&
		(in.Audience == "" || in.Audience == "zh" || in.Audience == "en") &&
		(in.URL != "" || in.Body != "") &&
		(in.URL == "" || webAddress(in.URL)) &&
		(in.StartsAt == nil || in.EndsAt == nil || in.EndsAt.After(*in.StartsAt))
	if !valid {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return store.NoticeInput{}, false
	}
	return in, true
}

// webAddress reports whether s is an absolute http(s) address with a host.
func webAddress(s string) bool {
	if len(s) > 2000 {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
}

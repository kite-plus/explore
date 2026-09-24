package api

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

func adminPage(c *gin.Context) (int, int, bool) {
	limit, offset := 30, 0
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			return 0, 0, false
		}
		limit = n
	}
	if raw := c.Query("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > 100000 {
			return 0, 0, false
		}
		offset = n
	}
	return limit, offset, true
}

func (s *Server) adminOverview(c *gin.Context) {
	stats, err := s.Store.AdminOverview(c.Request.Context())
	if err != nil {
		s.storeError(c, err)
		return
	}
	count, seen, err := s.Store.WorkerHeartbeats(c.Request.Context())
	if err != nil {
		s.storeError(c, err)
		return
	}
	paused, err := s.Store.Setting(c.Request.Context(), "crawler_paused")
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"stats": stats, "worker_online": count > 0, "worker_count": count, "worker_last_seen_at": utc(seen), "crawler_paused": paused == "true"})
}

func (s *Server) adminUsers(c *gin.Context) {
	limit, offset, ok := adminPage(c)
	if !ok {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	users, total, err := s.Store.AdminUsers(c.Request.Context(), c.Query("q"), limit, offset)
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"data": users, "total": total})
}

func (s *Server) adminUpdateUser(c *gin.Context) {
	var body struct {
		Disabled *bool `json:"disabled"`
		IsAdmin  *bool `json:"is_admin"`
	}
	if c.ShouldBindJSON(&body) != nil || (body.Disabled == nil) == (body.IsAdmin == nil) {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if c.GetString("admin_user_id") == c.Param("id") {
		s.fail(c, http.StatusForbidden, codeInvalidRequest)
		return
	}
	var err error
	if body.Disabled != nil {
		err = s.Store.SetUserDisabled(c.Request.Context(), c.Param("id"), *body.Disabled)
	} else {
		err = s.Store.SetUserAdminByID(c.Request.Context(), c.Param("id"), *body.IsAdmin)
	}
	if err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) adminEntries(c *gin.Context) {
	limit, offset, ok := adminPage(c)
	if !ok {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	entries, total, err := s.Store.AdminEntries(c.Request.Context(), c.Query("q"), limit, offset)
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"data": entries, "total": total})
}

func (s *Server) adminUpdateEntry(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		Hidden *bool  `json:"hidden"`
		Reason string `json:"reason"`
	}
	if err != nil || id < 1 || c.ShouldBindJSON(&body) != nil || body.Hidden == nil ||
		(*body.Hidden && (strings.TrimSpace(body.Reason) == "" || utf8.RuneCountInString(body.Reason) > 500)) {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if err := s.Store.SetEntryHidden(c.Request.Context(), id, *body.Hidden, strings.TrimSpace(body.Reason), reviewer(c)); err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type takedownBody struct {
	TargetType string `json:"target_type"`
	BlogHost   string `json:"blog_host"`
	EntryID    int64  `json:"entry_id"`
	Reason     string `json:"reason"`
}

func (s *Server) createTakedown(c *gin.Context, requesterID string) {
	var body takedownBody
	if c.ShouldBindJSON(&body) != nil || (body.TargetType != "blog" && body.TargetType != "entry") ||
		strings.TrimSpace(body.BlogHost) == "" || (body.TargetType == "entry" && body.EntryID < 1) ||
		utf8.RuneCountInString(strings.TrimSpace(body.Reason)) < 5 || utf8.RuneCountInString(body.Reason) > 1000 {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if err := s.Store.CreateTakedownRequest(c.Request.Context(), body.TargetType, strings.ToLower(strings.TrimSpace(body.BlogHost)),
		body.EntryID, requesterID, strings.TrimSpace(body.Reason)); err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusCreated)
}

func (s *Server) createReport(c *gin.Context)        { s.createTakedown(c, currentUser(c).ID) }
func (s *Server) adminCreateTakedown(c *gin.Context) { s.createTakedown(c, "") }

func (s *Server) adminTakedowns(c *gin.Context) {
	status := c.DefaultQuery("status", "pending")
	if status != "pending" && status != "approved" && status != "rejected" {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	rows, err := s.Store.TakedownRequests(c.Request.Context(), status, 200)
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"data": rows})
}

func (s *Server) adminReviewTakedown(c *gin.Context) {
	var body struct {
		Decision   string `json:"decision"`
		ReviewNote string `json:"review_note"`
	}
	if c.ShouldBindJSON(&body) != nil || (body.Decision != "approved" && body.Decision != "rejected") ||
		(body.Decision == "rejected" && strings.TrimSpace(body.ReviewNote) == "") || utf8.RuneCountInString(body.ReviewNote) > 500 {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if err := s.Store.ReviewTakedown(c.Request.Context(), c.Param("id"), body.Decision,
		strings.TrimSpace(body.ReviewNote), reviewer(c)); err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) adminSettings(c *gin.Context) {
	settings, err := s.Store.SystemSettings(c.Request.Context())
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"data": settings})
}

func (s *Server) adminUpdateSetting(c *gin.Context) {
	key := c.Param("key")
	var body struct {
		Value string `json:"value"`
	}
	if c.ShouldBindJSON(&body) != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	switch key {
	case "registration_enabled", "submissions_enabled", "crawler_paused":
		if body.Value != "true" && body.Value != "false" {
			s.fail(c, http.StatusBadRequest, codeInvalidRequest)
			return
		}
	case "site_notice":
		if utf8.RuneCountInString(body.Value) > 280 {
			s.fail(c, http.StatusBadRequest, codeInvalidRequest)
			return
		}
	default:
		s.fail(c, http.StatusNotFound, codeNotFound)
		return
	}
	if err := s.Store.SetSetting(c.Request.Context(), key, body.Value, reviewer(c)); err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) siteConfig(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	notice, err := s.Store.Setting(c.Request.Context(), "site_notice")
	if err != nil {
		s.storeError(c, err)
		return
	}
	registration, err := s.Store.Setting(c.Request.Context(), "registration_enabled")
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"notice": notice, "registration_enabled": registration == "true"})
}

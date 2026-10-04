package api

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/store"
)

func (s *Server) adminUsers(c *gin.Context) {
	limit, offset, ok := adminPage(c)
	q := store.AdminUserQuery{Search: c.Query("q"), Status: c.Query("status"), Role: c.Query("role"),
		Sort: c.DefaultQuery("sort", "created"), Asc: c.Query("order") == "asc", Limit: limit, Offset: offset}
	if !ok || (q.Status != "" && q.Status != "active" && q.Status != "disabled") ||
		(q.Role != "" && q.Role != "admin" && q.Role != "reader") || !store.ValidUserSort(q.Sort) ||
		(c.Query("order") != "" && c.Query("order") != "asc" && c.Query("order") != "desc") {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	users, total, counts, err := s.Store.AdminUsers(c.Request.Context(), q)
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"data": users, "total": total, "counts": counts})
}

func (s *Server) adminUser(c *gin.Context) {
	detail, err := s.Store.AdminUserDetail(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, detail)
}

// ownAccount refuses changes an admin could lock themself out with; their
// own name and password are changed from the profile page.
func (s *Server) ownAccount(c *gin.Context) bool {
	if c.GetString("admin_user_id") != c.Param("id") {
		return false
	}
	s.fail(c, http.StatusForbidden, codeOwnAccount)
	return true
}

// adminUpdateUser changes one thing per request: disabled (with a reason when
// disabling), is_admin or display_name.
func (s *Server) adminUpdateUser(c *gin.Context) {
	var body struct {
		Disabled    *bool   `json:"disabled"`
		Reason      string  `json:"reason"`
		IsAdmin     *bool   `json:"is_admin"`
		DisplayName *string `json:"display_name"`
	}
	if c.ShouldBindJSON(&body) != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	fields := 0
	for _, set := range []bool{body.Disabled != nil, body.IsAdmin != nil, body.DisplayName != nil} {
		if set {
			fields++
		}
	}
	reason := strings.TrimSpace(body.Reason)
	if fields != 1 || (body.Disabled != nil && *body.Disabled && (reason == "" || utf8.RuneCountInString(reason) > 500)) {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if body.DisplayName == nil && s.ownAccount(c) {
		return
	}
	var err error
	switch {
	case body.DisplayName != nil:
		name := strings.TrimSpace(*body.DisplayName)
		if n := utf8.RuneCountInString(name); n < 1 || n > 80 {
			s.fail(c, http.StatusBadRequest, codeInvalidRequest)
			return
		}
		err = s.Store.RenameUser(c.Request.Context(), c.Param("id"), name)
	case body.Disabled != nil:
		err = s.Store.SetUserDisabled(c.Request.Context(), c.Param("id"), *body.Disabled, reason, reviewer(c))
	default:
		err = s.Store.SetUserAdminByID(c.Request.Context(), c.Param("id"), *body.IsAdmin)
	}
	if err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) adminDeleteUser(c *gin.Context) {
	if s.ownAccount(c) {
		return
	}
	if err := s.Store.DeleteUser(c.Request.Context(), c.Param("id")); err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) adminRevokeSessions(c *gin.Context) {
	if s.ownAccount(c) {
		return
	}
	revoked, err := s.Store.RevokeSessions(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"revoked": revoked})
}

func (s *Server) adminReleaseBlog(c *gin.Context) {
	if err := s.Store.ReleaseBlog(c.Request.Context(), c.Param("id"), strings.ToLower(c.Param("host"))); err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

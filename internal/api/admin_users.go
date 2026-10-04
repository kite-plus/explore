package api

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

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

// adminUpdateUser changes either the profile (display_name, email or both)
// or exactly one of disabled (with a reason when disabling) and is_admin.
func (s *Server) adminUpdateUser(c *gin.Context) {
	var body struct {
		Disabled    *bool   `json:"disabled"`
		Reason      string  `json:"reason"`
		IsAdmin     *bool   `json:"is_admin"`
		DisplayName *string `json:"display_name"`
		Email       *string `json:"email"`
	}
	if c.ShouldBindJSON(&body) != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	profile := body.DisplayName != nil || body.Email != nil
	switches := 0
	for _, set := range []bool{body.Disabled != nil, body.IsAdmin != nil} {
		if set {
			switches++
		}
	}
	reason := strings.TrimSpace(body.Reason)
	if (profile && switches > 0) || (!profile && switches != 1) ||
		(body.Disabled != nil && *body.Disabled && (reason == "" || utf8.RuneCountInString(reason) > 500)) {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if !profile && s.ownAccount(c) {
		return
	}
	var err error
	switch {
	case profile:
		var name, email *string
		if body.DisplayName != nil {
			n := strings.TrimSpace(*body.DisplayName)
			if runes := utf8.RuneCountInString(n); runes < 1 || runes > 80 {
				s.fail(c, http.StatusBadRequest, codeInvalidRequest)
				return
			}
			name = &n
		}
		if body.Email != nil {
			e := validEmail(*body.Email)
			if e == "" {
				s.fail(c, http.StatusBadRequest, codeInvalidRequest)
				return
			}
			email = &e
		}
		err = s.Store.UpdateUserProfile(c.Request.Context(), c.Param("id"), name, email)
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

// adminResetPassword sets a password the admin passes on to the account's
// owner, for someone who cannot sign in; there is no self-service reset.
func (s *Server) adminResetPassword(c *gin.Context) {
	if s.ownAccount(c) {
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&body) != nil || len(body.Password) < minPasswordLength || len(body.Password) > maxPasswordLength {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		s.storeError(c, err)
		return
	}
	if err := s.Store.ResetPassword(c.Request.Context(), c.Param("id"), string(hash), reviewer(c)); err != nil {
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

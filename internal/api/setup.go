package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/kite-plus/explore/internal/store"
)

// setupState tells the admin whether to show the setup wizard: a fresh
// install has no admin account, and the wizard creates the first one.
func (s *Server) setupState(c *gin.Context) {
	pending, err := s.Store.SetupPending(c.Request.Context())
	if err != nil {
		s.storeError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	writeJSON(c, http.StatusOK, gin.H{"required": pending, "min_password_length": minPasswordLength})
}

// verifySetupCode lets the wizard check the code before it asks for the
// account; completeSetup checks it again.
func (s *Server) verifySetupCode(c *gin.Context) {
	var body struct {
		Code string `json:"code"`
	}
	if c.ShouldBindJSON(&body) != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if err := s.Store.CheckSetupCode(c.Request.Context(), body.Code); err != nil {
		s.setupError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// completeSetup creates the first admin account and signs it in.
func (s *Server) completeSetup(c *gin.Context) {
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		s.fail(c, http.StatusUnsupportedMediaType, codeInvalidRequest)
		return
	}
	var body struct {
		Code                string `json:"code"`
		Email               string `json:"email"`
		Password            string `json:"password"`
		DisplayName         string `json:"display_name"`
		RegistrationEnabled *bool  `json:"registration_enabled"`
		SubmissionsEnabled  *bool  `json:"submissions_enabled"`
	}
	if c.ShouldBindJSON(&body) != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	email, name, ok := validAccount(body.Email, body.DisplayName, body.Password)
	if !ok {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		s.storeError(c, err)
		return
	}
	settings := map[string]string{}
	if body.RegistrationEnabled != nil {
		settings["registration_enabled"] = strconv.FormatBool(*body.RegistrationEnabled)
	}
	if body.SubmissionsEnabled != nil {
		settings["submissions_enabled"] = strconv.FormatBool(*body.SubmissionsEnabled)
	}
	u, err := s.Store.CompleteSetup(c.Request.Context(), body.Code, email, string(hash), name, settings)
	if err != nil {
		s.setupError(c, err)
		return
	}
	s.Log.Info("setup complete")
	s.startSession(c, u)
}

func (s *Server) setupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrSetupDone):
		s.fail(c, http.StatusConflict, codeSetupDone)
	case errors.Is(err, store.ErrSetupCode):
		s.fail(c, http.StatusForbidden, codeSetupCode)
	case errors.Is(err, store.ErrConflict):
		s.fail(c, http.StatusConflict, codeConflict)
	default:
		s.storeError(c, err)
	}
}

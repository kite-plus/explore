package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/i18n"
	"github.com/kite-plus/explore/internal/store"
)

// Error codes; see docs/design/api.md section 5.
const (
	codeInvalidRequest = "invalid_request"
	codeInvalidURL     = "invalid_url"
	codeInvalidCursor  = "invalid_cursor"
	codeUnauthorized   = "unauthorized"
	codeExcluded       = "excluded"
	codeNotFound       = "not_found"
	codeAlreadyListed  = "already_listed"
	codeAlreadyPending = "already_pending"
	codeNotPending     = "not_pending"
	codeCheckFailed    = "check_failed"
	codeRateLimited    = "rate_limited"
	codeInternal       = "internal"
)

var messages = map[string]struct{ en, zh string }{
	codeInvalidRequest: {"The request is malformed.", "请求格式不正确。"},
	codeInvalidURL:     {"The address is not a public http or https URL.", "地址不是公网的 http 或 https 地址。"},
	codeInvalidCursor:  {"The cursor is not valid.", "游标无效。"},
	codeUnauthorized:   {"A valid maintainer token is required.", "需要有效的维护者令牌。"},
	codeExcluded:       {"This blog has left Explore or was blocked.", "这个博客已经退出 Explore，或者被屏蔽了。"},
	codeNotFound:       {"Not found.", "没有找到。"},
	codeAlreadyListed:  {"This blog is already listed.", "这个博客已经收录了。"},
	codeAlreadyPending: {"This blog already has a submission waiting for review.", "这个博客已经有一条等待审核的提交。"},
	codeNotPending:     {"This submission has already been reviewed.", "这条提交已经审核过了。"},
	codeCheckFailed:    {"The blog did not pass the check; see the report.", "博客没有通过检查，原因见检查报告。"},
	codeRateLimited:    {"Too many requests; try again later.", "请求太频繁，请稍后再试。"},
	codeInternal:       {"Something went wrong on our side.", "服务端出错了。"},
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func lang(c *gin.Context) i18n.Lang { return i18n.Match(c.GetHeader("Accept-Language")) }

func message(code string, l i18n.Lang) string {
	m := messages[code]
	if l == i18n.Chinese {
		return m.zh
	}
	return m.en
}

// fail writes an error in the reader's language. extra adds fields next to
// "error", such as the check report.
func (s *Server) fail(c *gin.Context, status int, code string, extra ...gin.H) {
	body := gin.H{"error": errorBody{Code: code, Message: message(code, lang(c))}}
	for _, e := range extra {
		for k, v := range e {
			body[k] = v
		}
	}
	c.Header("Vary", "Accept-Language")
	c.Header("Cache-Control", "no-store")
	writeJSON(c, status, body)
	c.Abort()
}

// storeError maps store errors to responses; anything unexpected is logged
// and reported as internal.
func (s *Server) storeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.fail(c, http.StatusNotFound, codeNotFound)
	case errors.Is(err, store.ErrListed):
		s.fail(c, http.StatusConflict, codeAlreadyListed)
	case errors.Is(err, store.ErrPending):
		s.fail(c, http.StatusConflict, codeAlreadyPending)
	case errors.Is(err, store.ErrNotPending):
		s.fail(c, http.StatusConflict, codeNotPending)
	case errors.Is(err, store.ErrExcluded):
		s.fail(c, http.StatusForbidden, codeExcluded)
	default:
		s.Log.Error("store", "route", c.FullPath(), "error", err)
		s.fail(c, http.StatusInternalServerError, codeInternal)
	}
}

func encode(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	// Every value passed here is built from plain types.
	_ = enc.Encode(v)
	return buf.Bytes()
}

func writeJSON(c *gin.Context, status int, v any) {
	c.Data(status, "application/json; charset=utf-8", encode(v))
}

// cached writes a public response with a weak ETag and answers a matching
// If-None-Match with 304, so the frontend and a CDN can reuse it.
func cached(c *gin.Context, maxAge time.Duration, contentType string, body []byte) {
	sum := sha256.Sum256(body)
	etag := `W/"` + hex.EncodeToString(sum[:12]) + `"`
	c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", int(maxAge.Seconds())))
	c.Header("ETag", etag)
	for _, candidate := range strings.Split(c.GetHeader("If-None-Match"), ",") {
		if strings.TrimSpace(candidate) == etag || strings.TrimSpace(candidate) == "*" {
			c.Status(http.StatusNotModified)
			return
		}
	}
	c.Data(http.StatusOK, contentType, body)
}

// Cursors are opaque to clients: base64url of "unix-nanos:id".
func encodeCursor(cur store.Cursor) string {
	raw := strconv.FormatInt(cur.At.UnixNano(), 10) + ":" + strconv.FormatInt(cur.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (*store.Cursor, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	at, id, ok := strings.Cut(string(raw), ":")
	if !ok {
		return nil, errors.New("malformed cursor")
	}
	nanos, err1 := strconv.ParseInt(at, 10, 64)
	n, err2 := strconv.ParseInt(id, 10, 64)
	if err1 != nil || err2 != nil {
		return nil, errors.New("malformed cursor")
	}
	return &store.Cursor{At: time.Unix(0, nanos).UTC(), ID: n}, nil
}

// pageParams reads cursor, limit and lang, shared by the list endpoints.
func (s *Server) pageParams(c *gin.Context) (cur *store.Cursor, limit int, language string, ok bool) {
	cur, err := decodeCursor(c.Query("cursor"))
	if err != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidCursor)
		return nil, 0, "", false
	}
	limit = 30
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			s.fail(c, http.StatusBadRequest, codeInvalidRequest)
			return nil, 0, "", false
		}
		limit = n
	}
	language = strings.ToLower(c.Query("lang"))
	if language != "" && !validLang(language) {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return nil, 0, "", false
	}
	return cur, limit, language, true
}

// validLang accepts BCP 47 shaped tags such as "zh" or "zh-cn", which also
// keeps LIKE wildcards out of the query.
func validLang(s string) bool {
	parts := strings.Split(s, "-")
	for i, p := range parts {
		if (i == 0 && (len(p) < 2 || len(p) > 3)) || (i > 0 && (len(p) < 2 || len(p) > 8)) {
			return false
		}
		for _, r := range p {
			letter := r >= 'a' && r <= 'z'
			digit := i > 0 && r >= '0' && r <= '9'
			if !letter && !digit {
				return false
			}
		}
	}
	return true
}

package api

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/store"
)

// maxSearchLength is the longest query searched, in characters.
const maxSearchLength = 100

// maxSearchTotal is as far as entry search counts; past it the total is a
// lower bound.
const maxSearchTotal = 1000

// searchJSON is a page of search results. Total is given on the first page
// only; TotalCapped says it is a lower bound.
type searchJSON[T any] struct {
	Data        []T     `json:"data"`
	NextCursor  *string `json:"next_cursor"`
	Total       *int    `json:"total"`
	TotalCapped bool    `json:"total_capped"`
}

// searchParams reads q, cursor, limit and lang for the search endpoints.
func (s *Server) searchParams(c *gin.Context) (store.SearchQuery, bool) {
	text := strings.TrimSpace(c.Query("q"))
	if text == "" || utf8.RuneCountInString(text) > maxSearchLength || !utf8.ValidString(text) {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return store.SearchQuery{}, false
	}
	cur, err := decodeSearchCursor(c.Query("cursor"))
	if err != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidCursor)
		return store.SearchQuery{}, false
	}
	limit, language, ok := s.limitAndLang(c)
	if !ok {
		return store.SearchQuery{}, false
	}
	return store.SearchQuery{Text: text, Lang: language, Limit: limit, Cursor: cur}, true
}

func (s *Server) searchEntries(c *gin.Context) {
	q, ok := s.searchParams(c)
	if !ok {
		return
	}
	// One extra row says whether there is a next page.
	limit := q.Limit
	q.Limit++
	rows, err := s.Store.SearchEntries(c.Request.Context(), q)
	if err != nil {
		s.storeError(c, err)
		return
	}
	out := searchJSON[entryJSON]{Data: make([]entryJSON, 0, len(rows))}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		next := encodeSearchCursor(store.SearchCursor{Top: last.Top, At: last.SortAt, ID: last.ID})
		out.NextCursor = &next
	}
	for _, r := range rows {
		e := toEntry(r.Entry)
		e.Blog = &blogRefJSON{Host: r.Blog.Host, Name: r.Blog.Name, SiteURL: r.Blog.SiteURL, Language: r.Blog.Language}
		out.Data = append(out.Data, e)
	}
	if q.Cursor == nil {
		total := len(rows)
		if out.NextCursor != nil {
			n, err := s.Store.CountSearchEntries(c.Request.Context(), q.Text, q.Lang, maxSearchTotal+1)
			if err != nil {
				s.storeError(c, err)
				return
			}
			total = min(n, maxSearchTotal)
			out.TotalCapped = n > maxSearchTotal
		}
		out.Total = &total
	}
	cached(c, time.Minute, "application/json; charset=utf-8", encode(out))
}

func (s *Server) searchBlogs(c *gin.Context) {
	q, ok := s.searchParams(c)
	if !ok {
		return
	}
	limit := q.Limit
	q.Limit++
	rows, total, err := s.Store.SearchBlogs(c.Request.Context(), q)
	if err != nil {
		s.storeError(c, err)
		return
	}
	out := searchJSON[blogJSON]{Data: make([]blogJSON, 0, len(rows))}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		next := encodeSearchCursor(store.SearchCursor{Top: last.Top, At: last.SortAt, ID: last.ID})
		out.NextCursor = &next
	}
	for _, b := range rows {
		out.Data = append(out.Data, toBlog(b.ListedBlog))
	}
	if q.Cursor == nil {
		out.Total = &total
	}
	cached(c, time.Minute, "application/json; charset=utf-8", encode(out))
}

// Search cursors are base64url of "top:unix-nanos:id", top being 1 or 0.
func encodeSearchCursor(cur store.SearchCursor) string {
	top := "0"
	if cur.Top {
		top = "1"
	}
	raw := top + ":" + strconv.FormatInt(cur.At.UnixNano(), 10) + ":" + strconv.FormatInt(cur.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeSearchCursor(s string) (*store.SearchCursor, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(string(raw), ":")
	if len(parts) != 3 || (parts[0] != "0" && parts[0] != "1") {
		return nil, errors.New("malformed cursor")
	}
	nanos, err1 := strconv.ParseInt(parts[1], 10, 64)
	id, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		return nil, errors.New("malformed cursor")
	}
	return &store.SearchCursor{Top: parts[0] == "1", At: time.Unix(0, nanos).UTC(), ID: id}, nil
}

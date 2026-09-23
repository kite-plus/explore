package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/publicfeed"
	"github.com/kite-plus/explore/internal/store"
)

type blogRefJSON struct {
	Host     string `json:"host"`
	Name     string `json:"name"`
	SiteURL  string `json:"site_url"`
	Language string `json:"language"`
}

type entryJSON struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	URL         string       `json:"url"`
	Excerpt     *string      `json:"excerpt"`
	ImageURL    *string      `json:"image_url"`
	PublishedAt *time.Time   `json:"published_at"`
	Tags        []string     `json:"tags"`
	Blog        *blogRefJSON `json:"blog,omitempty"`
}

type tagJSON struct {
	Slug string            `json:"slug"`
	Name map[string]string `json:"name"`
}

type blogJSON struct {
	Host            string     `json:"host"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	SiteURL         string     `json:"site_url"`
	FeedURL         string     `json:"feed_url"`
	Language        string     `json:"language"`
	Generator       string     `json:"generator"`
	LastPublishedAt *time.Time `json:"last_published_at"`
}

type listJSON[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor"`
}

func toEntry(e model.Entry) entryJSON {
	out := entryJSON{ID: strconv.FormatInt(e.ID, 10), Title: e.Title, URL: e.URL, PublishedAt: utc(e.PublishedAt), Tags: e.Tags}
	if out.Tags == nil {
		out.Tags = []string{}
	}
	if e.Excerpt != "" {
		excerpt := e.Excerpt
		out.Excerpt = &excerpt
	}
	if e.ImageURL != "" {
		imageURL := "/api/v1/entries/" + out.ID + "/image"
		out.ImageURL = &imageURL
	}
	return out
}

func toBlog(b store.ListedBlog) blogJSON {
	return blogJSON{
		Host: b.Host, Name: b.Name, Description: b.Description, SiteURL: b.SiteURL, FeedURL: b.FeedURL,
		Language: b.Language, Generator: string(b.Generator), LastPublishedAt: utc(b.LastPublishedAt),
	}
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func (s *Server) entries(c *gin.Context) {
	cur, limit, language, ok := s.pageParams(c)
	if !ok {
		return
	}
	tag := c.Query("tag")
	if _, known := model.TagBySlug(tag); tag != "" && !known {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	// One extra row says whether there is a next page.
	rows, err := s.Store.Stream(c.Request.Context(), store.StreamQuery{Lang: language, Tag: tag, Limit: limit + 1, Cursor: cur})
	if err != nil {
		s.storeError(c, err)
		return
	}
	out := listJSON[entryJSON]{Data: make([]entryJSON, 0, len(rows))}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		next := encodeCursor(store.Cursor{At: *last.PublishedAt, ID: last.ID})
		out.NextCursor = &next
	}
	for _, r := range rows {
		e := toEntry(r.Entry)
		e.Blog = &blogRefJSON{Host: r.Blog.Host, Name: r.Blog.Name, SiteURL: r.Blog.SiteURL, Language: r.Blog.Language}
		out.Data = append(out.Data, e)
	}
	cached(c, time.Minute, "application/json; charset=utf-8", encode(out))
}

// tags returns the tag list in display order, with names in both languages.
func (s *Server) tags(c *gin.Context) {
	out := struct {
		Data []tagJSON `json:"data"`
	}{Data: make([]tagJSON, 0, len(model.Tags))}
	for _, t := range model.Tags {
		out.Data = append(out.Data, tagJSON{Slug: t.Slug, Name: map[string]string{"zh": t.ZH, "en": t.EN}})
	}
	cached(c, time.Hour, "application/json; charset=utf-8", encode(out))
}

func (s *Server) blogs(c *gin.Context) {
	cur, limit, language, ok := s.pageParams(c)
	if !ok {
		return
	}
	rows, err := s.Store.Directory(c.Request.Context(), store.DirectoryQuery{Lang: language, Limit: limit + 1, Cursor: cur})
	if err != nil {
		s.storeError(c, err)
		return
	}
	out := listJSON[blogJSON]{Data: make([]blogJSON, 0, len(rows))}
	if len(rows) > limit {
		rows = rows[:limit]
		next := encodeCursor(directoryCursor(rows[limit-1]))
		out.NextCursor = &next
	}
	for _, b := range rows {
		out.Data = append(out.Data, toBlog(b))
	}
	cached(c, time.Minute, "application/json; charset=utf-8", encode(out))
}

// directoryCursor keys blogs without dated entries on the epoch, as the
// directory query sorts them.
func directoryCursor(b store.ListedBlog) store.Cursor {
	cur := store.Cursor{At: time.Unix(0, 0).UTC(), ID: b.ID}
	if b.LastPublishedAt != nil {
		cur.At = *b.LastPublishedAt
	}
	return cur
}

func (s *Server) blog(c *gin.Context) {
	b, entries, err := s.Store.VisibleBlog(c.Request.Context(), strings.ToLower(c.Param("host")))
	if err != nil {
		s.storeError(c, err)
		return
	}
	out := struct {
		Blog    blogJSON    `json:"blog"`
		Entries []entryJSON `json:"entries"`
	}{Blog: toBlog(b), Entries: make([]entryJSON, 0, len(entries))}
	for _, e := range entries {
		out.Entries = append(out.Entries, toEntry(e))
	}
	cached(c, time.Minute, "application/json; charset=utf-8", encode(out))
}

// feedXML is the home stream as RSS: the same rules, the newest 50 entries.
func (s *Server) feedXML(c *gin.Context) {
	rows, err := s.Store.Stream(c.Request.Context(), store.StreamQuery{Limit: 50})
	if err != nil {
		s.storeError(c, err)
		return
	}
	items := make([]publicfeed.Item, 0, len(rows))
	for _, r := range rows {
		items = append(items, publicfeed.Item{
			Title: r.Title, Link: r.URL, Excerpt: r.Excerpt, PublishedAt: *r.PublishedAt,
			BlogName: r.Blog.Name, BlogFeed: r.Blog.FeedURL,
		})
	}
	body, err := publicfeed.RSS(publicfeed.Channel{
		Title:       "Explore",
		Link:        s.PublicURL + "/",
		Self:        s.PublicURL + "/feed.xml",
		Description: "New posts from independent blogs · 独立博客的最新文章",
	}, items)
	if err != nil {
		s.storeError(c, err)
		return
	}
	cached(c, 5*time.Minute, "application/rss+xml; charset=utf-8", body)
}

func (s *Server) blogsOPML(c *gin.Context) {
	var outlines []publicfeed.Outline
	var cur *store.Cursor
	for {
		page, err := s.Store.Directory(c.Request.Context(), store.DirectoryQuery{Limit: 500, Cursor: cur})
		if err != nil {
			s.storeError(c, err)
			return
		}
		for _, b := range page {
			outlines = append(outlines, publicfeed.Outline{Name: b.Name, SiteURL: b.SiteURL, FeedURL: b.FeedURL})
		}
		if len(page) < 500 {
			break
		}
		next := directoryCursor(page[len(page)-1])
		cur = &next
	}
	body, err := publicfeed.OPML("Explore", outlines)
	if err != nil {
		s.storeError(c, err)
		return
	}
	cached(c, time.Hour, "text/x-opml; charset=utf-8", body)
}

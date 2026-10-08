package api

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/publicfeed"
	"github.com/kite-plus/explore/internal/store"
)

// feedTypes are the formats a following stream is published in, by the
// extension its address ends with.
var feedTypes = map[string]string{
	"json": "application/feed+json; charset=utf-8",
	"xml":  "application/rss+xml; charset=utf-8",
	"opml": "text/x-opml; charset=utf-8",
}

// followingFeedJSON is where a reader's following stream is published, in
// each format, or nulls while they keep it off.
type followingFeedJSON struct {
	JSONURL *string `json:"json_url"`
	RSSURL  *string `json:"rss_url"`
	OPMLURL *string `json:"opml_url"`
}

func (s *Server) followingFeedAddresses(token string) followingFeedJSON {
	if token == "" {
		return followingFeedJSON{}
	}
	at := func(ext string) *string {
		u := s.PublicURL + "/f/" + token + "." + ext
		return &u
	}
	return followingFeedJSON{JSONURL: at("json"), RSSURL: at("xml"), OPMLURL: at("opml")}
}

func (s *Server) followingFeedSettings(c *gin.Context) {
	token, err := s.Store.FeedToken(c.Request.Context(), currentUser(c).ID)
	if err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, s.followingFeedAddresses(token))
}

// publishFollowingFeed publishes the reader's following stream under a new
// token, so an address given out before stops answering.
func (s *Server) publishFollowingFeed(c *gin.Context) {
	token, err := randomToken()
	if err != nil {
		s.fail(c, http.StatusInternalServerError, codeInternal)
		return
	}
	if err := s.Store.SetFeedToken(c.Request.Context(), currentUser(c).ID, token); err != nil {
		s.storeError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, s.followingFeedAddresses(token))
}

func (s *Server) unpublishFollowingFeed(c *gin.Context) {
	if err := s.Store.SetFeedToken(c.Request.Context(), currentUser(c).ID, ""); err != nil {
		s.storeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// followingFeed serves a reader's following stream at the address they
// published it under, /f/{token}.json, .xml or .opml, for a blogroll on
// their own site. The address is all it takes, so it says nothing about the
// reader beyond the blogs they follow, and stays out of search engines.
func (s *Server) followingFeed(c *gin.Context) {
	token, ext, _ := strings.Cut(c.Param("file"), ".")
	contentType, known := feedTypes[ext]
	if token == "" || !known {
		s.fail(c, http.StatusNotFound, codeNotFound)
		return
	}
	ctx := c.Request.Context()
	userID, err := s.Store.FeedOwner(ctx, token)
	if err != nil {
		s.storeError(c, err)
		return
	}
	// Counted after the lookup, so only tokens in use take a place in memory.
	if ok, retry := s.feedLimit.allow(token); !ok {
		c.Header("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		s.fail(c, http.StatusTooManyRequests, codeRateLimited)
		return
	}

	var body []byte
	if ext == "opml" {
		blogs, err := s.Store.Subscriptions(ctx, userID)
		if err != nil {
			s.storeError(c, err)
			return
		}
		outlines := make([]publicfeed.Outline, 0, len(blogs))
		for _, b := range blogs {
			outlines = append(outlines, publicfeed.Outline{Name: b.Name, SiteURL: b.SiteURL, FeedURL: b.FeedURL})
		}
		body, err = publicfeed.OPML("Explore", outlines)
		if err != nil {
			s.storeError(c, err)
			return
		}
	} else {
		rows, err := s.Store.FollowingStream(ctx, userID, store.StreamQuery{Limit: 50})
		if err != nil {
			s.storeError(c, err)
			return
		}
		items := make([]publicfeed.Item, 0, len(rows))
		for _, r := range rows {
			item := publicfeed.Item{
				Title: r.Title, Link: r.URL, Excerpt: r.Excerpt,
				BlogName: r.Blog.Name, BlogFeed: r.Blog.FeedURL, BlogSite: r.Blog.SiteURL,
				BlogIcon: s.PublicURL + "/api/v1/blogs/" + url.PathEscape(r.Blog.Host) + "/favicon",
			}
			if r.PublishedAt != nil {
				item.PublishedAt = *r.PublishedAt
			}
			items = append(items, item)
		}
		ch := publicfeed.Channel{
			Title:       "Explore · Following",
			Link:        s.PublicURL + "/",
			Self:        s.PublicURL + "/f/" + token + "." + ext,
			Description: "New posts from the blogs a reader follows on Explore · 一位读者在 Explore 订阅的博客的最新文章",
		}
		if ext == "json" {
			body, err = publicfeed.JSON(ch, items)
		} else {
			body, err = publicfeed.RSS(ch, items)
		}
		if err != nil {
			s.storeError(c, err)
			return
		}
	}
	c.Header("X-Robots-Tag", "noindex")
	cached(c, 5*time.Minute, contentType, body)
}

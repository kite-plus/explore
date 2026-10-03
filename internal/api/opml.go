package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/check"
	"github.com/kite-plus/explore/internal/publicfeed"
	"github.com/kite-plus/explore/internal/store"
)

// An OPML file of a few thousand feeds fits in maxOPMLBytes; an import
// reads the first maxOPMLOutlines of them. See docs/design/api.md 1.
const (
	maxOPMLBytes    = 2 << 20
	maxOPMLOutlines = 1000
)

type outlineJSON struct {
	Title   string `json:"title"`
	SiteURL string `json:"site_url"`
	FeedURL string `json:"feed_url"`
}

func (s *Server) exportSubscriptions(c *gin.Context) {
	blogs, err := s.Store.Subscriptions(c.Request.Context(), currentUser(c).ID)
	if err != nil {
		s.storeError(c, err)
		return
	}
	outlines := make([]publicfeed.Outline, 0, len(blogs))
	for _, b := range blogs {
		outlines = append(outlines, publicfeed.Outline{Name: b.Name, SiteURL: b.SiteURL, FeedURL: b.FeedURL})
	}
	body, err := publicfeed.OPML("Explore", outlines)
	if err != nil {
		s.storeError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="explore-subscriptions.opml"`)
	c.Data(http.StatusOK, "text/x-opml; charset=utf-8", body)
}

// importSubscriptions follows the listed blogs an OPML file names and
// lists the rest, which the reader may submit.
func (s *Server) importSubscriptions(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxOPMLBytes)
	// A file over the size limit reads as cut off, so it is not OPML either.
	outlines, ignored, err := publicfeed.ReadOPML(c.Request.Body, maxOPMLOutlines)
	if err != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidOPML)
		return
	}
	items := make([]store.ImportItem, len(outlines))
	for i, o := range outlines {
		for _, raw := range []string{o.FeedURL, o.SiteURL} {
			if hosts, ok := hostVariants(raw); ok {
				items[i].Hosts = append(items[i].Hosts, hosts...)
			}
		}
		if u, err := check.ParseURL(o.FeedURL); err == nil {
			items[i].FeedURL = u.String()
		}
	}
	res, err := s.Store.ImportSubscriptions(c.Request.Context(), currentUser(c).ID, items)
	if err != nil {
		s.storeError(c, err)
		return
	}
	notListed := []outlineJSON{}
	for i, o := range outlines {
		if !res.Matched[i] {
			notListed = append(notListed, outlineJSON{Title: o.Name, SiteURL: o.SiteURL, FeedURL: o.FeedURL})
		}
	}
	writeJSON(c, http.StatusOK, gin.H{
		"outlines": len(outlines), "added": res.Added, "already_following": res.Already,
		"ignored": ignored, "not_listed": notListed,
	})
}

package api

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/png"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"

	"github.com/kite-plus/explore/internal/fetch"
)

const (
	maxFaviconBytes      = 256 << 10
	maxFaviconHTMLBytes  = 256 << 10
	faviconCacheEntries  = 128
	faviconCacheLifetime = 6 * time.Hour
	faviconBrowserMaxAge = time.Hour
)

var transparentFavicon = func() []byte {
	var body bytes.Buffer
	_ = png.Encode(&body, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	return body.Bytes()
}()

func (s *Server) blogFavicon(c *gin.Context) {
	siteURL, err := s.Store.VisibleBlogSiteURL(c.Request.Context(), strings.ToLower(c.Param("host")))
	if err != nil {
		s.storeError(c, err)
		return
	}
	if icon, ok := s.getFavicon(siteURL); ok {
		serveFavicon(c, icon)
		return
	}
	if s.ImageFetch == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	icon := s.fetchFavicon(c, siteURL)
	s.putFavicon(siteURL, icon)
	serveFavicon(c, icon)
}

func (s *Server) fetchFavicon(c *gin.Context, siteURL string) cachedImage {
	icon := cachedImage{body: transparentFavicon, contentType: "image/png", at: time.Now()}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	base, err := url.Parse(siteURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return icon
	}
	candidates := s.faviconLinks(ctx, siteURL)
	candidates = append(candidates, base.Scheme+"://"+base.Host+"/favicon.ico")
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		response, err := s.ImageFetch.Get(ctx, fetch.Request{
			URL: candidate, Accept: "image/png,image/x-icon,image/vnd.microsoft.icon,image/webp,image/gif,image/jpeg;q=0.8",
			MaxBytes: maxFaviconBytes,
		})
		if err != nil || response.Status != http.StatusOK {
			continue
		}
		if kind, ok := faviconType(response.Body); ok {
			return cachedImage{body: response.Body, contentType: kind, at: time.Now()}
		}
	}
	return icon
}

func (s *Server) faviconLinks(ctx context.Context, siteURL string) []string {
	response, err := s.ImageFetch.Get(ctx, fetch.Request{
		URL: siteURL, Accept: fetch.AcceptHTML, MaxBytes: maxFaviconHTMLBytes,
	})
	if err != nil || response.Status != http.StatusOK || !strings.Contains(strings.ToLower(response.ContentType), "html") {
		return nil
	}
	base, err := url.Parse(response.URL)
	if err != nil {
		return nil
	}
	var links []string
	tokens := html.NewTokenizer(bytes.NewReader(response.Body))
	for len(links) < 4 {
		switch tokens.Next() {
		case html.ErrorToken:
			return links
		case html.StartTagToken, html.SelfClosingTagToken:
			name, attrs := tokens.TagName()
			if !bytes.EqualFold(name, []byte("link")) {
				continue
			}
			var rel, href, kind string
			for attrs {
				key, value, more := tokens.TagAttr()
				attrs = more
				switch strings.ToLower(string(key)) {
				case "rel":
					rel = strings.ToLower(string(value))
				case "href":
					href = strings.TrimSpace(string(value))
				case "type":
					kind = strings.ToLower(string(value))
				}
			}
			if !faviconRel(rel) || href == "" || strings.Contains(kind, "svg") {
				continue
			}
			ref, err := url.Parse(href)
			if err != nil {
				continue
			}
			resolved := base.ResolveReference(ref)
			if (resolved.Scheme != "http" && resolved.Scheme != "https") || resolved.Host == "" || resolved.User != nil || strings.HasSuffix(strings.ToLower(resolved.Path), ".svg") {
				continue
			}
			links = append(links, resolved.String())
		}
	}
	return links
}

func faviconRel(rel string) bool {
	for _, part := range strings.Fields(rel) {
		if part == "icon" || part == "apple-touch-icon" || part == "apple-touch-icon-precomposed" {
			return true
		}
	}
	return false
}

func faviconType(body []byte) (string, bool) {
	if len(body) >= 6 && bytes.Equal(body[:4], []byte{0, 0, 1, 0}) {
		count := int(binary.LittleEndian.Uint16(body[4:6]))
		if count == 0 || count > 32 || len(body) < 6+16*count {
			return "", false
		}
		for i := range count {
			entry := body[6+16*i : 6+16*(i+1)]
			size := binary.LittleEndian.Uint32(entry[8:12])
			offset := binary.LittleEndian.Uint32(entry[12:16])
			if size > 0 && uint64(offset)+uint64(size) <= uint64(len(body)) {
				return "image/x-icon", true
			}
		}
		return "", false
	}
	return rasterImageType(body, 16, 1024)
}

func serveFavicon(c *gin.Context, icon cachedImage) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "public, max-age="+strconv.Itoa(int(faviconBrowserMaxAge.Seconds())))
	c.Data(http.StatusOK, icon.contentType, icon.body)
}

func (s *Server) getFavicon(siteURL string) (cachedImage, bool) {
	s.faviconMu.Lock()
	defer s.faviconMu.Unlock()
	icon, ok := s.favicons[siteURL]
	return icon, ok && time.Since(icon.at) < faviconCacheLifetime
}

func (s *Server) putFavicon(siteURL string, icon cachedImage) {
	s.faviconMu.Lock()
	defer s.faviconMu.Unlock()
	if s.favicons == nil {
		s.favicons = make(map[string]cachedImage)
	}
	if len(s.favicons) >= faviconCacheEntries {
		var oldestURL string
		var oldest time.Time
		for key, item := range s.favicons {
			if oldestURL == "" || item.at.Before(oldest) {
				oldestURL, oldest = key, item.at
			}
		}
		delete(s.favicons, oldestURL)
	}
	s.favicons[siteURL] = icon
}

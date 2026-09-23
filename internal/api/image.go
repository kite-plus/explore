package api

import (
	"bytes"
	"encoding/binary"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kite-plus/explore/internal/fetch"
)

const (
	maxImageBytes        = 2 << 20
	imageCacheEntries    = 32
	imageCacheLifetime   = 5 * time.Minute
	imageBrowserLifetime = time.Minute
)

type cachedImage struct {
	body        []byte
	contentType string
	at          time.Time
}

// entryImage serves only images referenced by visible feed entries. The
// source URL stays server-side, and the fetch client checks redirects,
// robots.txt, and DNS before making a bounded request.
func (s *Server) entryImage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.Status(http.StatusNotFound)
		return
	}
	source, err := s.Store.VisibleImageURL(c.Request.Context(), id)
	if err != nil {
		s.storeError(c, err)
		return
	}
	if image, ok := s.getImage(source); ok {
		s.serveImage(c, image)
		return
	}
	if s.ImageFetch == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	response, err := s.ImageFetch.Get(c.Request.Context(), fetch.Request{
		URL: source, Accept: "image/avif,image/webp,image/png,image/jpeg,image/gif;q=0.8", MaxBytes: maxImageBytes,
	})
	if err != nil || response.Status != http.StatusOK {
		c.Header("Cache-Control", "no-store")
		c.Status(http.StatusBadGateway)
		return
	}
	contentType, ok := imageType(response.Body)
	if !ok {
		c.Header("Cache-Control", "no-store")
		c.Status(http.StatusUnsupportedMediaType)
		return
	}
	image := cachedImage{body: response.Body, contentType: contentType, at: time.Now()}
	s.putImage(source, image)
	s.serveImage(c, image)
}

func (s *Server) serveImage(c *gin.Context, image cachedImage) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "public, max-age="+strconv.Itoa(int(imageBrowserLifetime.Seconds())))
	c.Data(http.StatusOK, image.contentType, image.body)
}

func (s *Server) getImage(source string) (cachedImage, bool) {
	s.imageMu.Lock()
	defer s.imageMu.Unlock()
	image, ok := s.images[source]
	return image, ok && time.Since(image.at) < imageCacheLifetime
}

func (s *Server) putImage(source string, image cachedImage) {
	s.imageMu.Lock()
	defer s.imageMu.Unlock()
	if s.images == nil {
		s.images = make(map[string]cachedImage)
	}
	if len(s.images) >= imageCacheEntries {
		var oldestURL string
		var oldest time.Time
		for url, item := range s.images {
			if oldestURL == "" || item.at.Before(oldest) {
				oldestURL, oldest = url, item.at
			}
		}
		delete(s.images, oldestURL)
	}
	s.images[source] = image
}

func imageType(body []byte) (string, bool) {
	return rasterImageType(body, 80, 6000)
}

func rasterImageType(body []byte, minSize, maxSize int) (string, bool) {
	contentType := http.DetectContentType(body)
	var width, height int
	switch contentType {
	case "image/jpeg", "image/png", "image/gif":
		config, _, err := image.DecodeConfig(bytes.NewReader(body))
		if err != nil {
			return "", false
		}
		width, height = config.Width, config.Height
	case "image/webp":
		width, height = webpSize(body)
	default:
		return "", false
	}
	return contentType, width >= minSize && height >= minSize && width <= maxSize && height <= maxSize
}

func webpSize(body []byte) (int, int) {
	if len(body) < 30 || string(body[:4]) != "RIFF" || string(body[8:12]) != "WEBP" {
		return 0, 0
	}
	switch string(body[12:16]) {
	case "VP8X":
		width := 1 + int(body[24]) + int(body[25])<<8 + int(body[26])<<16
		height := 1 + int(body[27]) + int(body[28])<<8 + int(body[29])<<16
		return width, height
	case "VP8 ":
		if string(body[23:26]) != "\x9d\x01\x2a" {
			return 0, 0
		}
		return int(binary.LittleEndian.Uint16(body[26:28]) & 0x3fff), int(binary.LittleEndian.Uint16(body[28:30]) & 0x3fff)
	case "VP8L":
		if body[20] != 0x2f {
			return 0, 0
		}
		width := 1 + int(body[21]) + int(body[22]&0x3f)<<8
		height := 1 + int(body[22]>>6) + int(body[23])<<2 + int(body[24]&0x0f)<<10
		return width, height
	}
	return 0, 0
}

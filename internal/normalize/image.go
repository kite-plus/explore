package normalize

import (
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/kite-plus/explore/internal/feed"
)

// imageURL prefers a visible image in the feed content. A parser-provided
// image is used when the content contains no image elements at all.
func imageURL(item feed.Item, article *url.URL) string {
	skipped := map[string]bool{}
	for _, fragment := range []string{item.Content, item.Summary} {
		z := html.NewTokenizer(strings.NewReader(fragment))
		for {
			token := z.Next()
			if token == html.ErrorToken {
				break
			}
			if token != html.StartTagToken && token != html.SelfClosingTagToken {
				continue
			}
			name, hasAttrs := z.TagName()
			if string(name) != "img" || !hasAttrs {
				continue
			}
			var src, lazySrc, width, height string
			for {
				key, value, more := z.TagAttr()
				switch string(key) {
				case "src":
					src = string(value)
				case "data-src":
					lazySrc = string(value)
				case "width":
					width = string(value)
				case "height":
					height = string(value)
				}
				if !more {
					break
				}
			}
			if tiny(width) || tiny(height) {
				if resolved := safeImageURL(article, src); resolved != "" {
					skipped[resolved] = true
				}
				continue
			}
			if resolved := safeImageURL(article, src); resolved != "" {
				return resolved
			}
			if resolved := safeImageURL(article, lazySrc); resolved != "" {
				return resolved
			}
		}
	}
	fallback := safeImageURL(article, item.Image)
	if !skipped[fallback] {
		return fallback
	}
	return ""
}

func tiny(value string) bool {
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(value), "px"))
	return err == nil && n <= 2
}

func safeImageURL(base *url.URL, raw string) string {
	resolved, ok := resolve(base, raw)
	if !ok || resolved.User != nil {
		return ""
	}
	resolved.Fragment = ""
	resolved.RawFragment = ""
	if len(resolved.String()) > 2000 || strings.HasSuffix(strings.ToLower(resolved.Path), ".svg") {
		return ""
	}
	return resolved.String()
}

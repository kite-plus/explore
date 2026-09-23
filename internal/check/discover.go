package check

import (
	"bytes"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// candidatePaths are the default feed addresses of common blog systems,
// tried when a page advertises no feed. See docs/design/worker.md 6.1.
var candidatePaths = []string{
	"/feed/",           // WordPress, Typecho with URL rewriting
	"/?feed=rss2",      // WordPress with plain permalinks
	"/rss.xml",         // Halo, Kite
	"/atom.xml",        // Hexo
	"/index.xml",       // Hugo
	"/feed.xml",        // Jekyll, Halo
	"/rss/",            // Ghost
	"/index.php/feed/", // Typecho without URL rewriting
}

var feedTypes = map[string]bool{
	"application/rss+xml":   true,
	"application/atom+xml":  true,
	"application/feed+json": true,
}

// page is what discovery reads from an HTML page.
type page struct {
	feeds     []string // alternate feed links, absolute, in page order
	generator string   // content of <meta name="generator">
	refresh   string   // absolute target of <meta http-equiv="refresh">
}

// readPage extracts feed links and the generator from HTML. Only the head
// matters, but a missing head is common enough to scan the whole document.
func readPage(body []byte, base *url.URL) page {
	var p page
	z := html.NewTokenizer(bytes.NewReader(body))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return p
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		name, hasAttr := z.TagName()
		if !hasAttr {
			continue
		}
		switch atom.Lookup(name) {
		case atom.Link:
			attrs := attributes(z)
			if !hasToken(attrs["rel"], "alternate") || !feedTypes[strings.ToLower(strings.TrimSpace(attrs["type"]))] {
				continue
			}
			if u, err := base.Parse(strings.TrimSpace(attrs["href"])); err == nil && attrs["href"] != "" {
				p.feeds = append(p.feeds, u.String())
			}
		case atom.Meta:
			attrs := attributes(z)
			if strings.EqualFold(attrs["name"], "generator") && p.generator == "" {
				p.generator = strings.TrimSpace(attrs["content"])
			}
			if strings.EqualFold(strings.TrimSpace(attrs["http-equiv"]), "refresh") && p.refresh == "" {
				if target := refreshTarget(attrs["content"]); target != "" {
					if u, err := base.Parse(target); err == nil {
						p.refresh = u.String()
					}
				}
			}
		case atom.Body:
			if len(p.feeds) > 0 && p.generator != "" {
				return p
			}
		}
	}
}

// refreshTarget reads the address from a refresh value such as
// "0; url=/zh/", in which the url= label and the quotes are optional.
func refreshTarget(content string) string {
	_, after, ok := strings.Cut(content, ";")
	if !ok {
		return ""
	}
	target := strings.TrimSpace(after)
	if len(target) >= 4 && strings.EqualFold(target[:4], "url=") {
		target = strings.TrimSpace(target[4:])
	}
	return strings.Trim(target, `"' `)
}

func attributes(z *html.Tokenizer) map[string]string {
	attrs := make(map[string]string)
	for {
		k, v, more := z.TagAttr()
		attrs[strings.ToLower(string(k))] = string(v)
		if !more {
			return attrs
		}
	}
}

func hasToken(list, token string) bool {
	for _, f := range strings.Fields(strings.ToLower(list)) {
		if f == token {
			return true
		}
	}
	return false
}

package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/net/html"

	"github.com/kite-plus/explore/internal/fetch"
	"github.com/kite-plus/explore/internal/normalize"
	"github.com/kite-plus/explore/internal/policy"
	"github.com/kite-plus/explore/internal/store"
)

// PageOnce reads the heads of the article pages due now, at most one per
// blog: the images and excerpts feeds leave out, and the posts sitemaps
// list. It returns how many it tried.
func (w *Worker) PageOnce(ctx context.Context) int {
	jobs, err := w.Store.ClaimPages(ctx, policy.PageChecksPerRound)
	if err != nil {
		if ctx.Err() == nil {
			w.log().Error("claiming article pages failed", "error", err)
		}
		return 0
	}
	asked := make([]int64, 0, len(jobs))
	for _, job := range jobs {
		asked = append(asked, job.BlogID)
	}
	// Sitemap pages have a quota of their own, so a backlog of covers to
	// fill in cannot starve them; a blog still gets one request a round.
	posts, err := w.Store.ClaimSitemapPages(ctx, policy.PageChecksPerRound, asked)
	if err != nil && ctx.Err() == nil {
		w.log().Error("claiming sitemap pages failed", "error", err)
	}
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Go(func() {
			if err := w.Store.RecordPage(ctx, job, w.readPage(ctx, job.URL)); err != nil && ctx.Err() == nil {
				w.log().Error("recording article page failed", "entry", job.EntryID, "error", err)
			}
		})
	}
	for _, job := range posts {
		wg.Go(func() {
			if err := w.Store.RecordSitemapPage(ctx, job, w.readPost(ctx, job)); err != nil && ctx.Err() == nil {
				w.log().Error("recording sitemap page failed", "url", job.URL, "error", err)
			}
		})
	}
	wg.Wait()
	return len(jobs) + len(posts)
}

// readPage reads what a feed entry's page head adds: a cover and an excerpt.
func (w *Worker) readPage(ctx context.Context, pageURL string) store.PageResult {
	h, ok, done := w.readHead(ctx, pageURL)
	if !ok {
		return store.PageResult{Done: done}
	}
	return store.PageResult{ImageURL: h.image, Excerpt: h.excerpt, Done: true}
}

// readPost reads a sitemap address's page head. It is a post when the page
// calls itself an article or gives its publish date, has a title, and does
// not ask to stay out of indexes.
func (w *Worker) readPost(ctx context.Context, job store.SitemapPageJob) store.SitemapPage {
	link, err := url.Parse(job.URL)
	if err != nil {
		return store.SitemapPage{Done: true}
	}
	identity := normalize.SitemapIdentity(link)
	h, ok, done := w.readHead(ctx, job.URL)
	if !ok {
		return store.SitemapPage{Done: done, Identity: identity}
	}
	title := cleanTitle(h.title, job.BlogName, h.siteName)
	if h.noIndex || title == "" || (!h.article && h.published == nil) {
		return store.SitemapPage{Done: true, Identity: identity}
	}
	return store.SitemapPage{
		Done: true, Post: true, Identity: identity, Title: title,
		PublishedAt: h.published, DateTrusted: h.published != nil,
		ImageURL: h.image, Excerpt: h.excerpt,
	}
}

// readHead fetches a page and reads its head. ok is false when there is no
// head to read; done then tells a lasting outcome from one worth retrying.
func (w *Worker) readHead(ctx context.Context, pageURL string) (h pageHead, ok, done bool) {
	ctx, cancel := context.WithTimeout(ctx, policy.PageCheckTimeout)
	defer cancel()
	resp, err := w.Fetch.Get(ctx, fetch.Request{
		URL: pageURL, Accept: fetch.AcceptHTML, MaxBytes: policy.PageHeadBytes, Truncate: true, StopAfter: "<body",
	})
	switch {
	case errors.Is(err, fetch.ErrRobotsDisallowed), errors.Is(err, fetch.ErrBadURL),
		errors.Is(err, fetch.ErrBlockedAddress), errors.Is(err, fetch.ErrTooManyRedirects):
		return pageHead{}, false, true
	case err != nil:
		return pageHead{}, false, false
	case resp.Status == http.StatusTooManyRequests || resp.Status >= 500:
		return pageHead{}, false, false
	case resp.Status != http.StatusOK:
		return pageHead{}, false, true
	}
	kind := strings.ToLower(resp.ContentType)
	if !strings.Contains(kind, "html") && !strings.Contains(http.DetectContentType(resp.Body), "html") {
		return pageHead{}, false, true
	}
	return parseHead(resp.Body, resp.URL), true, true
}

// pageHead is what a page's head says about the page. image and excerpt are
// already empty where the page's robots rules forbid them.
type pageHead struct {
	image, excerpt, title string
	siteName              string // og:site_name
	published             *time.Time
	article               bool // og:type or JSON-LD calls it an article
	noIndex               bool // it asks to stay out of indexes
}

// pageFromHTML reads a page head's cover image and description.
func pageFromHTML(body []byte, pageURL string) (image, excerpt string) {
	h := parseHead(body, pageURL)
	return h.image, h.excerpt
}

// parseHead reads a page's head from its Open Graph, Twitter and standard
// meta tags, its title and its JSON-LD. A robots meta tag for all crawlers
// or for KiteExplore can rule out parts: noindex or none rules out the page,
// nosnippet the description, noimageindex the image.
func parseHead(body []byte, pageURL string) pageHead {
	var h pageHead
	var ogImage, twitterImage, ogDescription, description, twitterDescription, ogTitle, titleTag, publishedTime, pageTime string
	allowImage, allowSnippet := true, true
	tokens := html.NewTokenizer(bytes.NewReader(body))
	var inTitle, inJSONLD bool
read:
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			break read
		case html.TextToken:
			switch {
			case inTitle:
				titleTag += string(tokens.Text())
			case inJSONLD:
				published, article, pageDate := fromJSONLD(tokens.Text())
				if article {
					h.article = true
					publishedTime = first(publishedTime, published)
				}
				pageTime = first(pageTime, pageDate)
			}
		case html.EndTagToken:
			inTitle, inJSONLD = false, false
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := tokens.TagName()
			switch {
			case bytes.EqualFold(name, []byte("body")):
				break read
			case bytes.EqualFold(name, []byte("title")):
				inTitle = true
				continue
			case bytes.EqualFold(name, []byte("script")):
				for hasAttr {
					var k, v []byte
					k, v, hasAttr = tokens.TagAttr()
					if strings.EqualFold(string(k), "type") && strings.EqualFold(strings.TrimSpace(string(v)), "application/ld+json") {
						inJSONLD = true
					}
				}
				continue
			case !bytes.EqualFold(name, []byte("meta")):
				continue
			}
			var key, content string
			for hasAttr {
				var k, v []byte
				k, v, hasAttr = tokens.TagAttr()
				switch strings.ToLower(string(k)) {
				case "name", "property":
					// Themes put og: and twitter: tags under either attribute.
					if key == "" {
						key = strings.ToLower(strings.TrimSpace(string(v)))
					}
				case "content":
					content = strings.TrimSpace(string(v))
				}
			}
			switch key {
			case "robots", strings.ToLower(policy.UserAgentToken):
				for _, rule := range strings.FieldsFunc(strings.ToLower(content), func(r rune) bool { return r == ',' || unicode.IsSpace(r) }) {
					switch rule {
					case "noindex", "none":
						h.noIndex = true
						allowImage, allowSnippet = false, false
					case "nosnippet":
						allowSnippet = false
					case "noimageindex":
						allowImage = false
					}
				}
			case "og:type":
				h.article = h.article || strings.EqualFold(content, "article")
			case "article:published_time":
				publishedTime = first(publishedTime, content)
			case "og:title":
				ogTitle = first(ogTitle, content)
			case "og:site_name":
				h.siteName = first(h.siteName, normalize.PlainText(content))
			case "og:image", "og:image:url", "og:image:secure_url":
				ogImage = first(ogImage, content)
			case "twitter:image", "twitter:image:src":
				twitterImage = first(twitterImage, content)
			case "og:description":
				ogDescription = first(ogDescription, content)
			case "description":
				description = first(description, content)
			case "twitter:description":
				twitterDescription = first(twitterDescription, content)
			}
		}
	}
	if allowImage {
		h.image = absoluteImage(pageURL, first(ogImage, twitterImage))
	}
	if allowSnippet {
		h.excerpt = normalize.Truncate(normalize.PlainText(first(ogDescription, description, twitterDescription)), policy.ExcerptMaxRunes)
	}
	h.title = first(strings.TrimSpace(ogTitle), strings.TrimSpace(titleTag))
	h.published = parseDate(publishedTime)
	if h.published == nil && h.article {
		// Some SEO plugins date only the WebPage node, not the article.
		h.published = parseDate(pageTime)
	}
	return h
}

// fromJSONLD finds an article and its publish date in a JSON-LD block, which
// may hold one object, a list or an @graph. pageDate is a WebPage's publish
// date, which does not make the page an article.
func fromJSONLD(raw []byte) (published string, article bool, pageDate string) {
	var doc any
	if json.Unmarshal(raw, &doc) != nil {
		return "", false, ""
	}
	var walk func(v any)
	walk = func(v any) {
		switch v := v.(type) {
		case []any:
			for _, item := range v {
				walk(item)
			}
		case map[string]any:
			d, _ := v["datePublished"].(string)
			switch {
			case isArticleType(v["@type"]):
				article = true
				published = first(published, d)
			case isType(v["@type"], "WebPage"):
				pageDate = first(pageDate, d)
			}
			walk(v["@graph"])
		}
	}
	walk(doc)
	return published, article, pageDate
}

// isType reports whether a JSON-LD @type, one name or a list, includes name.
func isType(t any, name string) bool {
	switch t := t.(type) {
	case string:
		return t == name
	case []any:
		return slices.Contains(t, any(name))
	}
	return false
}

func isArticleType(t any) bool {
	switch t := t.(type) {
	case string:
		return strings.HasSuffix(t, "Article") || t == "BlogPosting" || t == "SocialMediaPosting"
	case []any:
		for _, item := range t {
			if isArticleType(item) {
				return true
			}
		}
	}
	return false
}

// parseDate reads the ISO 8601 forms pages give their publish date in. A date
// before policy.EarliestDate counts as none.
func parseDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05-0700", "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil && !t.Before(policy.EarliestDate) {
			t = t.UTC()
			return &t
		}
	}
	return nil
}

// cleanTitle turns a page's title into a post's: plain text without the
// site's name, which themes often add before or after it. names are the
// blog's name and the page's og:site_name; they match with curly and straight
// quotes alike, as WordPress curls the quotes in a title but not in a name.
func cleanTitle(title string, names ...string) string {
	t := []rune(normalize.PlainText(title))
	for _, name := range names {
		n := []rune(strings.TrimSpace(normalize.PlainText(name)))
		if len(n) == 0 {
			continue
		}
		for _, sep := range []string{" - ", " | ", " – ", " — ", " · ", " _ "} {
			s := []rune(sep)
			if affix := append(append([]rune{}, s...), n...); len(t) > len(affix) && sameText(t[len(t)-len(affix):], affix) {
				t = t[:len(t)-len(affix)]
			}
			if affix := append(append([]rune{}, n...), s...); len(t) > len(affix) && sameText(t[:len(affix)], affix) {
				t = t[len(affix):]
			}
		}
	}
	return normalize.Truncate(strings.TrimSpace(string(t)), policy.TitleMaxRunes)
}

// sameText compares two runs of text, taking curly quotes for straight ones.
func sameText(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if straightQuote(a[i]) != straightQuote(b[i]) {
			return false
		}
	}
	return true
}

func straightQuote(r rune) rune {
	switch r {
	case '‘', '’':
		return '\''
	case '“', '”':
		return '"'
	}
	return r
}

// absoluteImage resolves an image reference against the page it came from,
// keeping only http and https addresses.
func absoluteImage(pageURL, ref string) string {
	if ref == "" {
		return ""
	}
	base, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}
	u, err := base.Parse(ref)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}

// first returns the first non-empty value.
func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

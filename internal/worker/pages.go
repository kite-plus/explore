package worker

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
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

// checkPages reads a round of article page heads once a minute, for the
// images and excerpts feeds leave out.
func (w *Worker) checkPages(ctx context.Context) {
	now := w.now()
	if !w.lastPageCheck.IsZero() && now.Sub(w.lastPageCheck) < time.Minute {
		return
	}
	w.lastPageCheck = now
	w.PageOnce(ctx)
}

// PageOnce reads the heads of the article pages due now and returns how many
// it tried.
func (w *Worker) PageOnce(ctx context.Context) int {
	jobs, err := w.Store.ClaimPages(ctx, policy.PageChecksPerMinute)
	if err != nil {
		if ctx.Err() == nil {
			w.log().Error("claiming article pages failed", "error", err)
		}
		return 0
	}
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Go(func() {
			if err := w.Store.RecordPage(ctx, job, w.readPage(ctx, job.URL)); err != nil && ctx.Err() == nil {
				w.log().Error("recording article page failed", "entry", job.EntryID, "error", err)
			}
		})
	}
	wg.Wait()
	return len(jobs)
}

func (w *Worker) readPage(ctx context.Context, pageURL string) store.PageResult {
	ctx, cancel := context.WithTimeout(ctx, policy.PageCheckTimeout)
	defer cancel()
	resp, err := w.Fetch.Get(ctx, fetch.Request{
		URL: pageURL, Accept: fetch.AcceptHTML, MaxBytes: policy.PageHeadBytes, Truncate: true,
	})
	switch {
	case errors.Is(err, fetch.ErrRobotsDisallowed), errors.Is(err, fetch.ErrBadURL),
		errors.Is(err, fetch.ErrBlockedAddress), errors.Is(err, fetch.ErrTooManyRedirects):
		return store.PageResult{Done: true}
	case err != nil:
		return store.PageResult{}
	case resp.Status == http.StatusTooManyRequests || resp.Status >= 500:
		return store.PageResult{}
	case resp.Status != http.StatusOK:
		return store.PageResult{Done: true}
	}
	kind := strings.ToLower(resp.ContentType)
	if !strings.Contains(kind, "html") && !strings.Contains(http.DetectContentType(resp.Body), "html") {
		return store.PageResult{Done: true}
	}
	image, excerpt := pageFromHTML(resp.Body, resp.URL)
	return store.PageResult{ImageURL: image, Excerpt: excerpt, Done: true}
}

// pageFromHTML reads a page head's cover image and description from its Open
// Graph, Twitter and standard meta tags. A robots meta tag for all crawlers
// or for KiteExplore can rule out either: noindex or none takes both,
// nosnippet the description, noimageindex the image.
func pageFromHTML(body []byte, pageURL string) (image, excerpt string) {
	var ogImage, twitterImage, ogDescription, description, twitterDescription string
	allowImage, allowSnippet := true, true
	tokens := html.NewTokenizer(bytes.NewReader(body))
read:
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			break read
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := tokens.TagName()
			if bytes.EqualFold(name, []byte("body")) {
				break read
			}
			if !bytes.EqualFold(name, []byte("meta")) {
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
						allowImage, allowSnippet = false, false
					case "nosnippet":
						allowSnippet = false
					case "noimageindex":
						allowImage = false
					}
				}
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
		image = absoluteImage(pageURL, first(ogImage, twitterImage))
	}
	if allowSnippet {
		excerpt = normalize.Truncate(normalize.PlainText(first(ogDescription, description, twitterDescription)), policy.ExcerptMaxRunes)
	}
	return image, excerpt
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

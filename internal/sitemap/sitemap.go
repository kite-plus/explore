// Package sitemap reads sitemaps (sitemaps.org) and learns what a blog's post
// addresses look like, to pick its posts out of everything a sitemap lists.
// It does no I/O.
package sitemap

import (
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"errors"
	"io"
	"net/url"
	"slices"
	"strings"
	"time"
)

// ErrNotSitemap is returned for a document that is neither a urlset nor a
// sitemapindex.
var ErrNotSitemap = errors.New("not a sitemap")

// ErrTooLarge is returned for a gzipped sitemap that unpacks past its limit.
var ErrTooLarge = errors.New("sitemap exceeds the size limit")

// URL is an address a sitemap lists.
type URL struct {
	Loc     string
	LastMod *time.Time
}

// Doc is one sitemap file: the addresses it lists or, for an index, the
// sitemaps it points to.
type Doc struct {
	URLs     []URL
	Children []string
}

// Parse reads a sitemap or a sitemap index, gzipped or not. A gzipped body
// may unpack to at most maxBytes; at most maxEntries entries are kept.
func Parse(body []byte, maxBytes int64, maxEntries int) (Doc, error) {
	if bytes.HasPrefix(body, []byte{0x1f, 0x8b}) {
		z, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return Doc{}, err
		}
		body, err = io.ReadAll(io.LimitReader(z, maxBytes+1))
		if err != nil {
			return Doc{}, err
		}
		if int64(len(body)) > maxBytes {
			return Doc{}, ErrTooLarge
		}
	}

	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.Strict = false
	var doc Doc
	var root, field string
	var loc, lastmod strings.Builder
	// Only an entry's direct children count: image and news extensions nest
	// their own loc inside a url.
	depth, entryDepth := 0, -1
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if root == "" {
				return Doc{}, err
			}
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			name := t.Name.Local
			switch {
			case root == "":
				if name != "urlset" && name != "sitemapindex" {
					return Doc{}, ErrNotSitemap
				}
				root = name
			case depth == 2 && ((root == "urlset" && name == "url") || (root == "sitemapindex" && name == "sitemap")):
				entryDepth = depth
				loc.Reset()
				lastmod.Reset()
			case entryDepth > 0 && depth == entryDepth+1 && (name == "loc" || name == "lastmod"):
				field = name
			}
		case xml.CharData:
			switch field {
			case "loc":
				loc.Write(t)
			case "lastmod":
				lastmod.Write(t)
			}
		case xml.EndElement:
			depth--
			switch {
			case field != "" && depth == entryDepth:
				field = ""
			case entryDepth > 0 && depth == entryDepth-1:
				entryDepth = -1
				l := strings.TrimSpace(loc.String())
				if l == "" {
					continue
				}
				if root == "sitemapindex" {
					doc.Children = append(doc.Children, l)
				} else {
					doc.URLs = append(doc.URLs, URL{Loc: l, LastMod: parseLastMod(lastmod.String())})
				}
				if len(doc.URLs)+len(doc.Children) >= maxEntries {
					return doc, nil
				}
			}
		}
	}
	if root == "" {
		return Doc{}, ErrNotSitemap
	}
	return doc, nil
}

// parseLastMod reads the W3C datetime forms sitemaps use.
func parseLastMod(s string) *time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04Z07:00", "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			t = t.UTC()
			return &t
		}
	}
	return nil
}

// Patterns is what a blog's post addresses look like, learned from examples.
// Addresses of the same form (path depth, trailing slash, extension and query
// keys) share one template: a path segment all examples agree on stays
// literal, numbers become digit classes and anything else a wildcard, so
// /2024/05/hello/ and /2023/11/world/ give one template. The last segment is
// always the post's own and never literal.
type Patterns struct {
	templates map[string][][]string
}

// Learn builds the patterns of a blog's post addresses from its feed's links.
func Learn(examples []*url.URL) Patterns {
	groups := map[string][][]string{}
	for _, u := range examples {
		if u == nil {
			continue
		}
		key, segments := form(u)
		if len(segments) == 0 && u.RawQuery == "" {
			continue // the home page is no post
		}
		groups[key] = append(groups[key], segments)
	}
	p := Patterns{templates: map[string][][]string{}}
	for key, all := range groups {
		template := make([]string, len(all[0]))
		for i := range template {
			values := make([]string, 0, len(all))
			for _, segments := range all {
				values = append(values, segments[i])
			}
			template[i] = generalize(values, i == len(template)-1)
		}
		p.templates[key] = append(p.templates[key], template)
	}
	return p
}

// Match reports whether u looks like one of the blog's posts. Patterns
// learned from nothing match everything.
func (p Patterns) Match(u *url.URL) bool {
	if len(p.templates) == 0 {
		return true
	}
	key, segments := form(u)
	for _, template := range p.templates[key] {
		if matches(template, segments) {
			return true
		}
	}
	return false
}

// form splits an address into its path segments and a key for everything
// else its form depends on.
func form(u *url.URL) (string, []string) {
	path := u.Path
	trailing := strings.HasSuffix(path, "/")
	path = strings.Trim(path, "/")
	var segments []string
	if path != "" {
		segments = strings.Split(path, "/")
	}
	ext := ""
	if n := len(segments); n > 0 {
		if dot := strings.LastIndexByte(segments[n-1], '.'); dot > 0 {
			ext = strings.ToLower(segments[n-1][dot:])
		}
	}
	keys := make([]string, 0, len(u.Query()))
	for k := range u.Query() {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	trail := "-"
	if trailing {
		trail = "/"
	}
	return strings.Join([]string{string(rune('0' + min(len(segments), 9))), trail, ext, strings.Join(keys, "&")}, "|"), segments
}

// generalize turns the values examples have at one path position into a
// template element: a literal, a digit class or a wildcard.
func generalize(values []string, last bool) string {
	numeric := !slices.ContainsFunc(values, func(v string) bool { return !isNumber(v) })
	switch {
	case numeric && last:
		// A post's own number: an ID of any length.
		return "#num"
	case numeric:
		class := dateClass(values[0])
		if slices.ContainsFunc(values, func(v string) bool { return dateClass(v) != class }) {
			return "#num"
		}
		return class
	case !last && !slices.ContainsFunc(values, func(v string) bool { return v != values[0] }):
		return values[0]
	default:
		return "*"
	}
}

func isNumber(s string) bool {
	return s != "" && strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

// dateClass names a number by its likely place in a dated address: a year,
// or a month or day.
func dateClass(s string) string {
	switch {
	case len(s) == 4:
		return "#year"
	case len(s) <= 2:
		return "#day"
	default:
		return "#num"
	}
}

func matches(template, segments []string) bool {
	if len(template) != len(segments) {
		return false
	}
	for i, t := range template {
		s := segments[i]
		switch {
		case t == "*":
			if s == "" {
				return false
			}
		case t == "#num":
			if !isNumber(s) {
				return false
			}
		case strings.HasPrefix(t, "#"):
			if !isNumber(s) || dateClass(s) != t {
				return false
			}
		default:
			if s != t {
				return false
			}
		}
	}
	return true
}

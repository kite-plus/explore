package normalize

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// wholePage reports whether a feed field holds a whole HTML page, as some
// feeds put in each item's description, rather than a fragment of the post.
// A page starts with a doctype or a head; a fragment wrapped in <html> and
// <body> is still a fragment.
func wholePage(s string) bool {
	z := html.NewTokenizer(strings.NewReader(s))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return false
		case html.DoctypeToken:
			return true
		case html.TextToken:
			if len(bytes.TrimSpace(z.Text())) > 0 {
				return false
			}
		case html.StartTagToken, html.EndTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			if a := atom.Lookup(name); a != atom.Html {
				return a == atom.Head
			}
		}
	}
}

// mainContent returns what a reader reads as the post on a whole page: the
// inside of its main element, else of its body. The page's head and the
// site's header and footer around the post are left out. A fragment comes
// back unchanged.
func mainContent(s string) string {
	if !wholePage(s) {
		return s
	}
	bodyStart, bodyEnd, mainStart, mainEnd := -1, len(s), -1, len(s)
	z := html.NewTokenizer(strings.NewReader(s))
	at := 0 // byte offset of the current token; tokens are contiguous
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		end := at + len(z.Raw())
		name, _ := z.TagName()
		switch a := atom.Lookup(name); {
		case a == atom.Body && tt == html.StartTagToken && bodyStart < 0:
			bodyStart = end
		case a == atom.Body && tt == html.EndTagToken && bodyStart >= 0:
			bodyEnd = at
		case a == atom.Main && tt == html.StartTagToken && mainStart < 0:
			mainStart = end
		case a == atom.Main && tt == html.EndTagToken && mainStart >= 0 && mainEnd == len(s):
			mainEnd = at
		}
		at = end
	}
	switch {
	case mainStart >= 0:
		return s[mainStart:mainEnd]
	case bodyStart >= 0:
		return s[bodyStart:bodyEnd]
	}
	return s
}

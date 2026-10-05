package normalize

import (
	"bytes"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// PlainText turns feed HTML into one line of text. Block elements become
// spaces so paragraphs do not run together, and the contents of script,
// style, svg and math are dropped. Tags that are not HTML elements, such as
// the <T> in a title about generics, are kept as the text the author wrote;
// the exception is a custom element such as <mjx-container> in text that has
// HTML elements, which is markup.
func PlainText(s string) string {
	if !strings.ContainsAny(s, "<&") {
		return collapse(s)
	}

	isHTML := sync.OnceValue(func() bool { return hasElements(s) })
	z := html.NewTokenizer(strings.NewReader(s))
	var b strings.Builder
	skipping, depth := atom.Atom(0), 0
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return collapse(b.String())
		case html.TextToken:
			if skipping == 0 {
				b.Write(z.Text())
			}
		case html.StartTagToken, html.EndTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			a := atom.Lookup(name)
			switch {
			case skipping != 0:
				// An svg can nest another svg, so count them to find the matching end tag.
				if a == skipping && tt == html.StartTagToken {
					depth++
				} else if a == skipping && tt == html.EndTagToken {
					if depth--; depth == 0 {
						skipping = 0
					}
				}
			case a == 0:
				if !customElement(name) || !isHTML() {
					b.Write(z.Raw())
				}
			case dropped[a]:
				if tt == html.StartTagToken {
					skipping, depth = a, 1
				}
			case blocks[a]:
				b.WriteByte(' ')
			}
		}
	}
}

// dropped are the elements whose contents are not prose. The tags inside svg
// and math are not HTML elements, so they would otherwise be kept as text.
var dropped = map[atom.Atom]bool{
	atom.Script: true, atom.Style: true, atom.Noscript: true, atom.Template: true,
	atom.Svg: true, atom.Math: true,
}

// customElement reports whether a lowercased tag name is a custom element's,
// which the HTML standard makes start with a letter and contain a hyphen.
func customElement(name []byte) bool {
	return bytes.IndexByte(name, '-') > 0 && 'a' <= name[0] && name[0] <= 'z'
}

// hasElements reports whether s has a tag of an HTML element, which tells
// markup from text such as a title about <md-chip>.
func hasElements(s string) bool {
	z := html.NewTokenizer(strings.NewReader(s))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return false
		case html.StartTagToken, html.EndTagToken, html.SelfClosingTagToken:
			if name, _ := z.TagName(); atom.Lookup(name) != 0 {
				return true
			}
		}
	}
}

var blocks = map[atom.Atom]bool{
	atom.P: true, atom.Br: true, atom.Div: true, atom.Li: true, atom.Ul: true, atom.Ol: true,
	atom.H1: true, atom.H2: true, atom.H3: true, atom.H4: true, atom.H5: true, atom.H6: true,
	atom.Blockquote: true, atom.Pre: true, atom.Table: true, atom.Tr: true, atom.Td: true,
	atom.Th: true, atom.Section: true, atom.Article: true, atom.Header: true, atom.Footer: true,
	atom.Figure: true, atom.Figcaption: true, atom.Hr: true, atom.Dd: true, atom.Dt: true,
}

func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Truncate shortens s to at most limit runes, the ellipsis included.
func Truncate(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return strings.TrimRight(string(runes[:limit-1]), " ") + "…"
}

package normalize

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// PlainText turns feed HTML into one line of text. Block elements become
// spaces so paragraphs do not run together, the contents of script and
// style are dropped, and tags that are not HTML elements, such as the <T>
// in a title about generics, are kept as the text the author wrote.
func PlainText(s string) string {
	if !strings.ContainsAny(s, "<&") {
		return collapse(s)
	}

	z := html.NewTokenizer(strings.NewReader(s))
	var b strings.Builder
	skipping := atom.Atom(0)
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
			case a == 0:
				if skipping == 0 {
					b.Write(z.Raw())
				}
			case a == atom.Script || a == atom.Style || a == atom.Noscript || a == atom.Template:
				if tt == html.StartTagToken && skipping == 0 {
					skipping = a
				} else if tt == html.EndTagToken && skipping == a {
					skipping = 0
				}
			case blocks[a]:
				b.WriteByte(' ')
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

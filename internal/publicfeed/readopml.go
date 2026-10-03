package publicfeed

import (
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"golang.org/x/net/html/charset"
)

// ErrNotOPML is a file that is not an OPML document.
var ErrNotOPML = errors.New("not an OPML document")

// ReadOPML returns the feeds an OPML file lists, out of any folders: the
// first max of them, and how many more there were. An outline counts when
// it has a feed or a site address; attribute names match in any case, as
// some readers write xmlurl.
func ReadOPML(r io.Reader, max int) (outlines []Outline, more int, err error) {
	d := xml.NewDecoder(r)
	d.CharsetReader = charset.NewReaderLabel
	root := false
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, 0, ErrNotOPML
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if !root {
			if start.Name.Local != "opml" {
				return nil, 0, ErrNotOPML
			}
			root = true
			continue
		}
		if start.Name.Local != "outline" {
			continue
		}
		var o Outline
		var text string
		for _, a := range start.Attr {
			switch strings.ToLower(a.Name.Local) {
			case "text":
				text = strings.TrimSpace(a.Value)
			case "title":
				o.Name = strings.TrimSpace(a.Value)
			case "xmlurl":
				o.FeedURL = strings.TrimSpace(a.Value)
			case "htmlurl":
				o.SiteURL = strings.TrimSpace(a.Value)
			}
		}
		if o.FeedURL == "" && o.SiteURL == "" {
			continue
		}
		if o.Name == "" {
			o.Name = text
		}
		if len(outlines) == max {
			more++
			continue
		}
		outlines = append(outlines, o)
	}
	if !root {
		return nil, 0, ErrNotOPML
	}
	return outlines, more, nil
}

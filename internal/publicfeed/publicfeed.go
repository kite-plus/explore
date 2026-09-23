// Package publicfeed renders Explore's own outputs: the home stream as
// RSS 2.0 and the blog list as OPML, so readers can take both elsewhere.
package publicfeed

import (
	"bytes"
	"encoding/xml"
	"time"
)

// Channel describes the feed as a whole.
type Channel struct {
	Title       string
	Link        string // Explore's home page
	Self        string // this feed's own address
	Description string
}

// Item is one stream entry. Link is always the post on the author's site.
type Item struct {
	Title       string
	Link        string
	Excerpt     string // omitted when empty
	PublishedAt time.Time
	BlogName    string
	BlogFeed    string
}

// Outline is one blog in the OPML list.
type Outline struct {
	Name    string
	SiteURL string
	FeedURL string
}

type rss struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Atom    string     `xml:"xmlns:atom,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Self          atomLink  `xml:"atom:link"`
	Description   string    `xml:"description"`
	LastBuildDate string    `xml:"lastBuildDate,omitempty"`
	Items         []rssItem `xml:"item"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type rssItem struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	GUID        rssGUID   `xml:"guid"`
	PubDate     string    `xml:"pubDate"`
	Description string    `xml:"description,omitempty"`
	Source      rssSource `xml:"source"`
}

type rssGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

// rssSource is the RSS 2.0 element that names the feed an item came from.
type rssSource struct {
	URL  string `xml:"url,attr"`
	Name string `xml:",chardata"`
}

// RSS renders the stream. lastBuildDate is the newest item's date, so the
// output only changes when the stream does.
func RSS(ch Channel, items []Item) ([]byte, error) {
	c := rssChannel{
		Title:       ch.Title,
		Link:        ch.Link,
		Self:        atomLink{Href: ch.Self, Rel: "self", Type: "application/rss+xml"},
		Description: ch.Description,
		Items:       make([]rssItem, 0, len(items)),
	}
	for i, it := range items {
		if i == 0 {
			c.LastBuildDate = it.PublishedAt.UTC().Format(time.RFC1123Z)
		}
		c.Items = append(c.Items, rssItem{
			Title:       it.Title,
			Link:        it.Link,
			GUID:        rssGUID{IsPermaLink: "true", Value: it.Link},
			PubDate:     it.PublishedAt.UTC().Format(time.RFC1123Z),
			Description: it.Excerpt,
			Source:      rssSource{URL: it.BlogFeed, Name: it.BlogName},
		})
	}
	return encode(rss{Version: "2.0", Atom: "http://www.w3.org/2005/Atom", Channel: c})
}

type opml struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    opmlHead `xml:"head"`
	Body    opmlBody `xml:"body"`
}

type opmlHead struct {
	Title string `xml:"title"`
}

type opmlBody struct {
	Outlines []opmlOutline `xml:"outline"`
}

type opmlOutline struct {
	Type    string `xml:"type,attr"`
	Text    string `xml:"text,attr"`
	Title   string `xml:"title,attr"`
	XMLURL  string `xml:"xmlUrl,attr"`
	HTMLURL string `xml:"htmlUrl,attr"`
}

// OPML renders the blog list for import into any feed reader.
func OPML(title string, blogs []Outline) ([]byte, error) {
	doc := opml{Version: "2.0", Head: opmlHead{Title: title}}
	for _, b := range blogs {
		doc.Body.Outlines = append(doc.Body.Outlines, opmlOutline{
			Type: "rss", Text: b.Name, Title: b.Name, XMLURL: b.FeedURL, HTMLURL: b.SiteURL,
		})
	}
	return encode(doc)
}

func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

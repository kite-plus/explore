// Package publicfeed renders Explore's own outputs: a stream as RSS 2.0 or
// JSON Feed 1.1 and a list of blogs as OPML, so readers can take them
// elsewhere.
package publicfeed

import (
	"bytes"
	"encoding/json"
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
	Excerpt     string    // omitted when empty
	PublishedAt time.Time // omitted when zero, for a date the feed gave untrusted
	BlogName    string
	BlogFeed    string
	// BlogSite and BlogIcon, the blog's home page and favicon, appear in JSON
	// Feed only: RSS has no place for them in an item.
	BlogSite string
	BlogIcon string
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
	PubDate     string    `xml:"pubDate,omitempty"`
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
		item := rssItem{
			Title:       it.Title,
			Link:        it.Link,
			GUID:        rssGUID{IsPermaLink: "true", Value: it.Link},
			Description: it.Excerpt,
			Source:      rssSource{URL: it.BlogFeed, Name: it.BlogName},
		}
		if !it.PublishedAt.IsZero() {
			item.PubDate = it.PublishedAt.UTC().Format(time.RFC1123Z)
			if i == 0 {
				c.LastBuildDate = item.PubDate
			}
		}
		c.Items = append(c.Items, item)
	}
	return encode(rss{Version: "2.0", Atom: "http://www.w3.org/2005/Atom", Channel: c})
}

type jsonFeed struct {
	Version     string     `json:"version"`
	Title       string     `json:"title"`
	HomePageURL string     `json:"home_page_url"`
	FeedURL     string     `json:"feed_url"`
	Description string     `json:"description,omitempty"`
	Items       []jsonItem `json:"items"`
}

type jsonItem struct {
	ID            string       `json:"id"`
	URL           string       `json:"url"`
	Title         string       `json:"title"`
	ContentText   string       `json:"content_text"`
	Summary       string       `json:"summary,omitempty"`
	DatePublished string       `json:"date_published,omitempty"`
	Authors       []jsonAuthor `json:"authors"`
}

type jsonAuthor struct {
	Name   string `json:"name"`
	URL    string `json:"url,omitempty"`
	Avatar string `json:"avatar,omitempty"`
}

// JSON renders the stream as JSON Feed 1.1. An item's author is its blog,
// with the blog's favicon as the avatar, which is what a blogroll draws.
func JSON(ch Channel, items []Item) ([]byte, error) {
	f := jsonFeed{
		Version:     "https://jsonfeed.org/version/1.1",
		Title:       ch.Title,
		HomePageURL: ch.Link,
		FeedURL:     ch.Self,
		Description: ch.Description,
		Items:       make([]jsonItem, 0, len(items)),
	}
	for _, it := range items {
		item := jsonItem{
			ID:    it.Link,
			URL:   it.Link,
			Title: it.Title,
			// JSON Feed wants every item to carry its content, and the
			// excerpt is all of it Explore keeps.
			ContentText: it.Excerpt,
			Summary:     it.Excerpt,
			Authors:     []jsonAuthor{{Name: it.BlogName, URL: it.BlogSite, Avatar: it.BlogIcon}},
		}
		if !it.PublishedAt.IsZero() {
			item.DatePublished = it.PublishedAt.UTC().Format(time.RFC3339)
		}
		f.Items = append(f.Items, item)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

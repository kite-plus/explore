// Package feed parses RSS, Atom and JSON Feed documents into one model.
package feed

import (
	"bytes"
	"errors"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

// ErrNotFeed means the document is not a feed at all, such as an HTML page,
// as opposed to a feed that fails to parse.
var ErrNotFeed = errors.New("not a feed")

// Feed is a parsed feed, whatever its format.
type Feed struct {
	Format      string // "rss", "atom" or "json"
	Title       string
	Description string
	SiteURL     string
	Language    string
	Generator   string
	Items       []Item
}

// Item is one feed entry. Content is only used to derive an excerpt when
// Summary is empty, and is dropped right after.
type Item struct {
	ID        string
	Link      string
	Title     string
	Summary   string
	Content   string
	Image     string
	Published *time.Time
	Updated   *time.Time
	// DateUnreadable means the item carries a date in a form that could
	// not be parsed, as opposed to no date at all.
	DateUnreadable bool
	Categories     []string
}

// Parse reads a feed document. Character sets other than UTF-8 are converted
// using the document's own declaration.
func Parse(body []byte) (*Feed, error) {
	parsed, err := gofeed.NewParser().Parse(bytes.NewReader(body))
	if errors.Is(err, gofeed.ErrFeedTypeNotDetected) {
		return nil, ErrNotFeed
	}
	if err != nil {
		return nil, err
	}

	f := &Feed{
		Format:      parsed.FeedType,
		Title:       strings.TrimSpace(parsed.Title),
		Description: strings.TrimSpace(parsed.Description),
		SiteURL:     strings.TrimSpace(parsed.Link),
		Language:    strings.TrimSpace(parsed.Language),
		Generator:   strings.TrimSpace(parsed.Generator),
		Items:       make([]Item, 0, len(parsed.Items)),
	}
	for _, it := range parsed.Items {
		if it == nil {
			continue
		}
		link := strings.TrimSpace(it.Link)
		if link == "" && len(it.Links) > 0 {
			link = strings.TrimSpace(it.Links[0])
		}
		f.Items = append(f.Items, Item{
			ID:        strings.TrimSpace(it.GUID),
			Link:      link,
			Title:     it.Title,
			Summary:   it.Description,
			Content:   it.Content,
			Image:     imageURL(it),
			Published: it.PublishedParsed,
			Updated:   it.UpdatedParsed,
			DateUnreadable: it.PublishedParsed == nil && it.UpdatedParsed == nil &&
				strings.TrimSpace(it.Published+it.Updated) != "",
			Categories: it.Categories,
		})
	}
	return f, nil
}

func imageURL(item *gofeed.Item) string {
	if item.Image == nil {
		return ""
	}
	return strings.TrimSpace(item.Image.URL)
}

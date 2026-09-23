package worker

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/kite-plus/explore/internal/fetch"
	"github.com/kite-plus/explore/internal/normalize"
	"github.com/kite-plus/explore/internal/policy"
	"github.com/kite-plus/explore/internal/store"
)

const descriptionRefreshEvery = 7 * 24 * time.Hour

func (w *Worker) refreshDescription(ctx context.Context, b store.Claimed, feedDescription string) {
	if b.DescriptionCheckedAt != nil && w.now().Sub(*b.DescriptionCheckedAt) < descriptionRefreshEvery {
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	description := ""
	resp, err := w.Fetch.Get(requestCtx, fetch.Request{
		URL: b.SiteURL, Accept: fetch.AcceptHTML, MaxBytes: policy.MaxHTMLBytes,
	})
	if err == nil && resp.Status == http.StatusOK {
		kind := strings.ToLower(resp.ContentType)
		if strings.Contains(kind, "html") || strings.Contains(http.DetectContentType(resp.Body), "html") {
			description = descriptionFromHTML(resp.Body)
		}
	}
	if description == "" {
		description = cleanDescription(feedDescription)
	}
	if err := w.Store.SetDescription(ctx, b.ID, description); err != nil && ctx.Err() == nil {
		w.log().Warn("storing blog description failed", "host", b.Host, "error", err)
	}
}

func descriptionFromHTML(body []byte) string {
	var standard, openGraph, twitter string
	tokens := html.NewTokenizer(bytes.NewReader(body))
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			if standard != "" {
				return standard
			}
			if openGraph != "" {
				return openGraph
			}
			return twitter
		case html.StartTagToken, html.SelfClosingTagToken:
			name, attrs := tokens.TagName()
			if bytes.EqualFold(name, []byte("body")) {
				if standard != "" {
					return standard
				}
				if openGraph != "" {
					return openGraph
				}
				return twitter
			}
			if !bytes.EqualFold(name, []byte("meta")) {
				continue
			}
			var nameAttr, property, content string
			for attrs {
				key, value, more := tokens.TagAttr()
				attrs = more
				switch strings.ToLower(string(key)) {
				case "name":
					nameAttr = strings.ToLower(strings.TrimSpace(string(value)))
				case "property":
					property = strings.ToLower(strings.TrimSpace(string(value)))
				case "content":
					content = cleanDescription(string(value))
				}
			}
			switch {
			case nameAttr == "description" && standard == "":
				standard = content
			case property == "og:description" && openGraph == "":
				openGraph = content
			case nameAttr == "twitter:description" && twitter == "":
				twitter = content
			}
		}
	}
}

func cleanDescription(raw string) string {
	return normalize.Truncate(normalize.PlainText(raw), 240)
}

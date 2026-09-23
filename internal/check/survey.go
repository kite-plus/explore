package check

import (
	"bytes"
	"context"
	"mime"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kite-plus/explore/internal/fetch"
	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/normalize"
	"github.com/kite-plus/explore/internal/policy"
)

// SurveyRecord is what the E0 survey learns about one blog: the check
// report and the measurements behind the thresholds of
// docs/design/architecture.md section 6.
type SurveyRecord struct {
	SiteURL string             `json:"site_url"`
	FeedURL string             `json:"feed_url,omitempty"`
	Report  *model.CheckReport `json:"report,omitempty"`
	// System is the report's generator or, when the feed was given and
	// names none, the one the home page names.
	System model.Generator `json:"system,omitempty"`
	Feed   *FeedStats      `json:"feed,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// FeedStats measures the feed a check found. Text lengths are runes of
// plain text, as medians over the items.
type FeedStats struct {
	Bytes   int    `json:"bytes"`
	Charset string `json:"charset,omitempty"`
	// Conditional is the status of a second request made with the
	// validators of the first, as the worker makes it: "304" when
	// conditional requests work, "none" when there were no validators,
	// "error" when the request failed.
	Conditional string `json:"conditional"`

	Items      int `json:"items"`
	Valid      int `json:"valid"`
	NoLink     int `json:"no_link,omitempty"`
	OffDomain  int `json:"off_domain,omitempty"`
	NoTitle    int `json:"no_title,omitempty"`
	Duplicate  int `json:"duplicate,omitempty"`
	Undated    int `json:"undated,omitempty"`
	Unreadable int `json:"unreadable_dates,omitempty"`
	Untrusted  int `json:"untrusted,omitempty"`

	MaxSameMinute int `json:"max_same_minute"`
	InWindow      int `json:"in_window"` // dated items inside policy.StreamWindow
	SpanDays      int `json:"span_days"` // from the oldest dated item to the newest

	SummaryRunes int `json:"summary_runes"` // over items with a summary
	TextRunes    int `json:"text_runes"`    // the longer of summary and content
}

// Survey checks one blog as Run does and measures its feed. To learn
// whether conditional requests really work it asks for the feed again with
// the validators it got, so it costs one request more than Run.
func (c *Checker) Survey(ctx context.Context, in Input) SurveyRecord {
	rec := SurveyRecord{SiteURL: in.SiteURL, FeedURL: in.FeedURL}
	r, loc, err := c.run(ctx, in)
	if err != nil {
		rec.Error = err.Error()
		return rec
	}
	rec.Report = r
	rec.System = r.Generator
	if loc != nil {
		rec.Feed = c.measure(ctx, r, loc)
		// Discovery reads the home page; a given feed skips it.
		if rec.System == model.GeneratorUnknown && in.FeedURL != "" {
			rec.System = c.pageGenerator(ctx, r.InputURL)
		}
	}
	return rec
}

func (c *Checker) pageGenerator(ctx context.Context, site string) model.Generator {
	resp, err := c.Fetch.Get(ctx, fetch.Request{URL: site, Accept: fetch.AcceptHTML, MaxBytes: policy.MaxHTMLBytes})
	if err != nil || resp.Status < 200 || resp.Status > 299 {
		return model.GeneratorUnknown
	}
	base, err := url.Parse(resp.URL)
	if err != nil {
		return model.GeneratorUnknown
	}
	return normalize.DetectGenerator(readPage(resp.Body, base).generator)
}

func (c *Checker) measure(ctx context.Context, r *model.CheckReport, loc *located) *FeedStats {
	resp := loc.resp
	fs := &FeedStats{
		Bytes:       len(resp.Body),
		Charset:     declaredCharset(resp.ContentType, resp.Body),
		Conditional: c.conditional(ctx, resp),
	}

	var host string
	if u, err := url.Parse(r.InputURL); err == nil {
		host = u.Hostname()
	}
	st := normalize.Snapshot(loc.feed, normalize.Blog{Host: host, FeedURL: resp.URL}).Stats
	fs.Items, fs.Valid, fs.NoLink, fs.OffDomain = st.Total, st.Valid, st.NoLink, st.OffDomain
	fs.NoTitle, fs.Duplicate, fs.Undated, fs.Untrusted = st.NoTitle, st.Duplicate, st.Undated, st.Untrusted

	now := c.now()
	perMinute := make(map[int64]int)
	var summaries, texts []int
	var oldest, newest time.Time
	for _, it := range loc.feed.Items {
		if it.DateUnreadable {
			fs.Unreadable++
		}
		s := utf8.RuneCountInString(normalize.PlainText(it.Summary))
		if s > 0 {
			summaries = append(summaries, s)
		}
		texts = append(texts, max(s, utf8.RuneCountInString(normalize.PlainText(it.Content))))

		t := it.Published
		if t == nil {
			t = it.Updated
		}
		if t == nil || t.Before(policy.EarliestDate) {
			continue
		}
		perMinute[t.Unix()/60]++
		if oldest.IsZero() || t.Before(oldest) {
			oldest = *t
		}
		if t.After(newest) {
			newest = *t
		}
		if t.After(now.Add(-policy.StreamWindow)) && !t.After(now.Add(policy.FutureTolerance)) {
			fs.InWindow++
		}
	}
	for _, n := range perMinute {
		fs.MaxSameMinute = max(fs.MaxSameMinute, n)
	}
	if !oldest.IsZero() {
		fs.SpanDays = int(newest.Sub(oldest).Hours() / 24)
	}
	fs.SummaryRunes, fs.TextRunes = median(summaries), median(texts)
	return fs
}

func (c *Checker) conditional(ctx context.Context, resp *fetch.Response) string {
	if resp.ETag == "" && resp.LastModified == "" {
		return "none"
	}
	again, err := c.Fetch.Get(ctx, fetch.Request{
		URL:          resp.URL,
		Accept:       fetch.AcceptFeed,
		ETag:         resp.ETag,
		LastModified: resp.LastModified,
	})
	if err != nil {
		return "error"
	}
	return strconv.Itoa(again.Status)
}

var xmlEncoding = regexp.MustCompile(`^\s*<\?xml[^>]*\sencoding\s*=\s*["']([^"']+)["']`)

// declaredCharset is the character set named by the Content-Type header or,
// failing that, the XML declaration; empty when neither names one.
func declaredCharset(contentType string, body []byte) string {
	if _, params, err := mime.ParseMediaType(contentType); err == nil && params["charset"] != "" {
		return strings.ToLower(params["charset"])
	}
	head := bytes.TrimPrefix(body[:min(len(body), 256)], []byte("\xef\xbb\xbf"))
	if m := xmlEncoding.FindSubmatch(head); m != nil {
		return strings.ToLower(string(m[1]))
	}
	return ""
}

func median(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	slices.Sort(xs)
	return xs[len(xs)/2]
}

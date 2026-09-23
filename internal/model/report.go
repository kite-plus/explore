package model

import "time"

// Severity decides whether a problem fails a check.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// ProblemCode identifies a check problem. Codes are stable; the text shown
// to people is looked up from them per language.
type ProblemCode string

const (
	ProblemFeedNotFound       ProblemCode = "feed_not_found"
	ProblemRobotsDisallowed   ProblemCode = "robots_disallowed"
	ProblemHTTPError          ProblemCode = "http_error"
	ProblemTooLarge           ProblemCode = "too_large"
	ProblemParseError         ProblemCode = "parse_error"
	ProblemLinksOffDomain     ProblemCode = "links_off_domain"
	ProblemLinksElsewhere     ProblemCode = "links_elsewhere"
	ProblemSiteMoved          ProblemCode = "site_moved"
	ProblemNoValidItems       ProblemCode = "no_valid_items"
	ProblemStale              ProblemCode = "stale"
	ProblemSomeLinksOffDomain ProblemCode = "some_links_off_domain"
	ProblemNoDates            ProblemCode = "no_dates"
	ProblemDatesUntrusted     ProblemCode = "dates_untrusted"
	ProblemIncludesNonPosts   ProblemCode = "includes_non_posts"
	ProblemNoConditionalGet   ProblemCode = "no_conditional_get"
	ProblemRedirected         ProblemCode = "redirected"
)

// CheckReport is what a check found. It is stored without hints; hints are
// added per language when the report is shown.
type CheckReport struct {
	InputURL     string      `json:"input_url"`
	FeedURL      string      `json:"feed_url,omitempty"`
	DiscoveredBy string      `json:"discovered_by,omitempty"`
	Format       string      `json:"format,omitempty"`
	Generator    Generator   `json:"generator,omitempty"`
	Title            string      `json:"title,omitempty"`
	Description      string      `json:"description,omitempty"`
	LatestEntryTitle string      `json:"latest_entry_title,omitempty"`
	Language         string      `json:"language,omitempty"`
	HTTP         *CheckHTTP  `json:"http,omitempty"`
	Items        *CheckItems `json:"items,omitempty"`
	Problems     []Problem   `json:"problems"`
	Passed       bool        `json:"passed"`
}

// CheckHTTP describes the response that carried the feed.
type CheckHTTP struct {
	Status       int  `json:"status"`
	ETag         bool `json:"etag"`
	LastModified bool `json:"last_modified"`
}

// CheckItems summarizes the feed's items after normalization.
type CheckItems struct {
	Total             int        `json:"total"`
	Valid             int        `json:"valid"`
	TrustedDates      int        `json:"trusted_dates"`
	LatestPublishedAt *time.Time `json:"latest_published_at,omitempty"`
}

// Problem is one finding of a check. Detail holds language-neutral facts
// such as a status code or a URL.
type Problem struct {
	Code     ProblemCode `json:"code"`
	Severity Severity    `json:"severity"`
	Count    int         `json:"count,omitempty"`
	Detail   string      `json:"detail,omitempty"`
	Hint     string      `json:"hint,omitempty"`
}

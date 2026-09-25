// Package policy holds the numbers behind the crawl and display rules in
// docs/design/architecture.md section 6. They are product rules, not
// configuration: change a value here together with that section.
package policy

import "time"

// Snapshot and entry limits.
const (
	EntriesPerBlog   = 20
	ExcerptMaxRunes  = 140 // includes the trailing ellipsis
	TitleMaxRunes    = 300
	IdentityMaxBytes = 500

	// A post's own categories, kept only as hints for the tagger.
	CategoriesPerEntry = 10
	CategoryMaxRunes   = 50

	// The tagger asks about at most this many entries a minute, which caps
	// the model's bill when a cleared cache has to be tagged again.
	TagsPerMinute = 20

	// More items than this sharing one minute marks their dates untrusted.
	SameMinuteLimit = 5
)

// EarliestDate is the first plausible publish date. Anything older,
// including the zero dates some generators emit, counts as undated.
var EarliestDate = time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC)

// Stream rules.
const (
	StreamPerBlogPerDay = 3
	FutureTolerance     = time.Hour
)

// Inclusion and health rules.
const (
	ActiveWithin     = 365 * 24 * time.Hour
	UnhealthyAfter   = 7 * 24 * time.Hour
	GoneRemovalAfter = 7 * 24 * time.Hour

	// Share of linked items above which off-domain links fail a check.
	OffDomainFailShare = 0.5
)

// Fetch rules.
const (
	FetchInterval    = 60 * time.Minute
	MaxFetchInterval = 6 * time.Hour
	MaxBackoff       = 24 * time.Hour
	FetchLease       = 10 * time.Minute

	ConnectTimeout = 5 * time.Second
	TLSTimeout     = 5 * time.Second
	HeaderTimeout  = 10 * time.Second
	FetchTimeout   = 30 * time.Second

	MaxFeedBytes   = 5 << 20
	MaxHTMLBytes   = 1 << 20
	MaxRobotsBytes = 500 << 10 // the minimum RFC 9309 asks crawlers to parse
	MaxRedirects   = 5
	RobotsTTL      = 24 * time.Hour

	LinkCheckInterval   = 24 * time.Hour
	LinkRetryInterval   = 6 * time.Hour
	LinkCheckLease      = 2 * time.Minute
	LinkCheckTimeout    = 10 * time.Second
	LinkChecksPerMinute = 8

	// An article page is read, head only, where its feed gives no image or
	// cuts the excerpt short, or to learn a post its sitemap lists. A round
	// reads at most one page per blog, and up to PageChecksPerRound of each
	// kind.
	PageHeadBytes      = 256 << 10
	PageCheckTimeout   = 12 * time.Second
	PageCheckLease     = 2 * time.Minute
	PageRetryInterval  = 6 * time.Hour
	PageRoundEvery     = 20 * time.Second
	PageChecksPerRound = 30

	// A blog's sitemap brings in the posts its feed no longer carries.
	SitemapCheckEvery = 24 * time.Hour
	SitemapMissRetry  = 7 * 24 * time.Hour
	SitemapLease      = 10 * time.Minute
	SitemapsPerMinute = 4
	SitemapMaxBytes   = 10 << 20 // one sitemap file, after gunzip
	SitemapMaxFiles   = 20       // files read for one blog
	SitemapMaxEntries = 50_000   // one file, the protocol's own limit
	SitemapMaxListed  = 100_000  // addresses read for one blog, all files together
	SitemapMaxPosts   = 1000     // post addresses kept per blog
	// A post read from its own page is known to answer, so its link is
	// checked rarely.
	SitemapLinkCheckInterval = 30 * 24 * time.Hour
)

// UserAgentToken is the product token robots.txt rules address.
const UserAgentToken = "KiteExplore"

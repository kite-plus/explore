// Package model defines the domain types shared by the store, the API and
// the worker.
package model

import "time"

// Generator is the blog system a feed was produced by. It only feeds
// statistics and check hints; it never changes how a feed is handled.
type Generator string

const (
	GeneratorWordPress Generator = "wordpress"
	GeneratorHalo      Generator = "halo"
	GeneratorHugo      Generator = "hugo"
	GeneratorHexo      Generator = "hexo"
	GeneratorTypecho   Generator = "typecho"
	GeneratorJekyll    Generator = "jekyll"
	GeneratorGhost     Generator = "ghost"
	GeneratorKite      Generator = "kite"
	GeneratorOther     Generator = "other"
	GeneratorUnknown   Generator = "unknown"
)

// BlogStatus is set by maintainers only; health is derived from fetch state.
type BlogStatus string

const (
	BlogActive BlogStatus = "active"
	BlogPaused BlogStatus = "paused"
)

// Blog is a listed blog together with its fetch state.
type Blog struct {
	ID           int64
	Host         string
	Name         string
	SiteURL      string
	FeedURL      string
	Language     string
	Generator    Generator
	Status       BlogStatus
	StatusNote   string
	ShowExcerpt  bool
	ExtraDomains []string

	ETag                string
	LastModified        string
	BodyHash            []byte
	FetchInterval       time.Duration
	NextFetchAt         time.Time
	LastFetchedAt       *time.Time
	LastSucceededAt     *time.Time
	ConsecutiveFailures int
	LastError           string
	GoneSince           *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Entry is one item of a blog's current feed. It never carries content.
type Entry struct {
	ID          int64
	BlogID      int64
	Identity    string
	URL         string
	Title       string
	Excerpt     string // empty when hidden by the author or absent
	PublishedAt *time.Time
	DateTrusted bool
}

// SubmissionStatus tracks a submission through review.
type SubmissionStatus string

const (
	SubmissionPending  SubmissionStatus = "pending"
	SubmissionApproved SubmissionStatus = "approved"
	SubmissionRejected SubmissionStatus = "rejected"
)

// Submission is a request to list a blog.
type Submission struct {
	ID         string
	Host       string
	SiteURL    string
	FeedURL    string
	Note       string
	Report     CheckReport
	Status     SubmissionStatus
	ReviewNote string
	ReviewedBy string
	ReviewedAt *time.Time
	BlogID     *int64
	CreatedAt  time.Time
}

// ExclusionReason records why a host may not be listed again.
type ExclusionReason string

const (
	ExcludedOptOut  ExclusionReason = "opt_out"
	ExcludedBlocked ExclusionReason = "blocked"
)

// ExcludedHost is a host that left or was blocked.
type ExcludedHost struct {
	Host      string
	Reason    ExclusionReason
	Note      string
	CreatedAt time.Time
}

// Package check finds a blog's feed and reports whether Explore can use it.
// It needs no database, so authors, the submission API and the E0 survey
// all run the same code.
package check

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/kite-plus/explore/internal/feed"
	"github.com/kite-plus/explore/internal/fetch"
	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/normalize"
	"github.com/kite-plus/explore/internal/policy"
)

// ErrInvalidURL means the input is not a usable http or https URL.
var ErrInvalidURL = errors.New("not an http or https URL")

// Checker runs checks.
type Checker struct {
	Fetch *fetch.Client
	Now   func() time.Time
}

// Input is what an author gives: the blog's address and, optionally, its
// feed address, which skips discovery.
type Input struct {
	SiteURL string
	FeedURL string
}

// ParseURL accepts an address as people type it: a missing scheme means
// https, and the fragment is dropped.
func ParseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrInvalidURL
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return nil, ErrInvalidURL
	}
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	if u.Path == "" {
		u.Path = "/"
	}
	return u, nil
}

// located is a feed that was found and parsed.
type located struct {
	requested string
	resp      *fetch.Response
	feed      *feed.Feed
	by        string
}

// Run checks one blog. The error is only for input that is not a URL;
// everything the check finds is in the report.
func (c *Checker) Run(ctx context.Context, in Input) (*model.CheckReport, error) {
	r, _, err := c.run(ctx, in)
	return r, err
}

// run is Run that also returns the feed it found, if any.
func (c *Checker) run(ctx context.Context, in Input) (*model.CheckReport, *located, error) {
	site, err := ParseURL(in.SiteURL)
	if err != nil {
		return nil, nil, err
	}
	r := &model.CheckReport{InputURL: site.String(), Generator: model.GeneratorUnknown, Problems: []model.Problem{}}

	var loc *located
	var generator string
	var prob *model.Problem
	if in.FeedURL != "" {
		fu, err := ParseURL(in.FeedURL)
		if err != nil {
			return nil, nil, err
		}
		loc, prob = c.tryFeed(ctx, fu.String(), "given", true)
	} else {
		loc, generator, prob = c.discover(ctx, site)
	}
	if generator != "" {
		r.Generator = normalize.DetectGenerator(generator)
	}
	if loc == nil {
		if prob == nil {
			prob = &model.Problem{Code: model.ProblemFeedNotFound, Severity: model.SeverityError}
		}
		add(r, *prob)
		return finish(r), nil, nil
	}

	c.describe(ctx, r, site, loc)
	return finish(r), loc, nil
}

// discover finds the feed of a site: the input itself, a feed the page
// links to, or the default address of a common blog system.
func (c *Checker) discover(ctx context.Context, site *url.URL) (*located, string, *model.Problem) {
	resp, err := c.Fetch.Get(ctx, fetch.Request{URL: site.String(), Accept: fetch.AcceptFeed})
	if err != nil {
		p := problemFrom(err)
		return nil, "", &p
	}
	if resp.Status < 200 || resp.Status > 299 {
		return nil, "", &model.Problem{Code: model.ProblemHTTPError, Severity: model.SeverityError, Detail: fmt.Sprintf("HTTP %d", resp.Status)}
	}
	f, err := feed.Parse(resp.Body)
	if err == nil {
		return &located{requested: site.String(), resp: resp, feed: f, by: "direct"}, "", nil
	}
	if !errors.Is(err, feed.ErrNotFeed) {
		return nil, "", &model.Problem{Code: model.ProblemParseError, Severity: model.SeverityError, Detail: err.Error()}
	}

	base, err := url.Parse(resp.URL)
	if err != nil {
		base = site
	}
	pg := readPage(resp.Body, base)
	tried := make(map[string]bool)
	var first *model.Problem
	remember := func(p *model.Problem) {
		// A robots.txt refusal explains a missing feed better than a 404.
		if p != nil && (first == nil || p.Code == model.ProblemRobotsDisallowed) {
			first = p
		}
	}

	for _, link := range pg.feeds {
		if tried[link] {
			continue
		}
		tried[link] = true
		if loc, p := c.tryFeed(ctx, link, "autodiscovery", false); loc != nil {
			return loc, pg.generator, nil
		} else if p != nil && p.Code == model.ProblemRobotsDisallowed {
			remember(p)
		}
	}
	for _, path := range candidatePaths {
		ref, _ := url.Parse(path)
		link := base.ResolveReference(ref).String()
		if tried[link] {
			continue
		}
		tried[link] = true
		if loc, p := c.tryFeed(ctx, link, "candidate", false); loc != nil {
			return loc, pg.generator, nil
		} else if p != nil && p.Code == model.ProblemRobotsDisallowed {
			remember(p)
		}
	}
	return nil, pg.generator, first
}

// tryFeed fetches and parses one address. When the address was given by
// the author, anything that is not a feed is a problem to report; during
// discovery it only means the search goes on.
func (c *Checker) tryFeed(ctx context.Context, link, by string, given bool) (*located, *model.Problem) {
	resp, err := c.Fetch.Get(ctx, fetch.Request{URL: link, Accept: fetch.AcceptFeed})
	if err != nil {
		p := problemFrom(err)
		return nil, &p
	}
	if resp.Status < 200 || resp.Status > 299 {
		if !given {
			return nil, nil
		}
		return nil, &model.Problem{Code: model.ProblemHTTPError, Severity: model.SeverityError, Detail: fmt.Sprintf("HTTP %d", resp.Status)}
	}
	f, err := feed.Parse(resp.Body)
	if err != nil {
		if !given {
			return nil, nil
		}
		return nil, &model.Problem{Code: model.ProblemParseError, Severity: model.SeverityError, Detail: err.Error()}
	}
	return &located{requested: link, resp: resp, feed: f, by: by}, nil
}

// describe fills the report from a located feed.
func (c *Checker) describe(ctx context.Context, r *model.CheckReport, site *url.URL, loc *located) {
	r.FeedURL = loc.requested
	if loc.resp.PermanentRedirect && loc.resp.URL != loc.requested {
		r.FeedURL = loc.resp.URL
		add(r, model.Problem{Code: model.ProblemRedirected, Severity: model.SeverityInfo, Detail: loc.resp.URL})
	}
	r.DiscoveredBy = loc.by
	r.Format = loc.feed.Format
	r.Title = normalize.Truncate(normalize.PlainText(loc.feed.Title), 100)
	r.Language = loc.feed.Language
	if g := normalize.DetectGenerator(loc.feed.Generator); g != model.GeneratorUnknown {
		r.Generator = g
	}
	r.HTTP = &model.CheckHTTP{Status: loc.resp.Status, ETag: loc.resp.ETag != "", LastModified: loc.resp.LastModified != ""}
	if !r.HTTP.ETag && !r.HTTP.LastModified {
		add(r, model.Problem{Code: model.ProblemNoConditionalGet, Severity: model.SeverityInfo})
	}

	res := normalize.Snapshot(loc.feed, normalize.Blog{Host: site.Hostname(), ShowExcerpt: true, FeedURL: loc.resp.URL})
	st := res.Stats
	latest := c.latest(res.Entries)
	r.Items = &model.CheckItems{Total: st.Total, Valid: st.Valid, TrustedDates: st.Trusted, LatestPublishedAt: latest}

	if linked := st.Total - st.NoLink; st.OffDomain > 0 && linked > 0 {
		p := model.Problem{Code: model.ProblemSomeLinksOffDomain, Severity: model.SeverityWarning, Count: st.OffDomain, Detail: topHost(st.OffDomainHosts)}
		if float64(st.OffDomain)/float64(linked) > policy.OffDomainFailShare {
			p.Severity = model.SeverityError
			p.Code = model.ProblemLinksElsewhere
			// A feed that names another site as its own home was built with the
			// wrong site address; one that names this site links out on purpose.
			if home := siteHost(loc.feed.SiteURL, loc.resp.URL); home != "" && !normalize.SameSite(home, site.Hostname(), nil) {
				p.Code = model.ProblemLinksOffDomain
			}
		}
		add(r, p)
	}
	if st.Valid == 0 {
		add(r, model.Problem{Code: model.ProblemNoValidItems, Severity: model.SeverityError})
	} else if latest == nil || latest.Before(c.now().Add(-policy.ActiveWithin)) {
		p := model.Problem{Code: model.ProblemStale, Severity: model.SeverityError}
		if latest != nil {
			p.Detail = latest.Format(time.DateOnly)
		}
		add(r, p)
	}
	if st.Undated > 0 {
		add(r, model.Problem{Code: model.ProblemNoDates, Severity: model.SeverityWarning, Count: st.Undated})
	}
	if st.Untrusted > 0 {
		add(r, model.Problem{Code: model.ProblemDatesUntrusted, Severity: model.SeverityWarning, Count: st.Untrusted})
	}
	if alt := c.postsFeed(ctx, r, res.Entries); alt != "" {
		add(r, model.Problem{Code: model.ProblemIncludesNonPosts, Severity: model.SeverityWarning, Detail: alt})
	}
}

// latest is the newest trusted date that is not in the future: a post
// dated years ahead says nothing about whether the blog is alive.
func (c *Checker) latest(entries []model.Entry) *time.Time {
	limit := c.now().Add(policy.FutureTolerance)
	for _, e := range entries {
		if e.DateTrusted && !e.PublishedAt.After(limit) {
			t := *e.PublishedAt
			return &t
		}
	}
	return nil
}

// postsFeed spots a Hugo home feed that also lists standalone pages and
// returns the posts section's feed when there is one.
func (c *Checker) postsFeed(ctx context.Context, r *model.CheckReport, entries []model.Entry) string {
	if r.Generator != model.GeneratorHugo {
		return ""
	}
	fu, err := url.Parse(r.FeedURL)
	if err != nil || fu.Path != "/index.xml" {
		return ""
	}
	standalone := false
	for _, e := range entries {
		if u, err := url.Parse(e.URL); err == nil {
			if p := strings.Trim(u.Path, "/"); p != "" && !strings.Contains(p, "/") {
				standalone = true
				break
			}
		}
	}
	if !standalone {
		return ""
	}
	alt := fu.ResolveReference(&url.URL{Path: "/posts/index.xml"}).String()
	if loc, _ := c.tryFeed(ctx, alt, "candidate", false); loc != nil {
		return alt
	}
	return ""
}

// siteHost is the host of the home page a feed declares, resolved against
// the feed's own address.
func siteHost(siteURL, feedURL string) string {
	if strings.TrimSpace(siteURL) == "" {
		return ""
	}
	base, err := url.Parse(feedURL)
	if err != nil {
		return ""
	}
	u, err := base.Parse(strings.TrimSpace(siteURL))
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// topHost is the most common host, ties broken by name for stable output.
func topHost(hosts map[string]int) string {
	best, n := "", 0
	for h, c := range hosts {
		if c > n || (c == n && h < best) {
			best, n = h, c
		}
	}
	return best
}

func (c *Checker) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// problemFrom turns a fetch error into a report problem. The detail keeps
// the technical reason without the Go request wrapper around it.
func problemFrom(err error) model.Problem {
	switch {
	case errors.Is(err, fetch.ErrRobotsDisallowed):
		return model.Problem{Code: model.ProblemRobotsDisallowed, Severity: model.SeverityError}
	case errors.Is(err, fetch.ErrTooLarge):
		return model.Problem{Code: model.ProblemTooLarge, Severity: model.SeverityError}
	case errors.Is(err, fetch.ErrBadURL):
		return model.Problem{Code: model.ProblemHTTPError, Severity: model.SeverityError, Detail: "address is not public"}
	}
	var be *fetch.BlockedError
	if errors.As(err, &be) {
		return model.Problem{Code: model.ProblemHTTPError, Severity: model.SeverityError, Detail: be.Error()}
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	return model.Problem{Code: model.ProblemHTTPError, Severity: model.SeverityError, Detail: err.Error()}
}

func add(r *model.CheckReport, p model.Problem) {
	r.Problems = append(r.Problems, p)
}

// finish decides the outcome: any error-level problem fails the check.
func finish(r *model.CheckReport) *model.CheckReport {
	r.Passed = true
	for _, p := range r.Problems {
		if p.Severity == model.SeverityError {
			r.Passed = false
		}
	}
	return r
}

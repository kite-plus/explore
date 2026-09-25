package worker

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/kite-plus/explore/internal/fetch"
	"github.com/kite-plus/explore/internal/policy"
	"github.com/kite-plus/explore/internal/sitemap"
	"github.com/kite-plus/explore/internal/store"
)

const acceptSitemap = "application/xml, text/xml;q=0.9, */*;q=0.1"

// sitemapDefaults are where sitemaps usually are when robots.txt names none.
var sitemapDefaults = []string{"/sitemap.xml", "/sitemap_index.xml", "/wp-sitemap.xml"}

// pageLoop reads a round of article pages every policy.PageRoundEvery, and
// the due sitemaps once a minute, until ctx is canceled.
func (w *Worker) pageLoop(ctx context.Context) {
	ticker := time.NewTicker(policy.PageRoundEvery)
	defer ticker.Stop()
	for {
		w.checkSitemaps(ctx)
		w.PageOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// checkSitemaps reads the sitemaps of a few due blogs once a minute.
func (w *Worker) checkSitemaps(ctx context.Context) {
	now := w.now()
	if !w.lastSitemapCheck.IsZero() && now.Sub(w.lastSitemapCheck) < time.Minute {
		return
	}
	w.lastSitemapCheck = now
	w.SitemapOnce(ctx)
}

// SitemapOnce reads the sitemaps of the blogs due now and returns how many
// blogs it took.
func (w *Worker) SitemapOnce(ctx context.Context) int {
	jobs, err := w.Store.ClaimSitemaps(ctx, policy.SitemapsPerMinute)
	if err != nil {
		if ctx.Err() == nil {
			w.log().Error("claiming sitemaps failed", "error", err)
		}
		return 0
	}
	for _, job := range jobs {
		w.readSitemap(ctx, job)
	}
	return len(jobs)
}

// readSitemap learns a blog's post addresses from its sitemap. Posts are
// told from its other pages by what the feed's links look like; the pages
// are read later, a few at a time (pages.go).
func (w *Worker) readSitemap(ctx context.Context, job store.SitemapJob) {
	log := w.log().With("host", job.Host)
	site, err := url.Parse(job.SiteURL)
	if err != nil {
		return
	}
	listed, found := w.collectSitemaps(ctx, site)
	if !found {
		if err := w.Store.PostponeSitemap(ctx, job.BlogID, w.now().Add(policy.SitemapMissRetry)); err != nil && ctx.Err() == nil {
			log.Error("postponing the sitemap failed", "error", err)
		}
		return
	}
	links, err := w.Store.FeedEntryURLs(ctx, job.BlogID)
	if err != nil {
		if ctx.Err() == nil {
			log.Error("reading feed links failed", "error", err)
		}
		return
	}
	var examples []*url.URL
	for _, l := range links {
		if u, err := url.Parse(l); err == nil {
			examples = append(examples, u)
		}
	}
	posts := postAddresses(listed, site, job.FeedURL, sitemap.Learn(examples))
	if err := w.Store.SyncSitemap(ctx, job.BlogID, posts, w.now().Add(policy.SitemapCheckEvery)); err != nil && ctx.Err() == nil {
		log.Error("storing the sitemap failed", "error", err)
		return
	}
	log.Info("sitemap read", "listed", len(listed), "posts", len(posts))
}

// collectSitemaps reads the sitemaps robots.txt names, or failing that the
// first of the usual places that has one, following index files up to
// policy.SitemapMaxFiles files in all. Only files on the blog's own site
// count.
func (w *Worker) collectSitemaps(ctx context.Context, site *url.URL) ([]sitemap.URL, bool) {
	var queue []string
	if named, err := w.Fetch.Sitemaps(ctx, site); err == nil {
		for _, n := range named {
			if u, err := url.Parse(n); err == nil && sameSite(u.Host, site.Host) {
				queue = append(queue, u.String())
			}
		}
	}
	defaults := map[string]bool{}
	if len(queue) == 0 {
		for _, p := range sitemapDefaults {
			d := site.ResolveReference(&url.URL{Path: p}).String()
			defaults[d] = true
			queue = append(queue, d)
		}
	}

	var urls []sitemap.URL
	seen := map[string]bool{}
	files := 0
	for len(queue) > 0 && files < policy.SitemapMaxFiles && len(urls) < policy.SitemapMaxListed {
		loc := queue[0]
		queue = queue[1:]
		if seen[loc] {
			continue
		}
		seen[loc] = true
		doc, ok := w.readSitemapFile(ctx, loc)
		if !ok {
			continue
		}
		files++
		urls = append(urls, doc.URLs...)
		for _, child := range doc.Children {
			if u, err := url.Parse(child); err == nil && sameSite(u.Host, site.Host) {
				queue = append(queue, u.String())
			}
		}
		if len(defaults) > 0 {
			// One of the usual places answered; the others would mostly
			// repeat it.
			queue = slices.DeleteFunc(queue, func(q string) bool { return defaults[q] })
			clear(defaults)
		}
	}
	return urls, files > 0
}

func (w *Worker) readSitemapFile(ctx context.Context, loc string) (sitemap.Doc, bool) {
	resp, err := w.Fetch.Get(ctx, fetch.Request{URL: loc, Accept: acceptSitemap, MaxBytes: policy.SitemapMaxBytes})
	if err != nil || resp.Status != http.StatusOK {
		return sitemap.Doc{}, false
	}
	doc, err := sitemap.Parse(resp.Body, policy.SitemapMaxBytes, policy.SitemapMaxEntries)
	if err != nil {
		return sitemap.Doc{}, false
	}
	return doc, true
}

// postAddresses keeps the listed addresses that look like the blog's posts,
// on its own site, newest first and at most policy.SitemapMaxPosts.
func postAddresses(listed []sitemap.URL, site *url.URL, feedURL string, patterns sitemap.Patterns) []store.SitemapURL {
	var out []store.SitemapURL
	seen := map[string]bool{}
	for _, l := range listed {
		u, err := url.Parse(strings.TrimSpace(l.Loc))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !sameSite(u.Host, site.Host) {
			continue
		}
		u.Fragment, u.RawFragment = "", ""
		if strings.Trim(u.Path, "/") == "" && u.RawQuery == "" {
			continue
		}
		if !patterns.Match(u) {
			continue
		}
		s := u.String()
		if len(s) > 2000 || s == feedURL || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, store.SitemapURL{URL: s, LastMod: l.LastMod})
	}
	slices.SortStableFunc(out, func(a, b store.SitemapURL) int {
		switch {
		case a.LastMod == nil && b.LastMod == nil:
			return 0
		case a.LastMod == nil:
			return 1
		case b.LastMod == nil:
			return -1
		default:
			return b.LastMod.Compare(*a.LastMod)
		}
	})
	if len(out) > policy.SitemapMaxPosts {
		out = out[:policy.SitemapMaxPosts]
	}
	return out
}

// sameSite reports whether two hosts are the same site, with or without www.
func sameSite(a, b string) bool {
	trim := func(h string) string { return strings.TrimPrefix(strings.ToLower(h), "www.") }
	return trim(a) == trim(b)
}

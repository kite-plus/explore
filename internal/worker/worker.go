// Package worker keeps every listed blog's entries in step with its feed,
// following docs/design/worker.md.
package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/kite-plus/explore/internal/feed"
	"github.com/kite-plus/explore/internal/fetch"
	"github.com/kite-plus/explore/internal/normalize"
	"github.com/kite-plus/explore/internal/policy"
	"github.com/kite-plus/explore/internal/store"
)

// maintenanceEvery is how often the daily maintenance pass runs.
const maintenanceEvery = 24 * time.Hour

// submissionRetention is how long reviewed submissions are kept; see
// docs/design/data-model.md section 5.
const submissionRetention = 90 * 24 * time.Hour

// Worker fetches due blogs. Zero values of the optional fields get sensible
// defaults.
type Worker struct {
	Store       *store.Store
	Fetch       *fetch.Client
	Log         *slog.Logger
	Concurrency int
	PollEvery   time.Duration
	Now         func() time.Time
	Rand        func() float64

	lastMaintenance time.Time
}

// Run polls until ctx is canceled.
func (w *Worker) Run(ctx context.Context) error {
	poll := w.PollEvery
	if poll == 0 {
		poll = 30 * time.Second
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		// A full batch means more are waiting, so keep going without a pause.
		for {
			n, err := w.RunOnce(ctx)
			if err != nil && ctx.Err() == nil {
				w.log().Error("claiming due blogs failed", "error", err)
			}
			if err != nil || n < w.concurrency() {
				break
			}
		}
		w.maintain(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// RunOnce claims one batch of due blogs and fetches them concurrently. It
// returns how many were claimed.
func (w *Worker) RunOnce(ctx context.Context) (int, error) {
	claimed, err := w.Store.ClaimDue(ctx, w.concurrency(), policy.FetchLease)
	if err != nil {
		return 0, err
	}
	var wg sync.WaitGroup
	for _, b := range claimed {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.process(ctx, b)
		}()
	}
	wg.Wait()
	return len(claimed), nil
}

func (w *Worker) process(ctx context.Context, b store.Claimed) {
	start := w.now()
	log := w.log().With("host", b.Host, "feed", b.FeedURL)

	// Without entries the cache was emptied or never filled; asking the
	// server for changes would keep it empty.
	req := fetch.Request{URL: b.FeedURL, Accept: fetch.AcceptFeed}
	if b.HasEntries {
		req.ETag, req.LastModified = b.ETag, b.LastModified
	}
	resp, err := w.Fetch.Get(ctx, req)
	if err != nil {
		// Only a group naming KiteExplore is the author opting out; the group
		// for every crawler is often an SEO template aimed at search engines.
		var refused *fetch.RobotsError
		gone := errors.As(err, &refused) && refused.Explicit
		w.fail(ctx, log, b, err.Error(), 0, gone)
		return
	}

	switch {
	case resp.Status == http.StatusNotModified && b.HasEntries:
		w.unchanged(ctx, log, b, resp, start)
		return
	case resp.Status == http.StatusGone:
		w.fail(ctx, log, b, "HTTP 410", 0, true)
		return
	case resp.Status < 200 || resp.Status > 299:
		w.fail(ctx, log, b, fmt.Sprintf("HTTP %d", resp.Status), resp.RetryAfter, false)
		return
	}

	sum := sha256.Sum256(resp.Body)
	if b.HasEntries && bytes.Equal(sum[:], b.BodyHash) {
		w.unchanged(ctx, log, b, resp, start)
		return
	}

	f, err := feed.Parse(resp.Body)
	if err != nil {
		// A broken feed is a failure, never an empty snapshot.
		w.fail(ctx, log, b, "parse: "+err.Error(), 0, false)
		return
	}
	res := normalize.Snapshot(f, normalize.Blog{Host: b.Host, ExtraDomains: b.ExtraDomains, ShowExcerpt: b.ShowExcerpt, FeedURL: resp.URL})

	st := store.FetchState{
		ETag:          resp.ETag,
		LastModified:  resp.LastModified,
		BodyHash:      sum[:],
		FetchInterval: nextInterval(b.FetchInterval, true),
		Generator:     normalize.DetectGenerator(f.Generator),
	}
	st.NextFetchAt = w.now().Add(jitter(st.FetchInterval, w.rand()))
	if resp.PermanentRedirect && resp.URL != b.FeedURL {
		if moved, err := url.Parse(resp.URL); err == nil && normalize.SameSite(moved.Hostname(), b.Host, b.ExtraDomains) {
			st.FeedURL = resp.URL
		} else {
			log.Warn("feed moved off the blog's domain; left for a maintainer", "to", resp.URL)
		}
	}

	if err := w.Store.SyncSnapshot(ctx, b.ID, res.Entries, st); err != nil {
		log.Error("storing the snapshot failed", "error", err)
		return
	}
	log.Info("fetched", "outcome", "changed", "status", resp.Status, "bytes", len(resp.Body),
		"entries", len(res.Entries), "off_domain", res.Stats.OffDomain, "no_link", res.Stats.NoLink,
		"untrusted", res.Stats.Untrusted, "duration", w.now().Sub(start))
}

func (w *Worker) unchanged(ctx context.Context, log *slog.Logger, b store.Claimed, resp *fetch.Response, start time.Time) {
	interval := nextInterval(b.FetchInterval, false)
	st := store.FetchState{ETag: resp.ETag, LastModified: resp.LastModified, FetchInterval: interval, NextFetchAt: w.now().Add(jitter(interval, w.rand()))}
	if err := w.Store.RecordUnchanged(ctx, b.ID, st); err != nil {
		log.Error("recording an unchanged fetch failed", "error", err)
		return
	}
	log.Info("fetched", "outcome", "unchanged", "status", resp.Status, "duration", w.now().Sub(start))
}

func (w *Worker) fail(ctx context.Context, log *slog.Logger, b store.Claimed, reason string, retryAfter time.Duration, gone bool) {
	delay := backoff(b.ConsecutiveFailures+1, retryAfter)
	f := store.Failure{Error: reason, NextFetchAt: w.now().Add(jitter(delay, w.rand())), Gone: gone}
	if err := w.Store.RecordFailure(ctx, b.ID, f); err != nil {
		log.Error("recording a failed fetch failed", "error", err)
		return
	}
	log.Warn("fetch failed", "error", reason, "gone", gone, "failures", b.ConsecutiveFailures+1, "retry_in", delay)
}

// maintain runs the daily pass. Across several workers, the advisory lock
// in the store lets only one of them do the work.
func (w *Worker) maintain(ctx context.Context) {
	now := w.now()
	if !w.lastMaintenance.IsZero() && now.Sub(w.lastMaintenance) < maintenanceEvery {
		return
	}
	m, err := w.Store.Maintain(ctx, now.Add(-policy.GoneRemovalAfter), now.Add(-submissionRetention))
	if err != nil {
		w.log().Error("maintenance failed", "error", err)
		return
	}
	w.lastMaintenance = now
	if m.Ran {
		w.log().Info("maintenance", "removed", m.Removed, "purged_submissions", m.Purged)
	}
}

func (w *Worker) concurrency() int {
	if w.Concurrency > 0 {
		return w.Concurrency
	}
	return 16
}

func (w *Worker) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return time.Now()
}

func (w *Worker) rand() float64 {
	if w.Rand != nil {
		return w.Rand()
	}
	return rand.Float64()
}

func (w *Worker) log() *slog.Logger {
	if w.Log != nil {
		return w.Log
	}
	return slog.Default()
}

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
	"os"
	"sync"
	"time"

	"github.com/kite-plus/explore/internal/feed"
	"github.com/kite-plus/explore/internal/fetch"
	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/normalize"
	"github.com/kite-plus/explore/internal/policy"
	"github.com/kite-plus/explore/internal/store"
)

// maintenanceEvery is how often the daily maintenance pass runs.
const maintenanceEvery = 24 * time.Hour

// submissionRetention is how long reviewed submissions are kept; see
// docs/design/data-model.md section 5.
const submissionRetention = 90 * 24 * time.Hour

// Tagger names the tags of one entry; see internal/tagger. An error means the
// entry waits for a later round.
type Tagger interface {
	Tag(ctx context.Context, job store.TagJob) ([]string, error)
}

// Worker fetches due blogs and tags new entries. Zero values of the optional
// fields get sensible defaults; a nil Tagger leaves entries untagged.
type Worker struct {
	Store       *store.Store
	Fetch       *fetch.Client
	Tagger      Tagger
	Log         *slog.Logger
	Concurrency int
	PollEvery   time.Duration
	Now         func() time.Time
	Rand        func() float64

	lastMaintenance time.Time
	lastLinkCheck   time.Time
	tagFailures     int
	nextTagAt       time.Time
}

// Run polls until ctx is canceled.
func (w *Worker) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	defer wg.Wait()
	workerID := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	wg.Go(func() { w.heartbeatLoop(ctx, workerID) })
	if w.Tagger != nil {
		wg.Go(func() { w.tagLoop(ctx) })
	}
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
		w.checkLinks(ctx)
		w.maintain(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (w *Worker) heartbeatLoop(ctx context.Context, workerID string) {
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := w.Store.RemoveWorkerHeartbeat(cleanup, workerID); err != nil {
			w.log().Warn("removing worker heartbeat failed", "error", err)
		}
	}()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		if err := w.Store.RecordWorkerHeartbeat(ctx, workerID); err != nil && ctx.Err() == nil {
			w.log().Error("recording worker heartbeat failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) checkLinks(ctx context.Context) {
	now := w.now()
	if !w.lastLinkCheck.IsZero() && now.Sub(w.lastLinkCheck) < time.Minute {
		return
	}
	w.lastLinkCheck = now
	w.LinkOnce(ctx)
}

func (w *Worker) LinkOnce(ctx context.Context) int {
	jobs, err := w.Store.ClaimLinks(ctx, policy.LinkChecksPerMinute)
	if err != nil {
		if ctx.Err() == nil {
			w.log().Error("claiming article links failed", "error", err)
		}
		return 0
	}
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Go(func() {
			code, err := w.Fetch.Probe(ctx, job.URL)
			status := linkStatus(code)
			interval := policy.LinkCheckInterval
			if status == model.LinkUnknown {
				interval = policy.LinkRetryInterval
			}
			if err := w.Store.RecordLinkStatus(ctx, job, status, interval); err != nil && ctx.Err() == nil {
				w.log().Error("recording article link status failed", "entry", job.EntryID, "error", err)
			}
			if err != nil && ctx.Err() == nil {
				w.log().Warn("article link check was inconclusive", "entry", job.EntryID, "error", err)
			}
		})
	}
	wg.Wait()
	return len(jobs)
}

func linkStatus(code int) model.LinkStatus {
	switch {
	case code >= 200 && code < 300:
		return model.LinkAvailable
	case code == http.StatusNotFound || code == http.StatusGone:
		return model.LinkUnavailable
	default:
		return model.LinkUnknown
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
	outcome := "failed"
	reason := "抓取任务意外结束"
	var httpStatus *int
	var entryCount *int
	defer func() {
		finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := w.Store.FinishFetchAttempt(finishCtx, b.AttemptID, outcome, httpStatus, entryCount, reason); err != nil {
			log.Error("recording fetch attempt failed", "error", err)
		}
	}()

	// Without entries the cache was emptied or never filled; asking the
	// server for changes would keep it empty.
	req := fetch.Request{URL: b.FeedURL, Accept: fetch.AcceptFeed}
	if b.HasEntries {
		req.ETag, req.LastModified = b.ETag, b.LastModified
	}
	resp, err := w.Fetch.Get(ctx, req)
	if err != nil {
		reason = err.Error()
		// Only a group naming KiteExplore is the author opting out; the group
		// for every crawler is often an SEO template aimed at search engines.
		var refused *fetch.RobotsError
		gone := errors.As(err, &refused) && refused.Explicit
		w.fail(ctx, log, b, err.Error(), 0, gone)
		return
	}
	status := resp.Status
	httpStatus = &status

	switch {
	case resp.Status == http.StatusNotModified && b.HasEntries:
		if err := w.unchanged(ctx, log, b, resp, start); err != nil {
			reason = "recording unchanged fetch: " + err.Error()
			return
		}
		outcome, reason = "unchanged", ""
		return
	case resp.Status == http.StatusGone:
		reason = "HTTP 410"
		w.fail(ctx, log, b, "HTTP 410", 0, true)
		return
	case resp.Status < 200 || resp.Status > 299:
		reason = fmt.Sprintf("HTTP %d", resp.Status)
		w.fail(ctx, log, b, reason, resp.RetryAfter, false)
		return
	}

	sum := sha256.Sum256(resp.Body)
	if b.HasEntries && bytes.Equal(sum[:], b.BodyHash) {
		if err := w.unchanged(ctx, log, b, resp, start); err != nil {
			reason = "recording unchanged fetch: " + err.Error()
			return
		}
		outcome, reason = "unchanged", ""
		return
	}

	f, err := feed.Parse(resp.Body)
	if err != nil {
		// A broken feed is a failure, never an empty snapshot.
		reason = "parse: " + err.Error()
		w.fail(ctx, log, b, reason, 0, false)
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
		reason = "storing snapshot: " + err.Error()
		log.Error("storing the snapshot failed", "error", err)
		return
	}
	outcome, reason = "changed", ""
	entries := len(res.Entries)
	entryCount = &entries
	w.refreshDescription(ctx, b, f.Description)
	log.Info("fetched", "outcome", "changed", "status", resp.Status, "bytes", len(resp.Body),
		"entries", len(res.Entries), "off_domain", res.Stats.OffDomain, "no_link", res.Stats.NoLink,
		"untrusted", res.Stats.Untrusted, "duration", w.now().Sub(start))
}

func (w *Worker) unchanged(ctx context.Context, log *slog.Logger, b store.Claimed, resp *fetch.Response, start time.Time) error {
	interval := nextInterval(b.FetchInterval, false)
	st := store.FetchState{ETag: resp.ETag, LastModified: resp.LastModified, FetchInterval: interval, NextFetchAt: w.now().Add(jitter(interval, w.rand()))}
	if err := w.Store.RecordUnchanged(ctx, b.ID, st); err != nil {
		log.Error("recording an unchanged fetch failed", "error", err)
		return err
	}
	w.refreshDescription(ctx, b, "")
	log.Info("fetched", "outcome", "unchanged", "status", resp.Status, "duration", w.now().Sub(start))
	return nil
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

// tagLoop tags a round of entries every minute until ctx is canceled.
func (w *Worker) tagLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		w.TagOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// TagOnce tags up to policy.TagsPerMinute untagged entries, newest first,
// and returns how many it tagged. After a failure it stops and waits longer
// each time, up to an hour: one failure usually means the next would fail
// too, from a wrong key, an outage or a spent budget.
func (w *Worker) TagOnce(ctx context.Context) int {
	if w.Tagger == nil || w.now().Before(w.nextTagAt) {
		return 0
	}
	jobs, err := w.Store.Untagged(ctx, policy.TagsPerMinute)
	if err != nil {
		w.log().Error("listing untagged entries failed", "error", err)
		return 0
	}
	tagged := 0
	for _, j := range jobs {
		tags, err := w.Tagger.Tag(ctx, j)
		if err != nil {
			if ctx.Err() != nil {
				return tagged
			}
			w.tagFailures++
			wait := min(time.Minute<<min(w.tagFailures-1, 6), time.Hour)
			w.nextTagAt = w.now().Add(wait)
			w.log().Warn("tagging failed", "entry", j.EntryID, "error", err, "retry_in", wait)
			return tagged
		}
		if err := w.Store.SetTags(ctx, j.EntryID, j.Title, tags); err != nil {
			w.log().Error("storing tags failed", "entry", j.EntryID, "error", err)
			return tagged
		}
		w.tagFailures = 0
		tagged++
	}
	if tagged > 0 {
		w.log().Info("tagged", "entries", tagged)
	}
	return tagged
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

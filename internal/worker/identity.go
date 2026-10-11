package worker

import (
	"context"
	"time"

	"github.com/kite-plus/explore/internal/model"
)

// IdentityEvents reads the identity provider's account events, oldest
// first; see internal/idp.
type IdentityEvents interface {
	Since(ctx context.Context, since, until time.Time) ([]model.IdentityEvent, error)
}

const (
	// identitySyncEvery is how often the reconciliation runs.
	identitySyncEvery = time.Hour
	// identitySettle leaves the newest events to the provider's pushes,
	// whose retries end well within it, so a late push cannot undo what the
	// reconciliation applied in order.
	identitySettle = 10 * time.Minute
)

func (w *Worker) identityLoop(ctx context.Context) {
	ticker := time.NewTicker(identitySyncEvery)
	defer ticker.Stop()
	for {
		if err := w.ReconcileIdentities(ctx); err != nil && ctx.Err() == nil {
			w.log().Error("reconciling identity events failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ReconcileIdentities applies the provider's account events since the last
// run, in order, which catches any its pushes lost (docs/design/accounts.md
// section 1), and then deletes accounts whose removal waited for another
// admin.
func (w *Worker) ReconcileIdentities(ctx context.Context) error {
	since, err := w.Store.IdentityCursor(ctx, w.IdentityIssuer)
	if err != nil {
		return err
	}
	until := w.now().Add(-identitySettle)
	if since.Before(until) {
		events, err := w.Identity.Since(ctx, since, until)
		if err != nil {
			return err
		}
		outcomes := map[string]int{}
		for _, e := range events {
			outcome, err := w.Store.ApplyIdentityEvent(ctx, w.IdentityIssuer, e)
			if err != nil {
				return err
			}
			outcomes[outcome]++
		}
		if err := w.Store.SetIdentityCursor(ctx, w.IdentityIssuer, until); err != nil {
			return err
		}
		if len(events) > 0 {
			w.log().Info("identity events reconciled", "events", len(events), "outcomes", outcomes)
		}
	}
	removed, err := w.Store.FinishIdentityRemovals(ctx)
	if err != nil {
		return err
	}
	if removed > 0 {
		w.log().Info("accounts removed after their identity", "accounts", removed)
	}
	return nil
}

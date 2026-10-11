package worker

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/store"
)

// IdentityDirectory reads the identity provider's account events and the
// state of an account; see internal/idp.
type IdentityDirectory interface {
	Events(ctx context.Context, from time.Time) ([]model.IdentityEvent, error)
	State(ctx context.Context, subject string) (model.IdentityState, error)
}

const (
	// identitySyncEvery is how often the reconciliation runs.
	identitySyncEvery = time.Hour
	// identityOverlap rereads events this far before the cursor, for ones
	// the provider committed slightly out of order.
	identityOverlap = 5 * time.Minute
)

func (w *Worker) identityLoop(ctx context.Context) {
	ticker := time.NewTicker(identitySyncEvery)
	defer ticker.Stop()
	for {
		if err := w.ReconcileIdentities(ctx); err != nil && ctx.Err() == nil {
			w.log().Error("reconciling identity accounts failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ReconcileIdentities catches what the provider's pushes lost
// (docs/design/accounts.md section 1): every account an event since the
// last run names gets the state the provider gives it now. An account the
// provider no longer knows is deleted only when a removal event says so
// too. Then accounts whose removal waited for another admin are deleted.
func (w *Worker) ReconcileIdentities(ctx context.Context) error {
	cursor, err := w.Store.IdentityCursor(ctx, w.IdentityIssuer)
	if err != nil {
		return err
	}
	from := cursor
	if !from.IsZero() {
		from = from.Add(-identityOverlap)
	}
	events, err := w.Identity.Events(ctx, from)
	if err != nil {
		return err
	}
	var subjects []string
	removals := map[string]bool{}
	next := cursor
	for _, e := range events {
		if !slices.Contains(subjects, e.Subject) {
			subjects = append(subjects, e.Subject)
		}
		if e.Type == model.RemovalEvent {
			removals[e.Subject] = true
		}
		if e.At.After(next) {
			next = e.At
		}
	}
	outcomes := map[string]int{}
	for _, sub := range subjects {
		if _, err := w.Store.UserByIdentity(ctx, w.IdentityIssuer, sub); errors.Is(err, store.ErrNotFound) {
			continue
		} else if err != nil {
			return err
		}
		state, err := w.Identity.State(ctx, sub)
		if err != nil {
			return err
		}
		if state == model.IdentityRemoved && !removals[sub] {
			w.log().Warn("identity provider has no such account, but no removal event says so")
			continue
		}
		outcome, err := w.Store.ApplyIdentityState(ctx, w.IdentityIssuer, sub, state)
		if err != nil {
			return err
		}
		outcomes[outcome]++
	}
	if next.After(cursor) {
		if err := w.Store.SetIdentityCursor(ctx, w.IdentityIssuer, next); err != nil {
			return err
		}
	}
	if len(outcomes) > 0 {
		w.log().Info("identity accounts reconciled", "events", len(events), "outcomes", outcomes)
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

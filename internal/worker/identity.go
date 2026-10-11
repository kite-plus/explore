package worker

import (
	"context"
	"errors"
	"fmt"
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
// too. An account that cannot be read or updated is skipped and read again
// next run: the cursor stops short of its first event. Accounts whose
// removal waited for another admin are deleted whatever else failed.
func (w *Worker) ReconcileIdentities(ctx context.Context) error {
	err := w.reconcileEvents(ctx)
	removed, ferr := w.Store.FinishIdentityRemovals(ctx)
	if ferr == nil && removed > 0 {
		w.log().Info("accounts removed after their identity", "accounts", removed)
	}
	return errors.Join(err, ferr)
}

func (w *Worker) reconcileEvents(ctx context.Context) error {
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
	first := map[string]time.Time{}
	removals := map[string]bool{}
	next := cursor
	for _, e := range events {
		if _, ok := first[e.Subject]; !ok {
			subjects = append(subjects, e.Subject)
			first[e.Subject] = e.At
		} else if e.At.Before(first[e.Subject]) {
			first[e.Subject] = e.At
		}
		if e.Type == model.RemovalEvent {
			removals[e.Subject] = true
		}
		if e.At.After(next) {
			next = e.At
		}
	}
	outcomes := map[string]int{}
	failed := 0
	for _, sub := range subjects {
		outcome, err := w.reconcileOne(ctx, sub, removals[sub])
		if err != nil {
			failed++
			w.log().Warn("reconciling an identity account failed; it is read again next run", "error", err)
			if retry := first[sub].Add(-time.Microsecond); retry.Before(next) {
				next = retry
			}
			continue
		}
		outcomes[outcome]++
	}
	if next.After(cursor) {
		if err := w.Store.SetIdentityCursor(ctx, w.IdentityIssuer, next); err != nil {
			return err
		}
	}
	if len(events) > 0 {
		w.log().Info("identity accounts reconciled", "events", len(events), "outcomes", outcomes, "failed", failed)
	}
	if failed > 0 {
		return fmt.Errorf("%d identity accounts could not be reconciled", failed)
	}
	return nil
}

// reconcileOne applies the provider's state to the account linked to sub,
// if any. removal says whether the events read include its removal.
func (w *Worker) reconcileOne(ctx context.Context, sub string, removal bool) (string, error) {
	if _, err := w.Store.UserByIdentity(ctx, w.IdentityIssuer, sub); errors.Is(err, store.ErrNotFound) {
		return store.IdentityIgnored, nil
	} else if err != nil {
		return "", err
	}
	state, err := w.Identity.State(ctx, sub)
	if err != nil {
		return "", err
	}
	if state == model.IdentityRemoved && !removal {
		w.log().Warn("identity provider has no such account, but no removal event says so")
		return store.IdentityUnchanged, nil
	}
	return w.Store.ApplyIdentityState(ctx, w.IdentityIssuer, sub, state)
}

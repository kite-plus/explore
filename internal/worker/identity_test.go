package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/store/storetest"
)

const issuer = "https://id.example.org"

type fakeDirectory struct {
	events    []model.IdentityEvent
	states    map[string]model.IdentityState
	err       error
	stateErrs map[string]error
	froms     []time.Time
	lookups   []string
}

func (f *fakeDirectory) Events(_ context.Context, from time.Time) ([]model.IdentityEvent, error) {
	f.froms = append(f.froms, from)
	if f.err != nil {
		return nil, f.err
	}
	var out []model.IdentityEvent
	for _, e := range f.events {
		if e.At.After(from) {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeDirectory) State(_ context.Context, subject string) (model.IdentityState, error) {
	f.lookups = append(f.lookups, subject)
	if err := f.stateErrs[subject]; err != nil {
		return "", err
	}
	if s, ok := f.states[subject]; ok {
		return s, nil
	}
	return model.IdentityRemoved, nil
}

func TestReconcileIdentities(t *testing.T) {
	st := storetest.New(t)
	ctx := context.Background()
	u, err := st.CreateIdentityUser(ctx, issuer, "sub-1", "reader@example.org", "Reader")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateIdentityUser(ctx, issuer, "sub-2", "second@example.org", "Second"); err != nil {
		t.Fatal(err)
	}
	dir := &fakeDirectory{states: map[string]model.IdentityState{"sub-1": model.IdentityDeactivated, "sub-2": model.IdentityActive},
		stateErrs: map[string]error{}}
	w := &Worker{Store: st, Identity: dir, IdentityIssuer: issuer}
	disabled := func(sub string) bool {
		t.Helper()
		found, err := st.UserByIdentity(ctx, issuer, sub)
		if err != nil {
			t.Fatal(err)
		}
		return found.Disabled
	}
	cursor := func() time.Time {
		t.Helper()
		c, err := st.IdentityCursor(ctx, issuer)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	t0 := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	dir.events = []model.IdentityEvent{
		{ID: "sub-1/3", Subject: "sub-1", Type: "user.deactivated", At: t0},
		{ID: "other/1", Subject: "other", Type: "user.deactivated", At: t0.Add(time.Minute)},
		{ID: "sub-1/4", Subject: "sub-1", Type: "user.reactivated", At: t0.Add(2 * time.Minute)},
	}
	if err := w.ReconcileIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	if !dir.froms[0].IsZero() || len(dir.lookups) != 1 || dir.lookups[0] != "sub-1" {
		t.Fatalf("froms = %v, lookups = %v", dir.froms, dir.lookups)
	}
	// What the provider says now counts, not the order of the events.
	if !disabled("sub-1") {
		t.Fatal("the account the provider says is deactivated is not disabled")
	}
	if c := cursor(); !c.Equal(t0.Add(2 * time.Minute)) {
		t.Fatalf("cursor = %v", c)
	}

	// One account that cannot be read is skipped; the others are handled and
	// the cursor stops just before its first event.
	t1 := t0.Add(time.Hour)
	dir.events = append(dir.events,
		model.IdentityEvent{ID: "sub-1/5", Subject: "sub-1", Type: "user.reactivated", At: t1},
		model.IdentityEvent{ID: "sub-2/2", Subject: "sub-2", Type: "user.deactivated", At: t1.Add(time.Minute)})
	dir.states["sub-1"], dir.states["sub-2"] = model.IdentityActive, model.IdentityDeactivated
	dir.stateErrs["sub-2"] = errors.New("provider hiccup")
	if err := w.ReconcileIdentities(ctx); err == nil {
		t.Fatal("a skipped account was not reported")
	}
	if disabled("sub-1") || disabled("sub-2") {
		t.Fatal("the readable account was not handled, or the unreadable one was")
	}
	if c := cursor(); !c.Equal(t1.Add(time.Minute - time.Microsecond)) {
		t.Fatalf("cursor with a skipped account = %v", c)
	}
	delete(dir.stateErrs, "sub-2")
	if err := w.ReconcileIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	if !disabled("sub-2") {
		t.Fatal("the skipped account was not handled on the next run")
	}
	if c := cursor(); !c.Equal(t1.Add(time.Minute)) {
		t.Fatalf("cursor after the retry = %v", c)
	}

	// Unknown to the provider without a removal event: left alone.
	dir.states["sub-2"] = model.IdentityActive
	delete(dir.states, "sub-1")
	dir.events = append(dir.events, model.IdentityEvent{ID: "sub-1/6", Subject: "sub-1", Type: "user.deactivated", At: t0.Add(2 * time.Hour)})
	if err := w.ReconcileIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserByIdentity(ctx, issuer, "sub-1"); err != nil {
		t.Fatalf("deleted without a removal event: %v", err)
	}

	// A removal that waited for another admin finishes on a later run, even
	// one whose event search fails.
	if err := st.SetUserAdminByID(ctx, u.ID, true); err != nil {
		t.Fatal(err)
	}
	dir.events = append(dir.events, model.IdentityEvent{ID: "sub-1/7", Subject: "sub-1", Type: model.RemovalEvent, At: t0.Add(3 * time.Hour)})
	if err := w.ReconcileIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	if found, err := st.UserByIdentity(ctx, issuer, "sub-1"); err != nil || !found.Disabled {
		t.Fatalf("the only admin: %+v, %v", found, err)
	}
	other, err := st.CreateUser(ctx, "admin@example.org", "hash", "Admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetUserAdminByID(ctx, other.ID, true); err != nil {
		t.Fatal(err)
	}
	before := cursor()
	dir.err = errors.New("provider down")
	if err := w.ReconcileIdentities(ctx); err == nil {
		t.Fatal("a failed event search reported success")
	}
	if _, err := st.UserByIdentity(ctx, issuer, "sub-1"); err == nil {
		t.Fatal("the removal did not finish while the event search failed")
	}
	if c := cursor(); !c.Equal(before) {
		t.Fatalf("cursor moved on a failed search: %v", c)
	}
}

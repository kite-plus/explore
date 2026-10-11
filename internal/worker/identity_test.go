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
	events  []model.IdentityEvent
	states  map[string]model.IdentityState
	err     error
	froms   []time.Time
	lookups []string
}

func (f *fakeDirectory) Events(_ context.Context, from time.Time) ([]model.IdentityEvent, error) {
	f.froms = append(f.froms, from)
	var out []model.IdentityEvent
	for _, e := range f.events {
		if e.At.After(from) {
			out = append(out, e)
		}
	}
	return out, f.err
}

func (f *fakeDirectory) State(_ context.Context, subject string) (model.IdentityState, error) {
	f.lookups = append(f.lookups, subject)
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
	dir := &fakeDirectory{states: map[string]model.IdentityState{"sub-1": model.IdentityLocked}}
	w := &Worker{Store: st, Identity: dir, IdentityIssuer: issuer}
	disabled := func() bool {
		t.Helper()
		found, err := st.UserByIdentity(ctx, issuer, "sub-1")
		if err != nil {
			t.Fatal(err)
		}
		return found.Disabled
	}
	t0 := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	dir.events = []model.IdentityEvent{
		{ID: "sub-1/3", Subject: "sub-1", Type: "user.locked", At: t0},
		{ID: "other/1", Subject: "other", Type: "user.locked", At: t0.Add(time.Minute)},
		{ID: "sub-1/4", Subject: "sub-1", Type: "user.unlocked", At: t0.Add(2 * time.Minute)},
	}
	if err := w.ReconcileIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	if !dir.froms[0].IsZero() || len(dir.lookups) != 1 || dir.lookups[0] != "sub-1" {
		t.Fatalf("froms = %v, lookups = %v", dir.froms, dir.lookups)
	}
	// What the provider says now counts, not the order of the events.
	if !disabled() {
		t.Fatal("the account the provider says is locked is not disabled")
	}
	if c, _ := st.IdentityCursor(ctx, issuer); !c.Equal(t0.Add(2 * time.Minute)) {
		t.Fatalf("cursor = %v", c)
	}

	// A failed read leaves the cursor where it was.
	dir.events = append(dir.events, model.IdentityEvent{ID: "sub-1/5", Subject: "sub-1", Type: "user.unlocked", At: t0.Add(time.Hour)})
	dir.states["sub-1"] = model.IdentityActive
	dir.err = errors.New("provider down")
	if err := w.ReconcileIdentities(ctx); err == nil {
		t.Fatal("a failed read reported success")
	}
	if c, _ := st.IdentityCursor(ctx, issuer); !c.Equal(t0.Add(2 * time.Minute)) {
		t.Fatalf("cursor after a failure = %v", c)
	}
	dir.err = nil
	if err := w.ReconcileIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	if last := dir.froms[len(dir.froms)-1]; !last.Equal(t0.Add(2*time.Minute - identityOverlap)) {
		t.Fatalf("from = %v", last)
	}
	if disabled() {
		t.Fatal("not restored once active")
	}

	// Unknown to the provider without a removal event: left alone.
	delete(dir.states, "sub-1")
	dir.events = append(dir.events, model.IdentityEvent{ID: "sub-1/6", Subject: "sub-1", Type: "user.locked", At: t0.Add(2 * time.Hour)})
	if err := w.ReconcileIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserByIdentity(ctx, issuer, "sub-1"); err != nil {
		t.Fatalf("deleted without a removal event: %v", err)
	}

	// A removal that waited for another admin finishes on the next run.
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
	if err := w.ReconcileIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserByIdentity(ctx, issuer, "sub-1"); err == nil {
		t.Fatal("the removal did not finish")
	}
}

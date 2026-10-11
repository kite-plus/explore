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

type fakeEvents struct {
	events []model.IdentityEvent
	err    error
	calls  [][2]time.Time
}

func (f *fakeEvents) Since(_ context.Context, since, until time.Time) ([]model.IdentityEvent, error) {
	f.calls = append(f.calls, [2]time.Time{since, until})
	var out []model.IdentityEvent
	for _, e := range f.events {
		if !e.At.Before(since) && !e.At.After(until) {
			out = append(out, e)
		}
	}
	return out, f.err
}

func TestReconcileIdentities(t *testing.T) {
	st := storetest.New(t)
	ctx := context.Background()
	u, err := st.CreateIdentityUser(ctx, issuer, "sub-1", "reader@example.org", "Reader")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	src := &fakeEvents{}
	w := &Worker{Store: st, Identity: src, IdentityIssuer: issuer, Now: func() time.Time { return now }}
	disabled := func() bool {
		t.Helper()
		found, err := st.UserByIdentity(ctx, issuer, "sub-1")
		if err != nil {
			t.Fatal(err)
		}
		return found.Disabled
	}

	// A lock and its unlock that the pushes lost, then a lock still settling.
	src.events = []model.IdentityEvent{
		{ID: "1", Subject: "sub-1", Change: model.IdentityLocked, At: now.Add(-50 * time.Minute)},
		{ID: "2", Subject: "sub-1", Change: model.IdentityUnlocked, At: now.Add(-40 * time.Minute)},
		{ID: "3", Subject: "sub-1", Change: model.IdentityDeactivated, At: now.Add(-30 * time.Minute)},
		{ID: "4", Subject: "sub-1", Change: model.IdentityLocked, At: now.Add(-5 * time.Minute)},
	}
	if err := w.ReconcileIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	if !src.calls[0][0].IsZero() || !src.calls[0][1].Equal(now.Add(-identitySettle)) {
		t.Fatalf("first read = %v", src.calls[0])
	}
	if !disabled() {
		t.Fatal("the deactivation was not applied")
	}
	if c, _ := st.IdentityCursor(ctx, issuer); !c.Equal(now.Add(-identitySettle)) {
		t.Fatalf("cursor = %v", c)
	}

	// A failed read leaves the cursor where it was.
	now = now.Add(time.Hour)
	src.err = errors.New("provider down")
	if err := w.ReconcileIdentities(ctx); err == nil {
		t.Fatal("a failed read reported success")
	}
	if c, _ := st.IdentityCursor(ctx, issuer); !c.Equal(now.Add(-time.Hour - identitySettle)) {
		t.Fatalf("cursor after a failure = %v", c)
	}
	src.err = nil
	src.events = append(src.events, model.IdentityEvent{ID: "5", Subject: "sub-1", Change: model.IdentityReactivated, At: now.Add(-20 * time.Minute)})
	if err := w.ReconcileIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	if last := src.calls[len(src.calls)-1]; !last[0].Equal(now.Add(-time.Hour-identitySettle)) || !last[1].Equal(now.Add(-identitySettle)) {
		t.Fatalf("second read = %v", last)
	}
	// Reactivated, but the later lock still holds.
	if !disabled() {
		t.Fatal("restored while still locked")
	}
	src.events = append(src.events, model.IdentityEvent{ID: "6", Subject: "sub-1", Change: model.IdentityUnlocked, At: now})
	now = now.Add(time.Hour)
	if err := w.ReconcileIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	if disabled() {
		t.Fatal("not restored once unlocked and reactivated")
	}

	// A removal that waited for another admin finishes on the next run.
	if err := st.SetUserAdminByID(ctx, u.ID, true); err != nil {
		t.Fatal(err)
	}
	src.events = append(src.events, model.IdentityEvent{ID: "7", Subject: "sub-1", Change: model.IdentityRemoved, At: now})
	now = now.Add(time.Hour)
	if err := w.ReconcileIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserByIdentity(ctx, issuer, "sub-1"); err != nil {
		t.Fatalf("the only admin was deleted: %v", err)
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

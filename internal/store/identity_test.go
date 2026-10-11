package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/kite-plus/explore/internal/model"
)

const testIssuer = "https://id.example.org"

func TestIdentityAccounts(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u, err := s.CreateIdentityUser(ctx, testIssuer, "sub-1", "new@example.com", "New")
	if err != nil {
		t.Fatal(err)
	}
	if u.PasswordHash != "" || u.LinkedAt == nil || u.Number != 1 {
		t.Fatalf("created = %+v", u)
	}
	found, err := s.UserByIdentity(ctx, testIssuer, "sub-1")
	if err != nil || found.ID != u.ID || found.LinkedAt == nil {
		t.Fatalf("found = %+v, %v", found, err)
	}
	if _, err := s.UserByIdentity(ctx, "https://other.example", "sub-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other issuer: %v", err)
	}
	if _, err := s.CreateIdentityUser(ctx, testIssuer, "sub-1", "again@example.com", "Again"); !errors.Is(err, ErrIdentityTaken) {
		t.Fatalf("same identity: %v", err)
	}
	local, err := s.CreateUser(ctx, "local@example.com", "hash", "Local")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateIdentityUser(ctx, testIssuer, "sub-2", "local@example.com", "Someone"); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("taken email: %v", err)
	}
	if got, _ := s.UserByEmail(ctx, "local@example.com"); got.LinkedAt != nil || got.PasswordHash != "hash" {
		t.Fatalf("local account changed: %+v", got)
	}

	if err := s.LinkIdentity(ctx, local.ID, testIssuer, "sub-1"); !errors.Is(err, ErrIdentityTaken) {
		t.Fatalf("link taken identity: %v", err)
	}
	if err := s.LinkIdentity(ctx, local.ID, testIssuer, "sub-2"); err != nil {
		t.Fatal(err)
	}
	linked, err := s.UserByEmail(ctx, "local@example.com")
	if err != nil || linked.PasswordHash != "" || linked.LinkedAt == nil {
		t.Fatalf("linked = %+v, %v", linked, err)
	}
	if err := s.LinkIdentity(ctx, local.ID, testIssuer, "sub-2"); err != nil {
		t.Fatalf("relinking the same identity: %v", err)
	}
	if err := s.LinkIdentity(ctx, local.ID, testIssuer, "sub-3"); !errors.Is(err, ErrAlreadyLinked) {
		t.Fatalf("second identity: %v", err)
	}
	if err := s.ResetPassword(ctx, local.ID, "new-hash", "admin@example.com"); !errors.Is(err, ErrPasswordOff) {
		t.Fatalf("reset a linked account's password: %v", err)
	}
	banned, err := s.CreateUser(ctx, "banned@example.com", "hash", "Banned")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetUserDisabled(ctx, banned.ID, true, "spam", "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkIdentity(ctx, banned.ID, testIssuer, "sub-4"); !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("link a disabled account: %v", err)
	}
	detail, err := s.AdminUserDetail(ctx, local.ID)
	if err != nil || len(detail.Identities) != 1 || detail.Identities[0].Subject != "sub-2" || detail.PasswordLogin {
		t.Fatalf("detail = %+v, %v", detail, err)
	}
}

func TestIdentitySessions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a, err := s.CreateIdentityUser(ctx, testIssuer, "sub-a", "a@example.com", "A")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.CreateIdentityUser(ctx, testIssuer, "sub-b", "b@example.com", "B")
	if err != nil {
		t.Fatal(err)
	}
	session := func(u User, name, sid, client string) [32]byte {
		hash := sha256.Sum256([]byte(name))
		if err := s.CreateSession(ctx, Session{UserID: u.ID, TokenHash: hash, Expires: time.Now().Add(time.Hour), SID: sid, Client: client}); err != nil {
			t.Fatal(err)
		}
		return hash
	}
	a1 := session(a, "a1", "sid-1", ClientWeb)
	a2 := session(a, "a2", "sid-1", ClientIOS)
	a3 := session(a, "a3", "sid-2", ClientWeb)
	b1 := session(b, "b1", "sid-1", ClientWeb)
	alive := func(hash [32]byte) bool {
		_, err := s.UserBySession(ctx, hash)
		return err == nil
	}
	if n, err := s.EndIdentitySessions(ctx, testIssuer, "sub-a", "sid-1"); err != nil || n != 2 {
		t.Fatalf("sid and sub: n = %d, err = %v", n, err)
	}
	if alive(a1) || alive(a2) || !alive(a3) || !alive(b1) {
		t.Fatal("wrong sessions ended")
	}
	if n, err := s.EndIdentitySessions(ctx, "https://other.example", "", "sid-2"); err != nil || n != 0 {
		t.Fatalf("other issuer: n = %d, err = %v", n, err)
	}
	if n, err := s.EndIdentitySessions(ctx, testIssuer, "", "sid-1"); err != nil || n != 1 || alive(b1) {
		t.Fatalf("sid only: n = %d, err = %v", n, err)
	}
	session(a, "a4", "", ClientWeb)
	if n, err := s.EndIdentitySessions(ctx, testIssuer, "sub-a", ""); err != nil || n != 2 {
		t.Fatalf("sub only: n = %d, err = %v", n, err)
	}
	if n, err := s.EndIdentitySessions(ctx, testIssuer, "", ""); err != nil || n != 0 {
		t.Fatalf("neither: n = %d, err = %v", n, err)
	}
}

func TestLoginFlows(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	state, browser := sha256.Sum256([]byte("state")), sha256.Sum256([]byte("browser"))
	f := LoginFlow{StateHash: state, BrowserHash: browser, Mode: "app", Nonce: "n", Verifier: "v", Challenge: "c",
		Client: ClientIOS, AppRedirect: "https://explore.example.org/app/auth/done", Expires: time.Now().Add(time.Minute)}
	if err := s.CreateLoginFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TakeLoginFlow(ctx, state, sha256.Sum256([]byte("other browser"))); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other browser: %v", err)
	}
	got, err := s.TakeLoginFlow(ctx, state, browser)
	if err != nil || got.Mode != "app" || got.Challenge != "c" || got.Client != ClientIOS || got.AppRedirect != f.AppRedirect || got.UserID != "" {
		t.Fatalf("taken = %+v, %v", got, err)
	}
	if _, err := s.TakeLoginFlow(ctx, state, browser); !errors.Is(err, ErrNotFound) {
		t.Fatalf("taken twice: %v", err)
	}
	f.StateHash, f.Expires = sha256.Sum256([]byte("old")), time.Now().Add(-time.Second)
	if err := s.CreateLoginFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TakeLoginFlow(ctx, f.StateHash, browser); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired: %v", err)
	}
	if err := s.CreateLoginFlow(ctx, LoginFlow{StateHash: state, BrowserHash: browser, Mode: "link", Nonce: "n", Verifier: "v", Expires: time.Now().Add(time.Minute)}); err == nil {
		t.Fatal("a link flow without an account was stored")
	}
	if err := s.PurgeSignIns(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestRedeemAppCode(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u, err := s.CreateIdentityUser(ctx, testIssuer, "sub-1", "app@example.com", "App")
	if err != nil {
		t.Fatal(err)
	}
	code := func(name string, expires time.Time) [32]byte {
		hash := sha256.Sum256([]byte(name))
		if err := s.CreateAppCode(ctx, AppCode{CodeHash: hash, UserID: u.ID, SID: "sid-1", Challenge: "challenge", Client: ClientAndroid, Expires: expires}); err != nil {
			t.Fatal(err)
		}
		return hash
	}
	right := func(c string) bool { return c == "challenge" }
	wrong := func(string) bool { return false }
	soon := time.Now().Add(time.Minute)
	ok := code("ok", soon)
	token := sha256.Sum256([]byte("token"))
	got, err := s.RedeemAppCode(ctx, ok, right, token, time.Now().Add(time.Hour))
	if err != nil || got.ID != u.ID {
		t.Fatalf("redeem = %+v, %v", got, err)
	}
	if _, err := s.UserBySession(ctx, token); err != nil {
		t.Fatalf("session: %v", err)
	}
	var client, sid string
	if err := s.pool.QueryRow(ctx, `SELECT client, sid FROM sessions WHERE token_hash = $1`, token[:]).Scan(&client, &sid); err != nil || client != ClientAndroid || sid != "sid-1" {
		t.Fatalf("session = %s %s, %v", client, sid, err)
	}
	if _, err := s.RedeemAppCode(ctx, ok, right, sha256.Sum256([]byte("token-2")), time.Now().Add(time.Hour)); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("reuse: %v", err)
	}
	if _, err := s.UserBySession(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("session after reuse: %v", err)
	}
	burned := code("burned", soon)
	if _, err := s.RedeemAppCode(ctx, burned, wrong, sha256.Sum256([]byte("token-3")), time.Now().Add(time.Hour)); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("wrong verifier: %v", err)
	}
	if _, err := s.RedeemAppCode(ctx, burned, right, sha256.Sum256([]byte("token-4")), time.Now().Add(time.Hour)); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("after a wrong verifier: %v", err)
	}
	expired := code("expired", time.Now().Add(-time.Second))
	if _, err := s.RedeemAppCode(ctx, expired, right, sha256.Sum256([]byte("token-5")), time.Now().Add(time.Hour)); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("expired: %v", err)
	}
	if _, err := s.RedeemAppCode(ctx, sha256.Sum256([]byte("unknown")), right, sha256.Sum256([]byte("token-6")), time.Now().Add(time.Hour)); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("unknown: %v", err)
	}
	disabled := code("disabled", soon)
	if err := s.SetUserDisabled(ctx, u.ID, true, "spam", "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedeemAppCode(ctx, disabled, right, sha256.Sum256([]byte("token-7")), time.Now().Add(time.Hour)); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("disabled account: %v", err)
	}
}

func TestReceiptsAndCursor(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if fresh, err := s.RememberReceipt(ctx, ReceiptLogoutToken, "jti-1", time.Now().Add(time.Hour)); err != nil || !fresh {
		t.Fatalf("first: %v %v", fresh, err)
	}
	if fresh, err := s.RememberReceipt(ctx, ReceiptLogoutToken, "jti-1", time.Now().Add(time.Hour)); err != nil || fresh {
		t.Fatalf("replay: %v %v", fresh, err)
	}
	if seen, err := s.SeenReceipt(ctx, ReceiptEvent, "jti-1"); err != nil || seen {
		t.Fatalf("other kind: %v %v", seen, err)
	}
	if _, err := s.RememberReceipt(ctx, ReceiptEvent, "e-1", time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if seen, err := s.SeenReceipt(ctx, ReceiptEvent, "e-1"); err != nil || seen {
		t.Fatalf("expired seen: %v %v", seen, err)
	}
	if fresh, err := s.RememberReceipt(ctx, ReceiptEvent, "e-1", time.Now().Add(time.Hour)); err != nil || !fresh {
		t.Fatalf("after expiry: %v %v", fresh, err)
	}

	if c, err := s.IdentityCursor(ctx, testIssuer); err != nil || !c.IsZero() {
		t.Fatalf("no cursor: %v %v", c, err)
	}
	at := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	if err := s.SetIdentityCursor(ctx, testIssuer, at); err != nil {
		t.Fatal(err)
	}
	if err := s.SetIdentityCursor(ctx, testIssuer, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if c, err := s.IdentityCursor(ctx, testIssuer); err != nil || !c.Equal(at.Add(time.Hour)) {
		t.Fatalf("cursor: %v %v", c, err)
	}
}

func TestApplyIdentityEvent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u, err := s.CreateIdentityUser(ctx, testIssuer, "sub-1", "reader@example.com", "Reader")
	if err != nil {
		t.Fatal(err)
	}
	session := sha256.Sum256([]byte("session"))
	if err := s.CreateSession(ctx, Session{UserID: u.ID, TokenHash: session, Expires: time.Now().Add(time.Hour), SID: "sid"}); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	n := 0
	apply := func(sub string, change model.IdentityChange, at time.Time, want string) {
		t.Helper()
		n++
		got, err := s.ApplyIdentityEvent(ctx, testIssuer, model.IdentityEvent{ID: string(rune('a' + n)), Subject: sub, Change: change, At: at})
		if err != nil || got != want {
			t.Fatalf("%s %s: got %q, %v; want %q", sub, change, got, err, want)
		}
	}
	state := func(id string) (disabled bool, by, reason string) {
		t.Helper()
		if err := s.pool.QueryRow(ctx, `SELECT disabled_at IS NOT NULL, disabled_by, disabled_reason FROM users WHERE id = $1`, id).Scan(&disabled, &by, &reason); err != nil {
			t.Fatal(err)
		}
		return
	}

	apply("nobody", model.IdentityLocked, t0, IdentityIgnored)
	apply("sub-1", model.IdentityLocked, t0, IdentityDisabled)
	if d, by, reason := state(u.ID); !d || by != "idp" || reason != "locked" {
		t.Fatalf("locked: %v %q %q", d, by, reason)
	}
	if _, err := s.UserBySession(ctx, session); !errors.Is(err, ErrNotFound) {
		t.Fatal("session survived the lock")
	}
	apply("sub-1", model.IdentityDeactivated, t0.Add(time.Minute), IdentityDisabled)
	if _, _, reason := state(u.ID); reason != "deactivated,locked" {
		t.Fatalf("reasons = %q", reason)
	}
	apply("sub-1", model.IdentityUnlocked, t0.Add(2*time.Minute), IdentityUnchanged)
	if d, _, reason := state(u.ID); !d || reason != "deactivated" {
		t.Fatalf("unlocked but deactivated: %v %q", d, reason)
	}
	// Delivered late: older than what was applied.
	apply("sub-1", model.IdentityLocked, t0.Add(90*time.Second), IdentityIgnored)
	apply("sub-1", model.IdentityReactivated, t0.Add(3*time.Minute), IdentityRestored)
	if d, by, reason := state(u.ID); d || by != "" || reason != "" {
		t.Fatalf("restored: %v %q %q", d, by, reason)
	}
	apply("sub-1", model.IdentityReactivated, t0.Add(3*time.Minute), IdentityUnchanged)

	// An admin's disabling is not the provider's to undo.
	if err := s.SetUserDisabled(ctx, u.ID, true, "spam", "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	apply("sub-1", model.IdentityLocked, t0.Add(4*time.Minute), IdentityUnchanged)
	apply("sub-1", model.IdentityUnlocked, t0.Add(5*time.Minute), IdentityUnchanged)
	if d, by, reason := state(u.ID); !d || by != "admin@example.com" || reason != "spam" {
		t.Fatalf("admin's disabling: %v %q %q", d, by, reason)
	}

	apply("sub-1", model.IdentityRemoved, t0.Add(6*time.Minute), IdentityDeleted)
	if _, err := s.UserByIdentity(ctx, testIssuer, "sub-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removed account: %v", err)
	}

	// The only active admin is kept, disabled, until another admin exists.
	admin, err := s.CreateIdentityUser(ctx, testIssuer, "sub-admin", "admin@example.com", "Admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetUserAdminByID(ctx, admin.ID, true); err != nil {
		t.Fatal(err)
	}
	apply("sub-admin", model.IdentityRemoved, t0, IdentityPendingRemoval)
	if d, by, reason := state(admin.ID); !d || by != "idp" || reason != "removed" {
		t.Fatalf("pending removal: %v %q %q", d, by, reason)
	}
	apply("sub-admin", model.IdentityUnlocked, t0.Add(time.Minute), IdentityUnchanged)
	if pending, err := s.SetupPending(ctx); err != nil || pending {
		t.Fatalf("setup reopened: %v %v", pending, err)
	}
	if n, err := s.FinishIdentityRemovals(ctx); err != nil || n != 0 {
		t.Fatalf("finished with no other admin: %d %v", n, err)
	}
	other, err := s.CreateUser(ctx, "other@example.com", "hash", "Other")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetUserAdminByID(ctx, other.ID, true); err != nil {
		t.Fatal(err)
	}
	if n, err := s.FinishIdentityRemovals(ctx); err != nil || n != 1 {
		t.Fatalf("finished: %d %v", n, err)
	}
	if _, err := s.UserByIdentity(ctx, testIssuer, "sub-admin"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pending account remains: %v", err)
	}
	// With another active admin, an admin's removal deletes at once.
	second, err := s.CreateIdentityUser(ctx, testIssuer, "sub-second", "second@example.com", "Second")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetUserAdminByID(ctx, second.ID, true); err != nil {
		t.Fatal(err)
	}
	apply("sub-second", model.IdentityRemoved, t0, IdentityDeleted)
}

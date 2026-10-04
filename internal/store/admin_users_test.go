package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

func TestAdminUserListFiltersAndSorts(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	blog := listBlog(t, s, "follow.example", "en")
	sync(t, s, blog.ID, entry("post", at(time.Now().Add(-time.Hour)), true))
	users := map[string]User{}
	for _, name := range []string{"admin", "reader", "banned", "follower"} {
		u, err := s.CreateUser(ctx, name+"@example.com", "unused-hash", name)
		if err != nil {
			t.Fatal(err)
		}
		users[name] = u
	}
	if err := s.SetUserAdminByID(ctx, users["admin"].ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUserDisabled(ctx, users["banned"].ID, true, "spam", "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSubscription(ctx, users["follower"].ID, blog.Host); err != nil {
		t.Fatal(err)
	}
	s.exec(t, `UPDATE users SET last_seen_at = now() - interval '1 hour' WHERE email = 'reader@example.com'`)
	s.exec(t, `UPDATE users SET last_seen_at = now() - interval '1 day' WHERE email = 'admin@example.com'`)

	list := func(q AdminUserQuery) ([]string, int64, AdminUserCounts) {
		t.Helper()
		q.Limit = 30
		rows, total, counts, err := s.AdminUsers(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		names := identities(rows, func(u AdminUser) string { return u.DisplayName })
		return names, total, counts
	}

	_, total, counts := list(AdminUserQuery{Sort: "created"})
	if total != 4 || counts != (AdminUserCounts{Active: 3, Disabled: 1, Admin: 1, Reader: 3}) {
		t.Fatalf("all: total=%d counts=%+v", total, counts)
	}
	names, total, counts := list(AdminUserQuery{Status: "disabled", Sort: "created"})
	if total != 1 || names[0] != "banned" || counts != (AdminUserCounts{Active: 3, Disabled: 1, Admin: 0, Reader: 1}) {
		t.Fatalf("disabled: %v total=%d counts=%+v", names, total, counts)
	}
	names, total, _ = list(AdminUserQuery{Status: "active", Role: "reader", Sort: "created"})
	if total != 2 || len(names) != 2 {
		t.Fatalf("active readers: %v total=%d", names, total)
	}
	if names, _, _ = list(AdminUserQuery{Search: users["follower"].ID, Sort: "created"}); len(names) != 1 || names[0] != "follower" {
		t.Fatalf("search by id: %v", names)
	}
	if names, _, _ = list(AdminUserQuery{Sort: "seen"}); names[0] != "reader" || names[1] != "admin" {
		t.Fatalf("most recently seen first: %v", names)
	}
	if names, _, _ = list(AdminUserQuery{Sort: "seen", Asc: true}); names[0] == "reader" || names[3] != "reader" {
		t.Fatalf("never seen first: %v", names)
	}
	if names, _, _ = list(AdminUserQuery{Sort: "subscriptions"}); names[0] != "follower" {
		t.Fatalf("most subscriptions first: %v", names)
	}
}

func TestAdminUserDetailAndCleanup(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	followed := listBlog(t, s, "followed.example", "en")
	sync(t, s, followed.ID, entry("post", at(time.Now().Add(-time.Hour)), true))
	owned := listBlog(t, s, "owned.example", "en")
	claimed := listBlog(t, s, "claimed.example", "en")
	u, err := s.CreateUser(ctx, "owner@example.com", "unused-hash", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	token := sha256.Sum256([]byte("owner-session"))
	if err := s.CreateSession(ctx, u.ID, token, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSubscription(ctx, u.ID, followed.Host); err != nil {
		t.Fatal(err)
	}
	s.exec(t, `INSERT INTO blog_owners(blog_id, user_id) VALUES ($1, $2)`, owned.ID, u.ID)
	if err := s.CreateBlogClaim(ctx, u.ID, claimed.Host, sha256.Sum256([]byte("claim")), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTakedownRequest(ctx, "blog", followed.Host, 0, u.ID, "copied posts"); err != nil {
		t.Fatal(err)
	}

	d, err := s.AdminUserDetail(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.LastSeenAt == nil || len(d.Sessions) != 1 || d.SubscriptionCount != 1 || len(d.Subscriptions) != 1 ||
		len(d.OwnedBlogs) != 1 || d.OwnedBlogs[0].Host != owned.Host || len(d.PendingClaims) != 1 ||
		len(d.Reports) != 1 || d.Reports[0].BlogHost != followed.Host {
		t.Fatalf("detail: %+v", d)
	}
	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		if _, err := s.AdminUserDetail(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("detail of %s: %v", id, err)
		}
	}

	if err := s.ReleaseBlog(ctx, u.ID, owned.Host); err != nil {
		t.Fatal(err)
	}
	if err := s.ReleaseBlog(ctx, u.ID, owned.Host); !errors.Is(err, ErrNotFound) {
		t.Fatalf("released twice: %v", err)
	}
	if n, err := s.RevokeSessions(ctx, u.ID); err != nil || n != 1 {
		t.Fatalf("revoke: %d, %v", n, err)
	}
	if _, err := s.UserBySession(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("session survived revoke: %v", err)
	}
	if _, err := s.RevokeSessions(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoke unknown: %v", err)
	}
	if err := s.RenameUser(ctx, u.ID, "Renamed"); err != nil {
		t.Fatal(err)
	}

	if err := s.SetUserDisabled(ctx, u.ID, true, "spam", "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUserAdminByID(ctx, u.ID, true); !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("admin access for a disabled account: %v", err)
	}
	d, err = s.AdminUserDetail(ctx, u.ID)
	if err != nil || d.DisplayName != "Renamed" || d.DisabledAt == nil || d.DisabledReason != "spam" ||
		d.DisabledBy != "admin@example.com" || len(d.OwnedBlogs) != 0 {
		t.Fatalf("after changes: %+v, %v", d, err)
	}
	if err := s.SetUserDisabled(ctx, u.ID, false, "", "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	d, err = s.AdminUserDetail(ctx, u.ID)
	if err != nil || d.DisabledAt != nil || d.DisabledReason != "" || d.DisabledBy != "" {
		t.Fatalf("restored: %+v, %v", d, err)
	}
}

func TestSessionsMarkTheAccountSeen(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u, err := s.CreateUser(ctx, "seen@example.com", "unused-hash", "Seen")
	if err != nil {
		t.Fatal(err)
	}
	seen := func() time.Time {
		t.Helper()
		var at *time.Time
		if err := s.pool.QueryRow(ctx, `SELECT last_seen_at FROM users WHERE id = $1`, u.ID).Scan(&at); err != nil || at == nil {
			t.Fatalf("last seen: %v, %v", at, err)
		}
		return *at
	}
	token := sha256.Sum256([]byte("seen-session"))
	if err := s.CreateSession(ctx, u.ID, token, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	seen()

	s.exec(t, `UPDATE users SET last_seen_at = now() - interval '1 hour' WHERE id = $1`, u.ID)
	stale := seen()
	if _, err := s.UserBySession(ctx, token); err != nil {
		t.Fatal(err)
	}
	fresh := seen()
	if !fresh.After(stale) {
		t.Fatalf("stale mark kept: %v", fresh)
	}

	s.exec(t, `UPDATE users SET last_seen_at = now() - interval '1 minute' WHERE id = $1`, u.ID)
	recent := seen()
	if _, err := s.UserBySession(ctx, token); err != nil {
		t.Fatal(err)
	}
	if !seen().Equal(recent) {
		t.Fatal("a recent mark was rewritten")
	}
}

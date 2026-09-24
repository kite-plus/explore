package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

func TestArticleModerationSurvivesResync(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	blog := listBlog(t, s, "moderation.example", "zh-CN")
	when := time.Now().Add(-time.Hour)
	sync(t, s, blog.ID, entry("post-1", &when, true))
	page, total, err := s.AdminEntries(ctx, "moderation.example", 30, 0)
	if err != nil || total != 1 || len(page) != 1 {
		t.Fatalf("admin entries: %v, total=%d, rows=%d", err, total, len(page))
	}
	if err := s.SetEntryHidden(ctx, page[0].ID, true, "违规内容", "operator"); err != nil {
		t.Fatal(err)
	}
	stream, err := s.Stream(ctx, StreamQuery{Limit: 10})
	if err != nil || len(stream) != 0 {
		t.Fatalf("hidden stream: %v, rows=%d", err, len(stream))
	}
	_, entries, err := s.VisibleBlog(ctx, blog.Host)
	if err != nil || len(entries) != 0 {
		t.Fatalf("hidden blog entry: %v, rows=%d", err, len(entries))
	}
	sync(t, s, blog.ID, entry("post-1", &when, true))
	stream, err = s.Stream(ctx, StreamQuery{Limit: 10})
	if err != nil || len(stream) != 0 {
		t.Fatalf("resynced stream: %v, rows=%d", err, len(stream))
	}
	if err := s.SetEntryHidden(ctx, page[0].ID, false, "", "operator"); err != nil {
		t.Fatal(err)
	}
	stream, err = s.Stream(ctx, StreamQuery{Limit: 10})
	if err != nil || len(stream) != 1 {
		t.Fatalf("restored stream: %v, rows=%d", err, len(stream))
	}
}

func TestDisabledUserLosesSessions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user, err := s.CreateUser(ctx, "reader@example.com", "unused-hash", "Reader")
	if err != nil {
		t.Fatal(err)
	}
	token := sha256.Sum256([]byte("test-session"))
	if err := s.CreateSession(ctx, user.ID, token, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUserDisabled(ctx, user.ID, true); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.UserByEmail(ctx, user.Email)
	if err != nil || !loaded.Disabled {
		t.Fatalf("disabled account: %+v, %v", loaded, err)
	}
	if _, err := s.UserBySession(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("session survived disable: %v", err)
	}
	if err := s.SetUserDisabled(ctx, user.ID, false); err != nil {
		t.Fatal(err)
	}
	loaded, err = s.UserByEmail(ctx, user.Email)
	if err != nil || loaded.Disabled {
		t.Fatalf("restored account: %+v, %v", loaded, err)
	}
}

func TestAdminRoleKeepsLastActiveAdministrator(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	first, err := s.CreateUser(ctx, "first@example.com", "unused-hash", "First")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateUser(ctx, "second@example.com", "unused-hash", "Second")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetUserAdminByID(ctx, first.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUserDisabled(ctx, first.ID, true); !errors.Is(err, ErrConflict) {
		t.Fatalf("last administrator disabled: %v", err)
	}
	if err := s.SetUserAdminByID(ctx, first.ID, false); !errors.Is(err, ErrConflict) {
		t.Fatalf("last administrator demoted: %v", err)
	}
	if err := s.SetUserAdminByID(ctx, second.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUserAdminByID(ctx, first.ID, false); err != nil {
		t.Fatal(err)
	}
}

func TestTakedownAndCrawlerSettings(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	blog := listBlog(t, s, "takedown.example", "en")
	if err := s.CreateTakedownRequest(ctx, "blog", blog.Host, 0, "", "博客所有者要求下架"); err != nil {
		t.Fatal(err)
	}
	requests, err := s.TakedownRequests(ctx, "pending", 10)
	if err != nil || len(requests) != 1 {
		t.Fatalf("pending requests: %v, rows=%d", err, len(requests))
	}
	if err := s.ReviewTakedown(ctx, requests[0].ID, "approved", "已核实", "operator"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.VisibleBlog(ctx, blog.Host); !errors.Is(err, ErrNotFound) {
		t.Fatalf("paused blog should not be visible: %v", err)
	}
	if err := s.SetSetting(ctx, "crawler_paused", "true", "operator"); err != nil {
		t.Fatal(err)
	}
	listBlog(t, s, "queued.example", "en")
	claimed, err := s.ClaimDue(ctx, 10, time.Minute)
	if err != nil || len(claimed) != 0 {
		t.Fatalf("paused crawler: %v, rows=%d", err, len(claimed))
	}
	if err := s.SetSetting(ctx, "crawler_paused", "false", "operator"); err != nil {
		t.Fatal(err)
	}
	claimed, err = s.ClaimDue(ctx, 10, time.Minute)
	if err != nil || len(claimed) != 1 || claimed[0].Host != "queued.example" {
		t.Fatalf("resumed crawler: %v, rows=%v", err, claimed)
	}
}

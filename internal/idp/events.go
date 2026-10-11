package idp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kite-plus/explore/internal/model"
)

// SignatureHeader carries the signature of a pushed event.
const SignatureHeader = "ZITADEL-Signature"

// signatureTolerance is how far a signature's time may be from ours.
const signatureTolerance = 5 * time.Minute

// ErrBadSignature means a pushed event is not signed with the shared secret,
// or was signed too long ago.
var ErrBadSignature = errors.New("bad signature")

// Sign computes the signature header ZITADEL sends with body: the time, and
// the hex HMAC-SHA256 of "<time>.<body>" under secret.
func Sign(body []byte, secret string, at time.Time) string {
	t := strconv.FormatInt(at.Unix(), 10)
	return "t=" + t + ",v1=" + hex.EncodeToString(mac(secret, t, body))
}

func mac(secret, t string, body []byte) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(t))
	h.Write([]byte("."))
	h.Write(body)
	return h.Sum(nil)
}

// VerifySignature checks a signature header against body and secret. Any
// one v1 value may match, as during a key rotation.
func VerifySignature(header string, body []byte, secret string, now time.Time) error {
	var t string
	var sigs [][]byte
	for part := range strings.SplitSeq(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			t = v
		case "v1":
			if sig, err := hex.DecodeString(v); err == nil {
				sigs = append(sigs, sig)
			}
		}
	}
	sec, err := strconv.ParseInt(t, 10, 64)
	if err != nil || len(sigs) == 0 {
		return ErrBadSignature
	}
	if d := now.Sub(time.Unix(sec, 0)); d > signatureTolerance || d < -signatureTolerance {
		return ErrBadSignature
	}
	want := mac(secret, t, body)
	for _, sig := range sigs {
		if hmac.Equal(sig, want) {
			return nil
		}
	}
	return ErrBadSignature
}

// userEventTypes are the provider's event types that can change an account
// here. Locking is not one: anyone who knows a reader's email can lock their
// account at the provider with wrong passwords, so it suspends nothing here,
// and the provider's user.locked and user.unlocked pushes are ignored.
var userEventTypes = []string{model.RemovalEvent, "user.deactivated", "user.reactivated"}

// pushed is the body of an event ZITADEL's Actions push to a webhook target.
type pushed struct {
	AggregateID   string      `json:"aggregateID"`
	AggregateType string      `json:"aggregateType"`
	Sequence      json.Number `json:"sequence"`
	EventType     string      `json:"event_type"`
	CreatedAt     time.Time   `json:"created_at"`
}

// ParseEvent reads a pushed event. ok is false for events that change
// nothing here, such as a profile edit.
func ParseEvent(body []byte) (e model.IdentityEvent, ok bool, err error) {
	var p pushed
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&p); err != nil {
		return model.IdentityEvent{}, false, fmt.Errorf("event: %w", err)
	}
	if !slices.Contains(userEventTypes, p.EventType) || (p.AggregateType != "" && p.AggregateType != "user") {
		return model.IdentityEvent{}, false, nil
	}
	if p.AggregateID == "" || p.Sequence == "" || p.CreatedAt.IsZero() {
		return model.IdentityEvent{}, false, errors.New("event: missing aggregateID, sequence or created_at")
	}
	return model.IdentityEvent{ID: p.AggregateID + "/" + p.Sequence.String(), Subject: p.AggregateID,
		Type: p.EventType, At: p.CreatedAt}, true, nil
}

// Directory reads the provider's accounts and their recorded events with
// a service account's token: ZITADEL's admin event search and user API,
// as the identity repository's README describes them. A pushed event is
// only a hint, since other services' tokens can read the signing key too,
// so the account's state comes from here before anything is done.
type Directory struct {
	Issuer string
	// Token is a personal access token of a machine user allowed to read
	// events and users.
	Token string
	HTTP  *http.Client
	// PageSize is how many events one request asks for; 0 means 1000.
	PageSize int
}

func (d *Directory) do(ctx context.Context, method, path string, body any) (int, []byte, error) {
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(d.Issuer, "/")+path, payload)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+d.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := d.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return resp.StatusCode, raw, nil
}

// eventsResponse is ZITADEL's answer to an event search.
type eventsResponse struct {
	Events []struct {
		Aggregate struct {
			ID   string          `json:"id"`
			Type json.RawMessage `json:"type"`
		} `json:"aggregate"`
		Sequence     json.Number     `json:"sequence"`
		CreationDate time.Time       `json:"creationDate"`
		Type         json.RawMessage `json:"type"`
	} `json:"events"`
}

// typeName reads an event or aggregate type, which ZITADEL writes either as
// a string or as {"type": "...", "localized": {...}}.
func typeName(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var o struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(raw, &o)
	return o.Type
}

// search runs one event search and returns its events, oldest first.
func (d *Directory) search(ctx context.Context, query map[string]any) (eventsResponse, error) {
	var page eventsResponse
	status, raw, err := d.do(ctx, http.MethodPost, "/admin/v1/events/_search", query)
	if err != nil {
		return page, err
	}
	if status != http.StatusOK {
		return page, fmt.Errorf("event search: HTTP %d: %.200s", status, raw)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&page); err != nil {
		return page, fmt.Errorf("event search: %w", err)
	}
	return page, nil
}

// pageOverlap is how far before the newest event read so far each next
// page starts. from is exclusive, so starting at the newest event itself
// would skip others of the same time that the page cut off, and events the
// provider commits slightly out of order land a little in the past.
const pageOverlap = 5 * time.Minute

// Events returns the account events recorded after from (the zero time
// means from the start), oldest first, reading page after page. Pages
// overlap, and events read before, by (aggregate, sequence), are skipped.
func (d *Directory) Events(ctx context.Context, from time.Time) ([]model.IdentityEvent, error) {
	size := d.PageSize
	if size <= 0 {
		size = 1000
	}
	var out []model.IdentityEvent
	seen := map[string]bool{}
	cursor, newest := from, from
	for {
		query := map[string]any{"asc": true, "limit": size, "aggregateTypes": []string{"user"}, "eventTypes": userEventTypes}
		if !cursor.IsZero() {
			query["from"] = cursor.UTC().Format(time.RFC3339Nano)
		}
		page, err := d.search(ctx, query)
		if err != nil {
			return nil, err
		}
		fresh := 0
		for _, ev := range page.Events {
			id := ev.Aggregate.ID + "/" + ev.Sequence.String()
			if seen[id] {
				continue
			}
			seen[id] = true
			fresh++
			if ev.CreationDate.After(newest) {
				newest = ev.CreationDate
			}
			if t := typeName(ev.Aggregate.Type); ev.Aggregate.ID != "" && (t == "" || t == "user") {
				out = append(out, model.IdentityEvent{ID: id, Subject: ev.Aggregate.ID, Type: typeName(ev.Type), At: ev.CreationDate})
			}
		}
		if len(page.Events) < size {
			return out, nil
		}
		// The next page starts pageOverlap before the newest event; when that
		// would not move on, as when the overlap holds a page of events read
		// before, just before the newest one (ZITADEL keeps microseconds);
		// failing that, at it. Only more than a page of events in one
		// microsecond can still be missed.
		next := newest.Add(-pageOverlap)
		if fresh == 0 || !next.After(cursor) {
			next = newest.Add(-time.Microsecond)
		}
		if !next.After(cursor) {
			next = newest
		}
		if !next.After(cursor) {
			return nil, fmt.Errorf("event search: no progress past %s", newest.Format(time.RFC3339Nano))
		}
		cursor = next
	}
}

// Removed reports whether the provider recorded the removal of the account
// with this subject. Only the event search, which needs the same token as
// the user API, says so; a pushed event could be forged by any service that
// can read the signing key.
func (d *Directory) Removed(ctx context.Context, subject string) (bool, error) {
	page, err := d.search(ctx, map[string]any{"asc": true, "limit": 10, "aggregateTypes": []string{"user"},
		"aggregateId": subject, "eventTypes": []string{model.RemovalEvent}})
	if err != nil {
		return false, err
	}
	for _, ev := range page.Events {
		if ev.Aggregate.ID == subject && typeName(ev.Type) == model.RemovalEvent {
			return true, nil
		}
	}
	return false, nil
}

// userStates maps ZITADEL's user states to what they mean here. An initial
// user has not finished signing up, and has no account here yet. A locked
// user counts as active (see userEventTypes); ZITADEL locks only active
// users, so a locked one is not deactivated as well.
var userStates = map[string]model.IdentityState{
	"USER_STATE_ACTIVE":   model.IdentityActive,
	"USER_STATE_INITIAL":  model.IdentityActive,
	"USER_STATE_LOCKED":   model.IdentityActive,
	"USER_STATE_INACTIVE": model.IdentityDeactivated,
	"USER_STATE_DELETED":  model.IdentityRemoved,
}

// userNotFoundID is the error ID ZITADEL gives a user it does not know
// (v4, checked against the local stack). Its message is translated, and
// other 404s, such as an unknown instance or path, carry other IDs.
const userNotFoundID = "QUERY-Dfbg2"

// userNotFound reports whether a 404 body is ZITADEL's error for a missing
// user, rather than any other not-found.
func userNotFound(raw []byte) bool {
	var body struct {
		Code    json.Number `json:"code"`
		Details []struct {
			ID string `json:"id"`
		} `json:"details"`
	}
	if json.Unmarshal(raw, &body) != nil || body.Code.String() != "5" {
		return false
	}
	for _, d := range body.Details {
		if d.ID == userNotFoundID {
			return true
		}
	}
	return false
}

// State reads the account's state now. A user the provider says it does not
// know is IdentityRemoved; any other answer that is not a user is an error.
func (d *Directory) State(ctx context.Context, subject string) (model.IdentityState, error) {
	status, raw, err := d.do(ctx, http.MethodGet, "/v2/users/"+url.PathEscape(subject), nil)
	if err != nil {
		return "", err
	}
	switch status {
	case http.StatusOK:
	case http.StatusNotFound:
		if userNotFound(raw) {
			return model.IdentityRemoved, nil
		}
		return "", fmt.Errorf("user: HTTP 404 that is not a missing user: %.200s", raw)
	default:
		return "", fmt.Errorf("user: HTTP %d: %.200s", status, raw)
	}
	var body struct {
		User struct {
			State string `json:"state"`
		} `json:"user"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return "", fmt.Errorf("user: %w", err)
	}
	state, ok := userStates[body.User.State]
	if !ok {
		return "", fmt.Errorf("user: unknown state %q", body.User.State)
	}
	return state, nil
}

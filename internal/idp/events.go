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
// here; the identity repository's README names them.
var userEventTypes = []string{model.RemovalEvent, "user.deactivated", "user.reactivated", "user.locked", "user.unlocked"}

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

// Events returns the account events recorded after from (the zero time
// means from the start), oldest first, reading page after page.
func (d *Directory) Events(ctx context.Context, from time.Time) ([]model.IdentityEvent, error) {
	size := d.PageSize
	if size <= 0 {
		size = 1000
	}
	var out []model.IdentityEvent
	seen := map[string]bool{}
	for {
		query := map[string]any{"asc": true, "limit": size, "aggregateTypes": []string{"user"}, "eventTypes": userEventTypes}
		if !from.IsZero() {
			query["from"] = from.UTC().Format(time.RFC3339Nano)
		}
		status, raw, err := d.do(ctx, http.MethodPost, "/admin/v1/events/_search", query)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("event search: HTTP %d: %.200s", status, raw)
		}
		var page eventsResponse
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&page); err != nil {
			return nil, fmt.Errorf("event search: %w", err)
		}
		fresh := 0
		for _, ev := range page.Events {
			id := ev.Aggregate.ID + "/" + ev.Sequence.String()
			if seen[id] {
				continue
			}
			seen[id] = true
			fresh++
			if ev.CreationDate.After(from) {
				from = ev.CreationDate
			}
			if t := typeName(ev.Aggregate.Type); ev.Aggregate.ID != "" && (t == "" || t == "user") {
				out = append(out, model.IdentityEvent{ID: id, Subject: ev.Aggregate.ID, Type: typeName(ev.Type), At: ev.CreationDate})
			}
		}
		// A short page is the last; a full page of events seen before means
		// more than a page share one time, which paging by time cannot pass.
		if len(page.Events) < size || fresh == 0 {
			return out, nil
		}
	}
}

// userStates maps ZITADEL's user states to what they mean here. An initial
// user has not finished signing up, and has no account here yet.
var userStates = map[string]model.IdentityState{
	"USER_STATE_ACTIVE":   model.IdentityActive,
	"USER_STATE_INITIAL":  model.IdentityActive,
	"USER_STATE_INACTIVE": model.IdentityDeactivated,
	"USER_STATE_LOCKED":   model.IdentityLocked,
	"USER_STATE_DELETED":  model.IdentityRemoved,
}

// State reads the account's state now. A user the provider does not know
// is IdentityRemoved.
func (d *Directory) State(ctx context.Context, subject string) (model.IdentityState, error) {
	status, raw, err := d.do(ctx, http.MethodGet, "/v2/users/"+url.PathEscape(subject), nil)
	if err != nil {
		return "", err
	}
	switch status {
	case http.StatusOK:
	case http.StatusNotFound:
		return model.IdentityRemoved, nil
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

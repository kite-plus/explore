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

// userChanges maps the provider's user event types to what they mean here.
var userChanges = map[string]model.IdentityChange{
	"user.removed":     model.IdentityRemoved,
	"user.deactivated": model.IdentityDeactivated,
	"user.reactivated": model.IdentityReactivated,
	"user.locked":      model.IdentityLocked,
	"user.unlocked":    model.IdentityUnlocked,
}

// UserEventTypes are the provider's event types that change an account here.
func UserEventTypes() []string {
	return []string{"user.removed", "user.deactivated", "user.reactivated", "user.locked", "user.unlocked"}
}

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
	change, known := userChanges[p.EventType]
	if !known || (p.AggregateType != "" && p.AggregateType != "user") {
		return model.IdentityEvent{}, false, nil
	}
	if p.AggregateID == "" || p.Sequence == "" || p.CreatedAt.IsZero() {
		return model.IdentityEvent{}, false, errors.New("event: missing aggregateID, sequence or created_at")
	}
	return model.IdentityEvent{ID: eventID(p.AggregateID, p.Sequence.String()), Subject: p.AggregateID,
		Change: change, At: p.CreatedAt}, true, nil
}

// An event is the sequence-th of its aggregate.
func eventID(aggregate, sequence string) string {
	return "user/" + aggregate + "/" + sequence
}

// Events reads the user events the provider recorded, for reconciling ones
// a push lost. Only ZITADEL's event search is implemented.
type Events struct {
	Issuer string
	// Token is a personal access token of a service user allowed to read
	// events.
	Token string
	HTTP  *http.Client
	// PageSize is how many events one request asks for; 0 means 500.
	PageSize int
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

// Since returns the account events recorded from since (inclusive; the zero
// time means from the start) up to until, oldest first.
func (e *Events) Since(ctx context.Context, since, until time.Time) ([]model.IdentityEvent, error) {
	size := e.PageSize
	if size <= 0 {
		size = 500
	}
	client := e.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	var out []model.IdentityEvent
	seen := map[string]bool{}
	for {
		dateRange := map[string]any{"until": until.UTC().Format(time.RFC3339Nano)}
		if !since.IsZero() {
			dateRange["since"] = since.UTC().Format(time.RFC3339Nano)
		}
		body, _ := json.Marshal(map[string]any{
			"asc": true, "limit": size, "aggregate_types": []string{"user"},
			"event_types": UserEventTypes(), "range": dateRange,
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(e.Issuer, "/")+"/admin/v1/events/_search", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+e.Token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("event search: HTTP %d: %.200s", resp.StatusCode, raw)
		}
		var page eventsResponse
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&page); err != nil {
			return nil, fmt.Errorf("event search: %w", err)
		}
		fresh := 0
		for _, ev := range page.Events {
			id := eventID(ev.Aggregate.ID, ev.Sequence.String())
			if seen[id] {
				continue
			}
			seen[id] = true
			fresh++
			if ev.CreationDate.After(since) {
				since = ev.CreationDate
			}
			change, known := userChanges[typeName(ev.Type)]
			if t := typeName(ev.Aggregate.Type); known && (t == "" || t == "user") && ev.Aggregate.ID != "" {
				out = append(out, model.IdentityEvent{ID: id, Subject: ev.Aggregate.ID, Change: change, At: ev.CreationDate})
			}
		}
		// A short page is the last; a page of events all seen before means
		// more than a page share one time, which paging by time cannot pass.
		if len(page.Events) < size || fresh == 0 {
			return out, nil
		}
	}
}

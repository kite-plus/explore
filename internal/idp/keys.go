package idp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// keyRefetch is how soon after a fetch an unknown key ID may fetch the
// provider's keys again. Without it every forged token naming a made-up
// key would cost a request to the provider.
const keyRefetch = 30 * time.Second

// signingAlgs are the asymmetric algorithms a provider may sign with; the
// verifier narrows them further to the ones discovery names.
var signingAlgs = []jose.SignatureAlgorithm{jose.RS256, jose.RS384, jose.RS512, jose.ES256, jose.ES384, jose.ES512,
	jose.PS256, jose.PS384, jose.PS512, jose.EdDSA}

// keySet verifies token signatures against the provider's published keys
// (an oidc.KeySet). It fetches them on first use and again for a key ID it
// does not know, at most every keyRefetch.
type keySet struct {
	url    string
	client *http.Client
	now    func() time.Time

	mu      sync.Mutex
	keys    []jose.JSONWebKey
	fetched time.Time
	failed  error
}

// fetchFailure carries, through the context go-oidc passes along, whether a
// verification failed for want of the keys rather than over the token.
type fetchFailure struct{ err error }

type fetchFailureKey struct{}

func withFetchFailure(ctx context.Context) (context.Context, *fetchFailure) {
	f := &fetchFailure{}
	return context.WithValue(ctx, fetchFailureKey{}, f), f
}

func (k *keySet) VerifySignature(ctx context.Context, raw string) ([]byte, error) {
	jws, err := jose.ParseSigned(raw, signingAlgs)
	if err != nil {
		return nil, fmt.Errorf("malformed token: %w", err)
	}
	if len(jws.Signatures) != 1 {
		return nil, errors.New("token has more than one signature")
	}
	kid := jws.Signatures[0].Header.KeyID
	keys, err := k.current(ctx, false)
	if err == nil {
		if payload, ok := verifyWith(jws, keys, kid); ok {
			return payload, nil
		}
		keys, err = k.current(ctx, true)
	}
	if err != nil {
		if f, ok := ctx.Value(fetchFailureKey{}).(*fetchFailure); ok {
			f.err = err
		}
		return nil, fmt.Errorf("fetching the provider's keys: %w", err)
	}
	if payload, ok := verifyWith(jws, keys, kid); ok {
		return payload, nil
	}
	return nil, errors.New("signature matches none of the provider's keys")
}

func verifyWith(jws *jose.JSONWebSignature, keys []jose.JSONWebKey, kid string) ([]byte, bool) {
	for _, key := range keys {
		if kid != "" && key.KeyID != kid {
			continue
		}
		if payload, err := jws.Verify(&key); err == nil {
			return payload, true
		}
	}
	return nil, false
}

// current returns the cached keys, fetching them when there are none or,
// with refresh, when the last fetch is older than keyRefetch.
func (k *keySet) current(ctx context.Context, refresh bool) ([]jose.JSONWebKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	recent := !k.fetched.IsZero() && k.now().Sub(k.fetched) < keyRefetch
	switch {
	case k.keys != nil && (!refresh || recent):
		return k.keys, nil
	case k.keys == nil && recent:
		// A failed fetch also waits, so an unreachable provider is not
		// asked again for every token.
		return nil, k.failed
	}
	k.fetched = k.now()
	set, err := k.fetch(ctx)
	if err != nil {
		// Keys fetched before stay in use; with none, the failure counts.
		if k.keys == nil {
			k.failed = err
			return nil, err
		}
		return k.keys, nil
	}
	k.keys, k.failed = set.Keys, nil
	return k.keys, nil
}

func (k *keySet) fetch(ctx context.Context) (jose.JSONWebKeySet, error) {
	var set jose.JSONWebKeySet
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.url, nil)
	if err != nil {
		return set, err
	}
	resp, err := k.client.Do(req)
	if err != nil {
		return set, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return set, err
	}
	if resp.StatusCode != http.StatusOK {
		return set, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if err := json.Unmarshal(body, &set); err != nil {
		return set, err
	}
	if set.Keys == nil {
		set.Keys = []jose.JSONWebKey{}
	}
	return set, nil
}

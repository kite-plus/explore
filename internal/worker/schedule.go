package worker

import (
	"time"

	"github.com/kite-plus/explore/internal/policy"
)

// nextInterval is the polling interval after a successful fetch: back to
// the base when the feed changed, stretched by half when it did not.
func nextInterval(prev time.Duration, changed bool) time.Duration {
	if changed || prev < policy.FetchInterval {
		return policy.FetchInterval
	}
	return min(time.Duration(float64(prev)*1.5), policy.MaxFetchInterval)
}

// backoff is the delay after the given number of consecutive failures. A
// server's Retry-After wins, within the same bounds.
func backoff(failures int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return max(policy.FetchInterval, min(retryAfter, policy.MaxBackoff))
	}
	d := policy.FetchInterval
	for i := 1; i < failures && d < policy.MaxBackoff; i++ {
		d *= 2
	}
	return min(d, policy.MaxBackoff)
}

// jitter spreads a delay over 90% to 110% so blogs listed together are not
// fetched together forever. r is uniform in [0, 1).
func jitter(d time.Duration, r float64) time.Duration {
	return time.Duration(float64(d) * (0.9 + 0.2*r))
}

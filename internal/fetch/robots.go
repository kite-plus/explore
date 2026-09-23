package fetch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/jimsmart/grobotstxt"

	"github.com/kite-plus/explore/internal/policy"
)

type robotsEntry struct {
	body     string
	allowAll bool
	expires  time.Time
}

// robotsAllow applies RFC 9309: a 4xx robots.txt allows everything, while a
// 5xx or an unreachable server means nothing may be fetched.
func (c *Client) robotsAllow(ctx context.Context, u *url.URL) error {
	origin := u.Scheme + "://" + u.Host

	c.mu.Lock()
	entry, ok := c.robots[origin]
	c.mu.Unlock()
	if !ok || c.now().After(entry.expires) {
		var err error
		entry, err = c.fetchRobots(ctx, origin)
		if err != nil {
			// Not cached: the next attempt asks again.
			return err
		}
		c.mu.Lock()
		c.robots[origin] = entry
		c.mu.Unlock()
	}

	if entry.allowAll {
		return nil
	}
	m := grobotstxt.NewRobotsMatcher()
	if m.AgentAllowed(entry.body, policy.UserAgentToken, u.String()) {
		return nil
	}
	return &RobotsError{Explicit: m.EverSeenSpecificAgent()}
}

func (c *Client) fetchRobots(ctx context.Context, origin string) (robotsEntry, error) {
	u, err := url.Parse(origin + "/robots.txt")
	if err != nil {
		return robotsEntry{}, err
	}
	// Anything past the size limit may be ignored, so it is cut off rather
	// than treated as an error.
	resp, err := c.do(ctx, u, Request{Accept: acceptRobots, MaxBytes: policy.MaxRobotsBytes}, true)
	if err != nil {
		// Both errors stay in the chain, so a private address is still
		// reported as one.
		return robotsEntry{}, fmt.Errorf("%w: %w", ErrRobotsUnreachable, err)
	}
	expires := c.now().Add(policy.RobotsTTL)
	switch {
	case resp.Status >= 200 && resp.Status < 300:
		return robotsEntry{body: string(resp.Body), expires: expires}, nil
	case resp.Status == http.StatusTooManyRequests:
		return robotsEntry{}, fmt.Errorf("%w: status %d", ErrRobotsUnreachable, resp.Status)
	case resp.Status >= 400 && resp.Status < 500:
		return robotsEntry{allowAll: true, expires: expires}, nil
	default:
		return robotsEntry{}, fmt.Errorf("%w: status %d", ErrRobotsUnreachable, resp.Status)
	}
}

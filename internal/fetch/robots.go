package fetch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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
	entry, err := c.robotsFor(ctx, u.Scheme+"://"+u.Host)
	if err != nil {
		return err
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

// Sitemaps returns the sitemaps named in the robots.txt of site's origin,
// read from the same cache as its rules.
func (c *Client) Sitemaps(ctx context.Context, site *url.URL) ([]string, error) {
	entry, err := c.robotsFor(ctx, site.Scheme+"://"+site.Host)
	if err != nil {
		return nil, err
	}
	var out []string
	for line := range strings.Lines(entry.body) {
		if hash := strings.IndexByte(line, '#'); hash >= 0 {
			line = line[:hash]
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "sitemap") {
			continue
		}
		u, err := site.Parse(strings.TrimSpace(value))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			continue
		}
		out = append(out, u.String())
	}
	return out, nil
}

func (c *Client) robotsFor(ctx context.Context, origin string) (robotsEntry, error) {
	c.mu.Lock()
	entry, ok := c.robots[origin]
	c.mu.Unlock()
	if ok && !c.now().After(entry.expires) {
		return entry, nil
	}
	entry, err := c.fetchRobots(ctx, origin)
	if err != nil {
		// Not cached: the next attempt asks again.
		return robotsEntry{}, err
	}
	c.mu.Lock()
	c.robots[origin] = entry
	c.mu.Unlock()
	return entry, nil
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

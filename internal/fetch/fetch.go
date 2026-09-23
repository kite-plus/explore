// Package fetch is how Explore talks to other people's servers. It
// identifies itself, honors robots.txt, bounds every response and refuses
// to connect to anything but public addresses.
package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kite-plus/explore/internal/policy"
)

// Accept headers. Feed requests list */* because Halo matches its feed
// routes on Accept and would not answer a narrower list.
const (
	AcceptFeed   = "application/rss+xml, application/atom+xml, application/feed+json, application/xml;q=0.9, text/xml;q=0.9, */*;q=0.8"
	AcceptHTML   = "text/html, application/xhtml+xml;q=0.9, */*;q=0.8"
	acceptRobots = "text/plain, */*;q=0.8"
)

var (
	ErrBadURL            = errors.New("only public http and https URLs on ports 80 and 443 can be fetched")
	ErrBlockedAddress    = errors.New("address is not public")
	ErrTooLarge          = errors.New("response exceeds the size limit")
	ErrTooManyRedirects  = errors.New("too many redirects")
	ErrRobotsDisallowed  = errors.New("disallowed by robots.txt")
	ErrRobotsUnreachable = errors.New("robots.txt is unreachable")
)

// BlockedError names the address a connection was refused for.
type BlockedError struct{ Addr string }

func (e *BlockedError) Error() string { return "address is not public: " + e.Addr }

func (e *BlockedError) Unwrap() error { return ErrBlockedAddress }

// RobotsError is a refusal by robots.txt. Explicit is set when the rule
// comes from a group naming KiteExplore rather than the group for every
// crawler: only then has the author turned Explore away in particular.
type RobotsError struct{ Explicit bool }

func (e *RobotsError) Error() string {
	if e.Explicit {
		return ErrRobotsDisallowed.Error() + " for " + policy.UserAgentToken
	}
	return ErrRobotsDisallowed.Error()
}

func (e *RobotsError) Unwrap() error { return ErrRobotsDisallowed }

// UserAgent is what Explore sends; the URL explains the crawler to whoever
// finds it in their logs.
func UserAgent(version, publicURL string) string {
	return fmt.Sprintf("%s/%s (+%s/bot)", policy.UserAgentToken, strings.TrimPrefix(version, "v"), strings.TrimRight(publicURL, "/"))
}

// Options configures a Client.
type Options struct {
	UserAgent string
	// AllowPrivate lifts the address and port checks. It exists for tests
	// and local development only.
	AllowPrivate bool
	// Timeout overrides policy.FetchTimeout, for tests.
	Timeout time.Duration
}

// Request is one GET. ETag and LastModified are the validators of an
// earlier response; at most one of them is sent.
type Request struct {
	URL          string
	Accept       string
	MaxBytes     int64
	ETag         string
	LastModified string
}

// Response is a completed GET, whatever its status.
type Response struct {
	Status       int
	Body         []byte
	ContentType  string
	ETag         string
	LastModified string
	// URL is where the body came from, after redirects.
	URL        string
	Redirected bool
	// PermanentRedirect is set when every redirect was a 301 or 308, so the
	// final URL can replace the requested one.
	PermanentRedirect bool
	RetryAfter        time.Duration
}

// Client fetches with Explore's rules. It is safe for concurrent use.
type Client struct {
	opts Options
	http *http.Client
	now  func() time.Time

	mu     sync.Mutex
	robots map[string]robotsEntry
}

// New returns a Client.
func New(o Options) *Client {
	c := &Client{opts: o, now: time.Now, robots: make(map[string]robotsEntry)}
	timeout := o.Timeout
	if timeout == 0 {
		timeout = policy.FetchTimeout
	}
	dialer := &net.Dialer{Timeout: policy.ConnectTimeout, Control: c.control}
	c.http = &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   policy.TLSTimeout,
			ResponseHeaderTimeout: policy.HeaderTimeout,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          64,
			MaxIdleConnsPerHost:   2,
			IdleConnTimeout:       90 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > policy.MaxRedirects {
				return ErrTooManyRedirects
			}
			if err := c.checkURL(req.URL); err != nil {
				return err
			}
			if t, ok := req.Context().Value(trackerKey{}).(*tracker); ok {
				t.count++
				if code := req.Response.StatusCode; code != http.StatusMovedPermanently && code != http.StatusPermanentRedirect {
					t.permanent = false
				}
				// Where a redirect leads must be allowed too; robots.txt
				// itself is exempt, or fetching it would need itself.
				if !t.robotsFile {
					return c.robotsAllow(req.Context(), req.URL)
				}
			}
			return nil
		},
	}
	return c
}

type trackerKey struct{}

type tracker struct {
	count      int
	permanent  bool
	robotsFile bool
}

// Get fetches a URL after checking robots.txt for it.
func (c *Client) Get(ctx context.Context, r Request) (*Response, error) {
	u, err := url.Parse(r.URL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadURL, err)
	}
	if err := c.checkURL(u); err != nil {
		return nil, err
	}
	if err := c.robotsAllow(ctx, u); err != nil {
		return nil, err
	}
	return c.do(ctx, u, r, false)
}

func (c *Client) Probe(ctx context.Context, rawURL string) (int, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrBadURL, err)
	}
	if err := c.checkURL(u); err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, policy.LinkCheckTimeout)
	defer cancel()
	if err := c.robotsAllow(ctx, u); err != nil {
		return 0, err
	}
	status, err := c.probeRequest(ctx, u, http.MethodHead)
	if err != nil || status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable || status >= 200 && status < 300 {
		return status, err
	}
	return c.probeRequest(ctx, u, http.MethodGet)
}

func (c *Client) probeRequest(ctx context.Context, u *url.URL, method string) (int, error) {
	ctx = context.WithValue(ctx, trackerKey{}, &tracker{})
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrBadURL, err)
	}
	req.Header.Set("User-Agent", c.opts.UserAgent)
	req.Header.Set("Accept", AcceptHTML)
	if method == http.MethodGet {
		req.Header.Set("Range", "bytes=0-0")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}

// do performs one GET. A robots.txt request is cut off at the size limit
// instead of failing, and its redirects skip the robots check.
func (c *Client) do(ctx context.Context, u *url.URL, r Request, robotsFile bool) (*Response, error) {
	t := &tracker{permanent: true, robotsFile: robotsFile}
	ctx = context.WithValue(ctx, trackerKey{}, t)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadURL, err)
	}
	req.Header.Set("User-Agent", c.opts.UserAgent)
	accept := r.Accept
	if accept == "" {
		accept = AcceptFeed
	}
	req.Header.Set("Accept", accept)
	// One validator, Last-Modified when there is one: compression layers
	// rewrite ETags (Apache appends -gzip, nginx and Cloudflare weaken them),
	// and WordPress answers 304 only when every validator it gets matches.
	// See docs/design/worker.md section 3.6.
	switch {
	case r.LastModified != "":
		req.Header.Set("If-Modified-Since", r.LastModified)
	case r.ETag != "":
		req.Header.Set("If-None-Match", r.ETag)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	limit := r.MaxBytes
	if limit <= 0 {
		limit = policy.MaxFeedBytes
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		if !robotsFile {
			return nil, ErrTooLarge
		}
		body = body[:limit]
	}

	out := &Response{
		Status:            resp.StatusCode,
		Body:              body,
		ContentType:       resp.Header.Get("Content-Type"),
		ETag:              resp.Header.Get("ETag"),
		LastModified:      resp.Header.Get("Last-Modified"),
		URL:               resp.Request.URL.String(),
		Redirected:        t.count > 0,
		PermanentRedirect: t.count > 0 && t.permanent,
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		out.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), c.now())
	}
	return out, nil
}

// checkURL rejects what can be judged from the URL alone. The connection
// itself is checked again in control, after DNS has been resolved.
func (c *Client) checkURL(u *url.URL) error {
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return ErrBadURL
	}
	if c.opts.AllowPrivate {
		return nil
	}
	if p := u.Port(); p != "" && p != "80" && p != "443" {
		return ErrBadURL
	}
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil && !Public(ip) {
		return &BlockedError{Addr: ip.String()}
	}
	return nil
}

// control runs for every connection, redirects included, with the address
// DNS resolved to. Checking here rather than on the URL defeats hosts that
// resolve to private addresses and DNS rebinding.
func (c *Client) control(_, address string, _ syscall.RawConn) error {
	if c.opts.AllowPrivate {
		return nil
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if port != "80" && port != "443" {
		return &BlockedError{Addr: address}
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}
	if !Public(ip) {
		return &BlockedError{Addr: ip.String()}
	}
	return nil
}

var nonPublic = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),   // NAT64 can reach private IPv4
	netip.MustParsePrefix("64:ff9b:1::/48"), // local-use NAT64
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"), // 6to4 embeds arbitrary IPv4
	netip.MustParsePrefix("fec0::/10"),
}

// Public reports whether ip is a public unicast address.
func Public(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return false
	}
	for _, p := range nonPublic {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

func parseRetryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs > 0 {
			return time.Duration(secs) * time.Second
		}
		return 0
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

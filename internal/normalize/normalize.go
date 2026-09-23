// Package normalize turns a parsed feed into the entries Explore keeps. It
// applies the link, identity, date and excerpt rules of docs/design/worker.md
// section 5 and does no I/O.
package normalize

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"

	"github.com/kite-plus/explore/internal/feed"
	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/policy"
)

// Blog is what normalization needs to know about the blog a feed belongs to.
type Blog struct {
	Host         string
	ExtraDomains []string
	ShowExcerpt  bool
	FeedURL      string
}

// Stats counts what happened to the feed's items. Every item lands in
// exactly one of Valid, NoLink, OffDomain, NoTitle or Duplicate.
type Stats struct {
	Total     int
	Valid     int
	NoLink    int
	OffDomain int
	NoTitle   int
	Duplicate int

	Undated   int // valid items without a usable publish date
	Untrusted int // valid items whose date was ruled untrusted
	Trusted   int

	// OffDomainHosts counts where off-domain links pointed.
	OffDomainHosts map[string]int `json:",omitempty"`
}

// Result is a blog's snapshot: at most policy.EntriesPerBlog entries.
type Result struct {
	Entries []model.Entry
	Stats   Stats
}

// Snapshot normalizes every item of f for blog b.
func Snapshot(f *feed.Feed, b Blog) Result {
	base, _ := url.Parse(b.FeedURL)
	host := strings.ToLower(b.Host)

	var res Result
	res.Stats.Total = len(f.Items)
	seen := make(map[string]bool, len(f.Items))
	valid := make([]model.Entry, 0, len(f.Items))

	for _, it := range f.Items {
		link, ok := resolve(base, it.Link)
		if !ok {
			res.Stats.NoLink++
			continue
		}
		if !SameSite(link.Hostname(), host, b.ExtraDomains) {
			res.Stats.OffDomain++
			if res.Stats.OffDomainHosts == nil {
				res.Stats.OffDomainHosts = make(map[string]int)
			}
			res.Stats.OffDomainHosts[strings.ToLower(link.Hostname())]++
			continue
		}
		title := Truncate(PlainText(it.Title), policy.TitleMaxRunes)
		if title == "" {
			res.Stats.NoTitle++
			continue
		}
		id := identity(it.ID, link)
		if seen[id] {
			res.Stats.Duplicate++
			continue
		}
		seen[id] = true

		e := model.Entry{
			Identity:    id,
			URL:         link.String(),
			Title:       title,
			PublishedAt: published(it),
			Categories:  categories(it.Categories),
		}
		if b.ShowExcerpt {
			e.Excerpt = excerpt(it)
			e.ImageURL = imageURL(it, link)
		}
		valid = append(valid, e)
	}

	res.Stats.Valid = len(valid)
	trust(valid, &res.Stats)

	// Newest first; undated last; ties keep feed order.
	sort.SliceStable(valid, func(i, j int) bool {
		a, c := valid[i].PublishedAt, valid[j].PublishedAt
		switch {
		case a == nil:
			return false
		case c == nil:
			return true
		default:
			return a.After(*c)
		}
	})
	if len(valid) > policy.EntriesPerBlog {
		valid = valid[:policy.EntriesPerBlog]
	}
	res.Entries = valid
	return res
}

// resolve makes a link absolute against the feed URL and accepts only http
// and https, which keeps javascript: and data: links out of every page.
func resolve(base *url.URL, raw string) (*url.URL, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, false
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, false
	}
	return u, true
}

// SameSite reports whether a link host belongs to the blog: the same
// registrable domain as the blog's host, or one of its extra domains. IP
// addresses and single-label hosts must match exactly.
func SameSite(linkHost, blogHost string, extra []string) bool {
	linkHost = strings.TrimSuffix(strings.ToLower(linkHost), ".")
	blogHost = strings.TrimSuffix(strings.ToLower(blogHost), ".")
	if linkHost == "" {
		return false
	}
	if linkHost == blogHost {
		return true
	}
	for _, d := range extra {
		d = strings.TrimSuffix(strings.ToLower(d), ".")
		if d != "" && (linkHost == d || strings.HasSuffix(linkHost, "."+d)) {
			return true
		}
	}
	a, ok1 := registrable(linkHost)
	c, ok2 := registrable(blogHost)
	return ok1 && ok2 && a == c
}

func registrable(host string) (string, bool) {
	if net.ParseIP(host) != nil {
		return "", false
	}
	d, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return "", false
	}
	return d, true
}

// identity prefers the feed's own ID. Without one, the link stands in,
// normalized so trivial differences do not create duplicates.
func identity(id string, link *url.URL) string {
	if id != "" {
		if len(id) > policy.IdentityMaxBytes {
			sum := sha256.Sum256([]byte(id))
			return hex.EncodeToString(sum[:])
		}
		return id
	}
	u := *link
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if (u.Scheme == "http" && u.Port() == "80") || (u.Scheme == "https" && u.Port() == "443") {
		u.Host = u.Hostname()
	}
	u.Fragment = ""
	u.RawFragment = ""
	s := u.String()
	if len(s) > policy.IdentityMaxBytes {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}
	return s
}

func published(it feed.Item) *time.Time {
	t := it.Published
	if t == nil {
		t = it.Updated
	}
	if t == nil || t.Before(policy.EarliestDate) {
		return nil
	}
	utc := t.UTC()
	return &utc
}

// placeholderCategories are what blog systems file a post under when the
// author picked nothing; they say nothing about the post.
var placeholderCategories = map[string]bool{"uncategorized": true, "未分类": true, "默认分类": true}

// categories cleans a post's own categories: plain text, no duplicates by
// case, no placeholders, at most policy.CategoriesPerEntry of them.
func categories(raw []string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, c := range raw {
		c = Truncate(PlainText(c), policy.CategoryMaxRunes)
		key := strings.ToLower(c)
		if c == "" || seen[key] || placeholderCategories[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
		if len(out) == policy.CategoriesPerEntry {
			break
		}
	}
	return out
}

func excerpt(it feed.Item) string {
	src := it.Summary
	if strings.TrimSpace(PlainText(src)) == "" {
		src = it.Content
	}
	return Truncate(PlainText(src), policy.ExcerptMaxRunes)
}

// trust marks dates untrusted when too many items share one minute, the
// signature of a build that stamped every undated post with its own time.
// It only looks at the snapshot, so rebuilding the cache gives the same answer.
func trust(entries []model.Entry, st *Stats) {
	perMinute := make(map[int64]int)
	for _, e := range entries {
		if e.PublishedAt != nil {
			perMinute[e.PublishedAt.Unix()/60]++
		}
	}
	for i := range entries {
		e := &entries[i]
		switch {
		case e.PublishedAt == nil:
			st.Undated++
		case perMinute[e.PublishedAt.Unix()/60] > policy.SameMinuteLimit:
			st.Untrusted++
		default:
			e.DateTrusted = true
			st.Trusted++
		}
	}
}

// DetectGenerator names the blog system behind a generator string from a
// feed or a page's generator meta tag.
func DetectGenerator(raw string) model.Generator {
	s := strings.ToLower(raw)
	switch {
	case s == "":
		return model.GeneratorUnknown
	case strings.Contains(s, "wordpress"):
		return model.GeneratorWordPress
	case strings.Contains(s, "halo"):
		return model.GeneratorHalo
	case strings.Contains(s, "hugo"):
		return model.GeneratorHugo
	case strings.Contains(s, "hexo"):
		return model.GeneratorHexo
	case strings.Contains(s, "typecho"):
		return model.GeneratorTypecho
	case strings.Contains(s, "jekyll"):
		return model.GeneratorJekyll
	case strings.Contains(s, "ghost"):
		return model.GeneratorGhost
	case strings.Contains(s, "kite"):
		return model.GeneratorKite
	default:
		return model.GeneratorOther
	}
}

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/kite-plus/explore/internal/buildinfo"
	"github.com/kite-plus/explore/internal/check"
	"github.com/kite-plus/explore/internal/fetch"
	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/policy"
)

// surveyTimeout bounds one blog; discovery may try every default address.
const surveyTimeout = 90 * time.Second

// fullTextRunes is the median item length above which a feed is counted
// as carrying full posts rather than summaries.
const fullTextRunes = 500

func newSurveyCmd() *cobra.Command {
	var concurrency int
	var report bool
	cmd := &cobra.Command{
		Use:   "survey <file>",
		Short: "Check many blogs and measure their feeds",
		Long: "survey checks every blog listed in a file, one per line as \"<site> [feed]\",\n" +
			"and writes one JSON record per blog to standard output: the check report\n" +
			"and measurements of the feed, such as its size, whether conditional\n" +
			"requests work and how its dates cluster. Lines starting with # are skipped.\n\n" +
			"With --report the file holds such records instead, and survey prints a\n" +
			"Markdown summary per blog system without fetching anything. This is how\n" +
			"the E0 survey of docs/design/worker.md section 10 is run.",
		Example: "  explore survey blogs.txt > survey.jsonl\n" +
			"  explore survey --report survey.jsonl",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if report {
				recs, err := readRecords(args[0])
				if err != nil {
					return err
				}
				writeSurveyReport(cmd.OutOrStdout(), recs)
				return nil
			}
			inputs, err := readInputs(args[0])
			if err != nil {
				return err
			}
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			c := &check.Checker{Fetch: fetch.New(fetch.Options{
				UserAgent:    fetch.UserAgent(buildinfo.Version, cfg.PublicURL),
				AllowPrivate: cfg.AllowPrivateNetworks,
			})}
			return survey(cmd.Context(), c, inputs, max(concurrency, 1), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "blogs checked at the same time")
	cmd.Flags().BoolVar(&report, "report", false, "summarize a file of survey records instead of surveying")
	return cmd
}

func readInputs(path string) ([]check.Input, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var inputs []check.Input
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		in := check.Input{SiteURL: fields[0]}
		if len(fields) > 1 {
			in.FeedURL = fields[1]
		}
		inputs = append(inputs, in)
	}
	return inputs, sc.Err()
}

func survey(ctx context.Context, c *check.Checker, inputs []check.Input, workers int, out, progress io.Writer) error {
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	jobs := make(chan check.Input)
	var (
		mu       sync.Mutex
		done     int
		writeErr error
		wg       sync.WaitGroup
	)
	for range workers {
		wg.Go(func() {
			for in := range jobs {
				bctx, cancel := context.WithTimeout(ctx, surveyTimeout)
				rec := c.Survey(bctx, in)
				cancel()
				mu.Lock()
				done++
				if err := enc.Encode(rec); err != nil && writeErr == nil {
					writeErr = err
				}
				_, _ = fmt.Fprintf(progress, "[%d/%d] %s\n", done, len(inputs), surveyLine(rec))
				mu.Unlock()
			}
		})
	}
send:
	for _, in := range inputs {
		select {
		case jobs <- in:
		case <-ctx.Done():
			break send
		}
	}
	close(jobs)
	wg.Wait()
	if writeErr != nil {
		return writeErr
	}
	return ctx.Err()
}

func surveyLine(rec check.SurveyRecord) string {
	if rec.Report == nil {
		return rec.SiteURL + "  " + rec.Error
	}
	var failed []string
	for _, p := range rec.Report.Problems {
		if p.Severity == model.SeverityError {
			failed = append(failed, string(p.Code))
		}
	}
	result := "passed"
	if !rec.Report.Passed {
		result = strings.Join(failed, ",")
	}
	return fmt.Sprintf("%s  %s  %s", rec.SiteURL, rec.System, result)
}

func readRecords(path string) ([]check.SurveyRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var recs []check.SurveyRecord
	dec := json.NewDecoder(f)
	for {
		var rec check.SurveyRecord
		err := dec.Decode(&rec)
		if errors.Is(err, io.EOF) {
			return recs, nil
		}
		if err != nil {
			return nil, fmt.Errorf("record %d: %w", len(recs)+1, err)
		}
		recs = append(recs, rec)
	}
}

// otherGenerators share the report's "Other" column.
var otherGenerators = []model.Generator{
	model.GeneratorTypecho, model.GeneratorJekyll, model.GeneratorGhost, model.GeneratorKite, model.GeneratorOther,
}

// surveyGroup is a column of the survey report: every blog of some systems,
// and those of them whose feed was found.
type surveyGroup struct {
	name  string
	recs  []check.SurveyRecord
	found []check.SurveyRecord
}

func groupRecords(recs []check.SurveyRecord) []*surveyGroup {
	named := []struct {
		name string
		gens []model.Generator
	}{
		{"WordPress", []model.Generator{model.GeneratorWordPress}},
		{"Halo", []model.Generator{model.GeneratorHalo}},
		{"Hugo", []model.Generator{model.GeneratorHugo}},
		{"Hexo", []model.Generator{model.GeneratorHexo}},
		{"Other", otherGenerators},
		{"Unknown", []model.Generator{model.GeneratorUnknown}},
		{"All", nil},
	}
	groups := make([]*surveyGroup, len(named))
	for i, n := range named {
		g := &surveyGroup{name: n.name}
		for _, r := range recs {
			if r.Report == nil || (n.gens != nil && !slices.Contains(n.gens, r.System)) {
				continue
			}
			g.recs = append(g.recs, r)
			if r.Feed != nil {
				g.found = append(g.found, r)
			}
		}
		groups[i] = g
	}
	return groups
}

// count counts the found feeds matching f, as "n/found".
func (g *surveyGroup) count(f func(check.SurveyRecord) bool) string {
	n := 0
	for _, r := range g.found {
		if f(r) {
			n++
		}
	}
	return fmt.Sprintf("%d/%d", n, len(g.found))
}

func (g *surveyGroup) values(f func(*check.FeedStats) int, keep func(int) bool) []int {
	var xs []int
	for _, r := range g.found {
		if v := f(r.Feed); keep == nil || keep(v) {
			xs = append(xs, v)
		}
	}
	return xs
}

// percentile is the nearest-rank percentile; zero for no values.
func percentile(xs []int, p float64) int {
	if len(xs) == 0 {
		return 0
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	i := int(math.Ceil(p*float64(len(s)))) - 1
	return s[max(i, 0)]
}

func spread(xs []int, unit func(int) string) string {
	if len(xs) == 0 {
		return "-"
	}
	return fmt.Sprintf("%s / %s / %s", unit(percentile(xs, 0.5)), unit(percentile(xs, 0.9)), unit(slices.Max(xs)))
}

func kib(n int) string { return fmt.Sprintf("%d", (n+1023)/1024) }

func plain(n int) string { return fmt.Sprintf("%d", n) }

type surveyRow struct {
	label string
	cell  func(g *surveyGroup) string
}

var surveyRows = []surveyRow{
	{"Blogs", func(g *surveyGroup) string { return plain(len(g.recs)) }},
	{"Feed found", func(g *surveyGroup) string { return fmt.Sprintf("%d/%d", len(g.found), len(g.recs)) }},
	{"Check passed", func(g *surveyGroup) string {
		n := 0
		for _, r := range g.recs {
			if r.Report.Passed {
				n++
			}
		}
		return fmt.Sprintf("%d/%d", n, len(g.recs))
	}},
	{"Found by autodiscovery", func(g *surveyGroup) string {
		return g.count(func(r check.SurveyRecord) bool { return r.Report.DiscoveredBy == "autodiscovery" })
	}},
	{"Found at a default address", func(g *surveyGroup) string {
		return g.count(func(r check.SurveyRecord) bool { return r.Report.DiscoveredBy == "candidate" })
	}},
	{"Format rss / atom / json", func(g *surveyGroup) string {
		n := map[string]int{}
		for _, r := range g.found {
			n[r.Report.Format]++
		}
		return fmt.Sprintf("%d / %d / %d", n["rss"], n["atom"], n["json"])
	}},
	{"Size KiB p50 / p90 / max", func(g *surveyGroup) string {
		return spread(g.values(func(f *check.FeedStats) int { return f.Bytes }, nil), kib)
	}},
	{"Over 1 MiB", func(g *surveyGroup) string {
		return g.count(func(r check.SurveyRecord) bool { return r.Feed.Bytes > 1<<20 })
	}},
	{"Sends ETag or Last-Modified", func(g *surveyGroup) string {
		return g.count(func(r check.SurveyRecord) bool { return r.Feed.Conditional != "none" })
	}},
	{"Answers 304 to a conditional request", func(g *surveyGroup) string {
		return g.count(func(r check.SurveyRecord) bool { return r.Feed.Conditional == "304" })
	}},
	{"Items p50 / p90 / max", func(g *surveyGroup) string {
		return spread(g.values(func(f *check.FeedStats) int { return f.Items }, nil), plain)
	}},
	{"Items in the stream window p50 / p90 / max", func(g *surveyGroup) string {
		return spread(g.values(func(f *check.FeedStats) int { return f.InWindow }, nil), plain)
	}},
	{fmt.Sprintf("More than %d items in the window", policy.EntriesPerBlog), func(g *surveyGroup) string {
		return g.count(func(r check.SurveyRecord) bool { return r.Feed.InWindow > policy.EntriesPerBlog })
	}},
	{"Days covered p50 / p90 / max", func(g *surveyGroup) string {
		return spread(g.values(func(f *check.FeedStats) int { return f.SpanDays }, nil), plain)
	}},
	{fmt.Sprintf("Full posts (median item over %d runes)", fullTextRunes), func(g *surveyGroup) string {
		return g.count(func(r check.SurveyRecord) bool { return r.Feed.TextRunes > fullTextRunes })
	}},
	{"Summary runes p50 / p90 / max", func(g *surveyGroup) string {
		return spread(g.values(func(f *check.FeedStats) int { return f.SummaryRunes }, func(v int) bool { return v > 0 }), plain)
	}},
	{"Has undated items", func(g *surveyGroup) string {
		return g.count(func(r check.SurveyRecord) bool { return r.Feed.Undated > 0 })
	}},
	{"Has unreadable dates", func(g *surveyGroup) string {
		return g.count(func(r check.SurveyRecord) bool { return r.Feed.Unreadable > 0 })
	}},
	{"Same minute: 2+ / 3+ / over the limit", func(g *surveyGroup) string {
		var two, three, over int
		for _, r := range g.found {
			m := r.Feed.MaxSameMinute
			if m >= 2 {
				two++
			}
			if m >= 3 {
				three++
			}
			if m > policy.SameMinuteLimit {
				over++
			}
		}
		return fmt.Sprintf("%d / %d / %d", two, three, over)
	}},
	{"Has off-domain links", func(g *surveyGroup) string {
		return g.count(func(r check.SurveyRecord) bool { return r.Feed.OffDomain > 0 })
	}},
	{"Mostly off-domain links", func(g *surveyGroup) string {
		return g.count(func(r check.SurveyRecord) bool {
			linked := r.Feed.Items - r.Feed.NoLink
			return linked > 0 && float64(r.Feed.OffDomain)/float64(linked) > policy.OffDomainFailShare
		})
	}},
	{"Has untitled items", func(g *surveyGroup) string {
		return g.count(func(r check.SurveyRecord) bool { return r.Feed.NoTitle > 0 })
	}},
	{"Declares a charset other than UTF-8", func(g *surveyGroup) string {
		return g.count(func(r check.SurveyRecord) bool { return !utf8Label(r.Feed.Charset) })
	}},
}

func utf8Label(s string) bool {
	return s == "" || s == "utf-8" || s == "utf8"
}

func writeSurveyReport(w io.Writer, recs []check.SurveyRecord) {
	groups := groupRecords(recs)
	p := func(format string, args ...any) { _, _ = fmt.Fprintf(w, format, args...) }
	row := func(label string, cells []string) {
		p("| %s | %s |\n", label, strings.Join(cells, " | "))
	}
	header := func(first string) {
		names := make([]string, len(groups))
		seps := make([]string, len(groups))
		for i, g := range groups {
			names[i], seps[i] = g.name, "---"
		}
		row(first, names)
		row("---", seps)
	}

	header("Measure")
	for _, r := range surveyRows {
		cells := make([]string, len(groups))
		for i, g := range groups {
			cells[i] = r.cell(g)
		}
		row(r.label, cells)
	}

	counts := map[model.ProblemCode][]int{}
	for i, g := range groups {
		for _, r := range g.recs {
			seen := map[model.ProblemCode]bool{}
			for _, pr := range r.Report.Problems {
				if seen[pr.Code] {
					continue
				}
				seen[pr.Code] = true
				if counts[pr.Code] == nil {
					counts[pr.Code] = make([]int, len(groups))
				}
				counts[pr.Code][i]++
			}
		}
	}
	codes := make([]model.ProblemCode, 0, len(counts))
	for c := range counts {
		codes = append(codes, c)
	}
	last := len(groups) - 1
	slices.SortFunc(codes, func(a, b model.ProblemCode) int {
		if d := counts[b][last] - counts[a][last]; d != 0 {
			return d
		}
		return strings.Compare(string(a), string(b))
	})
	p("\n")
	header("Problem")
	for _, c := range codes {
		cells := make([]string, len(groups))
		for i, n := range counts[c] {
			cells[i] = plain(n)
		}
		row("`"+string(c)+"`", cells)
	}

	tally := func(title string, keys func(r check.SurveyRecord) []string) {
		n := map[string]int{}
		for _, r := range recs {
			for _, k := range keys(r) {
				n[k]++
			}
		}
		if len(n) == 0 {
			return
		}
		ks := make([]string, 0, len(n))
		for k := range n {
			ks = append(ks, k)
		}
		slices.SortFunc(ks, func(a, b string) int {
			if d := n[b] - n[a]; d != 0 {
				return d
			}
			return strings.Compare(a, b)
		})
		parts := make([]string, 0, len(ks))
		for _, k := range ks {
			parts = append(parts, fmt.Sprintf("%s ×%d", k, n[k]))
		}
		p("\n%s: %s\n", title, strings.Join(parts, ", "))
	}
	tally("Other systems", func(r check.SurveyRecord) []string {
		if r.Report == nil || !slices.Contains(otherGenerators, r.System) {
			return nil
		}
		return []string{string(r.System)}
	})
	tally("Charsets other than UTF-8", func(r check.SurveyRecord) []string {
		if r.Feed == nil || utf8Label(r.Feed.Charset) {
			return nil
		}
		return []string{r.Feed.Charset}
	})
	tally("Conditional request answers", func(r check.SurveyRecord) []string {
		if r.Feed == nil || r.Feed.Conditional == "none" {
			return nil
		}
		return []string{r.Feed.Conditional}
	})
	tally("HTTP errors", func(r check.SurveyRecord) []string {
		if r.Report == nil {
			return nil
		}
		var ks []string
		for _, pr := range r.Report.Problems {
			if pr.Code == model.ProblemHTTPError {
				ks = append(ks, errorKind(pr.Detail))
			}
		}
		return ks
	})
	tally("Invalid input", func(r check.SurveyRecord) []string {
		if r.Error == "" {
			return nil
		}
		return []string{r.Error}
	})
}

// errorKind drops the addresses from a fetch error so the same failure on
// different blogs is counted together.
func errorKind(detail string) string {
	switch {
	case strings.HasPrefix(detail, "HTTP "):
		return detail
	case strings.Contains(detail, "no such host"):
		return "no such host"
	case strings.Contains(detail, "deadline exceeded"), strings.Contains(detail, "timeout"):
		return "timeout"
	case strings.Contains(detail, "certificate"), strings.Contains(detail, "tls"):
		return "tls"
	case strings.Contains(detail, "connection refused"):
		return "connection refused"
	case strings.Contains(detail, "connection reset"), strings.Contains(detail, "EOF"):
		return "connection reset"
	case strings.Contains(detail, "not public"):
		return "address not public"
	case strings.Contains(detail, "redirects"):
		return "too many redirects"
	}
	return "other"
}

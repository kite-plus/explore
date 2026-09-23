package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kite-plus/explore/internal/check"
	"github.com/kite-plus/explore/internal/model"
)

func TestReadInputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blogs.txt")
	body := "# seed list\nhttps://a.example/\n\n  b.example  https://b.example/atom.xml\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readInputs(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []check.Input{{SiteURL: "https://a.example/"}, {SiteURL: "b.example", FeedURL: "https://b.example/atom.xml"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("inputs = %+v, want %+v", got, want)
	}
}

func TestSurveyReport(t *testing.T) {
	recs := []check.SurveyRecord{
		{
			SiteURL: "https://wp.example/",
			System:  model.GeneratorWordPress,
			Report:  &model.CheckReport{Generator: model.GeneratorWordPress, FeedURL: "https://wp.example/feed/", DiscoveredBy: "autodiscovery", Format: "rss", Passed: true},
			Feed:    &check.FeedStats{Bytes: 2048, Conditional: "304", Items: 10, Valid: 10, InWindow: 3, SummaryRunes: 120, TextRunes: 900, MaxSameMinute: 1},
		},
		{
			SiteURL: "https://hexo.example/",
			System:  model.GeneratorHexo,
			Report: &model.CheckReport{Generator: model.GeneratorHexo, Problems: []model.Problem{
				{Code: model.ProblemHTTPError, Severity: model.SeverityError, Detail: "HTTP 403"},
			}},
		},
		{SiteURL: "ftp://bad.example/", Error: "not an http or https URL"},
	}
	var b strings.Builder
	writeSurveyReport(&b, recs)
	out := b.String()
	for _, want := range []string{
		"| Measure | WordPress | Halo | Hugo | Hexo | Other | Unknown | All |",
		"| Blogs | 1 | 0 | 0 | 1 | 0 | 0 | 2 |",
		"| Feed found | 1/1 | 0/0 | 0/0 | 0/1 | 0/0 | 0/0 | 1/2 |",
		"| Answers 304 to a conditional request | 1/1 | 0/0 | 0/0 | 0/0 | 0/0 | 0/0 | 1/1 |",
		"| Full posts (median item over 500 runes) | 1/1 |",
		"| Size KiB p50 / p90 / max | 2 / 2 / 2 | - |",
		"| `http_error` | 0 | 0 | 0 | 1 | 0 | 0 | 1 |",
		"HTTP errors: HTTP 403 ×1",
		"Invalid input: not an http or https URL ×1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
}

func TestPercentile(t *testing.T) {
	xs := []int{5, 1, 4, 2, 3, 10, 9, 8, 7, 6}
	for _, c := range []struct {
		p    float64
		want int
	}{{0.5, 5}, {0.9, 9}, {1, 10}, {0, 1}} {
		if got := percentile(xs, c.p); got != c.want {
			t.Errorf("percentile(%v) = %d, want %d", c.p, got, c.want)
		}
	}
	if xs[0] != 5 {
		t.Error("percentile sorted its input")
	}
}

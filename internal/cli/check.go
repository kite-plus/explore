package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/text/width"

	"github.com/kite-plus/explore/internal/buildinfo"
	"github.com/kite-plus/explore/internal/check"
	"github.com/kite-plus/explore/internal/fetch"
	"github.com/kite-plus/explore/internal/i18n"
	"github.com/kite-plus/explore/internal/model"
)

// errCheckFailed gives a failed check a non-zero exit status for scripts.
var errCheckFailed = errors.New("check failed")

func newCheckCmd() *cobra.Command {
	var feedURL, lang string
	cmd := &cobra.Command{
		Use:   "check <url>",
		Short: "Check whether Explore can use a blog's feed",
		Long: "check finds the feed of a blog, or reads the feed at the given address,\n" +
			"and reports what Explore would do with it. It needs no database.\n\n" +
			"Set EXPLORE_ALLOW_PRIVATE_NETWORKS=true to check a blog running on this\n" +
			"machine or a private network. A local proxy in fake-IP mode resolves every\n" +
			"name to a reserved address such as 198.18.0.0/15, so checks from such a\n" +
			"machine need the same setting.",
		Example: "  explore check https://blog.example.com/\n" +
			"  explore check blog.example.com --feed https://blog.example.com/atom.xml --lang zh-CN",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			c := &check.Checker{Fetch: fetch.New(fetch.Options{
				UserAgent:    fetch.UserAgent(buildinfo.Version, cfg.PublicURL),
				AllowPrivate: cfg.AllowPrivateNetworks,
			})}
			report, err := c.Run(cmd.Context(), check.Input{SiteURL: args[0], FeedURL: feedURL})
			if err != nil {
				return err
			}
			l := i18n.Parse(lang)
			check.Localize(report, l)
			if jsonOut(cmd) {
				if err := writeJSON(cmd.OutOrStdout(), report); err != nil {
					return err
				}
			} else {
				printReport(cmd, report, l)
			}
			if !report.Passed {
				return errCheckFailed
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&feedURL, "feed", "", "feed address; skips discovery")
	cmd.Flags().StringVar(&lang, "lang", "en", "language of the report: en or zh-CN")
	return cmd
}

type labels struct {
	checked, feed, system, http, items, result, passed, failed, yes, no string
	by                                                                  map[string]string
	severity                                                            map[model.Severity]string
	counts                                                              func(it *model.CheckItems) string
}

var reportLabels = map[i18n.Lang]labels{
	i18n.English: {
		checked: "Checked", feed: "Feed", system: "System", http: "HTTP", items: "Items",
		result: "Result", passed: "passed", failed: "failed", yes: "yes", no: "no",
		by: map[string]string{
			"direct": "the address itself", "autodiscovery": "the page's feed link",
			"candidate": "a default address", "given": "the given address",
		},
		severity: map[model.Severity]string{model.SeverityError: "error", model.SeverityWarning: "warning", model.SeverityInfo: "info"},
		counts: func(it *model.CheckItems) string {
			s := fmt.Sprintf("%d in feed, %d usable, %d with trusted dates", it.Total, it.Valid, it.TrustedDates)
			if it.LatestPublishedAt != nil {
				s += ", latest " + it.LatestPublishedAt.Format(time.DateOnly)
			}
			return s
		},
	},
	i18n.Chinese: {
		checked: "检查地址", feed: "订阅源", system: "博客系统", http: "HTTP", items: "文章",
		result: "结果", passed: "通过", failed: "未通过", yes: "有", no: "无",
		by: map[string]string{
			"direct": "输入的地址本身", "autodiscovery": "页面声明的订阅地址",
			"candidate": "默认地址", "given": "填写的订阅地址",
		},
		severity: map[model.Severity]string{model.SeverityError: "错误", model.SeverityWarning: "警告", model.SeverityInfo: "提示"},
		counts: func(it *model.CheckItems) string {
			s := fmt.Sprintf("订阅源 %d 篇，可用 %d 篇，日期可信 %d 篇", it.Total, it.Valid, it.TrustedDates)
			if it.LatestPublishedAt != nil {
				s += "，最新 " + it.LatestPublishedAt.Format(time.DateOnly)
			}
			return s
		},
	},
}

// pad fills s to cols terminal columns; wide characters such as Chinese
// take two.
func pad(s string, cols int) string {
	w := 0
	for _, r := range s {
		switch width.LookupRune(r).Kind() {
		case width.EastAsianWide, width.EastAsianFullwidth:
			w += 2
		default:
			w++
		}
	}
	if w >= cols {
		return s + " "
	}
	return s + strings.Repeat(" ", cols-w)
}

func printReport(cmd *cobra.Command, r *model.CheckReport, lang i18n.Lang) {
	lb := reportLabels[lang]
	row := func(label, value string) { printf(cmd, "%s%s\n", pad(label, 11), value) }

	row(lb.checked, r.InputURL)
	if r.FeedURL != "" {
		row(lb.feed, fmt.Sprintf("%s  (%s, %s)", r.FeedURL, r.Format, lb.by[r.DiscoveredBy]))
	}
	if r.Generator != "" && r.Generator != model.GeneratorUnknown {
		row(lb.system, string(r.Generator))
	}
	if r.HTTP != nil {
		yn := func(b bool) string {
			if b {
				return lb.yes
			}
			return lb.no
		}
		row(lb.http, fmt.Sprintf("%d, ETag %s, Last-Modified %s", r.HTTP.Status, yn(r.HTTP.ETag), yn(r.HTTP.LastModified)))
	}
	if r.Items != nil {
		row(lb.items, lb.counts(r.Items))
	}

	if len(r.Problems) > 0 {
		printf(cmd, "\n")
	}
	for _, p := range r.Problems {
		head := string(p.Code)
		if p.Count > 0 {
			head += fmt.Sprintf(" ×%d", p.Count)
		}
		if p.Detail != "" {
			head += "  " + p.Detail
		}
		printf(cmd, "%s%s\n", pad(lb.severity[p.Severity], 9), head)
		if p.Hint != "" {
			printf(cmd, "%s%s\n", pad("", 9), strings.TrimSpace(p.Hint))
		}
	}

	printf(cmd, "\n")
	if r.Passed {
		row(lb.result, lb.passed)
	} else {
		row(lb.result, lb.failed)
	}
}

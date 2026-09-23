package feed

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func load(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "feeds", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseFormats(t *testing.T) {
	cases := []struct {
		file   string
		format string
		items  int
	}{
		{"wordpress/feed.xml", "rss", 2},
		{"halo/rss.xml", "rss", 1},
		{"hugo/index.xml", "rss", 3},
		{"hexo/atom.xml", "atom", 2},
		{"hexo/rss2-no-guid.xml", "rss", 2},
		{"edge/feed.json", "json", 2},
		{"edge/gbk.xml", "rss", 1},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			f, err := Parse(load(t, c.file))
			if err != nil {
				t.Fatal(err)
			}
			if f.Format != c.format {
				t.Errorf("format = %q, want %q", f.Format, c.format)
			}
			if len(f.Items) != c.items {
				t.Errorf("items = %d, want %d", len(f.Items), c.items)
			}
		})
	}
}

func TestParseGBK(t *testing.T) {
	f, err := Parse(load(t, "edge/gbk.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := f.Items[0].Title, "用 GBK 编码的中文标题"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}

func TestParseSingleDigitDay(t *testing.T) {
	f, err := Parse(load(t, "halo/single-digit-day.xml"))
	if err != nil {
		t.Fatal(err)
	}
	got := f.Items[0].Published
	want := time.Date(2026, time.September, 3, 1, 2, 3, 0, time.UTC)
	if got == nil || !got.Equal(want) {
		t.Errorf("published = %v, want %v", got, want)
	}
}

func TestParseGenerator(t *testing.T) {
	cases := map[string]string{
		"wordpress/feed.xml": "https://wordpress.org/?v=7.1.2",
		"hugo/index.xml":     "Hugo",
		"hexo/atom.xml":      "Hexo https://hexo.io/",
		"halo/rss.xml":       "Halo v2.26.1",
	}
	for file, want := range cases {
		f, err := Parse(load(t, file))
		if err != nil {
			t.Fatal(err)
		}
		if f.Generator != want {
			t.Errorf("%s: generator = %q, want %q", file, f.Generator, want)
		}
	}
}

func TestParseRejectsHTML(t *testing.T) {
	_, err := Parse([]byte("<!doctype html><html><head><title>Blog</title></head><body></body></html>"))
	if !errors.Is(err, ErrNotFeed) {
		t.Fatalf("err = %v, want ErrNotFeed", err)
	}
}

func TestParseBrokenFeed(t *testing.T) {
	_, err := Parse([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><item><title>x</ti`))
	if err == nil || errors.Is(err, ErrNotFeed) {
		t.Fatalf("err = %v, want a parse error", err)
	}
}

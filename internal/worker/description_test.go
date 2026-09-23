package worker

import (
	"strings"
	"testing"
)

func TestDescriptionFromHTML(t *testing.T) {
	tests := []struct {
		html string
		want string
	}{
		{`<meta property="og:description" content="OG"><meta name="description" content="Site &amp; notes">`, "Site & notes"},
		{`<meta property="og:description" content="Open Graph">`, "Open Graph"},
		{`<meta name="twitter:description" content="Social description">`, "Social description"},
		{`<meta name="description" content="   "><body><meta name="description" content="Ignored">`, ""},
	}
	for _, tt := range tests {
		if got := descriptionFromHTML([]byte(tt.html)); got != tt.want {
			t.Errorf("descriptionFromHTML(%q) = %q, want %q", tt.html, got, tt.want)
		}
	}
	if got := cleanDescription(strings.Repeat("字", 300)); len([]rune(got)) != 240 || !strings.HasSuffix(got, "…") {
		t.Errorf("long description was not limited to 240 characters: %q", got)
	}
}

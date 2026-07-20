package engine

import "testing"

func TestFixedLiteralMatchers(t *testing.T) {
	tests := []struct {
		name  string
		match func(string) bool
		yes   []string
		no    []string
	}{
		{name: "byline", match: matchesByline, yes: []string{"BYLINE", "post-author-name"}, no: []string{"by-line", "writer"}},
		{name: "positive", match: matchesPositive, yes: []string{"ArticleBody", "BLOG-post"}, no: []string{"navigation", "footer"}},
		{name: "negative", match: matchesNegative, yes: []string{"HID", "hid panel", "panel hid", "sideBAR"}, no: []string{"hiding", "child", "article"}},
		{name: "share", match: matchesShareElement, yes: []string{"SHARE", "tools_share_item", "sharedaddy-box"}, no: []string{"shareholder", "fooshare", "sharedaddyesque"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, input := range tt.yes {
				if !tt.match(input) {
					t.Errorf("did not match %q", input)
				}
			}
			for _, input := range tt.no {
				if tt.match(input) {
					t.Errorf("unexpectedly matched %q", input)
				}
			}
		})
	}
}

package engine

import (
	"strings"
	"testing"
)

func TestParseDecodesHTMLEntitiesInMetadata(t *testing.T) {
	const source = `<html><head>
<meta property="og:title" content="Title &amp;copy;">
<meta name="author" content="Ada &amp;nbsp; Lovelace">
<meta name="description" content="A summary &amp;mdash; encoded.">
<meta property="og:site_name" content="Example &amp;reg;">
</head><body><article><p>Article content.</p></article></body></html>`

	opts := DefaultOptions()
	opts.CharThreshold = 0
	article, err := Parse(strings.NewReader(source), "https://example.com/article", &opts)
	if err != nil {
		t.Fatal(err)
	}

	if article.Title != "Title ©" {
		t.Errorf("Title = %q, want %q", article.Title, "Title ©")
	}
	if article.Byline != "Ada \u00a0 Lovelace" {
		t.Errorf("Byline = %q, want %q", article.Byline, "Ada \u00a0 Lovelace")
	}
	if article.Excerpt != "A summary — encoded." {
		t.Errorf("Excerpt = %q, want %q", article.Excerpt, "A summary — encoded.")
	}
	if article.SiteName != "Example ®" {
		t.Errorf("SiteName = %q, want %q", article.SiteName, "Example ®")
	}
}

func TestDecodeHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "named", input: "&copy; &nbsp; &mdash;", want: "© \u00a0 —"},
		{name: "decimal", input: "&#169; &#160; &#8212;", want: "© \u00a0 —"},
		{name: "hexadecimal", input: "&#xA9; &#xA0; &#x2014;", want: "© \u00a0 —"},
		{name: "malformed", input: "&bogus; &#; &;", want: "&bogus; &#; &;"},
		{name: "semicolonless", input: "&copy and &#169 and &#xA9", want: "© and © and ©"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeHTML(tt.input); got != tt.want {
				t.Fatalf("decodeHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

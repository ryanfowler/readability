package readability

import (
	"errors"
	"strings"
	"testing"
)

func TestParseFragment(t *testing.T) {
	opts := DefaultOptions()
	opts.CharThreshold = 0
	article, err := Parse(`<article><h1>Fragment Title</h1><p>A fragment article with readable content.</p></article>`, "https://example.com/story", &opts)
	if err != nil {
		t.Fatalf("Parse(fragment): %v", err)
	}
	if !strings.Contains(article.TextContent, "fragment article") {
		t.Fatalf("TextContent = %q, want fragment content", article.TextContent)
	}
}

func TestParseNoContentErrorContract(t *testing.T) {
	_, err := Parse(`<html><body></body></html>`, "https://example.com/", nil)
	if !errors.Is(err, ErrNoContent) {
		t.Fatalf("errors.Is(err, ErrNoContent) = false; err=%v", err)
	}
}

func TestNewDocumentNoBodyDetection(t *testing.T) {
	tests := []struct {
		name   string
		source string
		noBody bool
	}{
		{
			name:   "explicit head-only document",
			source: `<html><head><title>No Body</title></head></html>`,
			noBody: true,
		},
		{
			name:   "omitted body with content after head",
			source: `<html><head><title>Story</title></head><article>Content</article></html>`,
		},
		{
			name:   "header is not head",
			source: `<!doctype html><html><header>Heading</header><article>Content</article></html>`,
		},
		{
			name:   "body text in attribute is not body element",
			source: `<html data-example="<body"><head><title>Title</title></head><body>Content</body></html>`,
		},
		{
			name:   "fragment gets synthesized body",
			source: `<header>Heading</header><article>Content</article>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDocument(tt.source)
			if got := errors.Is(err, ErrNoBody); got != tt.noBody {
				t.Fatalf("errors.Is(err, ErrNoBody) = %v, want %v; err=%v", got, tt.noBody, err)
			}
		})
	}
}

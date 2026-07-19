package readability

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/net/html"
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

func TestParseNoBodyDetection(t *testing.T) {
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
			_, err := Parse(tt.source, "https://example.com/", nil)
			if got := errors.Is(err, ErrNoBody); got != tt.noBody {
				t.Fatalf("errors.Is(err, ErrNoBody) = %v, want %v; err=%v", got, tt.noBody, err)
			}
		})
	}
}

func TestParsedNodeAPIsDoNotMutateInput(t *testing.T) {
	source := `<html><head><title>Node API</title></head><body><article><p>` + strings.Repeat("Useful article prose. ", 20) + `</p></article></body></html>`
	root, err := html.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}

	var before strings.Builder
	if err := html.Render(&before, root); err != nil {
		t.Fatal(err)
	}
	if !IsProbablyReaderableNode(root, nil) {
		t.Fatal("IsProbablyReaderableNode() = false, want true")
	}

	opts := DefaultOptions()
	// Force all extraction retries so they must restore from the caller-owned,
	// read-only tree without mutating it.
	opts.CharThreshold = 1_000_000
	first, err := ParseNode(root, "https://example.com/article", &opts)
	if err != nil {
		t.Fatalf("first ParseNode: %v", err)
	}
	second, err := ParseNode(root, "https://example.com/article", &opts)
	if err != nil {
		t.Fatalf("second ParseNode: %v", err)
	}
	if *first != *second {
		t.Fatal("repeated ParseNode calls returned different articles")
	}

	var after strings.Builder
	if err := html.Render(&after, root); err != nil {
		t.Fatal(err)
	}
	if after.String() != before.String() {
		t.Fatal("ParseNode mutated its input")
	}
}

func TestExtractionUsesAllRetryStagesBelowThreshold(t *testing.T) {
	root, err := parseHTML(`<html><body><article><p>Short but non-empty article text.</p></article></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := newEngineFromReadOnlyNode(root, "https://example.com/article", func(options *engineOptions) {
		options.charThreshold = 1_000_000
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Parse(); err != nil {
		t.Fatal(err)
	}
	if got, want := len(engine.attempts), 4; got != want {
		t.Fatalf("attempt count = %d, want %d", got, want)
	}
}

func TestParsedNodeAPIsAcceptBodyRoot(t *testing.T) {
	body := newElement("body")
	article := newElement("article")
	paragraph := newElement("p")
	paragraph.AppendChild(&html.Node{Type: html.TextNode, Data: strings.Repeat("Useful body-rooted article prose. ", 20)})
	article.AppendChild(paragraph)
	body.AppendChild(article)

	var before strings.Builder
	if err := html.Render(&before, body); err != nil {
		t.Fatal(err)
	}
	readerableOptions := DefaultReaderableOptions()
	readerableOptions.MinScore = 0
	if !IsProbablyReaderableNode(body, &readerableOptions) {
		t.Fatal("IsProbablyReaderableNode() = false for body-rooted tree")
	}

	opts := DefaultOptions()
	opts.CharThreshold = 0
	result, err := ParseNode(body, "https://example.com/article", &opts)
	if err != nil {
		t.Fatalf("ParseNode: %v", err)
	}
	if !strings.Contains(result.TextContent, "body-rooted article prose") {
		t.Fatalf("TextContent = %q, want body-rooted article", result.TextContent)
	}

	var after strings.Builder
	if err := html.Render(&after, body); err != nil {
		t.Fatal(err)
	}
	if after.String() != before.String() {
		t.Fatal("ParseNode mutated its body-rooted input")
	}
}

func TestParsedNodeAPIsRejectMissingBody(t *testing.T) {
	root := &html.Node{Type: html.DocumentNode}
	if _, err := ParseNode(root, "", nil); !errors.Is(err, ErrNoBody) {
		t.Fatalf("ParseNode error = %v, want ErrNoBody", err)
	}
	if IsProbablyReaderableNode(root, nil) {
		t.Fatal("IsProbablyReaderableNode() = true for document without body")
	}
}

package engine

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"golang.org/x/net/html"
)

func TestParseFragment(t *testing.T) {
	opts := DefaultOptions()
	opts.CharThreshold = 0
	article, err := Parse(strings.NewReader(`<article><h1>Fragment Title</h1><p>A fragment article with readable content.</p></article>`), "https://example.com/story", &opts)
	if err != nil {
		t.Fatalf("Parse(fragment): %v", err)
	}
	if !strings.Contains(article.TextContent, "fragment article") {
		t.Fatalf("TextContent = %q, want fragment content", article.TextContent)
	}
}

func TestParseIncludesContentNode(t *testing.T) {
	article, err := Parse(strings.NewReader(`<html><body><article><p>Parsed article content.</p></article></body></html>`), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if article.Node == nil {
		t.Fatal("Node = nil")
	}
	if got := innerHTML(article.Node); got != article.Content {
		t.Fatalf("Node inner HTML = %q, want Content %q", got, article.Content)
	}
}

func TestParseNormalizesTextContent(t *testing.T) {
	article, err := Parse(strings.NewReader("<html><body><article><p>  First\n\t paragraph.  </p>\n<p>Second\u00a0 paragraph. 😀 </p></article></body></html>"), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	const want = "First paragraph. Second paragraph. 😀"
	if article.TextContent != want {
		t.Fatalf("TextContent = %q, want %q", article.TextContent, want)
	}
	if article.Length != characterCount(want) {
		t.Fatalf("Length = %d, want %d", article.Length, characterCount(want))
	}
}

func TestParsePreservesPreformattedTextContent(t *testing.T) {
	const input = `<html><body><article>
<p>Code sample:</p>
<pre>first line
    indented line
	Tabbed line</pre>
<p>After sample.</p>
</article></body></html>`
	article, err := Parse(strings.NewReader(input), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	const want = "Code sample: first line\n    indented line\n\tTabbed line After sample."
	if article.TextContent != want {
		t.Fatalf("TextContent = %q, want %q", article.TextContent, want)
	}
	if article.Length != characterCount(want) {
		t.Fatalf("Length = %d, want %d", article.Length, characterCount(want))
	}
}

func TestParseAvoidsRedundantWhitespaceAroundPreformattedText(t *testing.T) {
	const input = `<html><body><article><p>Before</p>
<pre>

  code
</pre>
<p>After</p></article></body></html>`
	article, err := Parse(strings.NewReader(input), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	const want = "Before\n  code\nAfter"
	if article.TextContent != want {
		t.Fatalf("TextContent = %q, want %q", article.TextContent, want)
	}
}

func TestParseNoContentErrorContract(t *testing.T) {
	_, err := Parse(strings.NewReader(`<html><body></body></html>`), "https://example.com/", nil)
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
			_, err := Parse(strings.NewReader(tt.source), "https://example.com/", nil)
			if got := errors.Is(err, ErrNoBody); got != tt.noBody {
				t.Fatalf("errors.Is(err, ErrNoBody) = %v, want %v; err=%v", got, tt.noBody, err)
			}
		})
	}
}

func TestParseReturnsReaderError(t *testing.T) {
	readErr := errors.New("reader failed")
	_, err := Parse(iotest.ErrReader(readErr), "https://example.com/", nil)
	if err != readErr {
		t.Fatalf("Parse() error = %v, want reader error %v", err, readErr)
	}
}

func TestParseRetryRestoresIndependentTrees(t *testing.T) {
	_, restore, err := parseHTMLReaderWithRestore(strings.NewReader(`<html><body><article>retry source</article></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	first := restore()
	findElement(first, "body").Data = "changed"
	second := restore()
	if got := findElement(second, "body").Data; got != "body" {
		t.Fatalf("restored body tag = %q, want body", got)
	}
}

func TestParseReplaysReaderForRetries(t *testing.T) {
	const prefix = "ignored prefix"
	const source = `<html><body><article><p>Short but non-empty article text.</p></article></body></html>`

	tests := []struct {
		name   string
		reader func() io.Reader
	}{
		{
			name: "strings.Reader",
			reader: func() io.Reader {
				r := strings.NewReader(prefix + source)
				_, _ = r.Seek(int64(len(prefix)), io.SeekStart)
				return r
			},
		},
		{
			name: "bytes.Reader",
			reader: func() io.Reader {
				r := bytes.NewReader([]byte(prefix + source))
				_, _ = r.Seek(int64(len(prefix)), io.SeekStart)
				return r
			},
		},
		{
			name: "bytes.Buffer",
			reader: func() io.Reader {
				r := bytes.NewBufferString(prefix + source)
				r.Next(len(prefix))
				return r
			},
		},
		{
			name: "buffered fallback",
			reader: func() io.Reader {
				return io.LimitReader(strings.NewReader(source), int64(len(source)))
			},
		},
	}

	opts := DefaultOptions()
	opts.CharThreshold = 1_000_000
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			article, err := Parse(tt.reader(), "https://example.com/", &opts)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(article.TextContent, "Short but non-empty") {
				t.Fatalf("TextContent = %q, want article text", article.TextContent)
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
	firstValue, secondValue := *first, *second
	firstNode, secondNode := firstValue.Node, secondValue.Node
	firstValue.Node, secondValue.Node = nil, nil
	if firstValue != secondValue {
		t.Fatal("repeated ParseNode calls returned different articles")
	}
	if firstNode == nil || secondNode == nil || innerHTML(firstNode) != innerHTML(secondNode) {
		t.Fatal("repeated ParseNode calls returned different content nodes")
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
	options := defaultExtractionOptions()
	options.charThreshold = 1_000_000
	extractor := newExtractorFromReadOnlyNode(root, "https://example.com/article", options)
	if _, err := extractor.extract(); err != nil {
		t.Fatal(err)
	}
	if got, want := len(extractor.attempts), 4; got != want {
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

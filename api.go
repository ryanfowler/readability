package readability

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// Article contains the extracted article and its metadata.
//
// Content and Node can contain unsafe HTML. Sanitize them before you add them
// to a web page. Metadata fields are empty when the source does not supply a
// value.
type Article struct {
	// Title is the article title.
	Title string `json:"title"`
	// Byline identifies the article author.
	Byline string `json:"byline"`
	// Dir is the text direction, such as "ltr" or "rtl".
	Dir string `json:"dir"`
	// Lang is the article language from the document metadata.
	Lang string `json:"lang"`
	// Content is the processed inner HTML of Node. It is not sanitized.
	Content string `json:"content"`
	// Node is the processed article element. Its inner HTML is Content when the
	// package returns the Article. Node is not included in JSON because an
	// html.Node contains cyclic links.
	Node *html.Node `json:"-"`
	// TextContent is the article text. It uses one space for each normal
	// whitespace sequence. It retains whitespace in preformatted elements.
	TextContent string `json:"textContent"`
	// Length is the length of TextContent in UTF-16 code units.
	Length int `json:"length"`
	// Excerpt is the article description or a short extract from the content.
	Excerpt string `json:"excerpt"`
	// SiteName is the name of the source site.
	SiteName string `json:"siteName"`
	// PublishedTime is the publication time from the document metadata. The
	// package does not change its source format.
	PublishedTime string `json:"publishedTime"`
}

// Options controls article extraction.
//
// Pass nil to Parse or ParseNode to use the defaults. To change selected
// fields, first call DefaultOptions and then change the returned value. A
// non-nil Options value supplies all options. Zero values have a function. A
// nil AllowedVideoRegex is the exception; it selects the built-in allowlist.
type Options struct {
	// MaxElemsToParse is the maximum number of HTML elements that extraction
	// accepts. Zero removes the limit.
	MaxElemsToParse int
	// NbTopCandidates is the number of top article candidates to compare.
	NbTopCandidates int
	// CharThreshold is the minimum result length in UTF-16 code units. The
	// package retries extraction if the result is shorter. Each retry uses less
	// strict cleanup rules. If all results are too short, the package returns the
	// longest nonempty result. Zero prevents retries.
	CharThreshold int
	// ClassesToPreserve lists the CSS classes to retain during class cleanup.
	// This field has no effect when KeepClasses is true.
	ClassesToPreserve []string
	// KeepClasses retains all CSS classes when it is true.
	KeepClasses bool
	// DisableJSONLD prevents metadata extraction from JSON-LD when it is true.
	DisableJSONLD bool
	// AllowedVideoRegex identifies video URLs that cleanup can retain. A nil
	// value selects the built-in allowlist.
	AllowedVideoRegex *regexp.Regexp
	// LinkDensityModifier changes the link-density limits that the cleanup rules
	// use to remove a candidate.
	LinkDensityModifier float64
	// Logger receives extraction log records. A nil value turns logs off. The
	// package does not use the global slog logger.
	Logger *slog.Logger
	// Debug enables additional verbose log records. Logger must be non-nil to
	// receive these records.
	Debug bool
}

// ReaderableOptions controls the fast readerability heuristic.
//
// Pass nil to a readerability function to use the defaults. To change selected
// fields, first call DefaultReaderableOptions and then change the returned
// value. A non-nil ReaderableOptions value supplies all options.
type ReaderableOptions struct {
	// MinScore is the score that the document must exceed.
	MinScore float64
	// MinContentLength is the minimum candidate length in UTF-16 code units.
	MinContentLength int
}

// DefaultOptions returns an Options value with the Mozilla defaults.
func DefaultOptions() Options {
	return Options{NbTopCandidates: 5, CharThreshold: 500, ClassesToPreserve: []string{"page"}, AllowedVideoRegex: videos}
}

// DefaultReaderableOptions returns a ReaderableOptions value with the Mozilla
// defaults.
func DefaultReaderableOptions() ReaderableOptions {
	return ReaderableOptions{MinScore: 20, MinContentLength: 140}
}

var (
	// ErrNoContent means that extraction did not produce article content.
	ErrNoContent = errors.New("readability: no content")
	// ErrNoBody means that the supplied HTML tree does not have a body element.
	ErrNoBody = errors.New("readability: document has no body")
	// ErrInvalidURL means that the nonempty page URL is not an absolute HTTP or
	// HTTPS URL with a host.
	ErrInvalidURL = errors.New("readability: invalid URL")
)

// TooManyElementsError reports that a document exceeds MaxElemsToParse.
type TooManyElementsError struct {
	// Count is the number of elements in the document.
	Count int
	// Max is the configured maximum number of elements.
	Max int
}

func (e *TooManyElementsError) Error() string {
	return fmt.Sprintf("readability: %d elements exceeds maximum %d", e.Count, e.Max)
}

func parseHTML(input string) (*html.Node, error) {
	return parseHTMLReader(strings.NewReader(input))
}

func replayableReader(input io.Reader) (io.Reader, func() io.Reader) {
	switch r := input.(type) {
	case *strings.Reader:
		snapshot := *r
		return r, func() io.Reader {
			clone := snapshot
			return &clone
		}
	case *bytes.Reader:
		snapshot := *r
		return r, func() io.Reader {
			clone := snapshot
			return &clone
		}
	case *bytes.Buffer:
		source := r.Bytes()
		return r, func() io.Reader { return bytes.NewReader(source) }
	default:
		var source bytes.Buffer
		return io.TeeReader(input, &source), func() io.Reader {
			return bytes.NewReader(source.Bytes())
		}
	}
}

func parseHTMLReader(input io.Reader) (*html.Node, error) {
	doc, _, err := parseHTMLReaderWithRestore(input)
	return doc, err
}

func parseHTMLReaderWithRestore(input io.Reader) (*html.Node, func() *html.Node, error) {
	reader, replay := replayableReader(input)
	doc, err := html.Parse(reader)
	if err != nil {
		return nil, nil, err
	}
	bodyNode := findElement(doc, "body")
	if bodyNode == nil {
		return nil, nil, ErrNoBody
	}
	// html.Parse synthesizes a body. Only an empty synthesized body is
	// ambiguous, so avoid tokenizing every normal document a second time.
	hasBodyContent := false
	for child := bodyNode.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode ||
			(child.Type == html.TextNode && strings.TrimSpace(child.Data) != "") {
			hasBodyContent = true
			break
		}
	}
	if !hasBodyContent {
		// Preserve ErrNoBody for complete, explicitly head-only documents.
		// Inspect tokens rather than substrings so <header> and text or attributes
		// containing "<body" do not count.
		var hasHTML, hasHead, hasBody bool
		tokenizer := html.NewTokenizer(replay())
		for {
			tokenType := tokenizer.Next()
			if tokenType == html.ErrorToken {
				break
			}
			if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
				continue
			}
			name, _ := tokenizer.TagName()
			switch string(name) {
			case "html":
				hasHTML = true
			case "head":
				hasHead = true
			case "body":
				hasBody = true
			}
			if hasBody {
				break
			}
		}
		if hasHTML && hasHead && !hasBody {
			return nil, nil, ErrNoBody
		}
	}
	restore := func() *html.Node {
		doc, _ := html.Parse(replay())
		return doc
	}
	return doc, restore, nil
}

// Parse reads HTML from input and extracts an article.
//
// pageURL can be empty. If it is not empty, it must be an absolute HTTP or
// HTTPS URL with a host. Parse uses pageURL and the document base URL to
// resolve relative links and media URLs.
//
// Pass nil for options to use the defaults. Parse returns input read errors
// directly. Other errors support errors.Is with ErrNoBody, ErrInvalidURL, or
// ErrNoContent. Parse can also return a *TooManyElementsError.
func Parse(input io.Reader, pageURL string, options *Options) (*Article, error) {
	root, restore, err := parseHTMLReaderWithRestore(input)
	if err != nil {
		return nil, err
	}
	return parseNode(root, pageURL, options, restore)
}

// ParseNode extracts an article from a parsed HTML tree.
//
// root can be a complete document or a tree with a body root. ParseNode does
// not change root. The caller must not change root while ParseNode uses it.
//
// pageURL can be empty. If it is not empty, it must be an absolute HTTP or
// HTTPS URL with a host. ParseNode uses pageURL and the document base URL to
// resolve relative links and media URLs.
//
// Pass nil for options to use the defaults. ParseNode returns an error that
// supports errors.Is with ErrNoBody, ErrInvalidURL, or ErrNoContent. It can
// also return a *TooManyElementsError.
func ParseNode(root *html.Node, pageURL string, options *Options) (*Article, error) {
	return parseNode(root, pageURL, options, nil)
}

func parseNode(root *html.Node, pageURL string, options *Options, restore func() *html.Node) (*Article, error) {
	if root == nil || findElement(root, "body") == nil {
		return nil, ErrNoBody
	}
	if pageURL != "" {
		page, err := url.ParseRequestURI(pageURL)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
		}
		if !page.IsAbs() || page.Hostname() == "" ||
			(!strings.EqualFold(page.Scheme, "http") && !strings.EqualFold(page.Scheme, "https")) {
			return nil, fmt.Errorf("%w: page URL must be an absolute HTTP or HTTPS URL with a host", ErrInvalidURL)
		}
	}
	o := DefaultOptions()
	if options != nil {
		o = *options
		o.ClassesToPreserve = append([]string(nil), options.ClassesToPreserve...)
	}
	if o.MaxElemsToParse > 0 {
		count := 0
		walkElements(root, func(*html.Node) bool {
			count++
			return false
		})
		if count > o.MaxElemsToParse {
			return nil, &TooManyElementsError{Count: count, Max: o.MaxElemsToParse}
		}
	}
	configure := func(x *engineOptions) {
		x.maxElemsToParse = o.MaxElemsToParse
		x.nbTopCandidates = o.NbTopCandidates
		x.charThreshold = o.CharThreshold
		x.classesToPreserve = append([]string(nil), o.ClassesToPreserve...)
		x.keepClasses = o.KeepClasses
		x.disableJSONLD = o.DisableJSONLD
		x.linkDensityModifier = o.LinkDensityModifier
		x.logger = o.Logger
		x.debug = o.Debug
		if o.AllowedVideoRegex != nil {
			x.allowedVideoRegex = o.AllowedVideoRegex
		}
	}
	var e *engine
	var err error
	if restore == nil {
		e, err = newEngineFromReadOnlyNode(root, pageURL, configure)
	} else {
		e, err = newEngineFromOwnedNode(root, restore, pageURL, configure)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoContent, err)
	}
	r, err := e.Parse()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoContent, err)
	}
	return &Article{
		Title:         r.Title,
		Byline:        r.Byline,
		Dir:           r.Dir,
		Lang:          r.Lang,
		Content:       r.HTMLContent,
		Node:          r.Node,
		TextContent:   r.TextContent,
		Length:        r.Length,
		Excerpt:       r.Excerpt,
		SiteName:      r.SiteName,
		PublishedTime: r.PublishedTime,
	}, nil
}

// IsProbablyReaderable reports whether input is likely to contain an article.
//
// This function applies a fast heuristic. It does not extract the article. It
// returns false if it cannot parse a document body. Pass nil for options to
// use the defaults.
func IsProbablyReaderable(input string, options *ReaderableOptions) bool {
	root, err := parseHTML(input)
	return err == nil && IsProbablyReaderableNode(root, options)
}

// IsProbablyReaderableNode reports whether a parsed HTML tree is likely to
// contain an article.
//
// This function applies a fast heuristic. It does not extract the article.
// root can be a complete document or a tree with a body root. The function
// returns false if root is nil or has no body element. It does not change root.
// The caller must not change root while the function uses it. Pass nil for
// options to use the defaults.
func IsProbablyReaderableNode(root *html.Node, options *ReaderableOptions) bool {
	if root == nil || findElement(root, "body") == nil {
		return false
	}
	o := DefaultReaderableOptions()
	if options != nil {
		o = *options
	}
	return isProbablyReaderable(root, func(x *engineOptions) { x.minScore = o.MinScore; x.minContentLength = o.MinContentLength })
}

package readability

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// Article is the extracted article and metadata. Content and Node are
// unsanitized. Node is the parsed element whose inner HTML is Content.
type Article struct {
	Title   string `json:"title"`
	Byline  string `json:"byline"`
	Dir     string `json:"dir"`
	Lang    string `json:"lang"`
	Content string `json:"content"`
	// Node is the extracted article as a parsed HTML node. It is omitted from
	// JSON because html.Node contains cyclic links.
	Node *html.Node `json:"-"`
	// TextContent is plain text with whitespace collapsed to single spaces,
	// except inside preformatted elements.
	TextContent string `json:"textContent"`
	// Length is the length of TextContent in UTF-16 code units.
	Length        int    `json:"length"`
	Excerpt       string `json:"excerpt"`
	SiteName      string `json:"siteName"`
	PublishedTime string `json:"publishedTime"`
}

// Options controls article extraction.
//
// A non-nil Options value is used exactly as supplied; zero values can be
// meaningful (for example MaxElemsToParse == 0 disables the element limit).
// Callers that only want to override selected settings must start with
// DefaultOptions and then change those fields. Passing nil uses all defaults.
type Options struct {
	MaxElemsToParse int
	NbTopCandidates int
	// CharThreshold is the extracted text length, in UTF-16 code units, below
	// which extraction is retried with progressively less aggressive heuristics.
	// If every attempt remains below the threshold, the longest non-empty result
	// is returned.
	// A value of zero disables retries.
	CharThreshold       int
	ClassesToPreserve   []string
	KeepClasses         bool
	DisableJSONLD       bool
	AllowedVideoRegex   *regexp.Regexp
	LinkDensityModifier float64
	// Logger receives extraction logs. If nil, logging is disabled.
	Logger *slog.Logger
	Debug  bool
}

// ReaderableOptions controls the inexpensive readerability heuristic.
// A non-nil value is used exactly as supplied. Start with
// DefaultReaderableOptions when overriding only selected fields.
type ReaderableOptions struct {
	MinScore float64
	// MinContentLength is measured in UTF-16 code units.
	MinContentLength int
}

// DefaultOptions returns an independent Options value with Mozilla defaults.
func DefaultOptions() Options {
	return Options{NbTopCandidates: 5, CharThreshold: 500, ClassesToPreserve: []string{"page"}, AllowedVideoRegex: videos}
}

// DefaultReaderableOptions returns the Mozilla readerability defaults.
func DefaultReaderableOptions() ReaderableOptions {
	return ReaderableOptions{MinScore: 20, MinContentLength: 140}
}

var (
	ErrNoContent  = errors.New("readability: no content")
	ErrNoBody     = errors.New("readability: document has no body")
	ErrInvalidURL = errors.New("readability: invalid URL")
)

type TooManyElementsError struct{ Count, Max int }

func (e *TooManyElementsError) Error() string {
	return fmt.Sprintf("readability: %d elements exceeds maximum %d", e.Count, e.Max)
}

func parseHTML(input string) (*html.Node, error) {
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return nil, err
	}
	bodyNode := findElement(doc, "body")
	if bodyNode == nil {
		return nil, ErrNoBody
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
		tokenizer := html.NewTokenizer(strings.NewReader(input))
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
			return nil, ErrNoBody
		}
	}
	return doc, nil
}

// Parse extracts an article from HTML source. If pageURL is non-empty, it
// must be an absolute HTTP or HTTPS URL with a host.
func Parse(input, pageURL string, options *Options) (*Article, error) {
	root, err := parseHTML(input)
	if err != nil {
		return nil, err
	}
	// The parsed tree is owned by this call, so extraction can mutate it. If a
	// retry is needed, recreate the pristine tree lazily instead of cloning every
	// document up front. strings.Reader cannot make html.Parse fail.
	restore := func() *html.Node {
		doc, _ := html.Parse(strings.NewReader(input))
		return doc
	}
	return parseNode(root, pageURL, options, false, restore)
}

// ParseNode extracts an article from an already parsed HTML tree. Root may be
// a complete document or a body-rooted tree. It is not mutated, but must
// contain a body element and must not be mutated concurrently while ParseNode
// is running. If pageURL is non-empty, it must be an absolute HTTP or HTTPS
// URL with a host.
func ParseNode(root *html.Node, pageURL string, options *Options) (*Article, error) {
	return parseNode(root, pageURL, options, true, nil)
}

func parseNode(root *html.Node, pageURL string, options *Options, cloneInput bool, restore func() *html.Node) (*Article, error) {
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
			return nil, &TooManyElementsError{count, o.MaxElemsToParse}
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
	if cloneInput {
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

// IsProbablyReaderable reports whether HTML source is likely to contain an
// article.
func IsProbablyReaderable(input string, options *ReaderableOptions) bool {
	root, err := parseHTML(input)
	return err == nil && IsProbablyReaderableNode(root, options)
}

// IsProbablyReaderableNode reports whether an already parsed HTML tree is
// likely to contain an article. Root may be a complete document or a
// body-rooted tree. It is not mutated, but must contain a body element and must
// not be mutated concurrently while this function is running.
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

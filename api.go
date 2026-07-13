package readability

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

// Article is the extracted article and metadata. Content is unsanitized HTML.
type Article struct {
	Title         string `json:"title"`
	Byline        string `json:"byline"`
	Dir           string `json:"dir"`
	Lang          string `json:"lang"`
	Content       string `json:"content"`
	TextContent   string `json:"textContent"`
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
	MaxElemsToParse     int
	NbTopCandidates     int
	CharThreshold       int
	ClassesToPreserve   []string
	KeepClasses         bool
	DisableJSONLD       bool
	AllowedVideoRegex   *regexp.Regexp
	LinkDensityModifier float64
	Debug               bool
}

// ReaderableOptions controls the inexpensive readerability heuristic.
// A non-nil value is used exactly as supplied. Start with
// DefaultReaderableOptions when overriding only selected fields.
type ReaderableOptions struct {
	MinScore         float64
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
	ErrNoContent        = errors.New("readability: no content")
	ErrNoBody           = errors.New("readability: document has no body")
	ErrInvalidURL       = errors.New("readability: invalid URL")
	ErrDocumentConsumed = errors.New("readability: document already parsed")
)

type TooManyElementsError struct{ Count, Max int }

func (e *TooManyElementsError) Error() string {
	return fmt.Sprintf("readability: %d elements exceeds maximum %d", e.Count, e.Max)
}

// Document is a parsed document. Parse consumes it and may only be called once.
type Document struct {
	// root is the canonical HTML5 tree used directly by the extraction engine.
	root     *html.Node
	mu       sync.Mutex
	consumed bool
}

func NewDocument(input string) (*Document, error) {
	// Preserve the no-body error for complete, explicitly head-only documents;
	// html.Parse otherwise synthesizes a body. Inspect tokens rather than source
	// substrings so <header> and text/attributes containing "<body" do not count.
	var hasHTML, hasHead, hasBody bool
	tokenizer := html.NewTokenizer(strings.NewReader(input))
scanTags:
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			break
		}
		if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
			continue
		}
		name, _ := tokenizer.TagName() // TagName is normalized to lower case.
		switch string(name) {
		case "html":
			hasHTML = true
		case "head":
			hasHead = true
		case "body":
			hasBody = true
			// A body tag settles the only question this preliminary scan is
			// needed for. Avoid tokenizing the (usually much larger) body twice.
			break scanTags
		}
	}
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return nil, err
	}
	var bodyNode *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			bodyNode = n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if bodyNode == nil {
		return nil, ErrNoBody
	}
	if hasHTML && hasHead && !hasBody {
		// An omitted body tag is valid when content follows the head. Only retain
		// ErrNoBody for a genuinely head-only complete document.
		hasBodyContent := false
		for child := bodyNode.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode ||
				(child.Type == html.TextNode && strings.TrimSpace(child.Data) != "") {
				hasBodyContent = true
				break
			}
		}
		if !hasBodyContent {
			return nil, ErrNoBody
		}
	}
	return &Document{root: doc}, nil
}

func Parse(input, pageURL string, options *Options) (*Article, error) {
	d, err := NewDocument(input)
	if err != nil {
		return nil, err
	}
	return d.Parse(pageURL, options)
}

func (d *Document) Parse(pageURL string, options *Options) (*Article, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.consumed {
		return nil, ErrDocumentConsumed
	}
	d.consumed = true
	if pageURL != "" {
		if _, err := url.ParseRequestURI(pageURL); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
		}
	}
	o := DefaultOptions()
	if options != nil {
		o = *options
		o.ClassesToPreserve = append([]string(nil), options.ClassesToPreserve...)
	}
	if o.MaxElemsToParse > 0 {
		count := 0
		walkElements(d.root, func(*html.Node) bool {
			count++
			return false
		})
		if count > o.MaxElemsToParse {
			return nil, &TooManyElementsError{count, o.MaxElemsToParse}
		}
	}
	prepareMathJax(d.root)
	e, err := newEngine(d.root, pageURL, func(x *engineOptions) {
		x.maxElemsToParse = o.MaxElemsToParse
		x.nbTopCandidates = o.NbTopCandidates
		x.charThreshold = o.CharThreshold
		x.classesToPreserve = append([]string(nil), o.ClassesToPreserve...)
		x.keepClasses = o.KeepClasses
		x.disableJSONLD = o.DisableJSONLD
		x.linkDensityModifier = o.LinkDensityModifier
		x.debug = o.Debug
		if o.AllowedVideoRegex != nil {
			x.allowedVideoRegex = o.AllowedVideoRegex
		}
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoContent, err)
	}
	r, err := e.Parse()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoContent, err)
	}
	return &Article{r.Title, r.Byline, r.Dir, r.Lang, r.HTMLContent, r.TextContent, r.Length, r.Excerpt, r.SiteName, r.PublishedTime}, nil
}

func IsProbablyReaderable(input string, options *ReaderableOptions) bool {
	d, err := NewDocument(input)
	return err == nil && d.IsProbablyReaderable(options)
}
func (d *Document) IsProbablyReaderable(options *ReaderableOptions) bool {
	o := DefaultReaderableOptions()
	if options != nil {
		o = *options
	}
	return isProbablyReaderable(d.root, func(x *engineOptions) { x.minScore = o.MinScore; x.minContentLength = o.MinContentLength })
}

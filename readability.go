// Package readability extracts the main article and its metadata from HTML.
//
// Use Parse for HTML from an io.Reader. Use ParseNode for a tree parsed with
// golang.org/x/net/html. Use IsProbablyReaderable or
// IsProbablyReaderableNode when you only need a fast readerability check.
//
// Pass nil for an options pointer to use the defaults. To change selected
// options, call DefaultOptions or DefaultReaderableOptions first. Then change
// the returned value.
//
// Article.Content and Article.Node can contain unsafe HTML. The package does
// not sanitize them. Sanitize extracted HTML before you add it to a web page.
package readability

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"

	"github.com/ryanfowler/readability/internal/engine"
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

var (
	// ErrNoContent means that extraction did not produce article content.
	ErrNoContent = engine.ErrNoContent
	// ErrNoBody means that the supplied HTML tree does not have a body element.
	ErrNoBody = engine.ErrNoBody
	// ErrInvalidURL means that the nonempty page URL is not an absolute HTTP or
	// HTTPS URL with a host.
	ErrInvalidURL = engine.ErrInvalidURL
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

// DefaultOptions returns an Options value with the Mozilla defaults.
func DefaultOptions() Options {
	return optionsFromEngine(engine.DefaultOptions())
}

// DefaultReaderableOptions returns a ReaderableOptions value with the Mozilla
// defaults.
func DefaultReaderableOptions() ReaderableOptions {
	o := engine.DefaultReaderableOptions()
	return ReaderableOptions{MinScore: o.MinScore, MinContentLength: o.MinContentLength}
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
	article, err := engine.Parse(input, pageURL, optionsToEngine(options))
	if err != nil {
		return nil, publicError(err)
	}
	return articleFromEngine(article), nil
}

// ParseNode extracts an article from a parsed HTML tree.
//
// root can be a complete document or a tree with a body root. ParseNode does
// not change root. The caller must not change root while ParseNode uses it.
// pageURL can be empty; a nonempty value must be an absolute HTTP or HTTPS URL
// with a host. Pass nil for options to use the defaults.
func ParseNode(root *html.Node, pageURL string, options *Options) (*Article, error) {
	article, err := engine.ParseNode(root, pageURL, optionsToEngine(options))
	if err != nil {
		return nil, publicError(err)
	}
	return articleFromEngine(article), nil
}

// IsProbablyReaderable reports whether input is likely to contain an article.
// It applies a fast heuristic and does not extract the article.
func IsProbablyReaderable(input string, options *ReaderableOptions) bool {
	return engine.IsProbablyReaderable(input, readerableOptionsToEngine(options))
}

// IsProbablyReaderableNode reports whether a parsed HTML tree is likely to
// contain an article. It does not change root.
func IsProbablyReaderableNode(root *html.Node, options *ReaderableOptions) bool {
	return engine.IsProbablyReaderableNode(root, readerableOptionsToEngine(options))
}

func optionsToEngine(options *Options) *engine.Options {
	if options == nil {
		return nil
	}
	return &engine.Options{
		MaxElemsToParse:     options.MaxElemsToParse,
		NbTopCandidates:     options.NbTopCandidates,
		CharThreshold:       options.CharThreshold,
		ClassesToPreserve:   append([]string(nil), options.ClassesToPreserve...),
		KeepClasses:         options.KeepClasses,
		DisableJSONLD:       options.DisableJSONLD,
		AllowedVideoRegex:   options.AllowedVideoRegex,
		LinkDensityModifier: options.LinkDensityModifier,
		Logger:              options.Logger,
		Debug:               options.Debug,
	}
}

func optionsFromEngine(options engine.Options) Options {
	return Options{
		MaxElemsToParse:     options.MaxElemsToParse,
		NbTopCandidates:     options.NbTopCandidates,
		CharThreshold:       options.CharThreshold,
		ClassesToPreserve:   append([]string(nil), options.ClassesToPreserve...),
		KeepClasses:         options.KeepClasses,
		DisableJSONLD:       options.DisableJSONLD,
		AllowedVideoRegex:   options.AllowedVideoRegex,
		LinkDensityModifier: options.LinkDensityModifier,
		Logger:              options.Logger,
		Debug:               options.Debug,
	}
}

func readerableOptionsToEngine(options *ReaderableOptions) *engine.ReaderableOptions {
	if options == nil {
		return nil
	}
	return &engine.ReaderableOptions{
		MinScore: options.MinScore, MinContentLength: options.MinContentLength,
	}
}

func articleFromEngine(article *engine.Article) *Article {
	if article == nil {
		return nil
	}
	return &Article{
		Title: article.Title, Byline: article.Byline, Dir: article.Dir,
		Lang: article.Lang, Content: article.Content, Node: article.Node,
		TextContent: article.TextContent, Length: article.Length,
		Excerpt: article.Excerpt, SiteName: article.SiteName,
		PublishedTime: article.PublishedTime,
	}
}

func publicError(err error) error {
	var limit *engine.TooManyElementsError
	if errors.As(err, &limit) {
		return &TooManyElementsError{Count: limit.Count, Max: limit.Max}
	}
	return err
}

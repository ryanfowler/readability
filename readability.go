// Package readability extracts the main article and its metadata from HTML.
//
// Use Parse for HTML from an io.Reader. Use ParseNode for a tree parsed with
// golang.org/x/net/html. Use IsProbablyReaderable or
// IsProbablyReaderableNode when you only need a fast readerability check.
//
// Parse and the readerability functions use defaults when called without
// options. Pass functional options to change selected settings.
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

type options struct {
	maxElemsToParse     int
	nbTopCandidates     int
	charThreshold       int
	classesToPreserve   []string
	keepClasses         bool
	disableJSONLD       bool
	allowedVideoRegex   *regexp.Regexp
	linkDensityModifier float64
	logger              *slog.Logger
	debug               bool
}

// Option configures article extraction. Options are applied in order, so a
// later option overrides an earlier option for the same setting.
type Option func(*options)

// WithMaxElemsToParse sets the maximum number of HTML elements accepted during
// extraction. Zero removes the limit.
func WithMaxElemsToParse(max int) Option {
	return func(o *options) { o.maxElemsToParse = max }
}

// WithNbTopCandidates sets the number of top article candidates to compare.
func WithNbTopCandidates(count int) Option {
	return func(o *options) { o.nbTopCandidates = count }
}

// WithCharThreshold sets the minimum result length in UTF-16 code units. The
// package retries extraction with less strict cleanup when a result is shorter.
// Zero prevents retries.
func WithCharThreshold(threshold int) Option {
	return func(o *options) { o.charThreshold = threshold }
}

// WithClassesToPreserve sets the CSS classes retained during class cleanup. It
// has no effect when WithKeepClasses(true) is also used.
func WithClassesToPreserve(classes ...string) Option {
	classes = append([]string(nil), classes...)
	return func(o *options) {
		o.classesToPreserve = append([]string(nil), classes...)
	}
}

// WithKeepClasses controls whether all CSS classes are retained.
func WithKeepClasses(keep bool) Option {
	return func(o *options) { o.keepClasses = keep }
}

// WithDisableJSONLD controls whether metadata extraction from JSON-LD is
// disabled.
func WithDisableJSONLD(disable bool) Option {
	return func(o *options) { o.disableJSONLD = disable }
}

// WithAllowedVideoRegex sets the pattern used to identify video URLs that
// cleanup can retain. A nil pattern selects the built-in allowlist.
func WithAllowedVideoRegex(pattern *regexp.Regexp) Option {
	return func(o *options) { o.allowedVideoRegex = pattern }
}

// WithLinkDensityModifier changes the link-density limits used by cleanup
// rules to remove a candidate.
func WithLinkDensityModifier(modifier float64) Option {
	return func(o *options) { o.linkDensityModifier = modifier }
}

// WithLogger sets the logger that receives extraction records. A nil logger
// turns logging off. The package does not use the global slog logger.
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) { o.logger = logger }
}

// WithDebug controls additional verbose log records. A non-nil logger is
// required to receive these records.
func WithDebug(debug bool) Option {
	return func(o *options) { o.debug = debug }
}

type readerableOptions struct {
	minScore         float64
	minContentLength int
}

// ReaderableOption configures the fast readerability heuristic. Options are
// applied in order.
type ReaderableOption func(*readerableOptions)

// WithMinScore sets the score that a document must exceed to be considered
// readerable.
func WithMinScore(score float64) ReaderableOption {
	return func(o *readerableOptions) { o.minScore = score }
}

// WithMinContentLength sets the minimum candidate length in UTF-16 code units.
func WithMinContentLength(length int) ReaderableOption {
	return func(o *readerableOptions) { o.minContentLength = length }
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

// TooManyElementsError reports that a document exceeds the limit set by
// WithMaxElemsToParse.
type TooManyElementsError struct {
	// Count is the number of elements in the document.
	Count int
	// Max is the configured maximum number of elements.
	Max int
}

func (e *TooManyElementsError) Error() string {
	return fmt.Sprintf("readability: %d elements exceeds maximum %d", e.Count, e.Max)
}

// Parse reads HTML from input and extracts an article.
//
// pageURL can be empty. If it is not empty, it must be an absolute HTTP or
// HTTPS URL with a host. Parse uses pageURL and the document base URL to
// resolve relative links and media URLs.
//
// With no options, Parse uses the Mozilla defaults. Parse returns input read
// errors directly. Other errors support errors.Is with ErrNoBody,
// ErrInvalidURL, or ErrNoContent. Parse can also return a
// *TooManyElementsError.
func Parse(input io.Reader, pageURL string, opts ...Option) (*Article, error) {
	article, err := engine.Parse(input, pageURL, optionsToEngine(opts))
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
// with a host. With no options, ParseNode uses the Mozilla defaults.
func ParseNode(root *html.Node, pageURL string, opts ...Option) (*Article, error) {
	article, err := engine.ParseNode(root, pageURL, optionsToEngine(opts))
	if err != nil {
		return nil, publicError(err)
	}
	return articleFromEngine(article), nil
}

// IsProbablyReaderable reports whether input is likely to contain an article.
// It applies a fast heuristic and does not extract the article.
func IsProbablyReaderable(input string, opts ...ReaderableOption) bool {
	return engine.IsProbablyReaderable(input, readerableOptionsToEngine(opts))
}

// IsProbablyReaderableNode reports whether a parsed HTML tree is likely to
// contain an article. It does not change root.
func IsProbablyReaderableNode(root *html.Node, opts ...ReaderableOption) bool {
	return engine.IsProbablyReaderableNode(root, readerableOptionsToEngine(opts))
}

func optionsToEngine(opts []Option) *engine.Options {
	defaults := engine.DefaultOptions()
	o := options{
		maxElemsToParse:     defaults.MaxElemsToParse,
		nbTopCandidates:     defaults.NbTopCandidates,
		charThreshold:       defaults.CharThreshold,
		classesToPreserve:   append([]string(nil), defaults.ClassesToPreserve...),
		keepClasses:         defaults.KeepClasses,
		disableJSONLD:       defaults.DisableJSONLD,
		allowedVideoRegex:   defaults.AllowedVideoRegex,
		linkDensityModifier: defaults.LinkDensityModifier,
		logger:              defaults.Logger,
		debug:               defaults.Debug,
	}
	for _, option := range opts {
		if option != nil {
			option(&o)
		}
	}
	return &engine.Options{
		MaxElemsToParse:     o.maxElemsToParse,
		NbTopCandidates:     o.nbTopCandidates,
		CharThreshold:       o.charThreshold,
		ClassesToPreserve:   append([]string(nil), o.classesToPreserve...),
		KeepClasses:         o.keepClasses,
		DisableJSONLD:       o.disableJSONLD,
		AllowedVideoRegex:   o.allowedVideoRegex,
		LinkDensityModifier: o.linkDensityModifier,
		Logger:              o.logger,
		Debug:               o.debug,
	}
}

func readerableOptionsToEngine(opts []ReaderableOption) *engine.ReaderableOptions {
	defaults := engine.DefaultReaderableOptions()
	o := readerableOptions{
		minScore:         defaults.MinScore,
		minContentLength: defaults.MinContentLength,
	}
	for _, option := range opts {
		if option != nil {
			option(&o)
		}
	}
	return &engine.ReaderableOptions{
		MinScore: o.minScore, MinContentLength: o.minContentLength,
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

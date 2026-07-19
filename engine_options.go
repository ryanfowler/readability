package readability

import (
	"log/slog"
	"regexp"

	"golang.org/x/net/html"
)

type engineOptions struct {
	maxElemsToParse     int
	nbTopCandidates     int
	charThreshold       int
	classesToPreserve   []string
	keepClasses         bool
	serializer          func(doc *html.Node) string
	html2text           func(htmlSrc string) string
	disableJSONLD       bool
	allowedVideoRegex   *regexp.Regexp
	minContentLength    int
	minScore            float64
	visibilityChecker   func(*html.Node) bool
	linkDensityModifier float64
	logger              *slog.Logger
	debug               bool
}

type engineOption func(*engineOptions)

func defaultOpts() *engineOptions {
	return &engineOptions{
		maxElemsToParse:   defaultMaxElemsToParse,
		nbTopCandidates:   defaultNTopCandidates,
		charThreshold:     defaultCharThreshold,
		classesToPreserve: classesToPreserve,
		allowedVideoRegex: videos,
		serializer: func(n *html.Node) string {
			return innerHTML(n)
		},
		minScore:          20,
		minContentLength:  140,
		visibilityChecker: isNodeVisible,
	}
}

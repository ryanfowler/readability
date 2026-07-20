package readability

import (
	"log/slog"
	"regexp"
)

// extractionOptions is the normalized internal configuration for one
// extraction. Readerability has separate options because it is an independent
// heuristic and does not use extraction state.
type extractionOptions struct {
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

func defaultExtractionOptions() extractionOptions {
	return extractionOptions{
		nbTopCandidates:   defaultNTopCandidates,
		charThreshold:     defaultCharThreshold,
		classesToPreserve: classesToPreserve,
		allowedVideoRegex: videos,
	}
}

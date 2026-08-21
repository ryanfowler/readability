package engine

import (
	"math/bits"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	unlikelyCandidateParts = []string{
		"-ad-", "ai2html", "banner", "breadcrumbs", "combx", "comment", "community", "cover-wrap", "disqus", "extra", "footer", "gdpr", "header", "legends", "menu", "related", "remark", "replies", "rss", "shoutbox", "sidebar", "skyscraper", "social", "sponsor", "supplemental", "ad-break", "agegate", "pagination", "pager", "popup", "yom-remote",
	}
	maybeCandidateParts = []string{"and", "article", "body", "column", "content", "main", "shadow"}
	bylineParts         = []string{"byline", "author", "dateline", "writtenby", "p-author"}
	positiveParts       = []string{
		"article", "body", "content", "entry", "hentry", "h-entry", "main",
		"page", "pagination", "post", "text", "blog", "story",
	}
	negativeParts = []string{
		"-ad-", "hidden", "banner", "combx", "comment", "com-", "contact",
		"foot", "footer", "footnote", "gdpr", "masthead", "media", "meta",
		"outbrain", "promo", "related", "scroll", "share", "shoutbox", "sidebar",
		"skyscraper", "sponsor", "shopping", "tags", "tool", "widget",
	}
)

// Precomputed searchers for the fixed literal sets above. They run for the
// class and id of nearly every visited element.
var (
	unlikelyCandidateMatcher = newLiteralMatcher(unlikelyCandidateParts...)
	maybeCandidateMatcher    = newLiteralMatcher(maybeCandidateParts...)
	bylineMatcher            = newLiteralMatcher(bylineParts...)
	positiveMatcher          = newLiteralMatcher(positiveParts...)
	negativeMatcher          = newLiteralMatcher(negativeParts...)
)

// literalMatcher reports whether s contains any of its fixed ASCII literals,
// comparing case-insensitively with the same semantics as
// strings.EqualFold. A 256-entry table maps each byte to the parts that can
// start with it, so almost every position in s is rejected by one load and a
// zero test instead of an inner comparison loop per part.
type literalMatcher struct {
	parts []string
	first [256]uint64
}

func newLiteralMatcher(parts ...string) *literalMatcher {
	m := &literalMatcher{parts: parts}
	for i, part := range parts {
		if part == "" {
			continue
		}
		if c := part[0]; c < utf8.RuneSelf {
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			m.first[c] |= 1 << i
		}
	}
	return m
}

func (m *literalMatcher) contains(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		var mask uint64
		c := s[i]
		if c < utf8.RuneSelf {
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			mask = m.first[c]
		} else {
			// EqualFold relates some non-ASCII runes to ASCII ones (for
			// example, ſ and s). These bytes are rare, so scan every part.
			r, size := utf8.DecodeRuneInString(s[i:])
			if r != utf8.RuneError || size > 1 {
				for _, part := range m.parts {
					f, _ := utf8.DecodeRuneInString(part)
					if runeFoldsEqual(r, f) && i+len(part) <= len(s) &&
						foldEqualAt(s[i:i+len(part)], part) {
						return true
					}
				}
			}
			i += size - 1
			continue
		}
		for mask != 0 {
			idx := bits.TrailingZeros64(mask)
			mask &= mask - 1
			part := m.parts[idx]
			end := i + len(part)
			if end > len(s) {
				continue
			}
			if foldEqualAt(s[i:end], part) {
				return true
			}
		}
	}
	return false
}

// foldEqualAt compares equal-length strings case-insensitively. The ASCII
// path avoids calling into unicode for overwhelmingly common class/id values.
func foldEqualAt(a, b string) bool {
	for i := 0; i < len(b); i++ {
		x, y := a[i], b[i]
		if x == y {
			continue
		}
		if x >= utf8.RuneSelf || y >= utf8.RuneSelf {
			// EqualFold includes a few non-ASCII runes in ASCII fold
			// classes (for example, the long s).
			return strings.EqualFold(a, b)
		}
		if x >= 'A' && x <= 'Z' {
			x += 'a' - 'A'
		}
		if y >= 'A' && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

// runeFoldsEqual reports whether two runes fold to the same value under
// Unicode simple case folding, matching strings.EqualFold's per-rune rule.
func runeFoldsEqual(a, b rune) bool {
	if a == b {
		return true
	}
	for s := unicode.SimpleFold(a); s != a; s = unicode.SimpleFold(s) {
		if s == b {
			return true
		}
	}
	return false
}

func matchesUnlikelyCandidate(s string) bool { return unlikelyCandidateMatcher.contains(s) }
func matchesMaybeCandidate(s string) bool    { return maybeCandidateMatcher.contains(s) }

func matchesByline(s string) bool { return bylineMatcher.contains(s) }

func matchesShareElement(s string) bool {
	if s == "" {
		return false
	}
	// Equivalent to (?i)(\b|_)(share|sharedaddy)(\b|_). Go regexp's word
	// boundaries are ASCII-only; an underscore is explicitly also a boundary.
	for _, part := range [...]string{"sharedaddy", "share"} {
		for i := 0; i+len(part) <= len(s); i++ {
			if i > 0 && isASCIIAlphaNumeric(s[i-1]) ||
				i+len(part) < len(s) && isASCIIAlphaNumeric(s[i+len(part)]) {
				continue
			}
			if strings.EqualFold(s[i:i+len(part)], part) {
				return true
			}
		}
	}
	return false
}

func isASCIIAlphaNumeric(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func matchesPositive(s string) bool { return positiveMatcher.contains(s) }

// containsFoldLiteral reports whether s contains part under EqualFold
// semantics. Only the fixed " hid " token search still needs a single-literal
// scan.
func containsFoldLiteral(s, part string) bool {
	for i := 0; i+len(part) <= len(s); i++ {
		if foldEqualAt(s[i:i+len(part)], part) {
			return true
		}
	}
	return false
}

func matchesNegative(s string) bool {
	if negativeMatcher.contains(s) {
		return true
	}
	// The original regexp treats "hid" as negative only when it is the whole
	// value or a space-delimited token.
	return strings.EqualFold(s, "hid") ||
		len(s) >= 4 && (strings.EqualFold(s[:4], "hid ") || strings.EqualFold(s[len(s)-4:], " hid")) ||
		containsFoldLiteral(s, " hid ")
}

// All of the regular expressions in use within readability.
// Defined up here so we don't instantiate them repeatedly in loops.
var (
	//extraneous           = regexp.MustCompile(`(?i)print|archive|comment|discuss|e[\-]?mail|share|reply|all|login|sign|single|utility`)
	//replaceFonts         = regexp.MustCompile(`(?i)<(\/?)font[^>]*>`)
	normalize = regexp.MustCompile(`\s{2,}`)
	videos    = regexp.MustCompile(`(?i)\/\/(www\.)?((dailymotion|youtube|youtube-nocookie|player\.vimeo|v\.qq)\.com|(archive|upload\.wikimedia)\.org|player\.twitch\.tv)`)
	//nextLink             = regexp.MustCompile(`(?i)(next|weiter|continue|>([^\|]|$)|»([^\|]|$))`)
	//prevLink             = regexp.MustCompile(`(prev|earl|old|new|<|«)`)
	tokenize   = regexp.MustCompile(`\W+`)
	whitespace = regexp.MustCompile(`^\s*$`)
	hasContent = regexp.MustCompile(`\S$`)
	srcsetUrl  = regexp.MustCompile(`(\S+)(\s+[\d.]+[xw])?(\s*(?:,|$))`)
	b64DataUrl = regexp.MustCompile(`(?i)^data:\s*([^\s;,]+)\s*;\s*base64\s*,`)
	// See: https://schema.org/Article
	jsonLdArticleTypes = map[string]struct{}{
		"Article": {}, "AdvertiserContentArticle": {}, "NewsArticle": {},
		"AnalysisNewsArticle": {}, "AskPublicNewsArticle": {}, "BackgroundNewsArticle": {},
		"OpinionNewsArticle": {}, "ReportageNewsArticle": {}, "ReviewNewsArticle": {},
		"Report": {}, "SatiricalArticle": {}, "ScholarlyArticle": {},
		"MedicalScholarlyArticle": {}, "SocialMediaPosting": {}, "BlogPosting": {},
		"LiveBlogPosting": {}, "DiscussionForumPosting": {}, "TechArticle": {},
		"APIReference": {},
	}
	titleFinalPart       = regexp.MustCompile(` [\|\-–—\\\/>»] `)
	titleSeparators      = regexp.MustCompile(` [\\\/>»] `)
	otherTitleSeparators = regexp.MustCompile(`(?i)(.*)[\|\-–—\\\/>»] .*`)
	titleFirstPart       = regexp.MustCompile(`(?i)^[^\|\-–—\\\/>»]*[\|\-–—\\\/>»]`)
	multipleWhitespaces  = regexp.MustCompile(`\s+`)
	singleWhitespace     = regexp.MustCompile(`\s`)
	singleDot            = regexp.MustCompile(`\.`)
	separators           = regexp.MustCompile(`[\|\-–—\\\/>»]+`)
	cdata                = regexp.MustCompile(`^\s*<!\[CDATA\[|\]\]>\s*$`)
	schemaUrl            = regexp.MustCompile(`^https?\:\/\/schema\.org\/?$`)
	// property is a space-separated list of values
	propertyPattern = regexp.MustCompile(`(?i)\s*(article|dc|dcterm|og|twitter)\s*:\s*(author|creator|description|published_time|title|site_name)\s*`)
	// name is a single value
	namePattern                   = regexp.MustCompile(`(?i)^\s*(?:(dc|dcterm|og|twitter|parsely|weibo:(article|webpage))\s*[-\.:]?\s*)?(author|creator|pub-date|description|title|site_name)\s*$`)
	imgExtensions                 = regexp.MustCompile(`\.(jpg|jpeg|png|webp)`)
	imgExtensionsWithSpacesAndNum = regexp.MustCompile(`\.(jpg|jpeg|png|webp)\s+\d`)
	imgExtensionsAmongText        = regexp.MustCompile(`^\s*\S+\.(jpg|jpeg|png|webp)\S*\s*$`)
)

package engine

import (
	"regexp"
	"strings"
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

func containsAnyFold(s string, parts []string) bool {
	if s == "" {
		return false
	}
	// DOM class and id values are overwhelmingly ASCII. Avoid regexp's
	// backtracking alternation while retaining its case-insensitive behavior.
	s = strings.ToLower(s)
	for _, part := range parts {
		if strings.Contains(s, part) {
			return true
		}
	}
	return false
}

func matchesUnlikelyCandidate(s string) bool { return containsAnyFold(s, unlikelyCandidateParts) }
func matchesMaybeCandidate(s string) bool    { return containsAnyFold(s, maybeCandidateParts) }

func containsFoldLiteral(s, part string) bool {
	for i := 0; i+len(part) <= len(s); i++ {
		match := true
		for j := 0; j < len(part); j++ {
			c := s[i+j]
			if c >= utf8.RuneSelf {
				// EqualFold includes a few non-ASCII runes in ASCII fold
				// classes (for example, the long s). Preserve that behavior
				// off the overwhelmingly common ASCII path.
				match = strings.EqualFold(s[i:i+len(part)], part)
				break
			}
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != part[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func matchesByline(s string) bool {
	if s == "" {
		return false
	}
	// This check runs for nearly every element during extraction. Its patterns
	// are fixed ASCII literals, so avoid both regexp backtracking and allocating
	// a lower-cased copy of each class/id value.
	for _, part := range bylineParts {
		if containsFoldLiteral(s, part) {
			return true
		}
	}
	return false
}

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

func matchesPositive(s string) bool {
	for _, part := range positiveParts {
		if containsFoldLiteral(s, part) {
			return true
		}
	}
	return false
}

func matchesNegative(s string) bool {
	for _, part := range negativeParts {
		if containsFoldLiteral(s, part) {
			return true
		}
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

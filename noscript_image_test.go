package readability

import (
	"strings"
	"testing"
)

func TestParseUnwrapsNoscriptImage(t *testing.T) {
	const source = `<html><head><title>Noscript image</title></head><body><article>
<p>This paragraph contains enough useful prose for the article extractor to retain the surrounding article and its image.</p>
<img src="placeholder.gif" alt="Story image"><noscript><img src="https://example.com/full-size.jpg" alt="Story image"></noscript>
<p>Additional article text ensures that image handling is exercised as part of normal extraction rather than an empty document.</p>
</article></body></html>`
	opts := DefaultOptions()
	opts.CharThreshold = 0
	article, err := Parse(source, "https://example.com/story", &opts)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !strings.Contains(article.Content, `src="https://example.com/full-size.jpg"`) {
		t.Fatalf("content does not contain noscript image: %s", article.Content)
	}
	if strings.Contains(article.Content, "<noscript") {
		t.Fatalf("content still contains noscript: %s", article.Content)
	}
}

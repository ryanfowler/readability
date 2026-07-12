package readability

import (
	"strings"
	"testing"
)

func TestParseRemovesImportantHiddenContent(t *testing.T) {
	source := `<html><head><title>Visibility Test</title></head><body>
		<article>
			<p>This visible paragraph contains enough useful article prose to be extracted.</p>
			<p style="display:none !important">DISPLAY-HIDDEN-CONTENT</p>
			<p style="visibility:hidden ! IMPORTANT">VISIBILITY-HIDDEN-CONTENT</p>
		</article>
	</body></html>`
	opts := DefaultOptions()
	opts.CharThreshold = 0
	article, err := Parse(source, "https://example.com/article", &opts)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for _, hidden := range []string{"DISPLAY-HIDDEN-CONTENT", "VISIBILITY-HIDDEN-CONTENT"} {
		if strings.Contains(article.TextContent, hidden) {
			t.Errorf("TextContent retained hidden text %q", hidden)
		}
	}
}

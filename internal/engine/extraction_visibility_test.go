package engine

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
			<p hidden>BOOLEAN-HIDDEN-CONTENT</p>
			<p style="display:NONE">CASE-DISPLAY-HIDDEN-CONTENT</p>
			<p style="visibility:HIDDEN">CASE-VISIBILITY-HIDDEN-CONTENT</p>
			<p aria-hidden="TRUE">ARIA-HIDDEN-CONTENT</p>
			<p style="display:none !important; display:block">PRIORITY-HIDDEN-CONTENT</p>
			<p style="display:none; display:block !important">PRIORITY-VISIBLE-CONTENT</p>
		</article>
	</body></html>`
	opts := DefaultOptions()
	opts.CharThreshold = 0
	article, err := Parse(strings.NewReader(source), "https://example.com/article", &opts)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for _, hidden := range []string{
		"DISPLAY-HIDDEN-CONTENT",
		"VISIBILITY-HIDDEN-CONTENT",
		"BOOLEAN-HIDDEN-CONTENT",
		"CASE-DISPLAY-HIDDEN-CONTENT",
		"CASE-VISIBILITY-HIDDEN-CONTENT",
		"ARIA-HIDDEN-CONTENT",
		"PRIORITY-HIDDEN-CONTENT",
	} {
		if strings.Contains(article.TextContent, hidden) {
			t.Errorf("TextContent retained hidden text %q", hidden)
		}
	}
	if !strings.Contains(article.TextContent, "PRIORITY-VISIBLE-CONTENT") {
		t.Error("TextContent removed content made visible by an important declaration")
	}
}

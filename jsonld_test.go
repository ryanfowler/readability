package readability

import "testing"

func TestJSONLDArticleTypeMatchesExactly(t *testing.T) {
	for _, typ := range []string{
		"Article",
		"NewsArticle",
		"APIReference",
		"https://schema.org/NewsArticle",
		"http://schema.org/APIReference",
	} {
		if !isJSONLDArticleType(typ) {
			t.Errorf("did not match article type %q", typ)
		}
	}
	for _, typ := range []string{
		"NotActuallyANewsArticleGarbage",
		"NewsArticleGarbage",
		"PrefixAPIReference",
		"https://example.com/NewsArticle",
		"https://schema.org/NewsArticleGarbage",
		"https://schema.org/PrefixNewsArticle",
	} {
		if isJSONLDArticleType(typ) {
			t.Errorf("unexpectedly matched %q", typ)
		}
	}
}

func TestJSONLDContextVocabOverride(t *testing.T) {
	doc, err := parseHTML(`<html><head><script type="application/ld+json">
	{
	  "@context": [
	    "https://schema.org",
	    {"@vocab": "https://example.com/"}
	  ],
	  "@type": "NewsArticle",
	  "headline": "Not a Schema.org article"
	}
	</script></head><body></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	if got := (&extractor{}).getJSONLD(doc); got != nil {
		t.Errorf("getJSONLD returned metadata for an overridden Schema.org vocabulary: %+v", got)
	}

	if isSchemaContext([]interface{}{
		"https://schema.org",
		map[string]interface{}{"@vocab": nil},
	}) {
		t.Error("Schema.org context remained active after an explicit @vocab reset")
	}
}

func TestJSONLDArrayFormsAndGraphContainerType(t *testing.T) {
	doc, err := parseHTML(`<html><head><script type="application/ld+json">
	{
	  "@context": ["https://example.com/context", {"@vocab": "https://schema.org"}],
	  "@type": "WebPage",
	  "headline": "Wrong container title",
	  "@graph": [{
	    "@type": ["Thing", "https://schema.org/NewsArticle"],
	    "headline": "Real article title",
	    "author": {"name": "Ada Author"},
	    "description": "Article summary",
	    "datePublished": "2025-01-02",
	    "publisher": {"name": "Example News"}
	  }]
	}
	</script></head><body></body></html>`)
	if err != nil {
		t.Fatal(err)
	}

	got := (&extractor{}).getJSONLD(doc)
	if got == nil {
		t.Fatal("getJSONLD returned nil")
	}
	if got.title != "Real article title" {
		t.Errorf("title = %q, want %q", got.title, "Real article title")
	}
	if got.byline != "Ada Author" {
		t.Errorf("byline = %q, want %q", got.byline, "Ada Author")
	}
	if got.excerpt != "Article summary" {
		t.Errorf("excerpt = %q, want %q", got.excerpt, "Article summary")
	}
	if got.publishedTime != "2025-01-02" {
		t.Errorf("publishedTime = %q, want %q", got.publishedTime, "2025-01-02")
	}
	if got.siteName != "Example News" {
		t.Errorf("siteName = %q, want %q", got.siteName, "Example News")
	}
}

package readability

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestParseRequiresAbsoluteHTTPPageURL(t *testing.T) {
	input := `<html><body><article><p>Article text.</p></article></body></html>`
	for _, pageURL := range []string{
		"/article",
		"example.com/article",
		"javascript:alert(1)",
		"ftp://example.com/article",
		"https:///article",
		"https://:443/article",
	} {
		t.Run(pageURL, func(t *testing.T) {
			_, err := Parse(input, pageURL, nil)
			if !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("Parse error = %v, want ErrInvalidURL", err)
			}
		})
	}
}

func TestBaseURIUsesFirstBaseElement(t *testing.T) {
	doc := &html.Node{Type: html.DocumentNode}
	htmlNode := newElement("html")
	doc.AppendChild(htmlNode)
	head := newElement("head")
	htmlNode.AppendChild(head)
	for _, href := range []string{"https://first.example/articles/", "https://second.example/"} {
		base := newElement("base")
		setAttribute(base, "href", href)
		head.AppendChild(base)
	}
	body := newElement("body")
	htmlNode.AppendChild(body)
	article := newElement("div")
	body.AppendChild(article)
	link := newElement("a")
	setAttribute(link, "href", "story.html")
	article.AppendChild(link)

	r := &engine{doc: doc, body: body, documentURI: "https://origin.example/page", options: defaultOpts(), nodeState: make(map[*html.Node]*nodeData)}
	r.fixRelativeUris(article)
	if got, want := getAttribute(link, "href"), "https://first.example/articles/story.html"; got != want {
		t.Errorf("href = %q, want %q", got, want)
	}
}

func TestFixRelativeUrisRemovesNormalizedJavascriptLinks(t *testing.T) {
	doc := &html.Node{Type: html.DocumentNode}
	body := newElement("body")
	doc.AppendChild(body)
	article := newElement("div")
	body.AppendChild(article)

	for _, href := range []string{
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		" \t\n JAVASCRIPT:alert(1) \r\n",
		"javascript://%zz",
	} {
		link := newElement("a")
		setAttribute(link, "href", href)
		link.AppendChild(&html.Node{Type: html.TextNode, Data: "link"})
		article.AppendChild(link)
	}
	safe := newElement("a")
	setAttribute(safe, "href", " HTTPS://example.com/safe ")
	safe.AppendChild(&html.Node{Type: html.TextNode, Data: "safe"})
	article.AppendChild(safe)

	r := &engine{doc: doc, body: body, documentURI: "https://example.com/article", options: defaultOpts(), nodeState: make(map[*html.Node]*nodeData)}
	r.fixRelativeUris(article)

	links := r.getAllNodesWithTag(article, "a")
	if len(links) != 1 {
		t.Fatalf("remaining links = %d, want 1", len(links))
	}
	if got := getAttribute(links[0], "href"); got != "https://example.com/safe" {
		t.Errorf("safe href = %q, want %q", got, "https://example.com/safe")
	}
}

func TestFixRelativeUrisRemovesJavascriptLinksWithEmbeddedASCIIWhitespace(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`<html><body><div>
		<a href="java&#x09;script:alert(1)">tab</a>
		<a href="java&#x0A;script:alert(1)">line feed</a>
		<a href="java&#x0D;script:alert(1)">carriage return</a>
	</div></body></html>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	body := findElement(doc, "body")
	article := findElement(body, "div")
	r := &engine{doc: doc, body: body, documentURI: "https://example.com/article", options: defaultOpts(), nodeState: make(map[*html.Node]*nodeData)}
	r.fixRelativeUris(article)

	if links := r.getAllNodesWithTag(article, "a"); len(links) != 0 {
		t.Errorf("remaining links = %d, want 0", len(links))
	}
}

func TestFixRelativeUrisWhitespaceOnlyAttributes(t *testing.T) {
	doc := &html.Node{Type: html.DocumentNode}
	body := newElement("body")
	doc.AppendChild(body)
	article := newElement("div")
	body.AppendChild(article)
	link := newElement("a")
	setAttribute(link, "href", " \t\n ")
	link.AppendChild(&html.Node{Type: html.TextNode, Data: "link"})
	article.AppendChild(link)
	image := newElement("img")
	setAttribute(image, "src", "  ")
	setAttribute(image, "poster", "\t")
	setAttribute(image, "srcset", "   ")
	article.AppendChild(image)
	r := &engine{doc: doc, body: body, documentURI: "https://example.com/article", options: defaultOpts(), nodeState: make(map[*html.Node]*nodeData)}
	r.fixRelativeUris(article)
	for _, check := range []struct {
		node *html.Node
		attr string
	}{{link, "href"}, {image, "src"}, {image, "poster"}, {image, "srcset"}} {
		if got := getAttribute(check.node, check.attr); got != "" {
			t.Errorf("%s = %q, want empty", check.attr, got)
		}
	}
}

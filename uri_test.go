package readability

import (
	"golang.org/x/net/html"
	"testing"
)

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

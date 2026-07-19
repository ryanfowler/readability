package readability_test

import (
	"fmt"
	"strings"

	"github.com/ryanfowler/readability"
	"golang.org/x/net/html"
)

func ExampleParse() {
	o := readability.DefaultOptions()
	o.CharThreshold = 0
	a, _ := readability.Parse(`<html><head><title>Hello</title></head><body><article><h1>Hello</h1><p>This is a sufficiently useful article paragraph.</p></article></body></html>`, "https://example.com/", &o)
	fmt.Println(a.Title)
	// Output: Hello
}

func ExampleIsProbablyReaderable() {
	fmt.Println(readability.IsProbablyReaderable(`<article><p>short</p></article>`, nil))
	// Output: false
}

func ExampleParseNode() {
	doc, _ := html.Parse(strings.NewReader(`<html><head><title>News</title></head><body><article><p>An article body with useful prose for extraction.</p></article></body></html>`))
	o := readability.DefaultOptions()
	o.CharThreshold = 0
	a, _ := readability.ParseNode(doc, "https://example.com/news", &o)
	fmt.Println(a.Title)
	// Output: News
}

func ExampleIsProbablyReaderableNode() {
	doc, _ := html.Parse(strings.NewReader(`<article><p>An article body with useful prose for extraction.</p></article>`))
	fmt.Println(readability.IsProbablyReaderableNode(doc, nil))
	// Output: false
}

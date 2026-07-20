package readability_test

import (
	"fmt"
	"strings"

	"github.com/ryanfowler/readability"
	"golang.org/x/net/html"
)

func ExampleParse() {
	const source = `<html>
<head><title>Hello</title></head>
<body><article><h1>Hello</h1><p>This is a useful article paragraph.</p></article></body>
</html>`

	article, err := readability.Parse(
		strings.NewReader(source),
		"https://example.com/",
		readability.WithCharThreshold(0),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(article.Title)
	// Output: Hello
}

func ExampleIsProbablyReaderable() {
	source := `<article><p>short</p></article>`
	fmt.Println(readability.IsProbablyReaderable(source))
	// Output: false
}

func ExampleParseNode() {
	const source = `<html>
<head><title>News</title></head>
<body><article><p>An article body with useful prose.</p></article></body>
</html>`

	document, err := html.Parse(strings.NewReader(source))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	article, err := readability.ParseNode(
		document,
		"https://example.com/news",
		readability.WithCharThreshold(0),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(article.Title)
	// Output: News
}

func ExampleIsProbablyReaderableNode() {
	source := `<article><p>An article body with useful prose.</p></article>`
	document, err := html.Parse(strings.NewReader(source))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(readability.IsProbablyReaderableNode(document))
	// Output: false
}

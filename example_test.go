package readability_test

import (
	"fmt"
	"github.com/ryanfowler/readability"
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

func ExampleDocument() {
	d, _ := readability.NewDocument(`<article><h1>News</h1><p>An article body with useful prose for extraction.</p></article>`)
	fmt.Println(d.IsProbablyReaderable(nil))
	// Output: false
}

# readability

[![Go Reference](https://pkg.go.dev/badge/github.com/ryanfowler/readability.svg)](https://pkg.go.dev/github.com/ryanfowler/readability)

`readability` extracts the main article and its metadata from HTML. It is an idiomatic Go port of [Mozilla Readability](https://github.com/mozilla/readability).

The package can return these values:

- Processed article HTML
- Plain article text
- The article title, author, excerpt, site name, language, text direction, and publication time
- A parsed `html.Node` for the article

> [!WARNING]
> `Article.Content` and `Article.Node` can contain unsafe HTML. The package does not sanitize these values. Sanitize the HTML before you add it to a web page.

## Requirements

- Go 1.23 or a later version

## Installation

```sh
go get github.com/ryanfowler/readability
```

## Quick start

Pass an HTML reader and the page URL to `Parse`:

```go
package main

import (
    "fmt"
    "log"
    "strings"

    "github.com/ryanfowler/readability"
)

func main() {
    source := `<html>
<head><title>Example article</title></head>
<body><article><p>This is the article text.</p></article></body>
</html>`

    article, err := readability.Parse(strings.NewReader(source), "https://example.com/news/1")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(article.Title)
    fmt.Println(article.TextContent)
}
```

Use the source page URL when it is available. The package uses this URL and the document `<base>` element to resolve relative links and media URLs. The page URL can be empty. A nonempty page URL must be an absolute HTTP or HTTPS URL with a host.

Calling `Parse` without functional options selects the default settings.

## Check a page before extraction

`IsProbablyReaderable` applies a fast heuristic. It does not extract the article. Use it when you only need to know if a page is likely to contain an article:

```go
if readability.IsProbablyReaderable(source) {
    article, err := readability.Parse(strings.NewReader(source), pageURL)
    if err != nil {
        log.Printf("extraction failed: %v", err)
        return
    }
    fmt.Println(article.Title)
}
```

A `true` result is not a guarantee that extraction will succeed. A `false` result does not mean that the HTML is invalid.

## Use a parsed HTML tree

Use `ParseNode` if you already parsed the HTML with `golang.org/x/net/html`. This prevents a second parse of the source:

```go
doc, err := html.Parse(strings.NewReader(source))
if err != nil {
    // Handle the error.
}

if readability.IsProbablyReaderableNode(doc) {
    article, err := readability.ParseNode(doc, pageURL)
    if err != nil {
        log.Printf("extraction failed: %v", err)
        return
    }
    fmt.Println(article.Title)
}
```

The node functions accept a complete document or a tree that has a `body` root. They do not change the supplied tree. You can use the same tree in later calls. Do not change the tree while either node function uses it.

## Configure extraction

Pass functional options after the page URL. Unspecified settings retain their defaults:

```go
article, err := readability.Parse(
    strings.NewReader(source),
    pageURL,
    readability.WithCharThreshold(100),
    readability.WithMaxElemsToParse(50_000),
)
```

Options are reusable and are applied from left to right. If a setting appears more than once, the last option wins.

| Functional option | Default | Function |
| --- | ---: | --- |
| `WithMaxElemsToParse(0)` | `0` | Sets the maximum number of HTML elements. Zero removes the limit. |
| `WithNbTopCandidates(5)` | `5` | Sets the number of top article candidates to compare. |
| `WithCharThreshold(500)` | `500` | Retries extraction when the result is shorter than this value. Zero prevents retries. |
| `WithClassesToPreserve("page")` | `[]string{"page"}` | Lists the CSS classes to retain when class cleanup is active. |
| `WithKeepClasses(false)` | `false` | Retains all CSS classes when true. |
| `WithDisableJSONLD(false)` | `false` | Prevents metadata extraction from JSON-LD when true. |
| `WithAllowedVideoRegex(pattern)` | Built-in allowlist | Identifies video URLs that cleanup can retain. A nil pattern selects the built-in allowlist. |
| `WithLinkDensityModifier(0)` | `0` | Changes the link-density limits that remove a candidate. |
| `WithLogger(logger)` | `nil` | Receives extraction log records. Nil turns logs off. |
| `WithDebug(false)` | `false` | Adds verbose debug data to log records when true. |

`WithCharThreshold` controls extraction retries. If a result is too short, the package retries with less strict removal and cleanup rules. If all results are too short, the package returns the longest nonempty result.

Pass `WithLogger` a `*slog.Logger` to receive logs. The logger handler controls the log level and output. The package does not use the global `slog` logger.

The heuristic has separate options:

```go
likely := readability.IsProbablyReaderable(
    source,
    readability.WithMinContentLength(200),
    readability.WithMinScore(20),
)
```

## Character counts

These values use UTF-16 code units:

- `Article.Length`
- The value passed to `WithCharThreshold`
- The value passed to `WithMinContentLength`

This rule matches JavaScript `String.length` and Mozilla Readability. Most characters count as one unit. A character outside the Basic Multilingual Plane, such as many emoji, counts as two units.

## Handle errors

Errors encountered while reading the input are returned directly. Use `errors.Is` with the package error values:

- `ErrNoBody`: the input does not contain a required `body` element.
- `ErrInvalidURL`: the page URL is not valid.
- `ErrNoContent`: extraction did not produce article content.

Use `errors.As` to inspect an element-limit error:

```go
_, err := readability.Parse(
    strings.NewReader(source),
    pageURL,
    readability.WithMaxElemsToParse(50_000),
)
if err != nil {
    var limitErr *readability.TooManyElementsError
    switch {
    case errors.As(err, &limitErr):
        log.Printf("document has %d elements; limit is %d", limitErr.Count, limitErr.Max)
    case errors.Is(err, readability.ErrNoContent):
        log.Print("no article content found")
    default:
        log.Printf("extraction failed: %v", err)
    }
}
```

## Output safety

Treat all extracted data as untrusted input.

- Sanitize `Article.Content` before you add it to a web page.
- Sanitize or validate `Article.Node` before you render it.
- Escape plain-text metadata for its output context.
- Pass `WithMaxElemsToParse` when you process HTML from an untrusted source and need an element limit.

## Compatibility

This package tracks Mozilla Readability.js at commit `ab4027a8b37669745016869a37a504727992b2ba`. The repository pins Mozilla's official 130-case test corpus in `tests/readability-js`.

Go parses and serializes the HTML instead of a browser DOM. As a result, attribute order and other HTML serialization details can differ.

## License

The project is licensed under the [Apache License 2.0](LICENSE); see [NOTICE](NOTICE) for Mozilla Readability and Arc90 Readability attribution. The pinned Mozilla test fixtures retain their original license.

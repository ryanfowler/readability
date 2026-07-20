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

Pass the HTML source and the page URL to `Parse`:

```go
package main

import (
    "fmt"
    "log"

    "github.com/ryanfowler/readability"
)

func main() {
    source := `<html>
<head><title>Example article</title></head>
<body><article><p>This is the article text.</p></article></body>
</html>`

    article, err := readability.Parse(source, "https://example.com/news/1", nil)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(article.Title)
    fmt.Println(article.TextContent)
}
```

Use the source page URL when it is available. The package uses this URL and the document `<base>` element to resolve relative links and media URLs. The page URL can be empty. A nonempty page URL must be an absolute HTTP or HTTPS URL with a host.

A `nil` options pointer selects the default options.

## Check a page before extraction

`IsProbablyReaderable` applies a fast heuristic. It does not extract the article. Use it when you only need to know if a page is likely to contain an article:

```go
if readability.IsProbablyReaderable(source, nil) {
    article, err := readability.Parse(source, pageURL, nil)
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

if readability.IsProbablyReaderableNode(doc, nil) {
    article, err := readability.ParseNode(doc, pageURL, nil)
    if err != nil {
        log.Printf("extraction failed: %v", err)
        return
    }
    fmt.Println(article.Title)
}
```

The node functions accept a complete document or a tree that has a `body` root. They do not change the supplied tree. You can use the same tree in later calls. Do not change the tree while either node function uses it.

## Configure extraction

Start with `DefaultOptions` when you want to change one or more extraction options:

```go
options := readability.DefaultOptions()
options.CharThreshold = 100
options.MaxElemsToParse = 50_000

article, err := readability.Parse(source, pageURL, &options)
```

Do not use a partial struct literal unless you need its zero values. A non-nil `Options` value supplies the full configuration. The exception is `AllowedVideoRegex`: a nil value keeps the built-in video allowlist.

| Option | Default | Function |
| --- | ---: | --- |
| `MaxElemsToParse` | `0` | Sets the maximum number of HTML elements. Zero removes the limit. |
| `NbTopCandidates` | `5` | Sets the number of top article candidates to compare. |
| `CharThreshold` | `500` | Retries extraction when the result is shorter than this value. Zero prevents retries. |
| `ClassesToPreserve` | `[]string{"page"}` | Lists the CSS classes to retain when class cleanup is active. |
| `KeepClasses` | `false` | Retains all CSS classes when true. |
| `DisableJSONLD` | `false` | Prevents metadata extraction from JSON-LD when true. |
| `AllowedVideoRegex` | Built-in allowlist | Identifies video URLs that cleanup can retain. |
| `LinkDensityModifier` | `0` | Changes the link-density limits that remove a candidate. |
| `Logger` | `nil` | Receives extraction log records. Nil turns logs off. |
| `Debug` | `false` | Adds verbose debug data to log records when true. |

`CharThreshold` controls extraction retries. If a result is too short, the package retries with less strict removal and cleanup rules. If all results are too short, the package returns the longest nonempty result.

Set `Options.Logger` to a `*slog.Logger` to receive logs. The logger handler controls the log level and output. The package does not use the global `slog` logger.

Use `DefaultReaderableOptions` to change the readerability heuristic:

```go
options := readability.DefaultReaderableOptions()
options.MinContentLength = 200
likely := readability.IsProbablyReaderable(source, &options)
```

## Character counts

These values use UTF-16 code units:

- `Article.Length`
- `Options.CharThreshold`
- `ReaderableOptions.MinContentLength`

This rule matches JavaScript `String.length` and Mozilla Readability. Most characters count as one unit. A character outside the Basic Multilingual Plane, such as many emoji, counts as two units.

## Handle errors

Use `errors.Is` with the package error values:

- `ErrNoBody`: the input does not contain a required `body` element.
- `ErrInvalidURL`: the page URL is not valid.
- `ErrNoContent`: extraction did not produce article content.

Use `errors.As` to inspect an element-limit error:

```go
_, err := readability.Parse(source, pageURL, &options)
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
- Set `MaxElemsToParse` when you process HTML from an untrusted source and need an element limit.

## Compatibility

This package tracks Mozilla Readability.js at commit `ab4027a8b37669745016869a37a504727992b2ba`. The repository pins Mozilla's official 130-case test corpus in `tests/readability-js`.

Go parses and serializes the HTML instead of a browser DOM. As a result, attribute order and other HTML serialization details can differ.

## License

The project is licensed under the [Apache License 2.0](LICENSE); see [NOTICE](NOTICE) for Mozilla Readability and Arc90 Readability attribution. The pinned Mozilla test fixtures retain their original license.

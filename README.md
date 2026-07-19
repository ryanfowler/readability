# readability

An idiomatic Go port of Mozilla Readability for extracting an article from HTML.

```sh
go get github.com/ryanfowler/readability
```

```go
article, err := readability.Parse(source, "https://example.com/story", nil)
```

Use `IsProbablyReaderable` for the inexpensive heuristic. If HTML is already parsed with `golang.org/x/net/html`, use `ParseNode` and `IsProbablyReaderableNode` to avoid parsing it again:

```go
doc, err := html.Parse(strings.NewReader(source))
if err != nil {
    // handle error
}
if readability.IsProbablyReaderableNode(doc, nil) {
    article, err := readability.ParseNode(doc, pageURL, nil)
    // handle article and err
}
```

The node APIs do not mutate the supplied tree, so it can be checked or parsed repeatedly. The caller must not mutate it concurrently while either function is running.

`DefaultOptions` and `DefaultReaderableOptions` return independent default values. A non-nil options struct is used exactly as supplied because several zero values are meaningful. To override selected settings, always begin with the relevant defaults:

```go
opts := readability.DefaultOptions()
opts.CharThreshold = 100
article, err := readability.Parse(source, pageURL, &opts)
```

Set `Options.Logger` to a `*slog.Logger` to receive extraction logs. Logging is disabled when it is nil; the package does not use slog's global default logger.

Passing `nil` selects all defaults. Do not use a partially populated literal such as `&readability.Options{CharThreshold: 100}` unless the other zero-valued settings are intentional.

`CharThreshold` controls extraction retries. When an attempt produces less text than the threshold, extraction runs again with progressively less aggressive candidate removal, class weighting, and conditional cleanup. If no attempt reaches the threshold, the longest non-empty result is returned. Set `CharThreshold` to zero to disable retries.

All advertised character counts—including `CharThreshold`, `ReaderableOptions.MinContentLength`, and `Article.Length`—use UTF-16 code units, matching JavaScript `String.length` and Mozilla Readability. Thus most characters count as one unit and characters outside the Basic Multilingual Plane (such as many emoji) count as two.

Errors support `errors.Is` for `ErrNoContent`, `ErrNoBody`, and `ErrInvalidURL`; use `errors.As` for `*TooManyElementsError`. Relative links and media URLs are resolved against the page URL and document base URL.

## Compatibility

Behavior tracks Mozilla Readability.js at commit `08be6b4bdb204dd333c9b7a0cfbc0e730b257252`; its official 130-case corpus is pinned as `tests/readability-js`. HTML is parsed and serialized with Go rather than a browser DOM, so inconsequential attribute ordering and HTML serialization can differ.

> **Security:** `Article.Content` and `Article.Node` are not sanitized. Never render either in a trusted page without applying an appropriate HTML sanitizer.

This implementation includes code derived from Mozilla Readability and Arc90 Readability under the Apache License 2.0. Mozilla's fixture license is retained in the pinned submodule.

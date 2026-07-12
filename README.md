# readability

An idiomatic Go port of Mozilla Readability for extracting an article from HTML.

```sh
go get github.com/ryanfowler/readability
```

```go
article, err := readability.Parse(source, "https://example.com/story", nil)
```

Use `IsProbablyReaderable` for the inexpensive heuristic, or `NewDocument` when checking before extraction. A `Document` can be checked repeatedly, but `Document.Parse` consumes it and returns `ErrDocumentConsumed` on reuse.

`DefaultOptions` and `DefaultReaderableOptions` return independent default values. A non-nil options struct is used exactly as supplied because several zero values are meaningful. To override selected settings, always begin with the relevant defaults:

```go
opts := readability.DefaultOptions()
opts.CharThreshold = 100
article, err := readability.Parse(source, pageURL, &opts)
```

Passing `nil` selects all defaults. Do not use a partially populated literal such as `&readability.Options{CharThreshold: 100}` unless the other zero-valued settings are intentional.

Errors support `errors.Is` for `ErrNoContent`, `ErrNoBody`, `ErrInvalidURL`, and `ErrDocumentConsumed`; use `errors.As` for `*TooManyElementsError`. Relative links and media URLs are resolved against the page URL and document base URL.

## Compatibility

Behavior tracks Mozilla Readability.js at commit `08be6b4bdb204dd333c9b7a0cfbc0e730b257252`; its official 130-case corpus is pinned as `tests/readability-js`. HTML is parsed and serialized with Go rather than a browser DOM, so inconsequential attribute ordering and HTML serialization can differ.

> **Security:** `Article.Content` is not sanitized. Never insert content into a trusted page without applying an appropriate HTML sanitizer.

This implementation includes code derived from Mozilla Readability and Arc90 Readability under the Apache License 2.0. Mozilla's fixture license is retained in the pinned submodule.

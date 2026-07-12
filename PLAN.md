# Plan: port `legible` from Rust to Go

## Goal and compatibility contract

Implement an idiomatic Go package in this repository that ports `/Users/ryanfowler/code/legible` (currently v0.4.2) while preserving Mozilla Readability.js behavior. Treat behavior—not a line-for-line Rust translation—as the contract. The primary guardrail is Mozilla's official test corpus at the exact revision currently used by Legible:

- upstream: `https://github.com/mozilla/readability.git`
- submodule path: `tests/readability-js`
- pinned commit: `08be6b4bdb204dd333c9b7a0cfbc0e730b257252` (`0.6.0-10-g08be6b4`)
- corpus: `tests/readability-js/test/test-pages`
- current corpus size: 130 cases, each containing `source.html`, `expected.html`, and `expected-metadata.json`
- verified Rust baseline (2026-07-12): 10 unit tests, all 130 corpus tests, and 12 runnable doc tests pass (`cargo test --all-features`)

Do not silently weaken compatibility assertions to make the Go implementation pass. Document any unavoidable DOM-parser/serializer discrepancy and prefer fixing behavior first. The returned article HTML remains **unsanitized**, matching Readability.js/Legible.

## Sources of truth

Use these in this order when behavior is ambiguous:

1. The pinned `tests/readability-js/Readability.js` and `Readability-readerable.js` implementation.
2. The pinned Mozilla fixtures and expected metadata/content.
3. The working Rust port under `/Users/ryanfowler/code/legible/src`.

The Rust port is about 5,136 lines and already resolves many non-JavaScript DOM issues. Port its current code, not an older crate release. Relevant mapping:

| Rust source | Go responsibility |
|---|---|
| `src/lib.rs`, `src/document.rs`, `src/error.rs`, `src/options.rs` | Public API, parsed document wrapper, options/defaults, errors |
| `src/constants.rs`, `src/selectors.rs` | flags, regexes, tag/attribute sets, compiled selectors |
| `src/dom/{mod,node,traversal}.rs` | DOM helpers, stable node-keyed score/table/stat state, traversal/mutation utilities |
| `src/readerable.rs` | fast readerability heuristic |
| `src/metadata.rs` | title, meta/OpenGraph and JSON-LD extraction, entity handling |
| `src/scoring.rs` | text normalization, visibility, class weights, link density, candidate scoring |
| `src/cleaning.rs` | document preparation, lazy/noscript images, tables, conditional cleaning |
| `src/readability.rs` | parse orchestration, retries/flags, candidate selection, sibling gathering, output post-processing |
| `src/logging.rs` | opt-in debug output |
| `tests/integration_tests.rs` | corpus test semantics to reproduce in Go |
| `benches/readability.rs` | benchmark page selection and workloads |

When updating compatibility after the initial port, compare against the pinned JavaScript directly; do not assume every Rust optimization is semantically perfect.

## Proposed Go API

Use module path `github.com/ryanfowler/readability` (`go.mod`) and root package name `readability`.

```go
type Article struct {
    Title         string
    Byline       string
    Dir          string
    Lang         string
    Content      string
    TextContent  string
    Length       int
    Excerpt      string
    SiteName     string
    PublishedTime string
}

type Options struct {
    MaxElemsToParse    int
    NbTopCandidates    int
    CharThreshold      int
    ClassesToPreserve  []string
    KeepClasses        bool
    DisableJSONLD      bool
    AllowedVideoRegex  *regexp.Regexp
    LinkDensityModifier float64
    Debug              bool
}

type ReaderableOptions struct {
    MinScore         float64
    MinContentLength int
}

func Parse(input string, pageURL string, options *Options) (*Article, error)
func IsProbablyReaderable(input string, options *ReaderableOptions) bool
func NewDocument(input string) (*Document, error)
func (d *Document) IsProbablyReaderable(options *ReaderableOptions) bool
func (d *Document) Parse(pageURL string, options *Options) (*Article, error)
```

Go has no `Option`, so empty strings represent absent article metadata and an empty `pageURL` means no base URL. Keep JSON tags (`title`, `byline`, `dir`, `lang`, `content`, `textContent`, `length`, `excerpt`, `siteName`, `publishedTime`) on `Article` for parity with Readability.js consumers. `NewDocument` should report only genuine HTML parser failures; normal malformed HTML should be repaired by the HTML5 parser. Document that `Document.Parse` mutates/consumes the internal tree and must not be reused afterward (or enforce this with a parsed flag and a sentinel error).

Expose defaults through constructors so zero-valued fields are not confused with intentional values:

```go
func DefaultOptions() Options
func DefaultReaderableOptions() ReaderableOptions
```

Defaults must match Legible/Readability.js: max elements `0`, top candidates `5`, character threshold `500`, preserved classes `[]string{"page"}`, keep classes false, JSON-LD enabled, default video regex, link density modifier `0`, debug false; readerable minimum score `20` and minimum content length `140`. Clone caller-provided options/slices before mutation.

Define sentinel/typed errors suitable for `errors.Is`/`errors.As`: `ErrNoContent`, `ErrNoBody`, `ErrInvalidURL`, and `TooManyElementsError{Count, Max int}`. Preserve the Rust failure timing: reject an invalid supplied URL before extraction and enforce the element cap before mutation.

Before freezing the API, add compile-time example tests. If repository-owner preference differs on optional strings or constructor shape, settle that once; compatibility work should not repeatedly churn the API.

## DOM and dependencies

Start with minimal dependencies:

- `golang.org/x/net/html` for an HTML5-compatible mutable DOM and serialization.
- `github.com/andybalholm/cascadia` only if selectors materially simplify the port; direct traversal is preferable for hot loops and simple selectors.

Do not use XML parsing. Readability depends on HTML5 error recovery, raw-text handling, implied `html/head/body`, comments, SVG, and attribute behavior. Evaluate the parser immediately against edge fixtures (`comment-inside-script-parsing`, `svg-parsing`, `mathjax`, `invalid-attributes`, `base-url-*`). If `x/net/html` creates a body for the no-body unit case, preserve the externally intended `ErrNoBody` semantics based on the parsed/source behavior rather than accidentally deleting that error.

Build a small internal DOM layer instead of scattering raw `html.Node` manipulation:

- case-normalized tag checks and tag-set helpers;
- preorder/postorder traversal and snapshot iteration for mutation-safe loops;
- ancestors, element siblings/children, replace/unwrap/remove/append/clone helpers;
- attribute get/set/remove and class token operations;
- normalized text and inner HTML serialization;
- match-string construction from class/id;
- selector helpers needed by the port.

Use `map[*html.Node]*readabilityData`, `map[*html.Node]bool`, and `map[*html.Node]nodeStats`; pointers are stable and comparable. Keep score/stat state separate from DOM attributes, as the Rust `NodeDataStore` does. Clear it between retry attempts. Be especially careful not to iterate linked sibling lists while removing/reparenting without saving `next` first.

Serialization will differ from Mozilla's browser DOM and Rust's `html5ever`. The corpus test intentionally compares extracted text at first, but URI rewriting, classes, tags, and attributes still require focused structural tests. Do not post-process serialized HTML with broad regexes.

## Semantic hazards to handle explicitly

- **Character counts:** never use byte length for text thresholds or `Article.Length`; use a single helper consistently. The Rust port uses Unicode scalar counts while JavaScript uses UTF-16 code units. First match the passing Rust baseline, then add non-BMP differential tests against JavaScript and choose/document JS-exact UTF-16 counting if results differ.
- **Whitespace:** centralize Readability normalization. Go's `strings.Fields`/Unicode whitespace behavior may not equal JavaScript `\s`; test NBSP, ideographic spaces, and line breaks.
- **Regexes:** copy patterns and case sensitivity from `constants.rs`/pinned JS. Go RE2 has different Unicode word-boundary/case-fold semantics and no lookaround/backreferences. Add regex table tests and implement manual scanners where needed (the Rust image extension helpers are already suitable models).
- **URLs:** use `net/url`, honoring a supplied page URL and document `<base>` exactly as fixture behavior requires. Resolve `href`, `src`, poster and other rewritten media attributes; preserve fragments, data URLs, javascript-link replacement behavior, protocol-relative links, and invalid URLs as Readability does.
- **HTML entities:** parser text is normally decoded, but metadata values and JSON-LD paths need the Rust behavior, including numeric invalid-code-point replacement and only intended named entities.
- **JSON-LD:** support arrays, `@graph`, string/object `@context`, article type filtering, headline/name disambiguation, author as string/object/array, description, publisher/site name, and date publication. JSON errors should be ignored, not fail parsing.
- **Ordering:** Go map iteration must never influence candidate ranking, metadata precedence, output order, or tie-breaking. Keep DOM-order slices for candidates and explicit stable sorting.
- **Floating point:** retain `float64` formulas and exact comparison order from Rust/JS.
- **Nil/mutation:** DOM helpers must tolerate detached nodes and absent parents. Clone/reparse from original HTML for retry attempts as the Rust parser does.
- **Output security:** stripping scripts during extraction is not sanitization; preserve the README warning.

## Implementation sequence

### 1. Repository and test scaffolding

1. Add `go.mod`, `.gitmodules`, and the pinned Mozilla submodule at `tests/readability-js`.
2. Add `testdata`/fixture-loading helpers that discover every `*/source.html`; fail if zero cases are found and assert the expected baseline count (130) so a missing submodule cannot produce a false green build. Keep the pin recorded in this file and README.
3. Port the Rust integration harness to `integration_test.go` as table-driven subtests named by fixture directory. For each case use base URL `http://fakehost/test/<case>`.
4. Decode `expected-metadata.json` with pointer fields so absent and explicitly empty values remain distinguishable. Assert title, byline, excerpt (normalized whitespace), site name, and direction exactly as the Rust harness does. Also assert `lang` and `publishedTime` once parity is established—the Rust struct reads them but currently omits published time and language checks, which is a coverage gap. Assert expected `readerable` where present.
5. Compare expected and actual content text using the same Jaccard set-of-words threshold (`>= 0.90`) initially. Parse HTML to extract text rather than using `<[^>]+>` once possible, because regex extraction mishandles literal angle brackets. Include useful mismatch previews.
6. Add an opt-in strict/differential test mode (for example `READABILITY_STRICT=1`) that compares normalized DOM structure/attributes and invokes Node against pinned `Readability.js`; normal CI must not require Node. This is invaluable when loose text similarity masks markup regressions.

### 2. Public types, defaults, and basic DOM layer

Implement `Article`, options, errors, `Document`, parser setup, DOM traversal/mutation, text/serialization helpers, logging, constants, and compiled regexes/selectors. Port the 10 existing Rust unit tests immediately, including readerable-then-parse, invalid URL, no body, and option/error behavior. Add tests for default cloning and repeated/concurrent independent parses (`go test -race`).

### 3. Readerability heuristic

Port `readerable.rs` first as a thin vertical slice:

- collect `p`, `pre`, `article`, plus unique parent `div`s of `br`s;
- skip invisible/unlikely nodes and paragraphs under list items;
- apply minimum content length and cumulative `sqrt(length-min)` scoring;
- return when score exceeds (not equals) the minimum.

Port the four Rust readerable unit tests and fixture `readerable` metadata tests. This validates parsing, text counting, visibility, regexes, and traversal before extraction complexity.

### 4. Metadata

Port title extraction, HTML entity handling, JSON-LD, OpenGraph and ordinary meta precedence. Keep metadata extraction in the same order as Rust: title and JSON-LD before scripts are removed, then remaining metadata after document preparation. Add focused table tests for the numbered metadata fixtures, schema context object, Parsely metadata, title separators/dashes, malformed JSON, and author forms.

### 5. Scoring and document preparation

Port `scoring.rs`, then preparation portions of `cleaning.rs`:

- normalized text stats/cache and comma/sentence scoring;
- node initialization by tag and class/id weights;
- link density (including modifier), visibility, phrasing/block detection, byline validity;
- script/style cleanup, `<br>` paragraph conversion, font replacement, DIV-to-P logic;
- noscript/lazy image repair and image/srcset detection;
- table classification and nested element simplification.

Write focused tests for every helper whose behavior can vary by parser. Run fixture subsets (`replace-*`, `lazy-image-*`, table, hidden/style, SVG/video) after each group.

### 6. Main extraction algorithm

Port `Readability` orchestration in the same phase/order as `readability.rs`:

1. validate URL and element count;
2. unwrap noscript images;
3. extract title/JSON-LD;
4. remove scripts and prepare document;
5. extract metadata;
6. scan/remove unlikely candidates and transform nodes;
7. score content nodes and ancestors;
8. rank top candidates, apply link density, and choose a top candidate;
9. gather qualifying siblings into a new article container;
10. prepare/clean the article;
11. retry by progressively disabling `FLAG_STRIP_UNLIKELYS`, `FLAG_WEIGHT_CLASSES`, and `FLAG_CLEAN_CONDITIONALLY` when below `CharThreshold`, reparsing the original HTML and clearing node state between attempts;
12. retain the best attempt or return `ErrNoContent`;
13. post-process relative URIs, classes, attributes, duplicate headers, and output HTML/text;
14. merge excerpt/metadata and populate direction/language/length.

Port `cleaning.rs` in lockstep with the calls from this flow rather than inventing a new pipeline. Preserve tie-breaking, sibling score thresholds, byline removal, header/title similarity, allowed video handling, data-table exemptions, and class preservation exactly.

### 7. Tighten compatibility

Once all 130 tests pass at the Rust harness's threshold:

- compare Go output, Rust output, and pinned JS output for every fixture and classify differences as text, metadata, structure, attributes, URL rewriting, or serialization only;
- add regression tests for every behavior fix;
- enable exact checks for `TextContent`, `Length`, `Lang`, `PublishedTime`, URL-bearing attributes, preserved classes, and key fixture DOM structures;
- prefer canonical DOM comparison (tag, ordered children, sorted attributes where order is irrelevant) over raw HTML string comparison;
- do not change the 0.90 test into the only definition of success: unique-word Jaccard ignores duplicates and ordering.

Exit criterion: all 130 corpus tests pass, all focused tests pass under `go test ./...`, race tests are clean, and no known behavioral delta from pinned Readability.js remains undocumented.

### 8. Benchmarks, CI, and documentation

- Port benchmark workloads from `benches/readability.rs` using Go `testing.B`: small (`basic-tags-cleaning`, `replace-brs`), medium (`medium-2`, `ars-1`, `heise`), large (`nytimes-5`, `wikipedia-2`, `yahoo-2`), complex (`buzzfeed-1`, `engadget`, `guardian-1`), plus readerability checks. Call `b.ReportAllocs()` and report bytes.
- Optimize only after parity. Likely safe optimizations are compiled regexes/selectors, direct traversal, cached normalized text stats, and pre-sized slices/maps. Re-run the full corpus after each optimization.
- Add GitHub Actions jobs that checkout submodules recursively, verify the pinned commit, run `gofmt -l` as a failure check, `go vet ./...`, `go test ./...`, and `go test -race ./...`. Optionally run benchmarks separately without gating on timing.
- Expand `README.md` with installation, extraction, readerability, reusable `Document`, all defaults, errors, URL behavior, compatibility pin, and the unsanitized-content warning. Add runnable `ExampleParse`, `ExampleIsProbablyReaderable`, and `ExampleDocument` tests.
- Add license/notice attribution for Mozilla fixture/submodule code as required by its license; retain this repository's existing Apache-2.0 license.

## Suggested file layout

```text
go.mod
readability.go          # public Parse and Article
options.go
errors.go
document.go
constants.go
selectors.go            # only if using cascadia
readerable.go
metadata.go
scoring.go
cleaning.go
parser.go               # internal readability state/main algorithm
dom.go                   # internal DOM/state helpers
*_test.go
integration_test.go
benchmark_test.go
tests/readability-js/    # pinned git submodule
.github/workflows/ci.yml
README.md
```

Keep helpers unexported unless consumers need them. Split files further if the direct port becomes unwieldy, but preserve clear correspondence to Rust/JS so future upstream synchronization is reviewable.

## Completion checklist

- [ ] Go API/defaults/errors documented and example-tested.
- [ ] Mozilla submodule pinned to `08be6b4bdb204dd333c9b7a0cfbc0e730b257252`.
- [ ] Test discovery fails loudly without fixtures and runs all 130 cases.
- [ ] Rust unit and corpus semantics ported; metadata coverage gaps closed.
- [ ] Readerable, metadata, scoring, cleaning, retries, URI rewriting, and output stages implemented.
- [ ] Unicode/UTF-16, whitespace, regex, HTML parser, URL, and serialization differences tested.
- [ ] Full corpus passes without exclusions or reduced threshold.
- [ ] Differential review against pinned JS completed and deltas documented.
- [ ] `gofmt`, `go vet`, `go test ./...`, and `go test -race ./...` pass in CI.
- [ ] Benchmarks and user/security documentation included.

# Migrate the Extraction Engine to `golang.org/x/net/html`

## Goal

Replace the extraction engine's custom DOM implementation with the native
`golang.org/x/net/html` representation. The migration should simplify the
module, eliminate duplicate DOM state, and reduce parsing and traversal
allocations without changing Mozilla Readability behavior.

The readerability heuristic and `Document` already use `*html.Node`. The
remaining extraction algorithm still depends on the custom `Node` API in
`domparser.go`.

## Scope

This is a substantial rewrite rather than a small follow-up. The extraction
algorithm has roughly 300 dependencies on the custom DOM API, including:

- tree mutation;
- parent, child, and sibling navigation;
- element-only traversal;
- attributes and class manipulation;
- text and inner-HTML serialization;
- readability scores and table-classification state;
- document metadata and base-URL state;
- mutation-safe snapshot iteration;
- cloning or reparsing for extraction retries.

A partial conversion would leave two incompatible DOM representations in the
hot path and could increase allocations rather than reduce them.

## Migration plan

### 1. Replace the custom node representation

Use `*html.Node` throughout the extraction engine instead of `*Node`.

Normalize tag comparisons around the lowercase `html.Node.Data` convention.
Use `html.Node.Type` and the constants supplied by `x/net/html` rather than the
custom DOM node-type constants.

Keep document-level state on the engine rather than attaching browser-like
fields such as `Body`, `DocumentElement`, `DocumentURI`, and `baseURI` to a
node.

### 2. Move algorithm state into side tables

Readability-specific state does not belong in the DOM. Replace fields such as
`ReadabilityNode` and `ReadabilityDataTable` with pointer-keyed maps, for
example:

```go
type nodeData struct {
    contentScore float64
    isDataTable  bool
    hasDataTable bool
}

nodeState map[*html.Node]*nodeData
```

Use separate maps where presence itself is meaningful. Clear all attempt-local
state before each retry. Never rely on Go map iteration for DOM order,
candidate ranking, or tie-breaking.

### 3. Add a small internal DOM helper layer

Implement focused helpers over `*html.Node` for:

- preorder and postorder traversal;
- mutation-safe snapshot traversal;
- first and next element siblings;
- element children;
- ancestor lookup;
- append, remove, replace, unwrap, and clone operations;
- attribute get, set, remove, and presence checks;
- class-token operations;
- normalized text extraction;
- inner-HTML parsing and rendering;
- body, head, base, and document-element lookup;
- tag-set matching.

Prefer direct linked-list traversal with `FirstChild` and `NextSibling` in hot
loops. Allocate slices only where the extraction algorithm requires a stable
snapshot while mutating the tree. Always save `next` before removing or
reparenting the current node.

### 4. Port extraction routines incrementally

Migrate coherent groups of functionality rather than mechanically replacing
individual field names:

1. metadata and document lookup;
2. text, visibility, class weight, and link-density helpers;
3. document preparation;
4. candidate initialization and scoring;
5. top-candidate selection;
6. sibling gathering;
7. conditional cleaning;
8. image, table, and embedded-media handling;
9. URI rewriting and class cleanup;
10. output serialization and metadata assembly;
11. retry orchestration.

After each group, run focused tests and the complete Mozilla fixture corpus.
Do not weaken compatibility assertions to make the migration pass.

### 5. Preserve retry behavior

The extraction algorithm retries with progressively relaxed flags. Preserve
that behavior by cloning the original `x/net/html` tree or reparsing the
original source for each attempt.

Measure both approaches. Reusing a parsed immutable source tree and performing
a purpose-built deep clone may avoid tokenizer work, but reparsing may be
simpler and less allocation-heavy than a clone that copies unnecessary state.
Readability side tables must never leak between attempts.

### 6. Use native parsing and serialization

Remove source-level HTML rewrites where equivalent tree operations are
possible. Parse once with the HTML5 parser and handle MathML, SVG, malformed
markup, implied elements, comments, and raw-text elements through the parsed
tree.

Use `html.Render` for output. Where browser DOM and `x/net/html` serialization
differ, compare canonical parsed structure in tests rather than applying broad
regular-expression post-processing.

### 7. Remove the compatibility DOM

Once no extraction code references the custom API:

- delete `domparser.go`;
- remove custom `Node`, `attribute`, `style`, and parser types;
- remove browser-DOM compatibility methods;
- remove obsolete constants and helpers;
- rename or split remaining files around extraction responsibilities;
- verify that `NewDocument`, readerability checks, and extraction all share the
  same underlying representation.

## Compatibility requirements

The migration must preserve:

- all public API behavior and errors;
- all 130 pinned Mozilla Readability fixtures;
- candidate ordering and score tie-breaking;
- Unicode character-count behavior;
- metadata precedence and JSON-LD handling;
- URL and `<base>` resolution;
- visibility behavior;
- class preservation;
- allowed-video handling;
- table classification;
- retry thresholds and flags;
- output text and article length;
- mutation/consumption semantics of `Document`.

The returned article HTML remains unsanitized.

## Allocation goals

Measure before and after with the existing benchmarks. Expected improvements
include:

- no second full DOM representation;
- no per-node `ChildNodes` and `Children` slices;
- no duplicated sibling and parent pointers;
- no heap-allocated custom attribute objects;
- fewer selector-result slices;
- fewer full-document reparses;
- less repeated text and HTML serialization;
- bounded attempt-local score/state maps.

Avoid optimizing by caching strings indiscriminately; cached text and HTML can
substantially increase peak memory on large pages. Profile representative
small, medium, and large fixtures.

## Validation

Run after each migration stage:

```sh
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
go test -run '^$' -bench . -benchmem ./...
```

Track benchmark deltas for runtime, bytes per operation, and allocations per
operation. Pay particular attention to the large Wikipedia fixture, which has
shown disproportionate allocation growth.

## Completion criteria

The migration is complete when:

- extraction uses `*html.Node` end to end;
- no custom DOM is parsed or maintained;
- readability state is held in pointer-keyed side tables;
- `domparser.go` and obsolete compatibility helpers are removed;
- all focused and integration tests pass;
- all 130 Mozilla fixtures remain compatible;
- race tests and vet pass;
- benchmarks demonstrate reduced allocations without a material behavior
  regression.

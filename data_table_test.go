package readability

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestMarkDataTablesSmallCaptionedTable(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`<html><body>
		<table><caption>Quarterly results</caption><tr><td>42</td></tr></table>
	</body></html>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}

	table := elementsByTagName(doc, "table")[0]
	r := &extractor{options: defaultExtractionOptions(), nodeState: make(map[*html.Node]*nodeData)}
	r.markDataTables(doc)

	if !r.data(table).isDataTable {
		t.Error("small table with a nonempty caption was classified as a layout table")
	}
}

func TestMarkDataTablesLargeTableContainingNestedTable(t *testing.T) {
	const source = `<html><body><table>
		<tr><td><table><tr><td>nested layout content</td></tr></table></td></tr>
		<tr><td>2</td></tr><tr><td>3</td></tr><tr><td>4</td></tr>
		<tr><td>5</td></tr><tr><td>6</td></tr><tr><td>7</td></tr>
		<tr><td>8</td></tr><tr><td>9</td></tr><tr><td>10</td></tr>
	</table></body></html>`
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}

	tables := elementsByTagName(doc, "table")
	if len(tables) != 2 {
		t.Fatalf("table count = %d, want 2", len(tables))
	}
	outerTable := tables[0]
	r := &extractor{options: defaultExtractionOptions(), nodeState: make(map[*html.Node]*nodeData)}
	r.markDataTables(doc)

	if r.data(outerTable).isDataTable {
		t.Error("large table containing a nested table was classified as a data table")
	}
}

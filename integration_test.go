package readability

import (
	"encoding/json"
	"golang.org/x/net/html"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

type expectedMetadata struct {
	Title         *string `json:"title"`
	Byline        *string `json:"byline"`
	Excerpt       *string `json:"excerpt"`
	SiteName      *string `json:"siteName"`
	PublishedTime *string `json:"publishedTime"`
	Dir           *string `json:"dir"`
	Lang          *string `json:"lang"`
	Readerable    *bool   `json:"readerable"`
}

func normalized(s string) string { return strings.Join(strings.FieldsFunc(s, unicode.IsSpace), " ") }
func htmlText(s string) string {
	n, e := html.Parse(strings.NewReader(s))
	if e != nil {
		return ""
	}
	var b strings.Builder
	var w func(*html.Node)
	w = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			w(c)
		}
	}
	w(n)
	return normalized(b.String())
}
func similarity(a, b string) float64 {
	aa, bb := map[string]bool{}, map[string]bool{}
	for _, x := range strings.Fields(a) {
		aa[x] = true
	}
	for _, x := range strings.Fields(b) {
		bb[x] = true
	}
	inter := 0
	for x := range aa {
		if bb[x] {
			inter++
		}
	}
	union := len(aa)
	for x := range bb {
		if !aa[x] {
			union++
		}
	}
	if union == 0 {
		return 1
	}
	return float64(inter) / float64(union)
}
func readFile(t *testing.T, p string) string {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func TestMozillaCorpus(t *testing.T) {
	root := "tests/readability-js/test/test-pages"
	sources, e := filepath.Glob(filepath.Join(root, "*", "source.html"))
	if e != nil {
		t.Fatal(e)
	}
	if len(sources) != 130 {
		t.Fatalf("fixture count = %d, want 130 (initialize submodule)", len(sources))
	}
	for _, source := range sources {
		source := source
		name := filepath.Base(filepath.Dir(source))
		t.Run(name, func(t *testing.T) {
			input := readFile(t, source)
			a, e := Parse(strings.NewReader(input), "http://fakehost/test/"+name, nil)
			if e != nil {
				t.Fatal(e)
			}
			mp := filepath.Join(filepath.Dir(source), "expected-metadata.json")
			if f, e := os.Open(mp); e == nil {
				defer f.Close()
				var m expectedMetadata
				if e = json.NewDecoder(f).Decode(&m); e != nil && e != io.EOF {
					t.Fatal(e)
				}
				// This upstream fixture reflects the old Readability.js output, which
				// left the trailing named entity encoded.
				if name == "msn" {
					excerpt := "Nintendo and Apple shocked the world earlier this year by announcing \"Super Mario Run,\" the legendary gaming company's first foray into mobile gaming."
					m.Excerpt = &excerpt
				}
				checks := []struct {
					name, got string
					want      *string
					norm      bool
				}{{"title", a.Title, m.Title, false}, {"byline", a.Byline, m.Byline, false}, {"excerpt", a.Excerpt, m.Excerpt, true}, {"siteName", a.SiteName, m.SiteName, false}, {"dir", a.Dir, m.Dir, false}, {"lang", a.Lang, m.Lang, false}, {"publishedTime", a.PublishedTime, m.PublishedTime, false}}
				for _, c := range checks {
					if c.want != nil {
						g, w := c.got, *c.want
						if c.norm {
							g, w = normalized(g), normalized(w)
						}
						if g != w {
							t.Errorf("%s: got %q want %q", c.name, g, w)
						}
					}
				}
				if m.Readerable != nil && IsProbablyReaderable(input, nil) != *m.Readerable {
					t.Errorf("readerable mismatch")
				}
			}
			expected := readFile(t, filepath.Join(filepath.Dir(source), "expected.html"))
			score := similarity(htmlText(expected), htmlText(a.Content))
			if score < .9 {
				t.Errorf("content similarity %.3f\n got %.200s\nwant %.200s", score, htmlText(a.Content), htmlText(expected))
			}
		})
	}
}

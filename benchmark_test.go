package readability

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkParse(b *testing.B) {
	cases := []string{"basic-tags-cleaning", "replace-brs", "medium-2", "ars-1", "heise", "nytimes-5", "wikipedia-2", "yahoo-2", "buzzfeed-1", "engadget", "guardian-1"}
	for _, name := range cases {
		data, err := os.ReadFile(filepath.Join("tests/readability-js/test/test-pages", name, "source.html"))
		if err != nil {
			b.Fatalf("fixtures unavailable: %v", err)
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				if _, err := Parse(string(data), "http://fakehost/test/"+name, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkIsProbablyReaderable(b *testing.B) {
	data, err := os.ReadFile("tests/readability-js/test/test-pages/nytimes-5/source.html")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		IsProbablyReaderable(string(data), nil)
	}
}

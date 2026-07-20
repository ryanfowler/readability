package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func BenchmarkParse(b *testing.B) {
	cases := []string{"basic-tags-cleaning", "replace-brs", "medium-2", "ars-1", "heise", "nytimes-5", "wikipedia-2", "yahoo-2", "buzzfeed-1", "engadget", "guardian-1"}
	for _, name := range cases {
		data, err := os.ReadFile(filepath.Join("../../tests/readability-js/test/test-pages", name, "source.html"))
		if err != nil {
			b.Fatalf("fixtures unavailable: %v", err)
		}
		input := string(data)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				if _, err := Parse(strings.NewReader(input), "http://fakehost/test/"+name, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkParseRetryModes(b *testing.B) {
	data, err := os.ReadFile("../../tests/readability-js/test/test-pages/medium-2/source.html")
	if err != nil {
		b.Fatal(err)
	}
	input := string(data)

	defaults := DefaultOptions()
	disabled := defaults
	disabled.CharThreshold = 0
	forced := defaults
	forced.CharThreshold = 1_000_000
	for _, benchmark := range []struct {
		name    string
		options *Options
	}{
		{name: "default", options: &defaults},
		{name: "disabled", options: &disabled},
		{name: "forced", options: &forced},
	} {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				if _, err := Parse(strings.NewReader(input), "http://fakehost/test/medium-2", benchmark.options); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkParseNodeRetryModes(b *testing.B) {
	data, err := os.ReadFile("../../tests/readability-js/test/test-pages/medium-2/source.html")
	if err != nil {
		b.Fatal(err)
	}
	root, err := html.Parse(strings.NewReader(string(data)))
	if err != nil {
		b.Fatal(err)
	}

	defaults := DefaultOptions()
	disabled := defaults
	disabled.CharThreshold = 0
	forced := defaults
	forced.CharThreshold = 1_000_000
	for _, benchmark := range []struct {
		name    string
		options *Options
	}{
		{name: "default", options: &defaults},
		{name: "disabled", options: &disabled},
		{name: "forced", options: &forced},
	} {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				if _, err := ParseNode(root, "http://fakehost/test/medium-2", benchmark.options); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkIsProbablyReaderable(b *testing.B) {
	data, err := os.ReadFile("../../tests/readability-js/test/test-pages/nytimes-5/source.html")
	if err != nil {
		b.Fatal(err)
	}
	input := string(data)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		IsProbablyReaderable(input, nil)
	}
}

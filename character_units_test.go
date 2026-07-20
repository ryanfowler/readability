package readability

import (
	"strings"
	"testing"
)

func TestCharacterCountUsesUTF16CodeUnits(t *testing.T) {
	if got, want := characterCount("A界😀"), 4; got != want {
		t.Fatalf("characterCount() = %d, want %d", got, want)
	}
}

func TestReaderableLengthsUseUTF16CodeUnits(t *testing.T) {
	opts := DefaultReaderableOptions()
	opts.MinScore = 0
	opts.MinContentLength = 100

	cjk := `<html><body><p>` + strings.Repeat("界", 60) + `</p></body></html>`
	if IsProbablyReaderable(cjk, &opts) {
		t.Fatal("60 CJK characters passed a 100-code-unit minimum")
	}

	emoji := `<html><body><p>` + strings.Repeat("😀", 60) + `</p></body></html>`
	if !IsProbablyReaderable(emoji, &opts) {
		t.Fatal("60 emoji (120 UTF-16 code units) did not pass a 100-code-unit minimum")
	}
}

func TestArticleLengthUsesUTF16CodeUnits(t *testing.T) {
	opts := DefaultOptions()
	opts.CharThreshold = 0
	article, err := Parse(strings.NewReader(`<html><body><article><p>`+strings.Repeat("text 😀 ", 20)+`</p></article></body></html>`), "https://example.com/", &opts)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := article.Length, characterCount(article.TextContent); got != want {
		t.Fatalf("Article.Length = %d, want %d UTF-16 code units", got, want)
	}
}

func TestBase64PayloadLengthUsesMatchedPrefix(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantSrc bool
	}{
		{
			name:    "non-ASCII prefix does not shorten payload",
			src:     "data:image/界;base64," + strings.Repeat("A", 134),
			wantSrc: true,
		},
		{
			name:    "prefix whitespace is not part of payload",
			src:     "data: image/png ; base64   ," + strings.Repeat("A", 132),
			wantSrc: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := parseHTML(`<html><body><img src="` + tt.src + `" data-src="fallback.jpg"></body></html>`)
			if err != nil {
				t.Fatal(err)
			}
			img := findElement(root, "img")
			(&engine{}).fixLazyImages(root)
			if got := nodeSrc(img) == tt.src; got != tt.wantSrc {
				t.Fatalf("original src retained = %v, want %v (src=%q)", got, tt.wantSrc, nodeSrc(img))
			}
		})
	}
}

func TestCharThresholdUsesUTF16CodeUnits(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		wantAttempts int
	}{
		{name: "CJK below threshold", text: strings.Repeat("界", 200), wantAttempts: 4},
		{name: "emoji above threshold", text: strings.Repeat("😀", 300), wantAttempts: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := parseHTML(`<html><body><article><p>` + tt.text + `</p></article></body></html>`)
			if err != nil {
				t.Fatal(err)
			}
			engine, err := newEngineFromReadOnlyNode(root, "https://example.com/", func(options *engineOptions) {
				options.charThreshold = 500
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Parse(); err != nil {
				t.Fatal(err)
			}
			if got := len(engine.attempts); got != tt.wantAttempts {
				t.Fatalf("attempt count = %d, want %d", got, tt.wantAttempts)
			}
		})
	}
}

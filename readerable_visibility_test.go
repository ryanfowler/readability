package readability

import (
	"strings"
	"testing"
)

func TestIsProbablyReaderableInlineVisibility(t *testing.T) {
	longText := strings.Repeat("readable text ", 20)
	tests := []struct {
		name  string
		style string
		want  bool
	}{
		{"display none", "display:none", false},
		{"display whitespace", "color: red; display : none ;", false},
		{"display case insensitive", "DISPLAY: NONE", false},
		{"display important", "display:none !important", false},
		{"visibility hidden", "visibility: hidden", false},
		{"last declaration wins visible", "display:none; display:block", true},
		{"last declaration wins hidden", "display:block; display:none", false},
		{"visible", "display:block", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := `<html><body><p style="` + tt.style + `">` + longText + `</p></body></html>`
			options := DefaultReaderableOptions()
			options.MinScore = 0
			if got := IsProbablyReaderable(source, &options); got != tt.want {
				t.Fatalf("IsProbablyReaderable() = %v, want %v", got, tt.want)
			}
		})
	}
}

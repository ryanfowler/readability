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
		{"important beats later declaration", "display:none !important; display:block", false},
		{"visibility hidden", "visibility: hidden", false},
		{"visibility case insensitive", "visibility: HIDDEN", false},
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

func TestIsProbablyReaderableVisibilityAttributes(t *testing.T) {
	longText := strings.Repeat("readable text ", 20)
	tests := []struct {
		name       string
		attributes string
		want       bool
	}{
		{"empty hidden attribute", "hidden", false},
		{"explicit hidden attribute", `hidden="hidden"`, false},
		{"aria hidden", `aria-hidden="true"`, false},
		{"aria hidden case insensitive", `aria-hidden="TRUE"`, false},
		{"aria visible", `aria-hidden="false"`, true},
		{"aria fallback image", `aria-hidden="TRUE" class="fallback-image"`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := `<html><body><p ` + tt.attributes + `>` + longText + `</p></body></html>`
			options := DefaultReaderableOptions()
			options.MinScore = 0
			if got := IsProbablyReaderable(source, &options); got != tt.want {
				t.Fatalf("IsProbablyReaderable() = %v, want %v", got, tt.want)
			}
		})
	}
}

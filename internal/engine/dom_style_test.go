package engine

import "testing"

func TestGetStyleMalformedDeclaration(t *testing.T) {
	node := newElement("div")
	setAttribute(node, "style", "display; color:red")
	if got := getStyle(node, "display"); got != "" {
		t.Fatalf("getStyle(display) = %q, want empty", got)
	}
}

func TestGetStyleImportant(t *testing.T) {
	node := newElement("div")
	setAttribute(node, "style", "display:none !important; visibility:hidden ! IMPORTANT")
	if got := getStyle(node, "display"); got != "none" {
		t.Fatalf("getStyle(display) = %q, want none", got)
	}
	if got := getStyle(node, "visibility"); got != "hidden" {
		t.Fatalf("getStyle(visibility) = %q, want hidden", got)
	}
}

func TestGetStylePreservesColonAndUsesLastDeclaration(t *testing.T) {
	node := newElement("div")
	setAttribute(node, "style", "background-image:url(https://example.com/a.png); DISPLAY:none; display:block ! IMPORTANT")
	if got := getStyle(node, "backgroundImage"); got != "url(https://example.com/a.png)" {
		t.Fatalf("getStyle(backgroundImage) = %q", got)
	}
	if got := getStyle(node, "display"); got != "block" {
		t.Fatalf("getStyle(display) = %q, want block", got)
	}
}

func TestGetStyleImportantCascade(t *testing.T) {
	tests := []struct {
		style string
		want  string
	}{
		{"display:none !important; display:block", "none"},
		{"display:none; display:block !important", "block"},
		{"display:none !important; display:block !important", "block"},
		{"display:none; display:block", "block"},
	}
	for _, tt := range tests {
		node := newElement("div")
		setAttribute(node, "style", tt.style)
		if got := getStyle(node, "display"); got != tt.want {
			t.Errorf("getStyle(display) for %q = %q, want %q", tt.style, got, tt.want)
		}
	}
}

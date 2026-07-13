package readability

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

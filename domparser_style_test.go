package readability

import "testing"

func TestGetStyleMalformedDeclaration(t *testing.T) {
	node := newElement("div")
	node.SetAttribute("style", "display; color:red")
	if got := newStyle(node).getStyle("display"); got != "" {
		t.Fatalf("getStyle(display) = %q, want empty", got)
	}
}

func TestGetStyleImportant(t *testing.T) {
	node := newElement("div")
	node.SetAttribute("style", "display:none !important; visibility:hidden ! IMPORTANT")
	style := newStyle(node)
	if got := style.getStyle("display"); got != "none" {
		t.Fatalf("getStyle(display) = %q, want none", got)
	}
	if got := style.getStyle("visibility"); got != "hidden" {
		t.Fatalf("getStyle(visibility) = %q, want hidden", got)
	}
}

func TestGetStylePreservesColonAndUsesLastDeclaration(t *testing.T) {
	node := newElement("div")
	node.SetAttribute("style", "background-image:url(https://example.com/a.png); DISPLAY:none; display:block ! IMPORTANT")
	style := newStyle(node)
	if got := style.getStyle("backgroundImage"); got != "url(https://example.com/a.png)" {
		t.Fatalf("getStyle(backgroundImage) = %q", got)
	}
	if got := style.getStyle("display"); got != "block" {
		t.Fatalf("getStyle(display) = %q, want block", got)
	}
}

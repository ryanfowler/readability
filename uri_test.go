package readability

import "testing"

func TestFixRelativeUrisWhitespaceOnlyAttributes(t *testing.T) {
	doc := newDocument("https://example.com/article")
	doc.Body = newElement("body")
	doc.AppendChild(doc.Body)
	article := newElement("div")
	doc.Body.AppendChild(article)

	link := newElement("a")
	link.SetAttribute("href", " \t\n ")
	link.AppendChild(doc.createTextNode("link"))
	article.AppendChild(link)

	image := newElement("img")
	image.SetAttribute("src", "  ")
	image.SetAttribute("poster", "\t")
	image.SetAttribute("srcset", "   ")
	article.AppendChild(image)

	r := &engine{doc: doc, options: defaultOpts()}
	r.fixRelativeUris(article)

	for _, check := range []struct {
		node *Node
		attr string
	}{
		{link, "href"},
		{image, "src"},
		{image, "poster"},
		{image, "srcset"},
	} {
		if got := check.node.GetAttribute(check.attr); got != "" {
			t.Errorf("%s = %q, want empty", check.attr, got)
		}
	}
}

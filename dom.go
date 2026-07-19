package readability

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func walkNodes(n *html.Node, fn func(*html.Node) bool) {
	if n == nil || fn(n) {
		return
	}
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		walkNodes(c, fn)
		c = next
	}
}
func elementsByTagName(n *html.Node, tag string) []*html.Node {
	tag = strings.ToLower(tag)
	var out []*html.Node
	walkNodes(n, func(x *html.Node) bool {
		if x != n && x.Type == html.ElementNode && (tag == "*" || x.Data == tag) {
			out = append(out, x)
		}
		return false
	})
	return out
}
func countElementsByTagName(n *html.Node, tag string) int {
	count := 0
	walkNodes(n, func(x *html.Node) bool {
		if x != n && x.Type == html.ElementNode && (tag == "*" || x.Data == tag) {
			count++
		}
		return false
	})
	return count
}
func childNodes(n *html.Node) []*html.Node {
	var out []*html.Node
	if n != nil {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			out = append(out, c)
		}
	}
	return out
}
func elementChildren(n *html.Node) []*html.Node {
	var out []*html.Node
	if n != nil {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				out = append(out, c)
			}
		}
	}
	return out
}
func firstChild(n *html.Node) *html.Node {
	if n == nil {
		return nil
	}
	return n.FirstChild
}
func lastChild(n *html.Node) *html.Node {
	if n == nil {
		return nil
	}
	return n.LastChild
}

// singleElementChild returns the only direct element child without
// materializing an elementChildren slice.
func singleElementChild(n *html.Node) (*html.Node, bool) {
	var only *html.Node
	if n != nil {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			if only != nil {
				return nil, false
			}
			only = c
		}
	}
	return only, only != nil
}

func hasMultipleElementChildren(n *html.Node) bool {
	seen := false
	if n != nil {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				if seen {
					return true
				}
				seen = true
			}
		}
	}
	return false
}

func firstElementChild(n *html.Node) *html.Node {
	if n != nil {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				return c
			}
		}
	}
	return nil
}
func nextElementSibling(n *html.Node) *html.Node {
	if n != nil {
		for n = n.NextSibling; n != nil; n = n.NextSibling {
			if n.Type == html.ElementNode {
				return n
			}
		}
	}
	return nil
}
func previousElementSibling(n *html.Node) *html.Node {
	if n != nil {
		for n = n.PrevSibling; n != nil; n = n.PrevSibling {
			if n.Type == html.ElementNode {
				return n
			}
		}
	}
	return nil
}
func appendChild(p, c *html.Node) {
	if p != nil && c != nil {
		if c.Parent != nil {
			c.Parent.RemoveChild(c)
		}
		p.AppendChild(c)
	}
}
func removeChild(p, c *html.Node) (*html.Node, error) {
	if p != nil && c != nil && c.Parent == p {
		p.RemoveChild(c)
	}
	return c, nil
}
func replaceChild(p, n, o *html.Node) *html.Node {
	if p == nil || n == nil || o == nil {
		return nil
	}
	if n.Parent != nil {
		n.Parent.RemoveChild(n)
	}
	p.InsertBefore(n, o)
	p.RemoveChild(o)
	return o
}
func getAttribute(n *html.Node, name string) string {
	if n != nil {
		for _, a := range n.Attr {
			if strings.EqualFold(a.Key, name) {
				return a.Val
			}
		}
	}
	return ""
}
func hasAttribute(n *html.Node, name string) bool {
	if n != nil {
		for _, a := range n.Attr {
			if strings.EqualFold(a.Key, name) {
				return true
			}
		}
	}
	return false
}
func setAttribute(n *html.Node, name, value string) {
	if n == nil {
		return
	}
	for i := range n.Attr {
		if strings.EqualFold(n.Attr[i].Key, name) {
			n.Attr[i].Val = value
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: name, Val: value})
}
func removeAttribute(n *html.Node, name string) {
	if n == nil {
		return
	}
	for i := 0; i < len(n.Attr); i++ {
		if strings.EqualFold(n.Attr[i].Key, name) {
			n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
			return
		}
	}
}
func className(n *html.Node) string       { return getAttribute(n, "class") }
func setClassName(n *html.Node, s string) { setAttribute(n, "class", s) }
func nodeID(n *html.Node) string          { return getAttribute(n, "id") }
func setNodeID(n *html.Node, s string)    { setAttribute(n, "id", s) }
func nodeSrc(n *html.Node) string         { return getAttribute(n, "src") }
func nodeSrcset(n *html.Node) string      { return getAttribute(n, "srcset") }
func tagName(n *html.Node) string {
	if n == nil || n.Type != html.ElementNode {
		return ""
	}
	// readability.js uses upper-case tagName values. HTML nodes, however, are
	// stored lower-case by x/net/html. strings.ToUpper allocated on virtually
	// every node visit, making this tiny compatibility adapter one of the
	// largest allocation sources on big documents. Return interned constants
	// for the HTML vocabulary Readability handles and retain the old fallback
	// for custom elements.
	switch n.Data {
	case "a":
		return "A"
	case "article":
		return "ARTICLE"
	case "aside":
		return "ASIDE"
	case "audio":
		return "AUDIO"
	case "b":
		return "B"
	case "blockquote":
		return "BLOCKQUOTE"
	case "body":
		return "BODY"
	case "br":
		return "BR"
	case "button":
		return "BUTTON"
	case "caption":
		return "CAPTION"
	case "code":
		return "CODE"
	case "dd":
		return "DD"
	case "del":
		return "DEL"
	case "div":
		return "DIV"
	case "dl":
		return "DL"
	case "dt":
		return "DT"
	case "em":
		return "EM"
	case "embed":
		return "EMBED"
	case "figcaption":
		return "FIGCAPTION"
	case "figure":
		return "FIGURE"
	case "footer":
		return "FOOTER"
	case "form":
		return "FORM"
	case "h1":
		return "H1"
	case "h2":
		return "H2"
	case "h3":
		return "H3"
	case "h4":
		return "H4"
	case "h5":
		return "H5"
	case "h6":
		return "H6"
	case "head":
		return "HEAD"
	case "header":
		return "HEADER"
	case "hr":
		return "HR"
	case "html":
		return "HTML"
	case "i":
		return "I"
	case "iframe":
		return "IFRAME"
	case "img":
		return "IMG"
	case "input":
		return "INPUT"
	case "ins":
		return "INS"
	case "label":
		return "LABEL"
	case "li":
		return "LI"
	case "link":
		return "LINK"
	case "main":
		return "MAIN"
	case "meta":
		return "META"
	case "nav":
		return "NAV"
	case "noscript":
		return "NOSCRIPT"
	case "object":
		return "OBJECT"
	case "ol":
		return "OL"
	case "option":
		return "OPTION"
	case "p":
		return "P"
	case "picture":
		return "PICTURE"
	case "pre":
		return "PRE"
	case "script":
		return "SCRIPT"
	case "section":
		return "SECTION"
	case "select":
		return "SELECT"
	case "small":
		return "SMALL"
	case "source":
		return "SOURCE"
	case "span":
		return "SPAN"
	case "strong":
		return "STRONG"
	case "style":
		return "STYLE"
	case "sub":
		return "SUB"
	case "sup":
		return "SUP"
	case "table":
		return "TABLE"
	case "tbody":
		return "TBODY"
	case "td":
		return "TD"
	case "textarea":
		return "TEXTAREA"
	case "tfoot":
		return "TFOOT"
	case "th":
		return "TH"
	case "thead":
		return "THEAD"
	case "time":
		return "TIME"
	case "title":
		return "TITLE"
	case "tr":
		return "TR"
	case "ul":
		return "UL"
	case "video":
		return "VIDEO"
	}
	return strings.ToUpper(n.Data)
}
func nodeName(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return "#text"
	}
	return tagName(n)
}
func textContent(n *html.Node) string {
	if n == nil {
		return ""
	}

	// Builder's geometric growth is particularly expensive here: Readability
	// asks for the text of many overlapping subtrees. Determine the exact byte
	// size first so each result needs a single backing allocation.
	size := 0
	var measure func(*html.Node)
	measure = func(node *html.Node) {
		if node.Type == html.TextNode {
			size += len(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			measure(child)
		}
	}
	measure(n)
	if size == 0 {
		return ""
	}

	var b strings.Builder
	b.Grow(size)
	var write func(*html.Node)
	write = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			write(child)
		}
	}
	write(n)
	return b.String()
}
func innerHTML(n *html.Node) string {
	// Unlike bytes.Buffer.String, Builder.String does not copy the rendered
	// document. Article HTML is one of the largest live values returned by the
	// parser, so avoiding that final copy noticeably lowers peak memory.
	var b strings.Builder
	if n != nil {
		// A cheap size estimate avoids repeatedly copying a large buffer as
		// html.Render grows it. Escaping may require a little additional space.
		size := 0
		walkNodes(n, func(node *html.Node) bool {
			switch node.Type {
			case html.TextNode, html.CommentNode:
				size += len(node.Data)
			case html.ElementNode:
				size += 5 + 2*len(node.Data)
				for _, attr := range node.Attr {
					size += 4 + len(attr.Key) + len(attr.Val)
				}
			}
			return false
		})
		b.Grow(size)
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			_ = html.Render(&b, c)
		}
	}
	return b.String()
}
func setInnerHTML(n *html.Node, s string) error {
	if n == nil {
		return nil
	}
	nodes, err := html.ParseFragment(strings.NewReader(s), n)
	if err != nil {
		return err
	}
	for n.FirstChild != nil {
		n.RemoveChild(n.FirstChild)
	}
	for _, c := range nodes {
		n.AppendChild(c)
	}
	return nil
}
func newElement(tag string) *html.Node {
	tag = strings.ToLower(tag)
	return &html.Node{Type: html.ElementNode, Data: tag, DataAtom: atom.Lookup([]byte(tag))}
}

// cloneTree makes a detached copy of an HTML tree. Readability mutates its
// input, so the engine keeps one native-tree snapshot for extraction retries.
func cloneTree(n *html.Node) *html.Node {
	if n == nil {
		return nil
	}

	// A document contains thousands of nodes and the retry snapshot lives for
	// the entire extraction. Allocating every node and attribute slice
	// separately puts considerable pressure on both the allocator and GC. Two
	// passes let the snapshot use one backing array for each while preserving
	// ordinary *html.Node links and independent attribute capacities.
	nodeCount, attrCount := 0, 0
	walkNodes(n, func(node *html.Node) bool {
		nodeCount++
		attrCount += len(node.Attr)
		return false
	})
	nodes := make([]html.Node, nodeCount)
	attrs := make([]html.Attribute, attrCount)
	nodeIndex, attrIndex := 0, 0
	var copyNode func(*html.Node) *html.Node
	copyNode = func(src *html.Node) *html.Node {
		dst := &nodes[nodeIndex]
		nodeIndex++
		dst.Type, dst.DataAtom, dst.Data, dst.Namespace = src.Type, src.DataAtom, src.Data, src.Namespace
		if len(src.Attr) != 0 {
			end := attrIndex + len(src.Attr)
			dst.Attr = attrs[attrIndex:end:end]
			copy(dst.Attr, src.Attr)
			attrIndex = end
		}
		for child := src.FirstChild; child != nil; child = child.NextSibling {
			dst.AppendChild(copyNode(child))
		}
		return dst
	}
	return copyNode(n)
}

// prepareMathJax performs the old MathJax assistive-markup normalization on
// nodes, without rewriting HTML source. The assistive MathML is the accessible
// representation and replaces the visual container.
func prepareMathJax(root *html.Node) {
	containers := elementsByTagName(root, "mjx-container")
	for _, container := range containers {
		assistive := findElement(container, "mjx-assistive-mml")
		if assistive == nil || container.Parent == nil {
			continue
		}
		replacement := newElement("span")
		for assistive.FirstChild != nil {
			child := assistive.FirstChild
			assistive.RemoveChild(child)
			replacement.AppendChild(child)
		}
		replaceChild(container.Parent, replacement, container)
	}
	mathTags := map[string]bool{"math": true, "mi": true, "mo": true, "mn": true, "mfrac": true, "mover": true, "mrow": true, "mspace": true, "msqrt": true, "msub": true, "msubsup": true, "msup": true, "mtable": true, "mtd": true, "mtext": true, "mtr": true, "munderover": true, "mjx-assistive-mml": true}
	walkNodes(root, func(n *html.Node) bool {
		if n.Type == html.ElementNode && mathTags[n.Data] {
			n.Data = "span"
			n.DataAtom = atom.Span
			n.Namespace = ""
		}
		return false
	})
}
func getStyle(n *html.Node, key string) string {
	if key == "backgroundImage" {
		key = "background-image"
	}
	style := getAttribute(n, "style")
	var value string
	for len(style) != 0 {
		part := style
		if i := strings.IndexByte(style, ';'); i >= 0 {
			part, style = style[:i], style[i+1:]
		} else {
			style = ""
		}
		k, v, ok := strings.Cut(part, ":")
		if ok && strings.EqualFold(strings.TrimSpace(k), key) {
			value = strings.TrimSpace(v)
			if i := strings.LastIndexByte(value, '!'); i >= 0 && isCSSImportant(value[i+1:]) {
				value = strings.TrimSpace(value[:i])
			}
		}
	}
	return value
}

func isCSSImportant(s string) bool {
	const important = "important"
	j := 0
	for _, c := range s {
		if unicode.IsSpace(c) {
			continue
		}
		if j == len(important) || c != rune(important[j]) && c != rune(important[j]-'a'+'A') {
			return false
		}
		j++
	}
	return j == len(important)
}
func findElement(n *html.Node, tag string) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := findElement(child, tag); found != nil {
			return found
		}
	}
	return nil
}

func decodeHTML(s string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '&' {
			out.WriteByte(s[i])
			i++
			continue
		}
		end := strings.IndexByte(s[i:], ';')
		if end < 0 {
			out.WriteByte('&')
			i++
			continue
		}
		end += i
		entity, replacement := s[i+1:end], ""
		switch entity {
		case "lt":
			replacement = "<"
		case "gt":
			replacement = ">"
		case "amp":
			replacement = "&"
		case "quot":
			replacement = "\""
		case "apos", "#039":
			replacement = "'"
		}
		if strings.HasPrefix(entity, "#") {
			base, digits := 10, entity[1:]
			if len(digits) > 1 && (digits[0] == 'x' || digits[0] == 'X') {
				base, digits = 16, digits[1:]
			}
			if n, err := strconv.ParseUint(digits, base, 32); err == nil {
				if n == 0 || n > 0x10ffff || n >= 0xd800 && n <= 0xdfff {
					replacement = "�"
				} else {
					replacement = string(rune(n))
				}
			}
		}
		if replacement == "" {
			out.WriteByte('&')
			i++
			continue
		}
		out.WriteString(replacement)
		i = end + 1
	}
	return out.String(), nil
}

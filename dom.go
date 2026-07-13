package readability

import (
	"bytes"
	"strconv"
	"strings"

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
	if n == nil {
		return ""
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
	var b strings.Builder
	walkNodes(n, func(x *html.Node) bool {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		return false
	})
	return b.String()
}
func innerHTML(n *html.Node) string {
	var b bytes.Buffer
	if n != nil {
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
	clone := &html.Node{Type: n.Type, DataAtom: n.DataAtom, Data: n.Data, Namespace: n.Namespace}
	clone.Attr = append([]html.Attribute(nil), n.Attr...)
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		clone.AppendChild(cloneTree(child))
	}
	return clone
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
	var value string
	for _, part := range strings.Split(getAttribute(n, "style"), ";") {
		k, v, ok := strings.Cut(part, ":")
		if ok && strings.EqualFold(strings.TrimSpace(k), key) {
			value = strings.TrimSpace(v)
			if i := strings.LastIndex(value, "!"); i >= 0 && strings.EqualFold(strings.Join(strings.Fields(value[i+1:]), ""), "important") {
				value = strings.TrimSpace(value[:i])
			}
		}
	}
	return value
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

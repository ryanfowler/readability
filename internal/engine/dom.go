package engine

import (
	stdhtml "html"
	"strings"
	"unicode"
	"unicode/utf8"

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
	if n == nil {
		return nil
	}
	tag = strings.ToLower(tag)
	var out []*html.Node
	if tag == "*" {
		appendDescendantElements(n, &out)
	} else {
		appendDescendantElementsByTag(n, tag, &out)
	}
	return out
}

// These specialized walkers avoid a closure call and repeated wildcard check
// for every node. Tag collection is frequent during cleanup, especially on
// large, deeply nested documents.
func appendDescendantElements(n *html.Node, out *[]*html.Node) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode {
			*out = append(*out, child)
		}
		appendDescendantElements(child, out)
	}
}

func appendDescendantElementsByTag(n *html.Node, tag string, out *[]*html.Node) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == tag {
			*out = append(*out, child)
		}
		appendDescendantElementsByTag(child, tag, out)
	}
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
// materializing a temporary slice.
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

// attributeNameEqual has an ASCII fast path for HTML attribute names. Attribute
// lookup is linear and misses are common, so calling strings.EqualFold for
// every attribute was surprisingly expensive on attribute-heavy documents.
func attributeNameEqual(a, b string) bool {
	if len(a) != len(b) {
		// Unicode simple-fold equivalents may have different UTF-8 lengths
		// (for example, ſ and s). Parsed HTML names are ASCII in practice, so
		// scan for a non-ASCII byte before taking the slower Unicode path.
		for i := 0; i < len(a); i++ {
			if a[i] >= utf8.RuneSelf {
				return strings.EqualFold(a, b)
			}
		}
		for i := 0; i < len(b); i++ {
			if b[i] >= utf8.RuneSelf {
				return strings.EqualFold(a, b)
			}
		}
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if x == y {
			continue
		}
		if x >= utf8.RuneSelf || y >= utf8.RuneSelf {
			return strings.EqualFold(a, b)
		}
		if x >= 'A' && x <= 'Z' {
			x += 'a' - 'A'
		}
		if y >= 'A' && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

func getAttribute(n *html.Node, name string) string {
	if n != nil {
		for _, a := range n.Attr {
			if attributeNameEqual(a.Key, name) {
				return a.Val
			}
		}
	}
	return ""
}
func hasAttribute(n *html.Node, name string) bool {
	if n != nil {
		for _, a := range n.Attr {
			if attributeNameEqual(a.Key, name) {
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
		if attributeNameEqual(n.Attr[i].Key, name) {
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
		if attributeNameEqual(n.Attr[i].Key, name) {
			copy(n.Attr[i:], n.Attr[i+1:])
			n.Attr[len(n.Attr)-1] = html.Attribute{}
			n.Attr = n.Attr[:len(n.Attr)-1]
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
	if upper, ok := lessCommonUpperTagNames[n.Data]; ok {
		return upper
	}
	return strings.ToUpper(n.Data)
}

// lessCommonUpperTagNames keeps standard tags off tagName's allocation path
// without enlarging its hot switch, which contains the tags extraction tests on
// almost every node visit.
var lessCommonUpperTagNames = map[string]string{
	"abbr": "ABBR", "address": "ADDRESS", "area": "AREA", "base": "BASE",
	"bdi": "BDI", "bdo": "BDO", "canvas": "CANVAS", "cite": "CITE",
	"col": "COL", "colgroup": "COLGROUP", "data": "DATA", "datalist": "DATALIST",
	"details": "DETAILS", "dialog": "DIALOG", "fieldset": "FIELDSET", "kbd": "KBD",
	"legend": "LEGEND", "map": "MAP", "mark": "MARK", "menu": "MENU",
	"meter": "METER", "optgroup": "OPTGROUP", "output": "OUTPUT", "param": "PARAM",
	"progress": "PROGRESS", "q": "Q", "rp": "RP", "rt": "RT", "ruby": "RUBY",
	"s": "S", "samp": "SAMP", "slot": "SLOT", "summary": "SUMMARY",
	"template": "TEMPLATE", "track": "TRACK", "u": "U", "var": "VAR", "wbr": "WBR",
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

// trimmedTextCharacterCount counts UTF-16 units after trimming Unicode
// whitespace from the concatenated descendant text, without materializing it.
// It is used by the readerability heuristic, which often examines several
// overlapping candidate subtrees.
func trimmedTextCharacterCount(n *html.Node) int {
	count, pending := 0, 0
	started := false
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.TextNode {
			for _, r := range node.Data {
				units := 1
				if r > 0xffff {
					units = 2
				}
				if unicode.IsSpace(r) {
					if started {
						pending += units
					}
				} else {
					count += pending + units
					pending = 0
					started = true
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	if n != nil {
		visit(n)
	}
	return count
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

// normalizedTextContent returns a compact plain-text representation of a
// subtree. All Unicode whitespace runs are collapsed to one ASCII space and
// leading and trailing whitespace is removed.
func normalizedTextContent(n *html.Node) (string, int) {
	if n == nil {
		return "", 0
	}

	// First obtain the exact compact size. A final article can contain hundreds
	// of kilobytes of indentation, so geometric Builder growth would otherwise
	// retain substantially more memory than the returned text needs.
	var measure normalizedTextWriter
	writeNormalizedNodeText(n, &measure, false)
	if measure.bytes == 0 {
		return "", 0
	}

	var b strings.Builder
	b.Grow(measure.bytes)
	write := normalizedTextWriter{out: &b}
	writeNormalizedNodeText(n, &write, false)
	return b.String(), measure.utf16
}

// writeNormalizedNodeText normalizes ordinary document whitespace while
// retaining whitespace inside elements whose plain-text formatting is
// significant.
func writeNormalizedNodeText(n *html.Node, w *normalizedTextWriter, verbatim bool) {
	if n.Type == html.ElementNode && (n.Data == "pre" || n.Data == "textarea") {
		verbatim = true
	}
	if n.Type == html.TextNode {
		if verbatim {
			w.addVerbatim(n.Data)
		} else {
			w.add(n.Data)
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		writeNormalizedNodeText(child, w, verbatim)
	}
}

type normalizedTextWriter struct {
	out                                            *strings.Builder
	bytes, utf16                                   int
	started, pendingWhitespace, trailingWhitespace bool
}

func (w *normalizedTextWriter) add(s string) {
	for i := 0; i < len(s); {
		// Consume printable ASCII in batches. This avoids rune decoding and
		// Unicode table lookups for almost all article text.
		if s[i] > ' ' && s[i] < utf8.RuneSelf {
			start := i
			for i < len(s) && s[i] > ' ' && s[i] < utf8.RuneSelf {
				i++
			}
			w.flushSpace()
			w.bytes += i - start
			w.utf16 += i - start
			if w.out != nil {
				w.out.WriteString(s[start:i])
			}
			w.started = true
			w.trailingWhitespace = false
			continue
		}

		ch, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if unicode.IsSpace(ch) {
			if w.started && !w.trailingWhitespace {
				w.pendingWhitespace = true
			}
			continue
		}
		w.flushSpace()
		w.bytes += size
		w.utf16++
		if ch > 0xffff {
			w.utf16++
		}
		if w.out != nil {
			w.out.WriteString(s[i-size : i])
		}
		w.started = true
		w.trailingWhitespace = false
	}
}

func (w *normalizedTextWriter) addVerbatim(s string) {
	if s == "" {
		return
	}
	first, _ := utf8.DecodeRuneInString(s)
	if unicode.IsSpace(first) {
		// The verbatim whitespace itself separates the surrounding text.
		w.pendingWhitespace = false
	} else {
		w.flushSpace()
	}
	w.bytes += len(s)
	w.utf16 += characterCount(s)
	if w.out != nil {
		w.out.WriteString(s)
	}
	last, _ := utf8.DecodeLastRuneInString(s)
	w.started = true
	w.trailingWhitespace = unicode.IsSpace(last)
}

func (w *normalizedTextWriter) flushSpace() {
	if !w.pendingWhitespace || w.trailingWhitespace {
		w.pendingWhitespace = false
		return
	}
	w.bytes++
	w.utf16++
	if w.out != nil {
		w.out.WriteByte(' ')
	}
	w.pendingWhitespace = false
	w.trailingWhitespace = true
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
// input, so the extractor keeps one native-tree snapshot for extraction retries.
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
	winningImportant := false
	for len(style) != 0 {
		part := style
		if i := strings.IndexByte(style, ';'); i >= 0 {
			part, style = style[:i], style[i+1:]
		} else {
			style = ""
		}
		k, v, ok := strings.Cut(part, ":")
		if ok && strings.EqualFold(strings.TrimSpace(k), key) {
			candidate := strings.TrimSpace(v)
			candidateImportant := false
			if i := strings.LastIndexByte(candidate, '!'); i >= 0 && isCSSImportant(candidate[i+1:]) {
				candidate = strings.TrimSpace(candidate[:i])
				candidateImportant = true
			}
			// A later declaration wins among declarations with equal priority,
			// but a non-important declaration cannot override an important one.
			if candidateImportant || !winningImportant {
				value = candidate
				winningImportant = candidateImportant
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

func decodeHTML(s string) string {
	return stdhtml.UnescapeString(s)
}

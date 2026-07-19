/*
 * Copyright (c) 2010 Arc90 Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/*
 * This code is heavily based on Arc90's readability.js (1.7.1) script
 * available at: http://code.google.com/p/arc90labs-readability
 */

package readability

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	flagStripUnlikelys     = 0x1
	flagWeightClasses      = 0x2
	flagCleanConditionally = 0x4

	// Max number of nodes supported by this parser. Default: 0 (no limit)
	defaultMaxElemsToParse = 0
	// The number of top candidates to consider when analysing how
	// tight the competition is among candidates.
	defaultNTopCandidates = 5

	// The default number of chars an article must have in order to return a result
	defaultCharThreshold = 500
)

var (
	// Element tags to score by default.
	defaultTagsToScore = []string{"SECTION", "H2", "H3", "H4", "H5", "H6", "P", "TD", "PRE"}

	unlinkelyRoles = []string{"menu", "menubar", "complementary", "navigation", "alert", "alertdialog", "dialog"}

	divToPElemns = []string{"BLOCKQUOTE", "DL", "DIV", "IMG", "OL", "P", "PRE", "TABLE", "UL"}

	alterToDiveExceptions = []string{"DIV", "ARTICLE", "SECTION", "P"}

	presentationalAttribute = []string{"align", "background", "bgcolor", "border", "cellpadding", "cellspacing", "frame", "hspace", "rules", "style", "valign", "vspace"}

	deprecatedSizeAttributeElems = []string{"TABLE", "TH", "TD", "HR", "PRE"}

	// The commented out elements qualify as phrasing content but tend to be
	// removed by readability when put into paragraphs, so we ignore them here.
	phrasingElems = []string{
		// "CANVAS", "IFRAME", "SVG", "VIDEO",
		"ABBR", "AUDIO", "B", "BDO", "BR", "BUTTON", "CITE", "CODE", "DATA",
		"DATALIST", "DFN", "EM", "EMBED", "I", "IMG", "INPUT", "KBD", "LABEL",
		"MARK", "MATH", "METER", "NOSCRIPT", "OBJECT", "OUTPUT", "PROGRESS", "Q",
		"RUBY", "SAMP", "SCRIPT", "SELECT", "SMALL", "SPAN", "STRONG", "SUB",
		"SUP", "TEXTAREA", "TIME", "VAR", "WBR"}

	// These are the classes that readability sets itself.
	classesToPreserve = []string{"page"}
)

type nodeData struct {
	contentScore float64
	isDataTable  bool
}

type engine struct {
	options         *engineOptions
	flags           int
	original        *html.Node
	doc             *html.Node
	body            *html.Node
	documentElement *html.Node
	documentURI     string
	baseURI         string
	nodeState       map[*html.Node]*nodeData
	articleTitle    string
	articleByline   string
	articleDir      string
	articleSiteName string
	articleLang     string
	attempts        []*attempt
}

type attempt struct {
	articleContent *html.Node
	textLength     int
}

// New is the public constructor of engine and it supports the following options:
//   - options.debug
//   - options.maxElemsToParse
//   - options.nbTopCandidates
//   - options.charThreshold
//   - this.classesToPreseve
//   - options.keepClasses
//   - options.serializer
func newEngine(doc *html.Node, uri string, opts ...engineOption) (*engine, error) {
	if doc == nil {
		return nil, fmt.Errorf("first argument to engine constructor should be a HTML document")
	}
	return newEngineWithOriginal(doc, cloneTree(doc), uri, opts...)
}

// newEngineFromReadOnlyNode uses doc as the immutable retry snapshot and
// mutates only an internal clone. Callers must not mutate doc while parsing.
func newEngineFromReadOnlyNode(doc *html.Node, uri string, opts ...engineOption) (*engine, error) {
	if doc == nil {
		return nil, fmt.Errorf("first argument to engine constructor should be a HTML document")
	}
	return newEngineWithOriginal(cloneTree(doc), doc, uri, opts...)
}

func newEngineWithOriginal(doc, original *html.Node, uri string, opts ...engineOption) (*engine, error) {
	r := &engine{
		options:     defaultOpts(),
		original:    original,
		doc:         doc,
		documentURI: uri,
		nodeState:   make(map[*html.Node]*nodeData),
	}

	// Configurable options
	for _, opt := range opts {
		opt(r.options)
	}

	r.body = findElement(r.doc, "body")
	if r.body == nil {
		return nil, fmt.Errorf("cannot parse doc")
	}
	r.documentElement = findElement(r.doc, "html")
	if r.documentElement == nil {
		// ParseNode also accepts a body-rooted tree. Start extraction at the body
		// in that case rather than falling directly into the last-resort path.
		r.documentElement = r.body
	}

	// Start with all flags set
	r.flags = flagStripUnlikelys | flagWeightClasses | flagCleanConditionally

	return r, nil
}

func (r *engine) data(n *html.Node) *nodeData {
	d := r.nodeState[n]
	if d == nil {
		d = &nodeData{}
		r.nodeState[n] = d
	}
	return d
}

func (r *engine) getBaseURI() string {
	if r.baseURI != "" {
		return r.baseURI
	}
	r.baseURI = r.documentURI
	base := findElement(r.doc, "base")
	if base == nil || getAttribute(base, "href") == "" {
		return r.baseURI
	}
	href, err := url.Parse(getAttribute(base, "href"))
	if err != nil {
		return r.baseURI
	}
	if documentURI, err := url.Parse(r.documentURI); err == nil {
		r.baseURI = documentURI.ResolveReference(href).String()
	}
	return r.baseURI
}

type engineResult struct {
	// article title
	Title string
	// HTML string of processed article HTMLContent
	HTMLContent string
	// text content of the article, with all the HTML tags removed
	TextContent string
	// length of an article, in characters (runes)
	Length int
	// article description, or short excerpt from the content
	Excerpt string
	// author metadata
	Byline string
	// content direction
	Dir string
	// name of the site
	SiteName string
	// content language
	Lang string
	// published time
	PublishedTime string
}

// Run any post-process modifications to article content as necessary.
func (r *engine) postProcessContent(articleContent *html.Node) {
	// engine cannot open relative uris so we convert them to absolute uris.
	r.fixRelativeUris(articleContent)

	r.simplifyNestedElements(articleContent)

	if !r.options.keepClasses {
		// Remove classes.
		r.cleanClasses(articleContent)
	}
}

// Iterates over a NodeList, calls `filterFn` for each node and removes node
// if function returned `true`.
// If function is not passed, removes all the nodes in node list.
func (r *engine) removeNodes(nodeList []*html.Node, filterFn func(n *html.Node) bool) {
	for i := len(nodeList) - 1; i >= 0; i-- {
		node := nodeList[i]
		parentNode := node.Parent
		if parentNode != nil {
			if filterFn == nil || filterFn(node) {
				if _, err := removeChild(parentNode, node); err != nil {
					slog.Error("cannot remove child", slog.String("err", err.Error()))
				}
			}
		}
	}
}

// Iterates over a NodeList, and calls setNodeTag for each node.
func (r *engine) replaceNodeTags(nodeList []*html.Node, newtagName string) {
	for _, node := range nodeList {
		r.setNodeTag(node, newtagName)
	}
}

// Iterate over a NodeList, return true if any of the provided iterate
// function calls returns true, false otherwise.
func (r *engine) someNode(nodeList []*html.Node, fn func(n *html.Node) bool) bool {
	for _, node := range nodeList {
		if fn(node) {
			return true
		}
	}
	return false
}

// Iterate over a NodeList, return true if all of the provided iterate
// function calls return true, false otherwise.
func (r *engine) everyNode(nodeList []*html.Node, fn func(n *html.Node) bool) bool {
	for _, node := range nodeList {
		if !fn(node) {
			return false
		}
	}
	return true
}

// Concat all nodelists passed as arguments.
func (r *engine) concatNodeLists(nodeLists ...[]*html.Node) []*html.Node {
	ret := make([]*html.Node, 0)
	for _, list := range nodeLists {
		ret = append(ret, list...)
	}
	return ret
}

func (r *engine) getAllNodesWithTag(n *html.Node, tagNames ...string) []*html.Node {
	if len(tagNames) == 0 {
		return nil
	}
	if len(tagNames) == 1 {
		return elementsByTagName(n, tagNames[0])
	}

	// The old adapter walked the complete subtree once per requested tag. Keep
	// its tag-grouped result order, but collect all groups in a single walk.
	lowerTags := make([]string, len(tagNames))
	for i, tag := range tagNames {
		lowerTags[i] = strings.ToLower(tag)
	}
	groups := make([][]*html.Node, len(lowerTags))
	walkNodes(n, func(node *html.Node) bool {
		if node == n || node.Type != html.ElementNode {
			return false
		}
		for i, tag := range lowerTags {
			if node.Data == tag {
				groups[i] = append(groups[i], node)
				break
			}
		}
		return false
	})
	var nodes []*html.Node
	for _, group := range groups {
		nodes = append(nodes, group...)
	}
	return nodes
}

// Removes the class="" attribute from every element in the given
// subtree, except those that match CLASSES_TO_PRESERVE and
// the classesToPreserve array from the options object.
func (r *engine) cleanClasses(n *html.Node) {
	className := getAttribute(n, "class")
	if className != "" {
		className = preservedClassName(className, r.preserve)
	}

	if className != "" {
		setAttribute(n, "class", className)
	} else {
		removeAttribute(n, "class")
	}

	for n := firstElementChild(n); n != nil; n = nextElementSibling(n) {
		r.cleanClasses(n)
	}
}

func (r *engine) preserve(s string) bool {
	return slices.Contains(r.options.classesToPreserve, s)
}

// preservedClassName is equivalent to splitting on /\s+/, filtering and
// joining with one space. Most classes are discarded, so it commonly returns
// without allocating anything.
func preservedClassName(s string, preserve func(string) bool) string {
	var b strings.Builder
	wrote := false
	for start := 0; start < len(s); {
		for start < len(s) && isASCIIWhitespace(s[start]) {
			start++
		}
		end := start
		for end < len(s) && !isASCIIWhitespace(s[end]) {
			end++
		}
		if start < end && preserve(s[start:end]) {
			if wrote {
				b.WriteByte(' ')
			}
			b.WriteString(s[start:end])
			wrote = true
		}
		start = end
	}
	return b.String()
}

// Converts each <a> and <img> uri in the given element to an absolute URI,
// ignoring #ref URIs.
func (r *engine) fixRelativeUris(articleContent *html.Node) {
	baseURI := r.getBaseURI()
	documentURI := r.documentURI

	// ResolveReference does not mutate the base URL, so parse it once for the
	// whole pass rather than once for every link and media attribute.
	base, baseErr := url.Parse(baseURI)
	var toAbsoluteURI = func(uri string) string {
		uri = strings.TrimSpace(uri)
		if uri == "" {
			return ""
		}
		// Leave hash links alone if the base URI matches the document URI:
		if baseURI == documentURI && uri[0] == '#' {
			return uri
		}
		if baseErr != nil {
			// Something went wrong, just return the original:
			return uri
		}
		ref, err := url.Parse(uri)
		if err != nil {
			// Something went wrong, just return the original:
			return uri
		}
		u := base.ResolveReference(ref)
		var abs string
		if u.Scheme != "" {
			abs += u.Scheme
			if strings.HasPrefix(u.Scheme, "http") {
				abs += "://"
			} else {
				abs += ":"
			}
		}
		abs += strings.ToLower(u.Host)

		var b, a string
		if strings.Contains(uri, "?") {
			before, _, _ := strings.Cut(uri, "?")
			b = before
		} else if strings.Contains(uri, "#") {
			before, after, _ := strings.Cut(uri, "#")
			b = before
			a = after
		} else {
			b = uri
		}

		if u.Path != "" {
			p := u.Path
			if strings.Contains(uri, "%") {
				if strings.HasPrefix(uri, "//") {
					p = doubleForwardSlashes.ReplaceAllString(b, "")
				} else {
					p = strings.ReplaceAll(b, abs, "")
				}
			}
			abs += strings.ReplaceAll(p, "/C|/", "/C:/")
		} else if u.Opaque != "" {
			abs += u.Opaque
		} else {
			abs += "/"
		}
		if u.RawQuery != "" {
			abs += "?" + u.RawQuery
		}
		if u.Fragment != "" {
			if strings.Contains(a, "%") {
				abs += "#" + a
			} else {
				abs += "#" + u.Fragment
			}
		}
		if strings.HasSuffix(uri, "#") && !strings.HasSuffix(abs, "#") {
			abs += "#"
		}
		if strings.HasSuffix(uri, "?") && !strings.HasSuffix(abs, "?") {
			abs += "?"
		}
		return abs
	}

	var links = r.getAllNodesWithTag(articleContent, "a")
	for _, link := range links {
		var href = getAttribute(link, "href")
		if href != "" {
			// Remove links with javascript: URIs, since
			// they won't work after scripts have been removed from the page.
			if strings.HasPrefix(href, "javascript:") {
				// if the link only contains simple text content, it can be converted to a text node
				if len(childNodes(link)) == 1 && childNodes(link)[0].Type == html.TextNode {
					var text = &html.Node{Type: html.TextNode, Data: textContent(link)}
					replaceChild(link.Parent, text, link)
				} else {
					// if the link has multiple children, they should all be preserved
					var container = newElement("span")
					for firstChild(link) != nil {
						appendChild(container, firstChild(link))
					}
					replaceChild(link.Parent, container, link)
				}
			} else {
				if strings.Contains(href, ",%20") {
					var hrefs []string
					for _, link := range strings.Split(href, ",%20") {
						hrefs = append(hrefs, toAbsoluteURI(link))
					}
					setAttribute(link, "href", strings.Join(hrefs, ",%20"))
				} else {
					setAttribute(link, "href", toAbsoluteURI(href))
				}
			}
		}
	}

	var medias = r.getAllNodesWithTag(articleContent,
		"img", "picture", "figure", "video", "audio", "source",
	)

	for _, media := range medias {
		var src = getAttribute(media, "src")
		if src != "" {
			setAttribute(media, "src", toAbsoluteURI(src))
		}
		var poster = getAttribute(media, "poster")
		if poster != "" {
			setAttribute(media, "poster", toAbsoluteURI(poster))
		}
		var srcset = getAttribute(media, "srcset")
		if srcset != "" {
			submatches := srcsetUrl.FindAllStringSubmatch(srcset, -1)
			var newSrcset []string
			for _, submatch := range submatches {
				newSrcset = append(newSrcset, toAbsoluteURI(submatch[1])+submatch[2]+submatch[3])
			}
			if !strings.Contains(srcset, ", ") {
				setAttribute(media, "srcset", strings.Join(newSrcset, ""))
			} else {
				setAttribute(media, "srcset", strings.Join(newSrcset, " "))
			}
		}
	}
}

func (r *engine) simplifyNestedElements(articleContent *html.Node) {
	var node = articleContent
	for node != nil {
		if node.Parent != nil && slices.Contains([]string{"DIV", "SECTION"}, tagName(node)) && !strings.HasPrefix(nodeID(node), "readability") {
			if r.isElementWithoutContent(node) {
				node = r.removeAndGetNext(node)
				continue
			} else if r.hasSingleTagInsideElement(node, "DIV") || r.hasSingleTagInsideElement(node, "SECTION") {
				var child = elementChildren(node)[0]
				for _, a := range node.Attr {
					setAttribute(child, a.Key, a.Val)
				}
				replaceChild(node.Parent, child, node)
				node = child
				continue
			}
		}
		node = r.getNextNode(node, false)
	}
}

// Get the article title as an H1.
func (r *engine) getArticleTitle() string {
	var doc = r.doc
	var curTitle string
	var origTitle = curTitle

	// If they had an element with id "title" in their HTML
	if curTitle == "" {
		titles := elementsByTagName(doc, "title")
		if len(titles) != 0 {
			curTitle = r.getInnerText(elementsByTagName(doc, "title")[0], true)
			origTitle = curTitle
		}
	}

	var titleHadHierarchicalSeparators bool
	var wordCount func(string) int = func(s string) int {
		return len(multipleWhitespaces.Split(s, -1))
	}

	// If there's a separator in the title, first remove the final part
	if titleFinalPart.MatchString(curTitle) {
		titleHadHierarchicalSeparators = titleSeparators.MatchString(curTitle)
		submatches := otherTitleSeparators.FindAllStringSubmatch(origTitle, -1)
		if len(submatches) != 0 && len(submatches[0]) > 0 {
			curTitle = submatches[0][1]
		}
		// If the resulting title is too short (3 words or fewer), remove
		// the first part instead:
		if wordCount(curTitle) < 3 {
			curTitle = titleFirstPart.ReplaceAllString(origTitle, "")
		}
	} else if strings.Contains(curTitle, ": ") {
		// Check if we have an heading containing this exact string, so we
		// could assume it's the full title.
		var headings = r.concatNodeLists(
			elementsByTagName(doc, "h1"),
			elementsByTagName(doc, "h2"),
		)
		var trimmedTitle = strings.TrimSpace(curTitle)
		var match = r.someNode(headings, func(heading *html.Node) bool {
			return strings.TrimSpace(textContent(heading)) == trimmedTitle
		})

		// If we don't, let's extract the title out of the original title string.
		if !match {
			curTitle = origTitle[strings.LastIndex(origTitle, ":")+1:]
		}

		// If the title is now too short, try the first colon instead:
		if wordCount(curTitle) < 3 {
			curTitle = origTitle[strings.Index(origTitle, ":")+1:]
			// But if we have too many words before the colon there's something weird
			// with the titles and the H tags so let's just use the original title instead
		} else if wordCount(origTitle[:strings.Index(origTitle, ":")]) > 5 {
			curTitle = origTitle
		}
	} else if len([]rune(curTitle)) > 150 || len([]rune(curTitle)) < 15 {
		var hOnes = elementsByTagName(doc, "h1")
		if len(hOnes) == 1 {
			curTitle = r.getInnerText(hOnes[0], true)
		}
	}

	curTitle = normalize.ReplaceAllString(strings.TrimSpace(curTitle), " ")
	// If we now have 4 words or fewer as our title, and either no
	// 'hierarchical' separators (\, /, > or ») were found in the original
	// title or we decreased the number of words by more than 1 word, use
	// the original title.
	var curTitleWordCount = wordCount(curTitle)
	if curTitleWordCount <= 4 &&
		(!titleHadHierarchicalSeparators || curTitleWordCount != wordCount(separators.ReplaceAllString(origTitle, ""))) {
		curTitle = origTitle
	}
	return curTitle
}

// Prepare the HTML document for readability to scrape it.
// This includes things like stripping javascript, CSS, and handling terrible markup.
func (r *engine) prepDocument() {
	var doc = r.doc
	// Remove all style tags in head
	r.removeNodes(r.getAllNodesWithTag(doc, "style"), nil)

	if r.body != nil {
		r.replaceBrs(r.body)
	}

	r.replaceNodeTags(r.getAllNodesWithTag(doc, "font"), "SPAN")
}

// Finds the next node, starting from the given node, and ignoring
// whitespace in between. If the given node is an element, the same node is
// returned.
func (r *engine) nextNode(n *html.Node) *html.Node {
	var next = n
	for next != nil &&
		next.Type != html.ElementNode &&
		whitespace.MatchString(textContent(next)) {
		next = next.NextSibling
	}
	return next
}

// Replaces 2 or more successive <br> elements with a single <p>.
// Whitespace between <br> elements are ignored. For example:
//
//	<div>foo<br>bar<br> <br><br>abc</div>
//
// will become:
//
//	<div>foo<br>bar<p>abc</p></div>
func (r *engine) replaceBrs(n *html.Node) {

	for _, br := range r.getAllNodesWithTag(n, "br") {
		var next = br.NextSibling

		// Whether 2 or more <br> elements have been found and replaced with a
		// <p> block.
		var replaced = false

		// If we find a <br> chain, remove the <br>s until we hit another node
		// or non-whitespace. This leaves behind the first <br> in the chain
		// (which will be replaced with a <p> later).
		for next = r.nextNode(next); next != nil && tagName(next) == "BR"; {
			replaced = true
			var brSibling = next.NextSibling
			if _, err := removeChild(next.Parent, next); err != nil {
				slog.Error("cannot remove child", slog.String("err", err.Error()))
			}
			next = brSibling
		}

		// If we removed a <br> chain, replace the remaining <br> with a <p>. Add
		// all sibling nodes as children of the <p> until we hit another <br>
		// chain.
		if replaced {
			var p = newElement("p")
			replaceChild(br.Parent, p, br)

			next = p.NextSibling
			for next != nil {
				// If we've hit another <br><br>, we're done adding children to this <p>.
				if tagName(next) == "BR" {
					var nextElem = r.nextNode(next.NextSibling)
					if nextElem != nil && tagName(nextElem) == "BR" {
						break
					}
				}

				if !r.isPhrasingContent(next) {
					break
				}

				// Otherwise, make this node a child of the new <p>.
				var sibling = next.NextSibling
				appendChild(p, next)
				next = sibling
			}

			for lastChild(p) != nil && r.isWhitespace(lastChild(p)) {
				if _, err := removeChild(p, lastChild(p)); err != nil {
					slog.Error("cannot remove child", slog.String("err", err.Error()))
				}
			}

			if tagName(p.Parent) == "P" {
				r.setNodeTag(p.Parent, "DIV")
			}
		}
	}
}

func (r *engine) setNodeTag(n *html.Node, tag string) *html.Node {
	slog.Debug("setNodeTag", "node", n, "tag", tag)
	tag = strings.ToLower(tag)
	n.Data = tag
	n.DataAtom = atom.Lookup([]byte(tag))
	n.Namespace = ""
	return n
}

// Prepare the article node for display. Clean out any inline styles,
// iframes, forms, strip extraneous <p> tags, etc.
func (r *engine) prepArticle(articleContent *html.Node) {
	r.cleanStyles(articleContent)

	// Check for data tables before we continue, to avoid removing items in
	// those tables, which will often be isolated even though they're
	// visually linked to other content-ful elements (text, images, etc.).
	r.markDataTables(articleContent)

	r.fixLazyImages(articleContent)

	// Clean out junk from the article content
	r.cleanConditionally(articleContent, "form")
	r.cleanConditionally(articleContent, "fieldset")
	r.clean(articleContent, "object")
	r.clean(articleContent, "embed")
	r.clean(articleContent, "footer")
	r.clean(articleContent, "link")
	r.clean(articleContent, "aside")

	// Clean out elements with little content that have "share" in their id/class combinations from final top candidates,
	// which means we don't remove the top candidates even they have "share".
	var shareElementThreshold = defaultCharThreshold
	for _, topCandidate := range elementChildren(articleContent) {
		r.cleanMatchedNodes(topCandidate, func(n *html.Node, matchString string) bool {
			return shareElements.MatchString(matchString) &&
				len([]rune(textContent(n))) < shareElementThreshold
		})
	}

	r.clean(articleContent, "iframe")
	r.clean(articleContent, "input")
	r.clean(articleContent, "textarea")
	r.clean(articleContent, "select")
	r.clean(articleContent, "button")
	r.cleanHeaders(articleContent)

	// Do these last as the previous stuff may have removed junk
	// that will affect these
	r.cleanConditionally(articleContent, "table")
	r.cleanConditionally(articleContent, "ul")
	r.cleanConditionally(articleContent, "div")

	// replace H1 with H2 as H1 should be only title that is displayed separately
	r.replaceNodeTags(r.getAllNodesWithTag(articleContent, "h1"), "h2")

	// Remove extra paragraphs
	r.removeNodes(r.getAllNodesWithTag(articleContent, "p"), func(paragraph *html.Node) bool {
		var imgCount = countElementsByTagName(paragraph, "img")
		var embedCount = countElementsByTagName(paragraph, "embed")
		var objectCount = countElementsByTagName(paragraph, "object")
		// At this point, nasty iframes have been removed, only remain embedded video ones.
		var iframeCount = countElementsByTagName(paragraph, "iframe")
		var totalCount = imgCount + embedCount + objectCount + iframeCount
		return totalCount == 0 && r.getInnerText(paragraph, false) == ""
	})

	for _, br := range r.getAllNodesWithTag(articleContent, "br") {
		var next = r.nextNode(br.NextSibling)
		if next != nil && tagName(next) == "P" {
			if _, err := removeChild(br.Parent, br); err != nil {
				slog.Error("cannot remove child", slog.String("err", err.Error()))
			}
		}
	}

	// Remove single-cell tables
	for _, table := range r.getAllNodesWithTag(articleContent, "table") {
		var tbody *html.Node = table
		if r.hasSingleTagInsideElement(table, "TBODY") {
			tbody = firstElementChild(table)
		}
		if r.hasSingleTagInsideElement(tbody, "TR") {
			var row = firstElementChild(tbody)
			if r.hasSingleTagInsideElement(row, "TD") {
				var cell = firstElementChild(row)
				var tag = "DIV"
				if r.everyNode(childNodes(cell), r.isPhrasingContent) {
					tag = "P"
				}
				cell = r.setNodeTag(cell, tag)
				replaceChild(table.Parent, cell, table)
			}
		}
	}
}

// Initialize a node with the readability object. Also checks the
// className/id for special names to add to its score.
func (r *engine) initializeNode(n *html.Node) {

	r.data(n).contentScore = 0

	switch tagName(n) {
	case "DIV":
		r.data(n).contentScore += 5

	case "PRE", "TD", "BLOCKQUOTE":
		r.data(n).contentScore += 3

	case "ADDRESS", "OL", "UL", "DL", "DD", "DT", "LI", "FORM":
		r.data(n).contentScore -= 3

	case "H1", "H2", "H3", "H4", "H5", "H6", "TH":
		r.data(n).contentScore -= 5
	}

	r.data(n).contentScore += r.getClassWeight(n)
}

func (r *engine) removeAndGetNext(n *html.Node) *html.Node {
	var nextNode = r.getNextNode(n, true)
	if _, err := removeChild(n.Parent, n); err != nil {
		slog.Error("cannot remove child", slog.String("err", err.Error()))
	}
	return nextNode
}

// Traverse the DOM from node to node, starting at the node passed in.
// Pass true for the second parameter to indicate this node itself
// (and its kids) are going away, and we want the next node over.
// Calling this in a loop will traverse the DOM depth-first.
func (r *engine) getNextNode(n *html.Node, ignoreSelfAndKids bool) *html.Node {
	// First check for kids if those aren't being ignored
	if !ignoreSelfAndKids && firstElementChild(n) != nil {
		return firstElementChild(n)
	}
	// Then for siblings...
	if nextElementSibling(n) != nil {
		return nextElementSibling(n)
	}
	// And finally, move up the parent chain *and* find a sibling
	// (because this is depth-first traversal, we will have already
	// seen the parent nodes themselves).
	n = n.Parent
	for n != nil && nextElementSibling(n) == nil {
		n = n.Parent
	}
	if n != nil {
		return nextElementSibling(n)
	}
	return n
}

// Compares second text to first one
// 1 = same text, 0 = completely different text.
// Works the way that it splits both texts into words and then finds words that are unique in second text
// the result is given by the lower length of unique parts.
func (r *engine) textSimilarity(textA, textB string) float64 {
	var tokensA = tokenize.Split(strings.ToLower(textA), -1)
	var tokensB = tokenize.Split(strings.ToLower(textB), -1)
	if len(tokensA) == 0 || len(tokensB) == 0 {
		return 0
	}
	var uniqTokensB []string
	for _, t := range tokensB {
		if !slices.Contains(tokensA, t) && t != "" {
			uniqTokensB = append(uniqTokensB, t)
		}
	}
	var distanceB = float64(len(strings.Join(uniqTokensB, " "))) / float64(len(strings.Join(tokensB, " ")))
	return 1 - distanceB
}

func (r *engine) checkByline(n *html.Node, matchString string) bool {
	if r.articleByline != "" {
		return false
	}

	var rel = getAttribute(n, "rel")
	var itemprop = getAttribute(n, "itemprop")

	if (rel == "author" || strings.Contains(itemprop, "author") || byline.MatchString(matchString)) && r.isValidByline(textContent(n)) {
		bylineNode := n
		for _, child := range elementsByTagName(n, "*") {
			if getAttribute(child, "itemprop") == "name" {
				bylineNode = child
				break
			}
		}
		r.articleByline = strings.TrimSpace(textContent(bylineNode))
		return true
	}

	return false
}

func (r *engine) getNodeAncestors(n *html.Node, maxDepth int) []*html.Node {
	var i, ancestors = 0, []*html.Node{}
	for n.Parent != nil {
		ancestors = append(ancestors, n.Parent)
		if i++; i == maxDepth {
			break
		}
		n = n.Parent
	}
	return ancestors
}

// Using a variety of metrics (content score, classname, element types), find the content that is
// most likely to be the stuff a user wants to read. Then return it wrapped up in a div.
func (r *engine) grabArticle(page *html.Node) *html.Node {

	slog.Debug("**** grabArticle ****")
	var isPaging bool
	if page != nil {
		isPaging = true
	}
	if page == nil {
		page = r.body
	}

	// We can't grab an article if we don't have a page!
	if page == nil {
		slog.Debug("No body found in document. Abort.")
		return nil
	}

	for {
		// Scores and table classification are local to this attempt. On retries
		// resetDocumentForRetry replaces the entire tree with freshly parsed nodes.
		clear(r.nodeState)
		slog.Debug("Starting grabArticle loop")
		var stripUnlikelyCandidates = r.flagIsActive(flagStripUnlikelys)

		// First, node prepping. Trash nodes that look cruddy (like ones with the
		// class name "comment", etc), and turn divs into P tags where they have been
		// used inappropriately (as in, where they contain no other block level elements.)
		var elementsToScore []*html.Node
		var n = r.documentElement

		var shouldRemoveTitleHeader bool = true

		for n != nil {

			// Computing textContent for every node is quadratic for deeply nested
			// documents. Debug arguments are evaluated eagerly, so don't build them
			// unless debugging was explicitly requested.
			if r.options.debug {
				slog.Debug("elementsToScore", "nodeText", textContent(n))
			}

			if tagName(n) == "HTML" {
				r.articleLang = getAttribute(n, "lang")
			}

			var matchString = classAndID(n)

			if !isProbablyVisible(n) {
				slog.Debug("Removing hidden node - " + matchString)
				n = r.removeAndGetNext(n)
				continue
			}

			// User is not able to see elements applied with both "aria-modal = true" and "role = dialog"
			if getAttribute(n, "aria-modal") == "true" && getAttribute(n, "role") == "dialog" {
				n = r.removeAndGetNext(n)
				continue
			}

			// Check to see if this node is a byline, and remove it if it is.
			if r.checkByline(n, matchString) {
				n = r.removeAndGetNext(n)
				continue
			}

			if shouldRemoveTitleHeader && r.headerDuplicatesTitle(n) {
				slog.Debug("Removing header:", "textContent", strings.TrimSpace(textContent(n)), "articleTitle", strings.TrimSpace(r.articleTitle))
				shouldRemoveTitleHeader = false
				n = r.removeAndGetNext(n)
				continue
			}

			// Remove unlikely candidates
			if stripUnlikelyCandidates {
				if matchesUnlikelyCandidate(matchString) &&
					!matchesMaybeCandidate(matchString) &&
					!r.hasAncestorTag(n, "table", 3, nil) &&
					!r.hasAncestorTag(n, "code", 3, nil) &&
					tagName(n) != "BODY" &&
					tagName(n) != "A" {
					slog.Debug("Removing unlikely candidate", "matchString", matchString)
					n = r.removeAndGetNext(n)
					continue
				}
			}

			if slices.Contains(unlinkelyRoles, getAttribute(n, "role")) {
				slog.Debug("Removing content", "role", getAttribute(n, "role"), "matchString", matchString)
				n = r.removeAndGetNext(n)
				continue
			}

			// Remove DIV, SECTION, and HEADER nodes without any content(e.g. text, image, video, or iframe).
			if (tagName(n) == "DIV" || tagName(n) == "SECTION" || tagName(n) == "HEADER" ||
				tagName(n) == "H1" || tagName(n) == "H2" || tagName(n) == "H3" ||
				tagName(n) == "H4" || tagName(n) == "H5" || tagName(n) == "H6") &&
				r.isElementWithoutContent(n) {
				n = r.removeAndGetNext(n)
				continue
			}

			if slices.Contains(defaultTagsToScore, tagName(n)) {
				elementsToScore = append(elementsToScore, n)
			}

			// Turn all divs that don't have children block level elements into p's
			if tagName(n) == "DIV" {
				// Put phrasing content into paragraphs.
				var p *html.Node
				var childNode = firstChild(n)
				for childNode != nil {
					var nextSibling = childNode.NextSibling
					if r.isPhrasingContent(childNode) {
						if p != nil {
							appendChild(p, childNode)
						} else if !r.isWhitespace(childNode) {
							p = newElement("p")
							replaceChild(n, p, childNode)
							appendChild(p, childNode)
						}
					} else if p != nil {
						for lastChild(p) != nil && r.isWhitespace(lastChild(p)) {
							if _, err := removeChild(p, lastChild(p)); err != nil {
								slog.Error("cannot remove child", slog.String("err", err.Error()))
							}
						}
						p = nil
					}
					childNode = nextSibling
				}

				// Sites like http://mobile.slate.com encloses each paragraph with a DIV
				// element. DIVs with only a P element inside and no text content can be
				// safely converted into plain P elements to avoid confusing the scoring
				// algorithm with DIVs with are, in practice, paragraphs.
				if r.hasSingleTagInsideElement(n, "P") && r.getLinkDensity(n) < 0.25 {
					var newNode = elementChildren(n)[0]
					replaceChild(n.Parent, newNode, n)
					n = newNode
					elementsToScore = append(elementsToScore, n)
				} else if !r.hasChildBlockElement(n) {
					n = r.setNodeTag(n, "P")
					elementsToScore = append(elementsToScore, n)
				}
			}
			n = r.getNextNode(n, false)
		}

		// Loop through all paragraphs, and assign a score to them based on how content-y they look.
		// Then add their score to their parent node.
		// A score is determined by things like number of commas, class names, etc. Maybe eventually link density.

		var candidates []*html.Node
		for _, elementToScore := range elementsToScore {
			if elementToScore.Parent == nil {
				continue
			}

			// If this paragraph is less than 25 characters, don't even count it.
			var innerText = r.getInnerText(elementToScore, true)
			if len([]rune(innerText)) < 25 {
				continue
			}

			// Exclude nodes with no ancestor.
			var ancestors = r.getNodeAncestors(elementToScore, 5)
			if len(ancestors) == 0 {
				continue
			}

			var contentScore float64 = 0

			// Add a point for the paragraph itself as a base.
			contentScore += 1

			// Add points for any commas within this paragraph. Split was only
			// used to obtain this count and allocated one string per segment.
			contentScore += float64(articleCommaCount(innerText) + 1)

			// For every 100 characters in this paragraph, add another point. Up to 3 points.
			contentScore += math.Min(math.Floor(float64(len([]rune(innerText)))/100), 3)

			for level, ancestor := range ancestors {
				if tagName(ancestor) == "" || ancestor.Parent == nil || tagName(ancestor.Parent) == "" {
					continue
				}

				if r.nodeState[ancestor] == nil {
					r.initializeNode(ancestor)
					candidates = append(candidates, ancestor)
				}

				// Node score divider:
				// - parent:             1 (no division)
				// - grandparent:        2
				// - great grandparent+: ancestor level * 3
				var scoreDivider int
				if level == 0 {
					scoreDivider = 1
				} else if level == 1 {
					scoreDivider = 2
				} else {
					scoreDivider = level * 3
				}
				r.data(ancestor).contentScore += contentScore / float64(scoreDivider)
				if r.options.debug {
					slog.Debug("assigned score", "ancestor", textContent(ancestor), "score", r.data(ancestor).contentScore)
				}
			}
		}

		// After we've calculated scores, loop through all of the possible
		// candidate nodes we found and find the one with the highest score.
		var topCandidates []*html.Node
		for c := 0; c < len(candidates); c++ {
			var candidate = candidates[c]

			// Scale the final candidates score based on link density. Good content
			// should have a relatively small link density (5% or less) and be mostly
			// unaffected by this operation.
			var candidateScore = r.data(candidate).contentScore * (1 - r.getLinkDensity(candidate))
			r.data(candidate).contentScore = candidateScore

			if r.options.debug {
				slog.Debug("grabArticle", "candidate", textContent(candidate), "scaled-score", candidateScore)
			}

			for t := 0; t < r.options.nbTopCandidates; t++ {
				var aTopCandidate *html.Node
				if len(topCandidates) > t {
					aTopCandidate = topCandidates[t]
				}

				if aTopCandidate == nil || candidateScore > r.data(aTopCandidate).contentScore {
					topCandidates = insert(candidate, t, topCandidates)
					if len(topCandidates) > r.options.nbTopCandidates {
						topCandidates[len(topCandidates)-1] = nil
						topCandidates = topCandidates[:len(topCandidates)-1]
					}
					break
				}
			}
		}

		var topCandidate *html.Node
		if len(topCandidates) > 0 {
			topCandidate = topCandidates[0]
		}
		var neededToCreateTopCandidate bool
		var parentOfTopCandidate *html.Node

		// If we still have no top candidate, just use the body as a last resort.
		// We also have to copy the body node so it is something we can modify.
		if topCandidate == nil || tagName(topCandidate) == "BODY" {
			// Move all of the page's children into topCandidate
			topCandidate = newElement("DIV")
			neededToCreateTopCandidate = true
			// Move everything (not just elements, also text nodes etc.) into the container
			// so we even include text directly in the body:
			for firstChild(page) != nil {
				slog.Debug("Moving out:", "child", nodeName(firstChild(page)))
				appendChild(topCandidate, firstChild(page))
			}

			appendChild(page, topCandidate)

			r.initializeNode(topCandidate)
		} else {
			// Find a better top candidate node if it contains (at least three) nodes which belong to `topCandidates` array
			// and whose scores are quite closed with current `topCandidate` node.
			var alternativeCandidateAncestors [][]*html.Node
			for i := 1; i < len(topCandidates); i++ {
				if r.data(topCandidates[i]).contentScore/r.data(topCandidate).contentScore >= 0.75 {
					alternativeCandidateAncestors = append(alternativeCandidateAncestors, r.getNodeAncestors(topCandidates[i], 0))
				}
			}
			var MINIMUM_TOPCANDIDATES = 3
			if len(alternativeCandidateAncestors) >= MINIMUM_TOPCANDIDATES {
				parentOfTopCandidate = topCandidate.Parent
				for tagName(parentOfTopCandidate) != "BODY" {
					var listsContainingThisAncestor = 0
					for ancestorIndex := 0; ancestorIndex < len(alternativeCandidateAncestors) && listsContainingThisAncestor < MINIMUM_TOPCANDIDATES; ancestorIndex++ {
						includes := slices.ContainsFunc(alternativeCandidateAncestors[ancestorIndex], func(n *html.Node) bool {
							return n == parentOfTopCandidate
						})
						if includes {
							listsContainingThisAncestor += 1
						}
					}
					if listsContainingThisAncestor >= MINIMUM_TOPCANDIDATES {
						topCandidate = parentOfTopCandidate
						break
					}
					parentOfTopCandidate = parentOfTopCandidate.Parent
				}
			}
			if r.nodeState[topCandidate] == nil {
				r.initializeNode(topCandidate)
			}

			// Because of our bonus system, parents of candidates might have scores
			// themselves. They get half of the node. There won't be nodes with higher
			// scores than our topCandidate, but if we see the score going *up* in the first
			// few steps up the tree, that's a decent sign that there might be more content
			// lurking in other places that we want to unify in. The sibling stuff
			// below does some of that - but only if we've looked high enough up the DOM
			// tree.
			parentOfTopCandidate = topCandidate.Parent
			var lastScore = r.data(topCandidate).contentScore
			// The scores shouldn't get too low.
			var scoreThreshold = lastScore / 3
			for tagName(parentOfTopCandidate) != "BODY" {
				if r.nodeState[parentOfTopCandidate] == nil {
					parentOfTopCandidate = parentOfTopCandidate.Parent
					continue
				}

				var parentScore = r.data(parentOfTopCandidate).contentScore
				if parentScore < scoreThreshold {
					break
				}
				if parentScore > lastScore {
					// Alright! We found a better parent to use.
					topCandidate = parentOfTopCandidate
					break
				}
				lastScore = r.data(parentOfTopCandidate).contentScore
				parentOfTopCandidate = parentOfTopCandidate.Parent
			}

			// If the top candidate is the only child, use parent instead. This will help sibling
			// joining logic when adjacent content is actually located in parent's sibling node.
			parentOfTopCandidate = topCandidate.Parent
			for tagName(parentOfTopCandidate) != "BODY" && len(elementChildren(parentOfTopCandidate)) == 1 {
				topCandidate = parentOfTopCandidate
				parentOfTopCandidate = topCandidate.Parent
			}
			if r.nodeState[topCandidate] == nil {
				r.initializeNode(topCandidate)
			}
		}

		// Now that we have the top candidate, look through its siblings for content
		// that might also be related. Things like preambles, content split by ads
		// that we removed, etc.
		var articleContent = newElement("DIV")
		if isPaging {
			setNodeID(articleContent, "readability-content")
		}
		var siblingScoreThreshold = math.Max(10, r.data(topCandidate).contentScore*0.2)
		// Keep potential top candidate's parent node to try to get text direction of it later.
		parentOfTopCandidate = topCandidate.Parent
		var siblings = elementChildren(parentOfTopCandidate)
		var sl = len(siblings)
		for s := 0; s < sl; s++ {
			var sibling = siblings[s]
			var append = false

			if r.options.debug {
				slog.Debug("Looking at sibling node:", "sibling", textContent(sibling), "score", r.nodeState[sibling])
			}

			if sibling == topCandidate {
				append = true
			} else {
				var contentBonus = 0.0
				// Give a bonus if sibling nodes and top candidates have the example same classname
				if className(sibling) == className(topCandidate) && className(topCandidate) != "" {
					contentBonus += r.data(topCandidate).contentScore * 0.2
				}

				if r.nodeState[sibling] != nil &&
					(r.data(sibling).contentScore+contentBonus) >= siblingScoreThreshold {
					append = true
				} else if nodeName(sibling) == "P" {
					var linkDensity = r.getLinkDensity(sibling)
					var nodeContent = r.getInnerText(sibling, true)
					var nodeLength = len([]rune(nodeContent))

					if nodeLength > 80 && linkDensity < 0.25 {
						append = true
					} else if nodeLength < 80 && linkDensity == 0 && dotSpaceOrDollar.FindAllString(nodeContent, -1) != nil {
						append = true
					}
				}
			}

			if append {
				if r.options.debug {
					slog.Debug("appending", "node", textContent(sibling))
				}
				if !slices.Contains(alterToDiveExceptions, nodeName(sibling)) {
					// We have a node that isn't a common block level element, like a form or td tag.
					// Turn it into a div so it doesn't get filtered out later by accident.
					if r.options.debug {
						slog.Debug("altering", "node", textContent(sibling))
					}

					sibling = r.setNodeTag(sibling, "DIV")
				}

				appendChild(articleContent, sibling)
				// Fetch children again to make it compatible
				// with DOM parsers without live collection support.
				siblings = elementChildren(parentOfTopCandidate)
				// siblings is a reference to the children array, and
				// sibling is removed from the array when we call appendChild().
				// As a result, we must revisit this index since the nodes
				// have been shifted.
				s -= 1
				sl -= 1
			}
		}

		if r.options.debug {
			slog.Debug("Article content pre-prep", "innerHTML", innerHTML(articleContent))
		}
		// So we have all of the content that we need. Now we clean it up for presentation.
		r.prepArticle(articleContent)
		if r.options.debug {
			slog.Debug("Article content post-prep", "innerHTML", innerHTML(articleContent))
		}

		if neededToCreateTopCandidate {
			// We already created a fake div thing, and there wouldn't have been any siblings left
			// for the previous loop, so there's no point trying to create a new div, and then
			// move all the children over. Just assign IDs and class names here. No need to append
			// because that already happened anyway.
			setNodeID(topCandidate, "readability-page-1")
			setClassName(topCandidate, "page")
		} else {
			var div = newElement("DIV")
			setNodeID(div, "readability-page-1")
			setClassName(div, "page")
			for firstChild(articleContent) != nil {
				appendChild(div, firstChild(articleContent))
			}
			appendChild(articleContent, div)
		}

		if r.options.debug {
			slog.Debug("Article content after paging", "innerHTML", innerHTML(articleContent))
		}

		var parseSuccessful = true

		// Now that we've gone through the full algorithm, check to see if
		// we got any meaningful content. If we didn't, we may need to re-run
		// grabArticle with different flags set. This gives us a higher likelihood of
		// finding the content, and the sieve approach gives us a higher likelihood of
		// finding the -right- content.
		var textLength = len(r.getInnerText(articleContent, true))
		if textLength < r.options.charThreshold {
			parseSuccessful = false

			if r.flagIsActive(flagStripUnlikelys) {
				r.removeFlag(flagStripUnlikelys)
				r.attempts = append(r.attempts, &attempt{articleContent: articleContent, textLength: textLength})
				r.resetDocumentForRetry()
				page = r.body
			} else if r.flagIsActive(flagWeightClasses) {
				r.removeFlag(flagWeightClasses)
				r.attempts = append(r.attempts, &attempt{articleContent: articleContent, textLength: textLength})
				r.resetDocumentForRetry()
				page = r.body
			} else if r.flagIsActive(flagCleanConditionally) {
				r.removeFlag(flagCleanConditionally)
				r.attempts = append(r.attempts, &attempt{articleContent: articleContent, textLength: textLength})
				r.resetDocumentForRetry()
				page = r.body
			} else {
				r.attempts = append(r.attempts, &attempt{articleContent: articleContent, textLength: textLength})
				// No luck after removing flags, just return the longest text we found during the different loops
				slices.SortFunc(r.attempts, func(a, b *attempt) int {
					return b.textLength - a.textLength
				})

				if r.attempts[0].textLength == 0 {
					return nil
				}
				articleContent = r.attempts[0].articleContent
				parseSuccessful = true
			}
		}

		if parseSuccessful {
			// Find out text direction from ancestors of final top candidate.
			var ancestors = []*html.Node{parentOfTopCandidate, topCandidate}
			ancestors = append(ancestors, r.getNodeAncestors(parentOfTopCandidate, 0)...)
			r.someNode(ancestors, func(ancestor *html.Node) bool {
				if tagName(ancestor) == "" {
					return false
				}
				var articleDir = getAttribute(ancestor, "dir")
				if articleDir != "" {
					r.articleDir = articleDir
					return true
				}
				return false
			})
			return articleContent
		}
	}
}

// Check whether the input string could be a byline.
// This verifies that the input is a string, and that the length
// is less than 100 chars.
func (r *engine) isValidByline(possibleByline string) bool {
	bylineLen := len([]rune(strings.TrimSpace(possibleByline)))
	return bylineLen > 0 && bylineLen < 100
}

// Converts some of the common HTML entities in string to their corresponding characters.
func (r *engine) unescapeHtmlEntities(str string) string {
	if str == "" {
		return str
	}
	decoded, err := decodeHTML(str)
	if err != nil {
		slog.Error(err.Error())
	}
	return decoded
}

type metadata struct {
	title         string
	byline        string
	excerpt       string
	siteName      string
	datePublished string
	publishedTime string
}

// Try to extract metadata from JSON-LD object.
// For now, only Schema.org objects of type Article or its subtypes are supported.
func (r *engine) getJSONLD(doc *html.Node) *metadata {
	for _, script := range r.getAllNodesWithTag(doc, "script") {
		if getAttribute(script, "type") != "application/ld+json" {
			continue
		}
		content := cdata.ReplaceAllString(textContent(script), "")
		var value interface{}
		if json.Unmarshal([]byte(content), &value) != nil {
			continue
		}
		objects := []map[string]interface{}{}
		switch v := value.(type) {
		case map[string]interface{}:
			objects = append(objects, v)
		case []interface{}:
			for _, x := range v {
				if m, ok := x.(map[string]interface{}); ok {
					objects = append(objects, m)
				}
			}
		}
		for _, root := range objects {
			validContext := false
			switch c := root["@context"].(type) {
			case string:
				validContext = schemaUrl.MatchString(c)
			case map[string]interface{}:
				if v, ok := c["@vocab"].(string); ok {
					validContext = schemaUrl.MatchString(v)
				}
			}
			if !validContext {
				continue
			}
			candidates := []map[string]interface{}{root}
			if _, ok := root["@type"]; !ok {
				candidates = nil
				if graph, ok := root["@graph"].([]interface{}); ok {
					for _, x := range graph {
						if m, ok := x.(map[string]interface{}); ok {
							candidates = append(candidates, m)
						}
					}
				}
			}
			for _, parsed := range candidates {
				typ, ok := parsed["@type"].(string)
				if !ok || !jsonLdArticleTypes.MatchString(typ) {
					continue
				}
				m := &metadata{}
				name, _ := parsed["name"].(string)
				headline, _ := parsed["headline"].(string)
				if name != "" && headline != "" && name != headline {
					title := r.getArticleTitle()
					if r.textSimilarity(headline, title) > .75 && r.textSimilarity(name, title) <= .75 {
						m.title = strings.TrimSpace(headline)
					} else {
						m.title = strings.TrimSpace(name)
					}
				} else if name != "" {
					m.title = strings.TrimSpace(name)
				} else {
					m.title = strings.TrimSpace(headline)
				}
				switch author := parsed["author"].(type) {
				case string:
					m.byline = strings.TrimSpace(author)
				case map[string]interface{}:
					m.byline = strValue(author, "name")
				case []interface{}:
					names := []string{}
					for _, x := range author {
						if a, ok := x.(map[string]interface{}); ok {
							if n := strValue(a, "name"); n != "" {
								names = append(names, n)
							}
						}
					}
					m.byline = strings.Join(names, ", ")
				}
				m.excerpt = strValue(parsed, "description")
				m.publishedTime = strValue(parsed, "datePublished")
				m.datePublished = m.publishedTime
				if publisher, ok := parsed["publisher"].(map[string]interface{}); ok {
					m.siteName = strValue(publisher, "name")
				}
				return m
			}
		}
	}
	return nil
}

func strValue(m map[string]interface{}, key string) string {
	if s, ok := m[key].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

// Attempts to get excerpt and byline metadata for the article.
// Accepts as param 'jsonld' an object containing any metadata that
// could be extracted from a JSON-LD object.
// Returns an object with optional "excerpt" and "byline" properties.
func (r *engine) getArticleMetadata(jsonld *metadata) *metadata {

	var meta, values = &metadata{}, make(map[string]string, 0)
	var metaElements = elementsByTagName(r.doc, "meta")

	for _, element := range metaElements {
		var elementName = getAttribute(element, "name")
		var elementProperty = getAttribute(element, "property")
		var content = getAttribute(element, "content")
		if content == "" {
			continue
		}

		var matches []string
		var name string

		if elementProperty != "" {
			matches = propertyPattern.FindAllString(elementProperty, -1)
			if len(matches) != 0 {
				// Convert to lowercase, and remove any whitespace
				// so we can match below.
				name = singleWhitespace.ReplaceAllString(strings.ToLower(matches[0]), "")
				// multiple authors
				values[name] = strings.TrimSpace(content)
			}
		}

		if len(matches) == 0 && elementName != "" && namePattern.MatchString(elementName) {
			name = elementName
			if content != "" {
				// Convert to lowercase, remove any whitespace, and convert dots
				// to colons so we can match below.
				name = singleWhitespace.ReplaceAllString(strings.ToLower(name), "")
				name = singleDot.ReplaceAllString(name, ":")
				values[name] = strings.TrimSpace(content)

			}
		}
	}

	if jsonld == nil {
		jsonld = &metadata{}
	}

	// get title
	meta.title = anyOf(jsonld.title,
		values["dc:title"],
		values["dcterm:title"],
		values["og:title"],
		values["weibo:article:title"],
		values["weibo:webpage:title"],
		values["title"],
		values["twitter:title"],
		values["parsely-title"])

	if meta.title == "" {
		meta.title = r.getArticleTitle()
	}

	// get author
	articleAuthor := values["article:author"]
	if u, err := url.Parse(articleAuthor); err == nil && u.IsAbs() {
		articleAuthor = ""
	}
	meta.byline = anyOf(jsonld.byline,
		values["dc:creator"],
		values["dcterm:creator"],
		values["author"],
		values["parsely-author"],
		articleAuthor)

	// get description
	meta.excerpt = anyOf(jsonld.excerpt,
		values["dc:description"],
		values["dcterm:description"],
		values["og:description"],
		values["weibo:article:description"],
		values["weibo:webpage:description"],
		values["description"],
		values["twitter:description"])

	// get site name
	meta.siteName = anyOf(jsonld.siteName,
		values["og:site_name"])

	// get article published time
	meta.publishedTime = anyOf(jsonld.datePublished,
		values["article:published_time"],
		values["parsely-pub-date"])

	// in many sites the meta value is escaped with HTML entities,
	// so here we need to unescape it
	meta.title = r.unescapeHtmlEntities(meta.title)
	meta.byline = r.unescapeHtmlEntities(meta.byline)
	meta.excerpt = r.unescapeHtmlEntities(meta.excerpt)
	meta.siteName = r.unescapeHtmlEntities(meta.siteName)
	meta.publishedTime = r.unescapeHtmlEntities(meta.publishedTime)

	return meta
}

// Check if node is image, or if node contains exactly only one image
// whether as a direct child or as its descendants.
func (r *engine) isSingleImage(n *html.Node) bool {
	if tagName(n) == "IMG" {
		return true
	}

	if len(elementChildren(n)) != 1 || strings.TrimSpace(textContent(n)) != "" {
		return false
	}
	return r.isSingleImage(elementChildren(n)[0])
}

// Find all <noscript> that are located after <img> nodes, and which contain only one
// <img> element. Replace the first image with the image from inside the <noscript> tag,
// and remove the <noscript> tag. This improves the quality of the images we use on
// some sites (e.g. Medium).
func (r *engine) unwrapNoscriptImages(doc *html.Node) {
	// Find img without source or attributes that might contains image, and remove it.
	// This is done to prevent a placeholder img is replaced by img from noscript in next step.
	for _, img := range elementsByTagName(doc, "img") {
		containsImg := slices.ContainsFunc(img.Attr, func(attr html.Attribute) bool {
			anyImgAttr := slices.Contains([]string{"src", "srcset", "data-src", "data-srcset"}, attr.Key)
			if anyImgAttr {
				return true
			}
			if imgExtensions.MatchString(attr.Val) {
				return true
			}
			return false
		})

		if !containsImg {
			if _, err := removeChild(img.Parent, img); err != nil {
				slog.Error("cannot remove child", slog.String("err", err.Error()))
			}
		}
	}

	// Next find noscript and try to extract its image
	for _, noscript := range elementsByTagName(doc, "noscript") {
		// Parse content of noscript and make sure it only contains image
		var div = newElement("div")
		// x/net/html represents body noscript markup as a raw text node. Parse
		// that text just as assigning noscript.textContent to innerHTML would.
		if err := setInnerHTML(div, textContent(noscript)); err != nil {
			slog.Debug("cannot parse noscript content", "error", err)
			continue
		}
		if !r.isSingleImage(div) {
			continue
		}

		// If noscript has previous sibling and it only contains image,
		// replace it with noscript content. However we also keep old
		// attributes that might contains image.
		var prevElement = previousElementSibling(noscript)
		if prevElement != nil && r.isSingleImage(prevElement) {
			var prevImg = prevElement
			if tagName(prevImg) != "IMG" {
				prevImg = elementsByTagName(prevElement, "img")[0]
			}

			var newImg = elementsByTagName(div, "img")[0]
			for i := 0; i < len(prevImg.Attr); i++ {
				var attr = prevImg.Attr[i]
				if attr.Val == "" {
					continue
				}

				if attr.Key == "src" || attr.Key == "srcset" || imgExtensions.MatchString(attr.Val) {
					if getAttribute(newImg, attr.Key) == attr.Val {
						continue
					}

					var attrName = attr.Key
					if hasAttribute(newImg, attrName) {
						attrName = "data-old-" + attrName
					}
					setAttribute(newImg, attrName, attr.Val)
				}
			}

			replaceChild(noscript.Parent, firstElementChild(div), prevElement)
		}
	}
}

// Removes script tags from the document.
func (r *engine) removeScripts(doc *html.Node) {
	r.removeNodes(r.getAllNodesWithTag(doc, "script", "noscript"), nil)
}

// Check if this node has only whitespace and a single element with given tag
// Returns false if the DIV node contains non-empty text nodes
// or if it contains no element with given tag or more than 1 element.
func (r *engine) hasSingleTagInsideElement(element *html.Node, tag string) bool {
	// There should be exactly 1 element child with given tag
	if len(elementChildren(element)) != 1 || tagName(elementChildren(element)[0]) != tag {
		return false
	}

	// And there should be no text nodes with real content
	return !r.someNode(childNodes(element), func(n *html.Node) bool {
		return n.Type == html.TextNode &&
			hasContent.MatchString(textContent(n))
	})
}

func (r *engine) isElementWithoutContent(n *html.Node) bool {
	if n.Type != html.ElementNode || hasNonWhitespaceText(n) {
		return false
	}

	// Preserve Readability's slightly unusual comparison of direct element
	// children with all descendant BR/HR elements, but avoid constructing three
	// node lists (and repeatedly materializing the subtree's text) to do it.
	directElements, breaks := 0, 0
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode {
			directElements++
		}
	}
	walkNodes(n, func(node *html.Node) bool {
		if node != n && node.Type == html.ElementNode && (node.Data == "br" || node.Data == "hr") {
			breaks++
		}
		return false
	})
	return directElements == 0 || directElements == breaks
}

// hasNonWhitespaceText answers the common emptiness question without building
// the complete textContent string. On large nested documents that string was
// rebuilt for every DIV/SECTION/heading ancestor, causing quadratic allocation.
func hasNonWhitespaceText(n *html.Node) bool {
	var visit func(*html.Node) bool
	visit = func(node *html.Node) bool {
		if node.Type == html.TextNode && strings.TrimSpace(node.Data) != "" {
			return true
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if visit(child) {
				return true
			}
		}
		return false
	}
	return visit(n)
}

// Determine whether element has any children block level elements.
func (r *engine) hasChildBlockElement(element *html.Node) bool {
	return r.someNode(childNodes(element), func(n *html.Node) bool {
		return slices.Contains(divToPElemns, tagName(n)) ||
			r.hasChildBlockElement(n)
	})
}

// Determine if a node qualifies as phrasing content.
// see: https://developer.mozilla.org/en-US/docs/Web/Guide/HTML/Content_categories#Phrasing_content
func (r *engine) isPhrasingContent(n *html.Node) bool {
	return n.Type == html.TextNode || slices.Contains(phrasingElems, tagName(n)) ||
		((tagName(n) == "A" || tagName(n) == "DEL" || tagName(n) == "INS") &&
			r.everyNode(childNodes(n), r.isPhrasingContent))
}

func (r *engine) isWhitespace(n *html.Node) bool {
	return (n.Type == html.TextNode && len(strings.TrimSpace(n.Data)) == 0) ||
		(n.Type == html.ElementNode && tagName(n) == "BR")
}

// normalizeWhitespaceRuns is equivalent to normalize.ReplaceAllString(s, " ").
// Keeping this hot path out of regexp avoids the regexp machine and its output
// allocation when (as is usual) the text contains no repeated whitespace.
func normalizeWhitespaceRuns(s string) string {
	start := -1
	for i := 0; i < len(s); i++ {
		if isASCIIWhitespace(s[i]) && i+1 < len(s) && isASCIIWhitespace(s[i+1]) {
			start = i
			break
		}
	}
	if start < 0 {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:start])
	for i := start; i < len(s); {
		if !isASCIIWhitespace(s[i]) {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i + 1
		for j < len(s) && isASCIIWhitespace(s[j]) {
			j++
		}
		if j-i >= 2 {
			b.WriteByte(' ')
		} else {
			b.WriteByte(s[i])
		}
		i = j
	}
	return b.String()
}

func isASCIIWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

// Get the inner text of a node - cross browser compatibly.
// This also strips out any excess whitespace to be found ('normalizeSpaces', defaults to true).
func (r *engine) getInnerText(e *html.Node, normalizeSpaces bool) string {
	text := strings.TrimSpace(textContent(e))
	if normalizeSpaces {
		return normalizeWhitespaceRuns(text)
	}
	return text
}

// articleCommaCount matches the comma variants in the Readability regexp.
func articleCommaCount(s string) int {
	count := strings.Count(s, ",")
	for _, ch := range s {
		switch ch {
		case '\u060c', '\ufe50', '\ufe10', '\ufe11', '\u2e41', '\u2e34', '\u2e32', '\uff0c':
			count++
		}
	}
	return count
}

// Remove the style attribute on every e and under.
// TODO: Test if getElementsByTagName(*) is faster.
func (r *engine) cleanStyles(e *html.Node) {
	if e == nil || strings.ToLower(tagName(e)) == "svg" {
		return
	}

	// Remove `style` and deprecated presentational attributes
	for i := 0; i < len(presentationalAttribute); i++ {
		removeAttribute(e, presentationalAttribute[i])
	}

	if slices.Contains(deprecatedSizeAttributeElems, tagName(e)) {
		removeAttribute(e, "width")
		removeAttribute(e, "height")
	}

	var cur = firstElementChild(e)
	for cur != nil {
		r.cleanStyles(cur)
		cur = nextElementSibling(cur)
	}
}

// Get the density of links as a percentage of the content
// This is the amount of text that is inside a link divided by the total text in the node.
func (r *engine) getLinkDensity(element *html.Node) float64 {
	textLength := normalizedTextRuneLen(element)
	if textLength == 0 {
		return 0
	}

	var linkLength float64
	for _, linkNode := range elementsByTagName(element, "a") {
		href := getAttribute(linkNode, "href")
		coefficient := 1.0
		if href != "" && hashUrl.MatchString(href) {
			coefficient = 0.3
		}
		linkLength += float64(normalizedTextRuneLen(linkNode)) * coefficient
	}

	return linkLength / float64(textLength)
}

// normalizedTextRuneLen computes the length used by getLinkDensity without
// materializing textContent (which can otherwise make nested candidates use
// quadratic amounts of temporary memory).
func normalizedTextRuneLen(element *html.Node) int {
	length, pending, asciiRun := 0, 0, 0
	started := false
	walkNodes(element, func(n *html.Node) bool {
		if n.Type != html.TextNode {
			return false
		}
		s := n.Data
		for i := 0; i < len(s); {
			// Article text is overwhelmingly ASCII. Avoid range's UTF-8 decoder
			// and unicode.IsSpace's table lookup on that common path.
			c := rune(s[i])
			size := 1
			if s[i] >= utf8.RuneSelf {
				c, size = utf8.DecodeRuneInString(s[i:])
			}
			i += size
			space := c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
			if space {
				if asciiRun == 0 {
					pending++
				}
				asciiRun++
				continue
			}
			if c >= utf8.RuneSelf && unicode.IsSpace(c) {
				pending++
				asciiRun = 0
				continue
			}
			if started {
				length += pending
			}
			pending, asciiRun = 0, 0
			length++
			started = true
		}
		return false
	})
	return length
}

// Get an elements class/id weight. Uses regular expressions to tell if this
// element looks good or bad.
func (r *engine) getClassWeight(e *html.Node) float64 {
	if !r.flagIsActive(flagWeightClasses) {
		return 0
	}

	weight := 0
	// Attribute lookup is linear. Keep the values instead of looking each one
	// up for the empty check and for both regular expressions.
	class, id := className(e), nodeID(e)
	if class != "" {
		if negative.MatchString(class) {
			weight -= 25
		}
		if positive.MatchString(class) {
			weight += 25
		}
	}
	if id != "" {
		if negative.MatchString(id) {
			weight -= 25
		}
		if positive.MatchString(id) {
			weight += 25
		}
	}

	return float64(weight)
}

// Clean a node of all elements of type "tag".
// (Unless it's a youtube/vimeo video. People love movies.)
func (r *engine) clean(e *html.Node, tag string) {

	var isEmbed = slices.Contains([]string{"object", "embed", "iframe"}, tag)

	r.removeNodes(r.getAllNodesWithTag(e, tag), func(element *html.Node) bool {
		// Allow youtube and vimeo videos through as people usually want to see those.
		if isEmbed {
			// First, check the elements attributes to see if any of them contain youtube or vimeo
			for i := 0; i < len(element.Attr); i++ {
				if r.options.allowedVideoRegex.MatchString(element.Attr[i].Val) {
					return false
				}
			}

			// For embed with <object> tag, check inner HTML as well.
			if tagName(element) == "object" && r.options.allowedVideoRegex.MatchString(innerHTML(element)) {
				return false
			}
		}
		return true
	})
}

// Check if a given node has one of its ancestor tag name matching the
// provided one.
func (r *engine) hasAncestorTag(n *html.Node, desiredTag string, maxDepth int, filterFn func(*html.Node) bool) bool {
	desiredTag = strings.ToUpper(desiredTag)
	var depth = 0
	for n.Parent != nil {
		if maxDepth > 0 && depth > maxDepth {
			return false
		}
		if tagName(n.Parent) == desiredTag && (filterFn == nil || filterFn(n.Parent)) {
			return true
		}
		n = n.Parent
		depth++
	}
	return false
}

// Return an object indicating how many rows and columns this table has.
func (r *engine) getRowAndColumnCount(table *html.Node) (int, int) {
	var rows = 0
	var columns = 0
	var trs = elementsByTagName(table, "tr")
	for i := 0; i < len(trs); i++ {
		var rowspan = getAttribute(trs[i], "rowspan")
		var rs int
		if rowspan != "" {
			num, err := strconv.Atoi(rowspan)
			if err != nil {
				slog.Error(err.Error())
			}
			rs = num
		}
		if rs != 0 {
			rows += rs
		} else {
			rows += 1
		}

		// Now look for column-related info
		var columnsInThisRow = 0
		var cells = elementsByTagName(trs[i], "td")
		for j := 0; j < len(cells); j++ {
			var colspan = getAttribute(cells[j], "colspan")
			var cs int
			if colspan != "" {
				num, err := strconv.Atoi(colspan)
				if err != nil {
					slog.Error(err.Error())
				}
				cs = num
			}
			if cs != 0 {
				columnsInThisRow += cs
			} else {
				columnsInThisRow += 1
			}
		}
		columns = int(math.Max(float64(columns), float64(columnsInThisRow)))
	}
	return rows, columns
}

// Look for 'data' (as opposed to 'layout') tables, for which we use
// similar checks as
// https://searchfox.org/mozilla-central/rev/f82d5c549f046cb64ce5602bfd894b7ae807c8f8/accessible/generic/TableAccessible.cpp#19
func (r *engine) markDataTables(root *html.Node) {

	var tables = elementsByTagName(root, "table")
	for i := 0; i < len(tables); i++ {
		var table = tables[i]
		var role = getAttribute(table, "role")
		if role == "presentation" {
			r.data(table).isDataTable = false
			continue
		}

		var datatable = getAttribute(table, "datatable")
		if datatable == "0" {
			r.data(table).isDataTable = false
			continue
		}

		var summary = getAttribute(table, "summary")
		if summary != "" {
			r.data(table).isDataTable = true
			continue
		}

		if captions := elementsByTagName(table, "caption"); len(captions) > 0 && captions[0] != nil && len(childNodes(captions[0])) > 0 {
			r.data(table).isDataTable = true
		}

		// If the table has a descendant with any of these tags, consider a data table:
		var dataTableDescendants = []string{"col", "colgroup", "tfoot", "thead", "th"}
		var descendantExists = func(tag string) bool {
			elements := elementsByTagName(table, tag)
			return len(elements) != 0 && elements[0] != nil
		}

		if slices.ContainsFunc(dataTableDescendants, descendantExists) {
			slog.Debug("Data table because found data-y descendant")
			r.data(table).isDataTable = true
			continue
		}

		// Nested tables indicate a layout table:
		if tables := elementsByTagName(table, "table"); len(tables) > 0 && tables[0] != nil {
			r.data(table).isDataTable = false
		}

		var rows, columns = r.getRowAndColumnCount(table)
		if rows >= 10 || columns > 4 {
			r.data(table).isDataTable = true
			continue
		}
		// Now just go by size entirely:
		r.data(table).isDataTable = (rows*columns > 10)
	}
}

// convert images and figures that have properties like data-src into images that can be loaded without JS
func (r *engine) fixLazyImages(root *html.Node) {

	for _, elem := range r.getAllNodesWithTag(root, "img", "picture", "figure") {
		// In some sites (e.g. Kotaku), they put 1px square image as base64 data uri in the src attribute.
		// So, here we check if the data uri is too short, just might as well remove it.

		if nodeSrc(elem) != "" && b64DataUrl.MatchString(nodeSrc(elem)) {
			// Make sure it's not SVG, because SVG can have a meaningful image in under 133 bytes.
			var parts = b64DataUrl.FindAllStringSubmatch(nodeSrc(elem), -1)
			if parts[0][1] == "image/svg+xml" {
				continue
			}

			// Make sure this element has other attributes which contains image.
			// If it doesn't, then this src is important and shouldn't be removed.
			var srcCouldBeRemoved = false
			for i := 0; i < len(elem.Attr); i++ {
				var attr = elem.Attr[i]
				if attr.Key == "src" {
					continue
				}

				if imgExtensions.MatchString(attr.Val) {
					srcCouldBeRemoved = true
					break
				}
			}

			// Here we assume if image is less than 100 bytes (or 133B after encoded to base64)
			// it will be too small, therefore it might be placeholder image.
			if srcCouldBeRemoved {
				var b64starts = base64Starts.FindStringIndex(nodeSrc(elem))[0] + 7
				var b64length = len([]rune(nodeSrc(elem))) - b64starts
				if b64length < 133 {
					removeAttribute(elem, "src")
				}
			}
		}

		// also check for "null" to work around https://github.com/jsdom/jsdom/issues/2580
		if (nodeSrc(elem) != "" || (nodeSrcset(elem) != "" && nodeSrcset(elem) != "null")) && !strings.Contains(strings.ToLower(className(elem)), "lazy") {
			continue
		}

		for j := 0; j < len(elem.Attr); j++ {
			attr := elem.Attr[j]
			if attr.Key == "src" || attr.Key == "srcset" || attr.Key == "alt" {
				continue
			}
			var copyTo string
			if imgExtensionsWithSpacesAndNum.MatchString(attr.Val) {
				copyTo = "srcset"
			} else if imgExtensionsAmongText.MatchString(attr.Val) {
				copyTo = "src"
			}

			if copyTo != "" {
				//if this is an img or picture, set the attribute directly
				if tagName(elem) == "IMG" || tagName(elem) == "PICTURE" {
					setAttribute(elem, copyTo, attr.Val)
				} else if tagName(elem) == "FIGURE" && len(r.getAllNodesWithTag(elem, "img", "picture")) == 0 {
					//if the item is a <figure> that does not contain an image or picture, create one and place it inside the figure
					//see the nytimes-3 testcase for an example
					var img = newElement("img")
					setAttribute(img, copyTo, attr.Val)
					appendChild(elem, img)
				}
			}
		}
	}
}

// Clean an element of all tags of type "tag" if they look fishy.
// "Fishy" is an algorithm based on content length, classnames, link density, number of images & embeds, etc.
func (r *engine) cleanConditionally(e *html.Node, tag string) {
	if !r.flagIsActive(flagCleanConditionally) {
		return
	}

	// Gather counts for other typical elements embedded within.
	// Traverse backwards so we can remove nodes at the same time
	// without effecting the traversal.
	//
	// TODO: Consider taking into account original contentScore here.

	r.removeNodes(r.getAllNodesWithTag(e, tag), func(n *html.Node) bool {
		// Candidate subtrees overlap heavily. Build this normalized text once and
		// reuse it for all metrics below instead of rebuilding it repeatedly.
		nodeText := r.getInnerText(n, true)

		// First check if this node IS data table, in which case don't remove it.
		var isDataTable = func(t *html.Node) bool {
			return r.nodeState[t] != nil && r.data(t).isDataTable
		}

		var isList = (tag == "ul" || tag == "ol")
		if !isList {
			var listLength = 0
			var listNodes = r.getAllNodesWithTag(n, "ul", "ol")
			for _, list := range listNodes {
				listLength += len(r.getInnerText(list, true))
			}
			isList = float64(listLength)/float64(len(nodeText)) > 0.9
		}

		if tag == "table" && isDataTable(n) {
			return false
		}

		// Next check if we're inside a data table, in which case don't remove it as well.
		if r.hasAncestorTag(n, "table", -1, isDataTable) {
			return false
		}

		if r.hasAncestorTag(n, "code", 3, nil) {
			return false
		}

		var weight = r.getClassWeight(n)

		slog.Debug("Cleaning Conditionally", "node", n)

		var contentScore = 0.0

		if weight+contentScore < 0 {
			return true
		}

		if strings.Count(nodeText, ",") < 10 {
			// If there are not very many commas, and the number of
			// non-paragraph elements is more than paragraphs or other
			// ominous signs, remove the element.
			var p = countElementsByTagName(n, "p")
			var img = countElementsByTagName(n, "img")
			var li = countElementsByTagName(n, "li") - 100
			var input = countElementsByTagName(n, "input")
			var headingLength int
			for _, heading := range r.getAllNodesWithTag(n, "h1", "h2", "h3", "h4", "h5", "h6") {
				headingLength += len(r.getInnerText(heading, true))
			}
			var headingDensity float64
			if len(nodeText) != 0 {
				headingDensity = float64(headingLength) / float64(len(nodeText))
			}

			var embedCount = 0
			var embeds = r.getAllNodesWithTag(n, "object", "embed", "iframe")

			for i := 0; i < len(embeds); i++ {
				// If this embed has attribute that matches video regex, don't delete it.
				for j := 0; j < len(embeds[i].Attr); j++ {
					if r.options.allowedVideoRegex != nil && r.options.allowedVideoRegex.MatchString(embeds[i].Attr[j].Val) {
						return false
					}
				}

				// For embed with <object> tag, check inner HTML as well.
				if tagName(embeds[i]) == "object" && r.options.allowedVideoRegex != nil && r.options.allowedVideoRegex.MatchString(innerHTML(embeds[i])) {
					return false
				}

				embedCount++
			}

			var linkDensity = r.getLinkDensity(n)
			var contentLength = utf8.RuneCountInString(nodeText)

			var haveToRemove = (img > 1 && float64(p)/float64(img) < 0.5 && !r.hasAncestorTag(n, "figure", 3, nil)) ||
				(!isList && li > p) ||
				(input > int(math.Floor(float64(p)/3.0))) ||
				(!isList && headingDensity < 0.9 && contentLength < 25 && (img == 0 || img > 2) && !r.hasAncestorTag(n, "figure", 3, nil)) ||
				(!isList && weight < 25 && linkDensity > 0.2+r.options.linkDensityModifier) ||
				(weight >= 25 && linkDensity > 0.5+r.options.linkDensityModifier) ||
				((embedCount == 1 && contentLength < 75) || embedCount > 1)

			// Allow simple lists of images to remain in pages
			if isList && haveToRemove {
				for _, child := range elementChildren(n) {
					// Don't filter in lists with li's that contain more than one child
					if len(elementChildren(child)) > 1 {
						return haveToRemove
					}
				}
				var liCount = countElementsByTagName(n, "li")
				// Only allow the list to remain if every li contains an image
				if img == liCount {
					return false
				}
			}
			return haveToRemove
		}
		return false
	})
}

// Clean out elements that match the specified conditions
func classAndID(n *html.Node) string {
	class, id := className(n), nodeID(n)
	if class == "" && id == "" {
		return " "
	}
	return class + " " + id
}

func (r *engine) cleanMatchedNodes(e *html.Node, filter func(*html.Node, string) bool) {
	var endOfSearchMarkerNode = r.getNextNode(e, true)
	var next = r.getNextNode(e, false)
	for next != nil && next != endOfSearchMarkerNode {
		if filter(next, classAndID(next)) {
			next = r.removeAndGetNext(next)
		} else {
			next = r.getNextNode(next, false)
		}
	}
}

// Clean out spurious headers from an Element.
func (r *engine) cleanHeaders(n *html.Node) {
	var headingNodes = r.getAllNodesWithTag(n, "h1", "h2")
	r.removeNodes(headingNodes, func(nn *html.Node) bool {
		var shouldRemove = r.getClassWeight(nn) < 0
		if shouldRemove {
			slog.Debug("Removing header with low class weight", "node", nn)
		}
		return shouldRemove
	})
}

// Check if this node is an H1 or H2 element whose content is mostly
// the same as the article title.
func (r *engine) headerDuplicatesTitle(n *html.Node) bool {
	if tagName(n) != "H1" && tagName(n) != "H2" {
		return false
	}
	var heading = r.getInnerText(n, false)
	slog.Debug("Evaluating similarity of header", "heading", heading, "articleTitle", r.articleTitle)
	return r.textSimilarity(r.articleTitle, heading) > 0.75
}

func (r *engine) flagIsActive(flag int) bool {
	return r.flags&flag > 0
}

func (r *engine) removeFlag(flag int) {
	r.flags = r.flags & ^flag
}

func isProbablyVisible(n *html.Node) bool {
	// Have to null-check node.style and node.className.indexOf to deal with SVG and MathML nodes.
	return getStyle(n, "display") != "none" && getStyle(n, "visibility") != "hidden" &&
		!hasAttribute(n, "hidden") &&
		(!hasAttribute(n, "aria-hidden") || getAttribute(n, "aria-hidden") != "true" || (className(n) != "" && strings.Contains(className(n), "fallback-image")))
}

// prepareDocumentTree applies the destructive normalization shared by the
// initial extraction and each retry. Metadata must be read before calling it.
func (r *engine) prepareDocumentTree() {
	r.unwrapNoscriptImages(r.doc)
	r.removeScripts(r.doc)
	r.prepDocument()
}

// resetDocumentForRetry restores the parser-produced tree without tokenizing
// or applying source-level HTML rewrites again.
func (r *engine) resetDocumentForRetry() {
	doc := cloneTree(r.original)
	r.doc = doc
	r.body = findElement(doc, "body")
	r.documentElement = findElement(doc, "html")
	if r.documentElement == nil {
		r.documentElement = r.body
	}
	clear(r.nodeState)
	prepareMathJax(r.doc)
	r.prepareDocumentTree()
}

// Runs readability.
// Workflow:
//  1. Prep the document by removing script tags, css, etc.
//  2. Build readability's DOM tree.
//  3. Grab the article content from the current dom tree.
//  4. Replace the current DOM tree with the new one.
//  5. Read peacefully.
func (r *engine) Parse() (*engineResult, error) {
	// Normalize only the mutable working tree. In particular, ParseNode must
	// leave the caller's parsed document untouched.
	prepareMathJax(r.doc)

	// Avoid parsing too large documents, as per configuration option
	if r.options.maxElemsToParse > 0 {
		var numTags = countElementsByTagName(r.doc, "*")
		if numTags > r.options.maxElemsToParse {
			return nil, fmt.Errorf("aborting parsing document: elements_found=%d", numTags)
		}
	}

	// Unwrap images before reading metadata, matching the normal preparation
	// order used by Readability.
	r.unwrapNoscriptImages(r.doc)

	// Extract JSON-LD metadata before removing scripts
	var jsonLd *metadata
	if !r.options.disableJSONLD {
		jsonLd = r.getJSONLD(r.doc)
	}

	r.removeScripts(r.doc)
	r.prepDocument()

	var metadata = r.getArticleMetadata(jsonLd)
	r.articleTitle = metadata.title

	var articleContent = r.grabArticle(nil)
	if articleContent == nil {
		return nil, fmt.Errorf("cannot grab article")
	}

	if r.options.debug {
		slog.Debug("grabbed", "articleContent.innerHTML", innerHTML(articleContent))
	}

	r.postProcessContent(articleContent)

	// If we haven't found an excerpt in the article's metadata, use the article's
	// first paragraph as the excerpt. This is used for displaying a preview of
	// the article's content.
	if metadata.excerpt == "" {
		var paragraphs = elementsByTagName(articleContent, "p")
		if len(paragraphs) > 0 {
			metadata.excerpt = strings.TrimSpace(textContent(paragraphs[0]))
		}
	}

	htmlContent := r.options.serializer(articleContent)

	var extractedText string
	if r.options.html2text != nil {
		extractedText = r.options.html2text(htmlContent)
	} else {
		extractedText = textContent(articleContent)
	}

	return &engineResult{
		Title:         r.articleTitle,
		Byline:        anyOf(metadata.byline, r.articleByline),
		Dir:           r.articleDir,
		Lang:          r.articleLang,
		HTMLContent:   htmlContent,
		TextContent:   extractedText,
		Length:        len([]rune(extractedText)),
		Excerpt:       metadata.excerpt,
		SiteName:      anyOf(metadata.siteName, r.articleSiteName),
		PublishedTime: metadata.publishedTime,
	}, nil
}

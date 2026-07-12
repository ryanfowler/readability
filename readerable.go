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
	"math"
	"strings"

	"golang.org/x/net/html"
)

func isNodeVisible(node *html.Node) bool {
	// Approximate the browser's style.display/style.visibility properties for
	// inline declarations. CSS property names and these keyword values are
	// ASCII case-insensitive, and the last declaration wins.
	var display, visibility string
	for _, declaration := range strings.Split(attr(node, "style"), ";") {
		name, value, ok := strings.Cut(declaration, ":")
		if !ok {
			continue
		}
		name = strings.ToLower(strings.TrimSpace(name))
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.TrimSpace(strings.TrimSuffix(value, "!important"))
		switch name {
		case "display":
			display = value
		case "visibility":
			visibility = value
		}
	}
	if display == "none" || visibility == "hidden" {
		return false
	}
	return attr(node, "hidden") == "" &&
		(attr(node, "aria-hidden") == "" || attr(node, "aria-hidden") != "true" ||
			(attr(node, "class") != "" && strings.Contains(attr(node, "class"), "fallback-image")))
}

// Decides whether or not the document is reader-able without parsing the whole thing.
// engineOptions:
//   - options.minContentLength (default 140), the minimum node content length used to decide if the document is readerable
//   - options.minScore (default 20), the minumum cumulated 'score' used to determine if the document is readerable
//   - options.visibilityChecker (default isNodeVisible), the function used to determine if a node is visible
func isProbablyReaderable(doc *html.Node, opts ...engineOption) bool {
	options := defaultOpts()
	for _, opt := range opts {
		opt(options)
	}

	score := 0.0
	candidate := func(n *html.Node) bool {
		if !options.visibilityChecker(n) {
			return false
		}

		matchString := attr(n, "class") + " " + attr(n, "id")
		if unlikelyCandidates.MatchString(matchString) &&
			!okMaybeItsACandidate.MatchString(matchString) {
			return false
		}
		if n.Data == "p" && hasAncestorTag(n, "li") {
			return false
		}

		textContentLength := len(strings.TrimSpace(textContent(n)))
		if textContentLength < options.minContentLength {
			return false
		}
		score += math.Sqrt(float64(textContentLength - options.minContentLength))
		return score > options.minScore
	}

	// Readability examines p/pre/article elements first, followed by each unique
	// div containing a direct br child. Walk the x/html links directly rather
	// than allocating selector result slices.
	if walkElements(doc, func(n *html.Node) bool {
		return (n.Data == "p" || n.Data == "pre" || n.Data == "article") && candidate(n)
	}) {
		return true
	}
	return walkElements(doc, func(n *html.Node) bool {
		if n.Data != "div" {
			return false
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && child.Data == "br" {
				return candidate(n)
			}
		}
		return false
	})
}

// walkElements visits descendant elements in document order and stops when fn
// returns true. It uses the linked representation provided by x/html.
func walkElements(root *html.Node, fn func(*html.Node) bool) bool {
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && fn(child) {
			return true
		}
		if walkElements(child, fn) {
			return true
		}
	}
	return false
}

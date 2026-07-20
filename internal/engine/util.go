package engine

import "golang.org/x/net/html"

// characterCount returns the number of UTF-16 code units in s. Readability's
// thresholds originate in JavaScript, where String.length has this semantic.
func characterCount(s string) int {
	count := 0
	for _, r := range s {
		count++
		if r > 0xffff {
			count++
		}
	}
	return count
}

func insert(newNode *html.Node, idx int, nodes []*html.Node) []*html.Node {
	nodes = append(nodes, nil)
	copy(nodes[idx+1:], nodes[idx:])
	nodes[idx] = newNode
	return nodes
}

func anyOf(strings ...string) string {
	for _, s := range strings {
		if s != "" {
			return s
		}
	}
	return ""
}

func hasAncestorTag(n *html.Node, tagName string) bool {
	for parent := n.Parent; parent != nil; parent = parent.Parent {
		if parent.Type == html.ElementNode && parent.Data == tagName {
			return true
		}
	}
	return false
}

func attr(n *html.Node, attrName string) string {
	for _, a := range n.Attr {
		if a.Key == attrName {
			return a.Val
		}
	}
	return ""
}

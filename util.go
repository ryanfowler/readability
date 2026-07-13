package readability

import (
	"slices"

	"golang.org/x/net/html"
)

func indexOf[T any](el *T, a []*T) int {
	return slices.IndexFunc(a, func(ell *T) bool {
		return ell == el
	})
}

func delete[T any](idx int, a []*T) []*T {
	copy(a[idx:], a[idx+1:])
	a[len(a)-1] = nil
	a = a[:len(a)-1]
	return a
}

func insert(newNode *html.Node, idx int, nodes []*html.Node) []*html.Node {
	nodes = append(nodes[:idx], append([]*html.Node{newNode}, nodes[idx:]...)...)
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

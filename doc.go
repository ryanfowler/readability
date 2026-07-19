// Package readability extracts the main article and its metadata from HTML.
//
// Use Parse for an HTML string. Use ParseNode for a tree that was parsed with
// golang.org/x/net/html. Use IsProbablyReaderable or
// IsProbablyReaderableNode when you only need a fast readerability check.
//
// Pass nil for an options pointer to use the defaults. To change selected
// options, call DefaultOptions or DefaultReaderableOptions first. Then change
// the returned value.
//
// Article.Content and Article.Node can contain unsafe HTML. The package does
// not sanitize them. Sanitize extracted HTML before you add it to a web page.
package readability

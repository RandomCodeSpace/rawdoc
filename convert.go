package main

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// convertToMarkdown is the main entry point: takes an *html.Node and returns markdown.
func convertToMarkdown(n *html.Node) string {
	var buf strings.Builder
	convertNode(&buf, n, 0, false)
	return cleanMarkdown(buf.String())
}

// convertNode recursively walks the node tree, writing markdown to buf.
func convertNode(buf *strings.Builder, n *html.Node, depth int, inBlockquote bool) {
	if n == nil {
		return
	}
	switch n.Type {
	case html.TextNode:
		text := collapseWhitespace(n.Data)
		if text != "" && text != " " || (text == " " && buf.Len() > 0) {
			buf.WriteString(text)
		}
	case html.ElementNode:
		convertElement(buf, n, depth, inBlockquote)
	case html.DocumentNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			convertNode(buf, c, depth, inBlockquote)
		}
	}
}

// convertChildren walks all children of n.
func convertChildren(buf *strings.Builder, n *html.Node, depth int, inBlockquote bool) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		convertNode(buf, c, depth, inBlockquote)
	}
}

// convertElement handles a single HTML element.
func convertElement(buf *strings.Builder, n *html.Node, depth int, inBlockquote bool) {
	tag := n.DataAtom.String()
	if tag == "" {
		tag = n.Data
	}
	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level := int(tag[1] - '0')
		prefix := strings.Repeat("#", level)
		buf.WriteString("\n\n")
		buf.WriteString(prefix)
		buf.WriteString(" ")
		var inner strings.Builder
		convertChildren(&inner, n, depth, inBlockquote)
		buf.WriteString(strings.TrimSpace(inner.String()))
		buf.WriteString("\n\n")

	case "p":
		buf.WriteString("\n\n")
		var inner strings.Builder
		convertChildren(&inner, n, depth, inBlockquote)
		text := strings.TrimSpace(inner.String())
		if inBlockquote {
			buf.WriteString("> ")
			buf.WriteString(text)
		} else {
			buf.WriteString(text)
		}
		buf.WriteString("\n\n")

	case "pre":
		code := findFirst(n, "code")
		if code != nil {
			lang := detectCodeLang(code)
			buf.WriteString("\n\n")
			buf.WriteString("```")
			buf.WriteString(lang)
			buf.WriteString("\n")
			text := textContent(code)
			buf.WriteString(text)
			if !strings.HasSuffix(text, "\n") {
				buf.WriteString("\n")
			}
			buf.WriteString("```")
			buf.WriteString("\n\n")
		} else {
			buf.WriteString("\n\n```\n")
			buf.WriteString(textContent(n))
			buf.WriteString("\n```\n\n")
		}

	case "code":
		// Check if parent is pre (handled there)
		if n.Parent != nil && n.Parent.DataAtom == atom.Pre {
			return
		}
		buf.WriteString("`")
		buf.WriteString(textContent(n))
		buf.WriteString("`")

	case "ul", "ol":
		buf.WriteString("\n")
		i := 0
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "li" {
				i++
				convertLI(buf, c, depth, tag == "ol", i)
			}
		}
		buf.WriteString("\n")

	case "li":
		convertLI(buf, n, depth, false, 1)

	case "a":
		href := getAttr(n, "href")
		var inner strings.Builder
		convertChildren(&inner, n, depth, inBlockquote)
		text := strings.TrimSpace(inner.String())
		if href != "" {
			buf.WriteString("[")
			buf.WriteString(text)
			buf.WriteString("](")
			buf.WriteString(href)
			buf.WriteString(")")
		} else {
			buf.WriteString(text)
		}

	case "img":
		alt := getAttr(n, "alt")
		src := getAttr(n, "src")
		buf.WriteString(fmt.Sprintf("![%s](%s)", alt, src))

	case "strong", "b":
		var inner strings.Builder
		convertChildren(&inner, n, depth, inBlockquote)
		buf.WriteString("**")
		buf.WriteString(inner.String())
		buf.WriteString("**")

	case "em", "i":
		var inner strings.Builder
		convertChildren(&inner, n, depth, inBlockquote)
		buf.WriteString("*")
		buf.WriteString(inner.String())
		buf.WriteString("*")

	case "table":
		convertTable(buf, n)

	case "blockquote":
		buf.WriteString("\n\n")
		var inner strings.Builder
		convertChildren(&inner, n, depth, true)
		innerText := strings.TrimSpace(inner.String())
		lines := strings.Split(innerText, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "> ") || line == ">" {
				buf.WriteString(line)
			} else {
				buf.WriteString("> ")
				buf.WriteString(line)
			}
			buf.WriteString("\n")
		}
		buf.WriteString("\n")

	case "hr":
		buf.WriteString("\n\n---\n\n")

	case "br":
		buf.WriteString("\n")

	case "dl":
		convertDefList(buf, n)

	case "dt":
		var inner strings.Builder
		convertChildren(&inner, n, depth, inBlockquote)
		buf.WriteString("\n**")
		buf.WriteString(strings.TrimSpace(inner.String()))
		buf.WriteString("**\n")

	case "dd":
		var inner strings.Builder
		convertChildren(&inner, n, depth, inBlockquote)
		buf.WriteString(strings.TrimSpace(inner.String()))
		buf.WriteString("\n")

	case "div", "section", "article", "main", "span", "figure", "figcaption":
		convertChildren(buf, n, depth, inBlockquote)

	case "thead", "tbody", "tfoot":
		convertChildren(buf, n, depth, inBlockquote)

	case "tr", "th", "td":
		convertChildren(buf, n, depth, inBlockquote)

	case "html", "head", "body":
		convertChildren(buf, n, depth, inBlockquote)

	case "script", "style", "noscript":
		// skip

	default:
		convertChildren(buf, n, depth, inBlockquote)
	}
}

// convertLI handles a list item (li), supporting nested lists.
func convertLI(buf *strings.Builder, li *html.Node, depth int, ordered bool, index int) {
	indent := strings.Repeat("  ", depth)

	// Collect text content (excluding nested lists)
	var textBuf strings.Builder
	for c := li.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			text := collapseWhitespace(c.Data)
			if text != "" {
				textBuf.WriteString(text)
			}
		} else if c.Type == html.ElementNode {
			switch c.Data {
			case "ul", "ol":
				// handled below
			default:
				var inner strings.Builder
				convertElement(&inner, c, depth+1, false)
				textBuf.WriteString(inner.String())
			}
		}
	}

	text := strings.TrimSpace(textBuf.String())

	if ordered {
		buf.WriteString(fmt.Sprintf("%s%d. %s\n", indent, index, text))
	} else {
		buf.WriteString(fmt.Sprintf("%s- %s\n", indent, text))
	}

	// Now handle nested lists
	for c := li.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		tag := c.Data
		if tag == "ul" || tag == "ol" {
			j := 0
			for nested := c.FirstChild; nested != nil; nested = nested.NextSibling {
				if nested.Type == html.ElementNode && nested.Data == "li" {
					j++
					convertLI(buf, nested, depth+1, tag == "ol", j)
				}
			}
		}
	}
}

// convertTable renders a table node as a pipe table.
func convertTable(buf *strings.Builder, table *html.Node) {
	buf.WriteString("\n\n")

	var headers []string
	var rows [][]string

	// thead rows
	thead := findFirst(table, "thead")
	if thead != nil {
		trNodes := findAll(thead, "tr")
		for i, tr := range trNodes {
			var cols []string
			for c := tr.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "th" || c.Data == "td") {
					cols = append(cols, strings.TrimSpace(textContent(c)))
				}
			}
			if i == 0 {
				headers = cols
			}
		}
	}

	// If no thead, use first tr's th/td as headers
	if len(headers) == 0 {
		firstTR := findFirst(table, "tr")
		if firstTR != nil {
			for c := firstTR.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "th" || c.Data == "td") {
					headers = append(headers, strings.TrimSpace(textContent(c)))
				}
			}
		}
	}

	// tbody rows
	tbody := findFirst(table, "tbody")
	if tbody != nil {
		trNodes := findAll(tbody, "tr")
		for _, tr := range trNodes {
			var cols []string
			for c := tr.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
					cols = append(cols, strings.TrimSpace(textContent(c)))
				}
			}
			rows = append(rows, cols)
		}
	}

	// If no tbody, use all rows except first as data rows
	if len(rows) == 0 {
		first := true
		allTRs := findAll(table, "tr")
		for _, tr := range allTRs {
			// Skip trs inside thead
			if tr.Parent != nil && tr.Parent.Data == "thead" {
				continue
			}
			if first {
				first = false
				continue
			}
			var cols []string
			for c := tr.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
					cols = append(cols, strings.TrimSpace(textContent(c)))
				}
			}
			rows = append(rows, cols)
		}
	}

	if len(headers) == 0 {
		return
	}

	// Write header row
	buf.WriteString("| ")
	buf.WriteString(strings.Join(headers, " | "))
	buf.WriteString(" |\n")

	// Write separator row
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = "---"
	}
	buf.WriteString("| ")
	buf.WriteString(strings.Join(seps, " | "))
	buf.WriteString(" |\n")

	// Write data rows
	for _, row := range rows {
		buf.WriteString("| ")
		buf.WriteString(strings.Join(row, " | "))
		buf.WriteString(" |\n")
	}

	buf.WriteString("\n")
}

// convertDefList renders a dl element.
func convertDefList(buf *strings.Builder, dl *html.Node) {
	buf.WriteString("\n\n")
	for c := dl.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		switch c.Data {
		case "dt":
			buf.WriteString("\n**")
			buf.WriteString(strings.TrimSpace(textContent(c)))
			buf.WriteString("**\n")
		case "dd":
			buf.WriteString(strings.TrimSpace(textContent(c)))
			buf.WriteString("\n")
		}
	}
	buf.WriteString("\n")
}

// detectCodeLang extracts the language from a class like "language-go".
func detectCodeLang(n *html.Node) string {
	class := getAttr(n, "class")
	if class == "" {
		return ""
	}
	for _, cls := range strings.Fields(class) {
		if strings.HasPrefix(cls, "language-") {
			return strings.TrimPrefix(cls, "language-")
		}
	}
	return ""
}

// --- Helpers for *html.Node tree walking ---

// getAttr returns the value of the named attribute, or empty string.
func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// hasAttr checks if the node has an attribute with the given key and value.
func hasAttr(n *html.Node, key, val string) bool {
	return getAttr(n, key) == val
}

// textContent collects all text nodes recursively.
func textContent(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	var buf strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		buf.WriteString(textContent(c))
	}
	return buf.String()
}

// findFirst finds the first descendant element with the given tag name.
func findFirst(n *html.Node, tag string) *html.Node {
	if n == nil {
		return nil
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			return c
		}
		if found := findFirst(c, tag); found != nil {
			return found
		}
	}
	return nil
}

// findAll finds all descendant elements with the given tag name.
func findAll(n *html.Node, tag string) []*html.Node {
	var result []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == tag {
				result = append(result, c)
			}
			walk(c)
		}
	}
	walk(n)
	return result
}

var wsRegexp = regexp.MustCompile(`[ \t]+`)

// collapseWhitespace collapses sequences of spaces/tabs into a single space.
func collapseWhitespace(s string) string {
	return wsRegexp.ReplaceAllString(s, " ")
}

var tripleNewlineRegexp = regexp.MustCompile(`\n{3,}`)

// cleanMarkdown collapses 3+ newlines to 2 and trims leading/trailing whitespace.
func cleanMarkdown(s string) string {
	s = tripleNewlineRegexp.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// Patterns for output optimization
var (
	// [¶](#section-anchor) — pilcrow anchors used by Go docs, MDN, etc.
	pilcrowPattern = regexp.MustCompile(`\s*\[¶\]\([^)]*\)`)
	// [text](#fragment) — fragment-only self-links, keep just the text
	fragmentLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\(#[^)]*\)`)
	// Lines that are only whitespace
	whitespaceLinePattern = regexp.MustCompile(`(?m)^[ \t]+$`)
	// 3+ consecutive blank lines → 2
	excessiveBlanks = regexp.MustCompile(`\n{3,}`)
	// Trailing whitespace on lines
	trailingWhitespace = regexp.MustCompile(`(?m)[ \t]+$`)
)

// optimizeMarkdown post-processes markdown to minimize token usage for AI agents.
func optimizeMarkdown(s string) string {
	// Strip pilcrow anchors: [¶](#...)
	s = pilcrowPattern.ReplaceAllString(s, "")

	// Convert fragment-only links to plain text: [text](#frag) → text
	s = fragmentLinkPattern.ReplaceAllString(s, "$1")

	// Strip trailing whitespace from lines
	s = trailingWhitespace.ReplaceAllString(s, "")

	// Collapse whitespace-only lines to empty lines
	s = whitespaceLinePattern.ReplaceAllString(s, "")

	// Collapse 3+ blank lines to 2
	s = excessiveBlanks.ReplaceAllString(s, "\n\n")

	return strings.TrimSpace(s)
}

// estimateTokens gives a rough token count using the ~4 chars/token heuristic
// typical for GPT/Claude tokenizers on English text.
func estimateTokens(byteCount int) int {
	if byteCount == 0 {
		return 0
	}
	return (byteCount + 3) / 4
}
